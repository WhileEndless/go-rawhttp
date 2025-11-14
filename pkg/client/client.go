// Package client provides the main HTTP client API.
package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/buffer"
	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
	"github.com/WhileEndless/go-rawhttp/pkg/transport"
)

const (
	maxHeaderBytes = 64 * 1024
)

// Options controls how the Client establishes connections and reads responses.
type Options struct {
	Scheme       string
	Host         string
	Port         int
	ConnectIP    string
	SNI          string
	DisableSNI   bool
	InsecureTLS  bool
	ConnTimeout  time.Duration
	DNSTimeout   time.Duration // DNS resolution timeout (0 = use ConnTimeout)
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	BodyMemLimit int64

	// Protocol selection (http/1.1 or http/2)
	Protocol string

	// HTTP/2 specific options
	HTTP2Settings *HTTP2Settings

	// Connection pooling and reuse
	ReuseConnection bool // Enable Keep-Alive and connection pooling

	// Upstream proxy support
	ProxyURL string // Upstream proxy URL (e.g., "http://proxy:8080" or "socks5://proxy:1080")

	// Custom TLS configuration
	CustomCACerts [][]byte // Custom root CA certificates in PEM format

	// TLSConfig allows direct passthrough of crypto/tls.Config for full TLS control.
	// If nil, default configuration will be used based on other options (InsecureTLS, SNI, etc.).
	// This provides maximum flexibility for custom TLS versions, cipher suites, client certificates, etc.
	TLSConfig *tls.Config `json:"-"`
}

// Response represents a parsed HTTP response.
type Response struct {
	StatusLine  string
	StatusCode  int
	Headers     map[string][]string
	Body        *buffer.Buffer
	Raw         *buffer.Buffer
	Timings     timing.Metrics
	BodyBytes   int64
	RawBytes    int64
	HTTPVersion string // "HTTP/1.1" or "HTTP/2"
	Metrics     *timing.Metrics

	// Connection metadata
	ConnectedIP          string // Actual IP address connected to (after DNS resolution)
	ConnectedPort        int    // Actual port connected to
	NegotiatedProtocol   string // Negotiated protocol (e.g., "HTTP/1.1", "HTTP/2", "h2")
	TLSVersion           string // TLS version used (e.g., "TLS 1.3")
	TLSCipherSuite       string // TLS cipher suite used
	TLSServerName        string // TLS Server Name (SNI)
	ConnectionReused     bool   // Whether the connection was reused from pool
}

// HTTP2Settings contains HTTP/2 specific configuration
type HTTP2Settings struct {
	EnableServerPush     bool
	EnableCompression    bool
	MaxConcurrentStreams uint32
	InitialWindowSize    uint32
	MaxFrameSize         uint32
	MaxHeaderListSize    uint32
	HeaderTableSize      uint32
}

// Client implements raw HTTP/1.1 transport.
type Client struct {
	transport *transport.Transport
}

// New returns a new Client instance.
func New() *Client {
	return &Client{
		transport: transport.New(),
	}
}

// NewWithTransport creates a Client with a custom transport.
func NewWithTransport(t *transport.Transport) *Client {
	return &Client{
		transport: t,
	}
}

// Do executes the HTTP request using raw sockets.
func (c *Client) Do(ctx context.Context, req []byte, opts Options) (*Response, error) {
	if c.transport == nil {
		return nil, errors.NewValidationError("client transport is nil")
	}

	if len(req) == 0 {
		return nil, errors.NewValidationError("request cannot be empty")
	}

	// Create timer for performance measurement
	timer := timing.NewTimer()

	// Create transport config
	transportConfig := transport.Config{
		Scheme:          opts.Scheme,
		Host:            opts.Host,
		Port:            opts.Port,
		ConnectIP:       opts.ConnectIP,
		SNI:             opts.SNI,
		DisableSNI:      opts.DisableSNI,
		InsecureTLS:     opts.InsecureTLS,
		ConnTimeout:     opts.ConnTimeout,
		DNSTimeout:      opts.DNSTimeout,
		ReadTimeout:     opts.ReadTimeout,
		WriteTimeout:    opts.WriteTimeout,
		ReuseConnection: opts.ReuseConnection,
		ProxyURL:        opts.ProxyURL,
		CustomCACerts:   opts.CustomCACerts,
		TLSConfig:       opts.TLSConfig,
	}

	// Establish connection
	conn, connMetadata, err := c.transport.Connect(ctx, transportConfig, timer)
	if err != nil {
		return nil, err
	}

	// Handle connection cleanup based on pooling settings
	shouldClose := !opts.ReuseConnection
	defer func() {
		if shouldClose {
			c.transport.CloseConnection(opts.Host, opts.Port, conn)
		} else {
			c.transport.ReleaseConnection(opts.Host, opts.Port, conn)
		}
	}()

	// Initialize response
	response := &Response{
		Headers: make(map[string][]string),
		Body:    buffer.New(opts.BodyMemLimit),
		// Raw buffer needs extra space for headers, status line, and HTTP overhead
		// 2x size ensures adequate space for headers + body without frequent disk spilling
		Raw: buffer.New(opts.BodyMemLimit * 2),
		// Set connection metadata
		ConnectedIP:        connMetadata.ConnectedIP,
		ConnectedPort:      connMetadata.ConnectedPort,
		NegotiatedProtocol: connMetadata.NegotiatedProtocol,
		TLSVersion:         connMetadata.TLSVersion,
		TLSCipherSuite:     connMetadata.TLSCipherSuite,
		TLSServerName:      connMetadata.TLSServerName,
		ConnectionReused:   connMetadata.ConnectionReused,
	}

	// Send request
	if err := c.sendRequest(conn, req, opts.WriteTimeout); err != nil {
		return nil, err
	}

	// Read response
	if err := c.readResponse(conn, response, opts.ReadTimeout, timer); err != nil {
		// Set metrics even for partial responses
		response.Timings = timer.GetMetrics()
		response.BodyBytes = response.Body.Size()
		response.RawBytes = response.Raw.Size()
		// IMPORTANT: Returning partial response with error
		// Caller MUST close response.Body and response.Raw even on error
		// Auto-close buffers on specific errors to prevent leaks
		if errors.IsTimeoutError(err) || errors.IsContextCanceled(err) {
			response.Body.Close()
			response.Raw.Close()
			return nil, err // Don't return partial response for timeout/cancel
		}
		return response, err // Return partial response for other errors
	}

	// Set final metrics
	response.Timings = timer.GetMetrics()
	response.BodyBytes = response.Body.Size()
	response.RawBytes = response.Raw.Size()

	return response, nil
}

func (c *Client) sendRequest(conn net.Conn, req []byte, writeTimeout time.Duration) error {
	if writeTimeout > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
			return errors.NewIOError("setting write deadline", err)
		}
		defer conn.SetWriteDeadline(time.Time{})
	}

	// Handle partial writes by writing all data
	written := 0
	for written < len(req) {
		n, err := conn.Write(req[written:])
		if err != nil {
			return errors.NewIOError("writing request", err)
		}
		written += n
	}

	return nil
}

func (c *Client) readResponse(conn net.Conn, response *Response, readTimeout time.Duration, timer *timing.Timer) error {
	if readTimeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			return errors.NewIOError("setting read deadline", err)
		}
	}

	reader := bufio.NewReader(conn)

	// Read status line
	timer.StartTTFB()
	statusLine, err := c.readLine(reader)
	timer.EndTTFB()
	if err != nil {
		return errors.NewProtocolError("reading status line", err)
	}

	response.StatusLine = statusLine
	if _, err := response.Raw.Write([]byte(statusLine + "\r\n")); err != nil {
		return err
	}

	// Parse status code and HTTP version
	if err := c.parseStatusLine(statusLine, response); err != nil {
		return err
	}

	// Read headers
	headers, err := c.readHeaders(reader, response.Raw)
	if err != nil {
		return err
	}
	response.Headers = headers

	// Read body based on headers
	return c.readBody(reader, response, headers)
}

func (c *Client) readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) >= 2 && line[len(line)-2:] == "\r\n" {
		return line[:len(line)-2], nil
	}
	return strings.TrimRight(line, "\n"), nil
}

func (c *Client) parseStatusLine(statusLine string, response *Response) error {
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return errors.NewProtocolError("invalid status line format", nil)
	}

	// Extract HTTP version
	if len(parts[0]) > 0 {
		response.HTTPVersion = parts[0] // e.g., "HTTP/1.1" or "HTTP/1.0"
	}

	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return errors.NewProtocolError("invalid status code", err)
	}

	response.StatusCode = code
	return nil
}

func (c *Client) readHeaders(reader *bufio.Reader, raw *buffer.Buffer) (map[string][]string, error) {
	headers := make(map[string][]string)
	total := 0
	var lastKey string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, errors.NewProtocolError("reading headers", err)
		}

		total += len(line)
		if total > maxHeaderBytes {
			return nil, errors.NewProtocolError("headers exceed maximum size", nil)
		}

		if _, err := raw.Write([]byte(line)); err != nil {
			return nil, err
		}

		if line == "\r\n" {
			break
		}

		trimmed := strings.TrimRight(line, "\r\n")

		// Handle header continuation (RFC 7230 Section 3.2.4)
		if strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
			if lastKey == "" {
				continue
			}
			idx := len(headers[lastKey]) - 1
			// Preserve original whitespace for raw HTTP client behavior
			headers[lastKey][idx] = headers[lastKey][idx] + strings.TrimSpace(trimmed)
			continue
		}

		// Parse header
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		headers[key] = append(headers[key], value)
		lastKey = key
	}

	return headers, nil
}

func (c *Client) readBody(reader *bufio.Reader, response *Response, headers map[string][]string) error {
	transferEncoding := c.getHeaderValue(headers, "Transfer-Encoding")
	contentLength := c.getHeaderValue(headers, "Content-Length")
	connectionHeader := c.getHeaderValue(headers, "Connection")

	switch {
	case strings.Contains(strings.ToLower(transferEncoding), "chunked"):
		return c.readChunkedBody(reader, response.Body, response.Raw, response.Headers)
	case contentLength != "":
		length, err := strconv.ParseInt(strings.TrimSpace(contentLength), 10, 64)
		if err != nil {
			return errors.NewProtocolError("invalid content-length", err)
		}
		// Protect against negative or excessively large Content-Length
		if length < 0 {
			return errors.NewProtocolError("negative content-length not allowed", nil)
		}
		// Reasonable limit: 1TB (can be adjusted based on needs)
		if length > 1024*1024*1024*1024 {
			return errors.NewProtocolError("content-length too large", nil)
		}
		return c.readFixedBody(reader, length, response.Body, response.Raw)
	default:
		return c.readUntilClose(reader, connectionHeader, response.Body, response.Raw)
	}
}

func (c *Client) getHeaderValue(headers map[string][]string, key string) string {
	if headers == nil {
		return ""
	}
	if values, ok := headers[textproto.CanonicalMIMEHeaderKey(key)]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c *Client) readChunkedBody(r *bufio.Reader, dst, raw *buffer.Buffer, headers map[string][]string) error {
	tp := textproto.NewReader(r)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return errors.NewProtocolError("reading chunk size", err)
		}

		if _, err := raw.Write([]byte(line + "\r\n")); err != nil {
			return err
		}

		size, err := strconv.ParseInt(strings.TrimSpace(strings.Split(line, ";")[0]), 16, 64)
		if err != nil {
			return errors.NewProtocolError("invalid chunk size", err)
		}

		if size == 0 {
			break
		}

		if _, err := io.CopyN(io.MultiWriter(dst, raw), tp.R, size); err != nil {
			return errors.NewIOError("reading chunk body", err)
		}

		crlf := make([]byte, 2)
		if _, err := io.ReadFull(tp.R, crlf); err != nil {
			return errors.NewIOError("reading chunk CRLF", err)
		}

		if _, err := raw.Write(crlf); err != nil {
			return err
		}
	}

	// Read trailers
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return errors.NewProtocolError("reading chunk trailer", err)
		}

		if _, err := raw.Write([]byte(line + "\r\n")); err != nil {
			return err
		}

		if line == "" {
			break
		}

		// Parse trailer header and add to headers map
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			key := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			headers[key] = append(headers[key], value)
		}
	}

	return nil
}

func (c *Client) readFixedBody(r *bufio.Reader, length int64, dst, raw *buffer.Buffer) error {
	if length <= 0 {
		return nil
	}

	_, err := io.CopyN(io.MultiWriter(dst, raw), r, length)
	if err != nil {
		return errors.NewIOError("reading fixed body", err)
	}

	return nil
}

func (c *Client) readUntilClose(r *bufio.Reader, connectionHeader string, dst, raw *buffer.Buffer) error {
	_, err := io.Copy(io.MultiWriter(dst, raw), r)
	if err != nil && err != io.EOF {
		return errors.NewIOError("reading until close", err)
	}

	return nil
}
