package http2

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	// Temporarily update transport options for this request
	originalOpts := c.transport.options
	if opts != nil {
		c.transport.options = opts
	}
	defer func() {
		c.transport.options = originalOpts
	}()
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
	conn, err := c.transport.Connect(ctx, host, port, scheme)
	if err != nil {
		return nil, errors.NewConnectionError(host, port, err)
	}
	// Don't close if connection pooling is enabled
	if !c.options.ReuseConnection {
		defer c.transport.CloseConnection(fmt.Sprintf("%s:%d", host, port))
	}
	timer.EndTCP()

	// Create a new stream with proper synchronization
	// Use connection's NextStreamID with mutex protection
	conn.mu.Lock()
	streamID := conn.NextStreamID
	conn.NextStreamID += 2 // Client uses odd stream IDs
	conn.mu.Unlock()

	// Register the stream
	stream := &Stream{
		ID:             streamID,
		State:          StateOpen,
		Request:        request,
		WindowSize:     65535,
		PeerWindowSize: 65535,
	}

	// Add to stream manager
	c.streamManager.mu.Lock()
	c.streamManager.streams[streamID] = stream
	c.streamManager.mu.Unlock()

	// Convert request to frames
	frames, err := c.converter.TextToFrames(rawRequest, stream.ID)
	if err != nil {
		return nil, errors.NewProtocolError("converting to frames", err)
	}

	// Send request frames
	timer.StartTTFB()
	for _, frame := range frames {
		if err := c.sendFrame(conn, frame); err != nil {
			return nil, errors.NewIOError("sending frame", err)
		}

		// Update stream state
		if hf, ok := frame.(*HeadersFrame); ok && hf.EndStream {
			c.streamManager.UpdateStreamState(stream.ID, StateHalfClosedLocal)
		} else if df, ok := frame.(*DataFrame); ok && df.EndStream {
			c.streamManager.UpdateStreamState(stream.ID, StateHalfClosedLocal)
		}
	}

	// Read response
	response, err := c.readResponse(ctx, conn, stream)
	if err != nil {
		return nil, err
	}
	timer.EndTTFB()

	// Calculate total time
	totalTime := time.Since(startTime)

	// Add timing information
	response.TotalTime = totalTime
	response.FrameStats = &FrameStats{
		FramesSent:     len(frames),
		FramesReceived: len(response.Frames),
	}

	return response, nil
}

// DoFrames sends raw frames directly (advanced usage)
func (c *Client) DoFrames(ctx context.Context, frames []Frame, host string, port int, scheme string) (*Response, error) {
	// Connect to server
	conn, err := c.transport.Connect(ctx, host, port, scheme)
	if err != nil {
		return nil, errors.NewConnectionError(host, port, err)
	}

	// Get stream ID from first frame
	if len(frames) == 0 {
		return nil, errors.NewValidationError("no frames provided")
	}
	streamID := frames[0].StreamID()

	// Register stream
	stream := &Stream{
		ID:    streamID,
		State: StateOpen,
	}
	c.streamManager.mu.Lock()
	c.streamManager.streams[streamID] = stream
	c.streamManager.mu.Unlock()

	// Send frames
	for _, frame := range frames {
		if err := c.sendFrame(conn, frame); err != nil {
			return nil, errors.NewIOError("sending frame", err)
		}
	}

	// Read response
	return c.readResponse(ctx, conn, stream)
}

// sendFrame sends a single frame with thread-safe access
func (c *Client) sendFrame(conn *Connection, frame Frame) error {
	// Lock the connection for thread-safe frame sending
	// This prevents concurrent writes to the Framer which would corrupt the stream
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Update connection activity
	conn.LastActivity = time.Now()

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

// readResponse reads the complete response for a stream
func (c *Client) readResponse(ctx context.Context, conn *Connection, stream *Stream) (*Response, error) {
	response := &Response{
		StreamID:    stream.ID,
		Headers:     make(map[string][]string),
		Frames:      []Frame{},
		HTTPVersion: "HTTP/2",
	}

	// Read frames until stream is complete
	for {
		// Check context
		select {
		case <-ctx.Done():
			return nil, errors.NewTimeoutError("reading response", 30*time.Second)
		default:
		}

		// Read next frame
		rawFrame, err := conn.Framer.ReadFrame()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.NewIOError("reading frame", err)
		}

		// Update connection activity
		conn.mu.Lock()
		conn.LastActivity = time.Now()
		conn.mu.Unlock()

		// Process frame based on type
		switch f := rawFrame.(type) {
		case *http2.HeadersFrame:
			if f.StreamID != stream.ID {
				continue // Frame for different stream
			}

			// Decode headers using connection's decoder
			converter := &Converter{
				encoder: conn.Encoder,
				decoder: conn.Decoder,
			}
			headers, err := converter.DecodeHeaders(f.HeaderBlockFragment())
			if err != nil {
				return nil, errors.NewProtocolError("decoding headers", err)
			}

			// Process status and headers
			for name, value := range headers {
				if name == ":status" {
					response.Status, _ = strconv.Atoi(value)
				} else if !strings.HasPrefix(name, ":") {
					response.Headers[name] = append(response.Headers[name], value)
				}
			}

			// Add to frames list
			response.Frames = append(response.Frames, &HeadersFrame{
				StreamId:   f.StreamID,
				Headers:    headers,
				EndStream:  f.StreamEnded(),
				EndHeaders: f.HeadersEnded(),
			})

			// Check if stream ended
			if f.StreamEnded() {
				return response, nil
			}

		case *http2.DataFrame:
			if f.StreamID != stream.ID {
				continue // Frame for different stream
			}

			// Append data
			data := f.Data()
			response.Body = append(response.Body, data...)

			// Send WINDOW_UPDATE to maintain flow control
			// This is critical for proper HTTP/2 flow control
			dataLen := len(data)
			if dataLen > 0 {
				// Update stream window
				if err := conn.Framer.WriteWindowUpdate(f.StreamID, uint32(dataLen)); err != nil {
					return nil, errors.NewIOError("sending stream window update", err)
				}
				// Update connection window
				if err := conn.Framer.WriteWindowUpdate(0, uint32(dataLen)); err != nil {
					return nil, errors.NewIOError("sending connection window update", err)
				}
			}

			// Add to frames list
			response.Frames = append(response.Frames, &DataFrame{
				StreamId:  f.StreamID,
				Data:      data,
				EndStream: f.StreamEnded(),
			})

			// Check if stream ended
			if f.StreamEnded() {
				return response, nil
			}

		case *http2.SettingsFrame:
			// ACK settings
			conn.Framer.WriteSettingsAck()

		case *http2.WindowUpdateFrame:
			// Update window size
			c.streamManager.UpdateWindowSize(f.StreamID, int32(f.Increment))

		case *http2.PingFrame:
			// Respond to PING with ACK
			conn.Framer.WritePing(true, f.Data)

		case *http2.GoAwayFrame:
			// Server is shutting down
			return nil, errors.NewProtocolError("server sent GOAWAY",
				fmt.Errorf("last stream: %d, error: %v", f.LastStreamID, f.ErrCode))

		case *http2.RSTStreamFrame:
			if f.StreamID == stream.ID {
				return nil, errors.NewProtocolError("stream reset",
					fmt.Errorf("error code: %v", f.ErrCode))
			}
		}
	}

	return response, nil
}

// Close closes the HTTP/2 client
func (c *Client) Close() error {
	return c.transport.Close()
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
