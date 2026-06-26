package http2

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// Client is an HTTP/2 client implementation
type Client struct {
	transport       *Transport
	converter       *Converter
	streamManager   *StreamManager
	streamProcessor *StreamProcessor
	options         *Options
}

// NewClient creates a new HTTP/2 client
func NewClient(opts *Options) *Client {
	if opts == nil {
		opts = DefaultOptions()
	}

	streamManager := NewStreamManager(opts.MaxConcurrentStreams)

	return &Client{
		transport:       NewTransport(opts),
		converter:       NewConverter(),
		streamManager:   streamManager,
		streamProcessor: NewStreamProcessor(streamManager),
		options:         opts,
	}
}

// Do performs an HTTP/2 request using HTTP/1.1-style raw request format
// Deprecated: Use DoWithOptions instead to pass TLS configuration
func (c *Client) Do(ctx context.Context, rawRequest []byte, host string, port int, scheme string) (*Response, error) {
	return c.DoWithOptions(ctx, rawRequest, host, port, scheme, c.options)
}

// DoWithOptions performs an HTTP/2 request with custom options
func (c *Client) DoWithOptions(ctx context.Context, rawRequest []byte, host string, port int, scheme string, opts *Options) (*Response, error) {
	if opts == nil {
		opts = c.options
	}

	// Start timing
	timer := timing.NewTimer()
	startTime := time.Now()

	// Parse the raw request
	request, err := c.converter.parseHTTP11Request(rawRequest)
	if err != nil {
		return nil, errors.NewProtocolError("parsing request", err)
	}

	// Override scheme and authority if provided
	if scheme != "" {
		request.Scheme = scheme
	}
	if host != "" {
		request.Authority = host
	}

	// Connect to server (connection will be reused if pooling enabled)
	timer.StartTCP()
	conn, err := c.transport.Connect(ctx, host, port, scheme, opts)
	if err != nil {
		return nil, errors.NewConnectionError(host, port, err)
	}
	timer.EndTCP()

	// Without pooling, the connection is single-use: close it (and its read loop)
	// when we're done.
	if !opts.ReuseConnection {
		defer conn.Close()
	}

	// Atomically allocate the stream ID, register it, and write the request frames.
	// HTTP/2 requires new streams to be opened in strictly increasing stream-ID order,
	// so ID allocation and the HEADERS write must happen under the same writeMu
	// critical section; otherwise concurrent requests could interleave (lower ID after
	// higher), which the server rejects with PROTOCOL_ERROR.
	timer.StartTTFB()
	stream, err := c.openStream(conn, rawRequest, request)
	if err != nil {
		return nil, err
	}
	defer c.unregisterStream(conn, stream)

	// Read response by consuming dispatched frames for this stream.
	response, err := c.readResponse(ctx, conn, stream, opts)
	if err != nil {
		return nil, err
	}
	timer.EndTTFB()

	// Calculate total time
	totalTime := time.Since(startTime)

	// Get detailed timing metrics from timer
	metrics := timer.GetMetrics()

	// Add timing information
	response.TotalTime = totalTime
	response.Metrics = &metrics
	response.FrameStats = &FrameStats{
		FramesReceived: len(response.Frames),
	}

	// Add connection metadata
	c.fillConnectionMetadata(response, conn, host, port, scheme, opts)

	return response, nil
}

// DoFrames sends raw frames directly (advanced usage)
func (c *Client) DoFrames(ctx context.Context, frames []Frame, host string, port int, scheme string) (*Response, error) {
	// Get stream ID from first frame
	if len(frames) == 0 {
		return nil, errors.NewValidationError("no frames provided")
	}

	// Connect to server
	conn, err := c.transport.Connect(ctx, host, port, scheme, c.options)
	if err != nil {
		return nil, errors.NewConnectionError(host, port, err)
	}
	if !c.options.ReuseConnection {
		defer conn.Close()
	}

	// Register the (caller-provided) stream ID with an inbox so the read loop can
	// route its frames here.
	streamID := frames[0].StreamID()
	stream := &Stream{
		ID:    streamID,
		State: StateOpen,
		inbox: make(chan frameEvent, streamInboxSize),
		done:  make(chan struct{}),
	}
	conn.mu.Lock()
	conn.Streams[streamID] = stream
	conn.mu.Unlock()
	defer c.unregisterStream(conn, stream)

	// Send frames
	for _, frame := range frames {
		if err := c.sendFrame(conn, frame); err != nil {
			c.transport.removeConnection(conn)
			conn.fail(wrapStaleHTTP2Error("sending frame", err))
			return nil, wrapStaleHTTP2Error("sending frame", err)
		}
	}

	// Read response
	return c.readResponse(ctx, conn, stream, c.options)
}

// streamInboxSize is the per-stream frame inbox buffer. It absorbs bursts of DATA
// frames between the request goroutine's reads without blocking the shared read loop.
const streamInboxSize = 64

// maxClientStreamID is the largest valid client-initiated (odd) stream ID (2^31-1).
const maxClientStreamID = 1<<31 - 1

// openStream atomically allocates the next client stream ID, registers a stream
// with a frame inbox, and writes the request frames — all under conn.writeMu so the
// HEADERS that opens the stream is written in strictly increasing stream-ID order
// (an HTTP/2 requirement). It returns a stale-classified error if the connection is
// already dead, its stream IDs are exhausted, or a frame write fails (so the caller
// can retry on a fresh connection).
func (c *Client) openStream(conn *Connection, rawRequest []byte, request *Request) (*Stream, error) {
	conn.touch()

	conn.writeMu.Lock()

	conn.mu.Lock()
	if conn.Closed {
		err := conn.connErr
		conn.mu.Unlock()
		conn.writeMu.Unlock()
		if err == nil {
			err = wrapStaleHTTP2Error("connect", errConnClosed)
		}
		return nil, err
	}
	if conn.NextStreamID > maxClientStreamID {
		conn.mu.Unlock()
		conn.writeMu.Unlock()
		c.transport.removeConnection(conn)
		exhErr := wrapStaleHTTP2Error("stream alloc", errStreamIDExhausted)
		conn.fail(exhErr)
		return nil, exhErr
	}
	streamID := conn.NextStreamID
	conn.NextStreamID += 2 // Client uses odd stream IDs
	stream := &Stream{
		ID:             streamID,
		State:          StateOpen,
		Request:        request,
		WindowSize:     65535,
		PeerWindowSize:  65535,
		inbox:          make(chan frameEvent, streamInboxSize),
		done:           make(chan struct{}),
	}
	conn.Streams[streamID] = stream
	conn.mu.Unlock()

	// Convert and write the request frames while still holding writeMu.
	frames, err := c.converter.TextToFrames(rawRequest, streamID)
	if err != nil {
		conn.writeMu.Unlock()
		c.unregisterStream(conn, stream)
		return nil, errors.NewProtocolError("converting to frames", err)
	}
	for _, frame := range frames {
		if werr := c.sendFrameLocked(conn, frame); werr != nil {
			conn.writeMu.Unlock()
			c.unregisterStream(conn, stream)
			c.transport.removeConnection(conn)
			conn.fail(wrapStaleHTTP2Error("sending frame", werr))
			return nil, wrapStaleHTTP2Error("sending frame", werr)
		}
	}

	conn.writeMu.Unlock()
	return stream, nil
}

// unregisterStream releases a stream: it closes done (so the read loop never blocks
// routing to a finished stream) and removes it from the connection's stream table
// (preventing unbounded growth on long-lived reused connections).
func (c *Client) unregisterStream(conn *Connection, stream *Stream) {
	close(stream.done)
	conn.mu.Lock()
	delete(conn.Streams, stream.ID)
	conn.mu.Unlock()
}

// cancelStream sends a best-effort RST_STREAM(CANCEL) so the server stops sending
// frames for an abandoned stream (timeout / context cancel) without tearing down the
// whole connection.
func (c *Client) cancelStream(conn *Connection, stream *Stream) {
	conn.writeMu.Lock()
	_ = conn.Framer.WriteRSTStream(stream.ID, http2.ErrCodeCancel)
	conn.writeMu.Unlock()
}

// sendFrame sends a single frame, acquiring conn.writeMu. All Framer writes are
// serialized via writeMu so concurrent requests multiplexed on the same connection
// never corrupt the Framer.
func (c *Client) sendFrame(conn *Connection, frame Frame) error {
	conn.touch()
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()
	return c.sendFrameLocked(conn, frame)
}

// sendFrameLocked writes a single frame. The caller MUST already hold conn.writeMu.
func (c *Client) sendFrameLocked(conn *Connection, frame Frame) error {
	switch f := frame.(type) {
	case *HeadersFrame:
		// Encode headers using connection's encoder directly
		// We need to ensure we use the same encoder that was initialized with the connection
		if conn.Encoder == nil {
			return fmt.Errorf("connection encoder not initialized")
		}

		// Get the connection's encoder buffer and reset it for this frame
		conn.EncoderBuf.Reset()

		// Encode pseudo-headers first (in order)
		pseudoOrder := []string{":method", ":path", ":scheme", ":authority", ":status"}
		for _, name := range pseudoOrder {
			if value, ok := f.Headers[name]; ok {
				err := conn.Encoder.WriteField(hpack.HeaderField{Name: name, Value: value})
				if err != nil {
					return fmt.Errorf("failed to encode pseudo-header %s: %w", name, err)
				}
			}
		}

		// Encode regular headers
		for name, value := range f.Headers {
			if !strings.HasPrefix(name, ":") {
				err := conn.Encoder.WriteField(hpack.HeaderField{Name: strings.ToLower(name), Value: value})
				if err != nil {
					return fmt.Errorf("failed to encode header %s: %w", name, err)
				}
			}
		}

		encoded := conn.EncoderBuf.Bytes()

		// Send HEADERS frame
		return conn.Framer.WriteHeaders(http2.HeadersFrameParam{
			StreamID:      f.StreamId,
			BlockFragment: encoded,
			EndStream:     f.EndStream,
			EndHeaders:    f.EndHeaders,
			Priority:      convertPriority(f.Priority),
		})

	case *DataFrame:
		// Send DATA frame
		return conn.Framer.WriteData(f.StreamId, f.EndStream, f.Data)

	default:
		return fmt.Errorf("unsupported frame type: %T", frame)
	}
}

// readResponse assembles the response for a stream by consuming frame events that
// the connection's read loop routes into the stream inbox. The per-request read
// timeout is enforced here (rolling: reset on each received frame), so an idle
// pooled connection is never torn down for lack of a request.
func (c *Client) readResponse(ctx context.Context, conn *Connection, stream *Stream, opts *Options) (*Response, error) {
	response := &Response{
		StreamID:    stream.ID,
		Headers:     make(map[string][]string),
		Frames:      []Frame{},
		HTTPVersion: "HTTP/2",
	}

	var timer *time.Timer
	var timeoutC <-chan time.Time
	if opts != nil && opts.ReadTimeout > 0 {
		timer = time.NewTimer(opts.ReadTimeout)
		defer timer.Stop()
		timeoutC = timer.C
	}
	resetTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(opts.ReadTimeout)
	}

	gotFrame := false

	for {
		select {
		case <-ctx.Done():
			c.cancelStream(conn, stream)
			if ctx.Err() == context.DeadlineExceeded {
				return nil, errors.NewTimeoutError("reading response", 0)
			}
			return nil, errors.NewProtocolError("context cancelled", ctx.Err())

		case <-timeoutC:
			// No response frame within ReadTimeout. If this is a reused connection and
			// we never saw a single frame, the pooled connection is almost certainly
			// dead: evict + tear it down so the request-level retry recovers on a fresh
			// connection. Otherwise just cancel this stream.
			if !gotFrame && conn.wasReused() {
				c.transport.removeConnection(conn)
				deadErr := wrapStaleHTTP2Error("read timeout", errConnClosed)
				conn.fail(deadErr)
				return nil, deadErr
			}
			c.cancelStream(conn, stream)
			return nil, errors.NewTimeoutError("reading response", opts.ReadTimeout)

		case <-conn.closedCh:
			// Connection died (EOF / reset / GOAWAY drain). Surface the stale error so
			// the request is retried on a fresh connection.
			conn.mu.RLock()
			err := conn.connErr
			conn.mu.RUnlock()
			if err == nil {
				err = wrapStaleHTTP2Error("connection closed", errConnClosed)
			}
			return nil, err

		case ev := <-stream.inbox:
			gotFrame = true
			resetTimer()

			switch ev.kind {
			case fkHeaders:
				for name, value := range ev.headers {
					if name == ":status" {
						response.Status, _ = strconv.Atoi(value)
						response.StatusText = getStatusText(response.Status)
					} else if !strings.HasPrefix(name, ":") {
						response.Headers[name] = append(response.Headers[name], value)
					}
				}
				response.Frames = append(response.Frames, &HeadersFrame{
					StreamId:   stream.ID,
					Headers:    ev.headers,
					EndStream:  ev.endStream,
					EndHeaders: true,
				})
				if ev.endStream {
					return response, nil
				}

			case fkData:
				response.Body = append(response.Body, ev.data...)
				response.Frames = append(response.Frames, &DataFrame{
					StreamId:  stream.ID,
					Data:      ev.data,
					EndStream: ev.endStream,
				})
				if ev.endStream {
					return response, nil
				}

			case fkRST:
				return nil, errors.NewProtocolError("stream reset",
					fmt.Errorf("error code: %v", ev.errCode))

			case fkConnErr:
				return nil, ev.err
			}
		}
	}
}

// fillConnectionMetadata populates connection metadata in the response
func (c *Client) fillConnectionMetadata(response *Response, conn *Connection, host string, port int, scheme string, opts *Options) {
	// Get remote address
	if conn.Conn != nil {
		if remoteAddr := conn.Conn.RemoteAddr(); remoteAddr != nil {
			if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
				response.ConnectedIP = tcpAddr.IP.String()
				response.ConnectedPort = tcpAddr.Port
			}
		}
	}

	// Get TLS information if this is an HTTPS connection
	if scheme == "https" {
		if tlsConn, ok := conn.Conn.(*tls.Conn); ok {
			state := tlsConn.ConnectionState()

			// TLS Version
			response.TLSVersion = getTLSVersionString(state.Version)

			// Cipher Suite
			response.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)

			// Negotiated Protocol (ALPN)
			response.NegotiatedProtocol = state.NegotiatedProtocol

			// Server Name (SNI)
			response.TLSServerName = state.ServerName
		}
	}

	// Connection reuse (v2.0.3+: use actual reuse status from connection)
	response.ConnectionReused = conn.wasReused()

	// Proxy information
	if opts != nil && opts.Proxy != nil {
		response.ProxyUsed = true
		response.ProxyType = opts.Proxy.Type

		proxyPort := opts.Proxy.Port
		if proxyPort == 0 {
			// Apply default ports
			switch opts.Proxy.Type {
			case "http":
				proxyPort = 8080
			case "https":
				proxyPort = 443
			case "socks4", "socks5":
				proxyPort = 1080
			}
		}
		response.ProxyAddr = fmt.Sprintf("%s:%d", opts.Proxy.Host, proxyPort)
	} else {
		response.ProxyUsed = false
		response.ProxyType = ""
		response.ProxyAddr = ""
	}
}

// getTLSVersionString converts TLS version constant to string
func getTLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionSSL30:
		return "SSL 3.0"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// Close closes the HTTP/2 client
func (c *Client) Close() error {
	return c.transport.Close()
}

// GetPoolStats returns HTTP/2 connection pool statistics (DEF-5).
// This provides visibility into connection reuse, active streams, and pool health.
// Returns stats from pool regardless of client-level options, since per-request
// ReuseConnection may have been used.
func (c *Client) GetPoolStats() *ConnectionPoolStats {
	return c.transport.GetPoolStats()
}

// Helper functions

func convertPriority(p *PriorityParam) http2.PriorityParam {
	if p == nil {
		return http2.PriorityParam{}
	}
	return http2.PriorityParam{
		StreamDep: p.StreamDependency,
		Exclusive: p.Exclusive,
		Weight:    p.Weight,
	}
}

// FormatResponse formats an HTTP/2 response as HTTP/1.1-style text
func (c *Client) FormatResponse(resp *Response) []byte {
	var buf bytes.Buffer

	// Status line
	statusText := getStatusText(resp.Status)
	buf.WriteString(fmt.Sprintf("HTTP/2 %d %s\r\n", resp.Status, statusText))

	// Headers
	for name, values := range resp.Headers {
		for _, value := range values {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n",
				c.converter.normalizeHeaderName(name), value))
		}
	}

	// Empty line
	buf.WriteString("\r\n")

	// Body
	if len(resp.Body) > 0 {
		buf.Write(resp.Body)
	}

	return buf.Bytes()
}

func getStatusText(code int) string {
	texts := map[int]string{
		100: "Continue",
		101: "Switching Protocols",
		200: "OK",
		201: "Created",
		202: "Accepted",
		204: "No Content",
		301: "Moved Permanently",
		302: "Found",
		304: "Not Modified",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		405: "Method Not Allowed",
		500: "Internal Server Error",
		502: "Bad Gateway",
		503: "Service Unavailable",
	}

	if text, ok := texts[code]; ok {
		return text
	}
	return "Unknown"
}
