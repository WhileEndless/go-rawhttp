package http2

import (
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/net/http2"
)

// frameEventKind classifies a dispatched stream event.
type frameEventKind int

const (
	fkHeaders frameEventKind = iota // decoded HEADERS for the stream
	fkData                          // a copy of a DATA payload
	fkRST                           // RST_STREAM for the stream
	fkConnErr                       // connection-level error targeted at the stream (e.g. GOAWAY)
)

// frameEvent is a safe, owned snapshot of a frame routed from the connection's
// single read loop to a waiting request goroutine. DATA payloads and decoded
// headers are copied/owned here because the underlying http2.Framer reuses its
// buffers after each ReadFrame call.
type frameEvent struct {
	kind      frameEventKind
	headers   map[string]string
	data      []byte
	endStream bool
	errCode   http2.ErrCode
	err       error
}

// touch records connection activity (used for idle/keepalive tracking).
func (c *Connection) touch() {
	c.mu.Lock()
	c.LastActivity = time.Now()
	c.mu.Unlock()
}

// routeEvent delivers an event to the stream's inbox without blocking the read
// loop indefinitely: it gives up if the stream finished (done) or the connection
// died (closedCh). Events for unknown/finished streams are dropped.
func (c *Connection) routeEvent(streamID uint32, ev frameEvent) {
	c.mu.RLock()
	s := c.Streams[streamID]
	c.mu.RUnlock()
	if s == nil {
		return
	}
	select {
	case s.inbox <- ev:
	case <-s.done:
	case <-c.closedCh:
	}
}

// removeConnection evicts a connection from the pool. It only removes the exact
// connection still mapped under its pool key (identity check) so a fresh
// replacement that reused the key during a race is never evicted. It does NOT
// close the socket; the read loop's fail() owns socket teardown.
func (t *Transport) removeConnection(conn *Connection) {
	if conn == nil {
		return
	}
	t.mu.Lock()
	if conn.PoolKey != "" {
		if cur, ok := t.connections[conn.PoolKey]; ok && cur == conn {
			delete(t.connections, conn.PoolKey)
		}
	}
	t.mu.Unlock()
}

// runReadLoop is the single per-connection reader. It owns all reads from the
// Framer, decodes/copies frame contents, and dispatches them to the right stream.
// When it terminates (connection error or GOAWAY drain), it evicts the connection
// from the pool and fails it so every waiting request observes the error.
func (t *Transport) runReadLoop(conn *Connection) {
	// Note: t.wg.Add(1) is performed by the caller (Connect) before starting this
	// goroutine, so Transport.Close() can never observe an Add racing with Wait.
	defer t.wg.Done()

	var termErr error
	for {
		// No read deadline here: an idle pooled connection must be able to block
		// waiting for server frames (GOAWAY/PING) without being torn down. Per-request
		// read timeouts are enforced by the request goroutine, which cancels its stream
		// (or fails a dead reused connection) on timeout.
		conn.Conn.SetReadDeadline(time.Time{})

		f, err := conn.Framer.ReadFrame()
		if err != nil {
			termErr = classifyReadErr(err)
			break
		}
		conn.touch()
		if term := t.dispatchFrame(conn, f); term != nil {
			termErr = term
			break
		}
	}

	t.removeConnection(conn)
	conn.fail(termErr)
}

// dispatchFrame handles a single inbound frame. It returns a non-nil error only
// for connection-terminal conditions (the read loop then stops).
func (t *Transport) dispatchFrame(conn *Connection, raw http2.Frame) error {
	switch f := raw.(type) {
	case *http2.HeadersFrame:
		// HPACK decoding is stateful and must happen in stream order; the read loop
		// is the single decoder user, so this is safe.
		dec := &Converter{decoder: conn.Decoder}
		headers, err := dec.DecodeHeaders(f.HeaderBlockFragment())
		if err != nil {
			// A header-block decoding failure desynchronizes HPACK state for the
			// whole connection; tear it down.
			return wrapStaleHTTP2Error("decoding headers", err)
		}
		conn.routeEvent(f.StreamID, frameEvent{
			kind:      fkHeaders,
			headers:   headers,
			endStream: f.StreamEnded(),
		})

	case *http2.DataFrame:
		// Copy the payload: the Framer reuses its read buffer after this returns.
		payload := f.Data()
		data := make([]byte, len(payload))
		copy(data, payload)

		// Replenish flow-control windows so the peer can keep sending.
		if len(data) > 0 {
			conn.writeMu.Lock()
			werr := conn.Framer.WriteWindowUpdate(f.StreamID, uint32(len(data)))
			if werr == nil {
				werr = conn.Framer.WriteWindowUpdate(0, uint32(len(data)))
			}
			conn.writeMu.Unlock()
			if werr != nil {
				return wrapStaleHTTP2Error("window update", werr)
			}
		}

		conn.routeEvent(f.StreamID, frameEvent{
			kind:      fkData,
			data:      data,
			endStream: f.StreamEnded(),
		})

	case *http2.RSTStreamFrame:
		conn.routeEvent(f.StreamID, frameEvent{kind: fkRST, errCode: f.ErrCode})

	case *http2.SettingsFrame:
		if !f.IsAck() {
			conn.writeMu.Lock()
			err := conn.Framer.WriteSettingsAck()
			conn.writeMu.Unlock()
			if err != nil {
				return wrapStaleHTTP2Error("settings ack", err)
			}
		}

	case *http2.PingFrame:
		// Only ACK peer-initiated PINGs; never ACK an ACK (avoids a ping loop).
		if !f.IsAck() {
			conn.writeMu.Lock()
			err := conn.Framer.WritePing(true, f.Data)
			conn.writeMu.Unlock()
			if err != nil {
				return wrapStaleHTTP2Error("ping ack", err)
			}
		}

	case *http2.GoAwayFrame:
		return t.handleGoAway(conn, f)

	case *http2.WindowUpdateFrame:
		// Connection-level send-window accounting is not required for correct reads.

	case *http2.PushPromiseFrame:
		// Server push is disabled by default; ignore promised streams.
	}

	return nil
}

// handleGoAway evicts the connection from the pool and notifies any streams the
// server will not service (ID greater than LastStreamID) so they retry on a fresh
// connection. Streams at or below LastStreamID are allowed to finish; the read loop
// keeps running until the server closes the connection (EOF) or all work drains.
func (t *Transport) handleGoAway(conn *Connection, f *http2.GoAwayFrame) error {
	t.removeConnection(conn)

	goErr := wrapStaleHTTP2Error("server sent GOAWAY",
		fmt.Errorf("%w: last stream %d, code %v", errGoAway, f.LastStreamID, f.ErrCode))

	conn.mu.Lock()
	conn.goAwayReceived = true
	conn.goAwayLastStreamID = f.LastStreamID
	var rejected []*Stream
	remaining := 0
	for id, s := range conn.Streams {
		if id > f.LastStreamID {
			rejected = append(rejected, s)
		} else {
			remaining++
		}
	}
	conn.mu.Unlock()

	for _, s := range rejected {
		select {
		case s.inbox <- frameEvent{kind: fkConnErr, err: goErr}:
		case <-s.done:
		case <-conn.closedCh:
		}
	}

	// If nothing remains to be serviced, terminate the loop now.
	if remaining == 0 {
		return goErr
	}
	return nil
}

// classifyReadErr maps a Framer read error to a stale-classifiable error so the
// request-level retry can recover on a fresh connection.
func classifyReadErr(err error) error {
	if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
		return wrapStaleHTTP2Error("reading frame", errConnClosed)
	}
	var nerr net.Error
	if stderrors.As(err, &nerr) && nerr.Timeout() {
		return wrapStaleHTTP2Error("read timeout", errConnClosed)
	}
	return wrapStaleHTTP2Error("reading frame", err)
}
