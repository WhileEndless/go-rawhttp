// Package client provides the main HTTP client API.
package client

import (
	"bufio"
	"bytes"
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

// ProxyConfig provides detailed configuration for upstream proxy connections.
// This struct offers fine-grained control over proxy behavior, including
// authentication, timeouts, custom headers, and protocol-specific options.
//
// Supported proxy types:
//   - "http": HTTP proxy using CONNECT method (RFC 7231)
//   - "https": HTTP proxy over TLS connection
//   - "socks4": SOCKS version 4 proxy (IPv4 only, RFC 1928)
//   - "socks5": SOCKS version 5 proxy (full-featured, RFC 1928)
//
// Basic usage:
//
//	proxy := &ProxyConfig{
//	    Type:     "socks5",
//	    Host:     "proxy.example.com",
//	    Port:     1080,
//	    Username: "user",
//	    Password: "secret",
//	}
//
// For simple use cases, use ParseProxyURL instead:
//
//	proxy := ParseProxyURL("socks5://user:secret@proxy.example.com:1080")
type ProxyConfig struct {
	// Type specifies the proxy protocol.
	// Valid values: "http", "https", "socks4", "socks5"
	// Required field.
	Type string `json:"type"`

	// Host is the proxy server hostname or IP address.
	// Required field.
	Host string `json:"host"`

	// Port is the proxy server port number.
	// If zero, defaults are used:
	//   - http: 8080
	//   - https: 443
	//   - socks4/socks5: 1080
	Port int `json:"port"`

	// Username for proxy authentication (optional).
	// - HTTP/HTTPS: Used in Proxy-Authorization header (Basic auth)
	// - SOCKS4: Used as user ID field
	// - SOCKS5: Used in username/password authentication
	Username string `json:"username,omitempty"`

	// Password for proxy authentication (optional).
	// Only used for HTTP/HTTPS and SOCKS5 proxies.
	// Ignored for SOCKS4 (which only has username/user ID).
	Password string `json:"password,omitempty"`

	// ConnTimeout specifies the timeout for connecting to the proxy server.
	// If zero, Options.ConnTimeout is used.
	// This is separate from the timeout for connecting to the target server.
	ConnTimeout time.Duration `json:"conn_timeout,omitempty"`

	// ProxyHeaders specifies custom headers to include in the HTTP CONNECT request.
	// Only applies to "http" and "https" proxy types.
	// Ignored for SOCKS proxies.
	//
	// Example:
	//   ProxyHeaders: map[string]string{
	//       "X-Custom-Header": "value",
	//       "Proxy-Connection": "keep-alive",
	//   }
	ProxyHeaders map[string]string `json:"proxy_headers,omitempty"`

	// TLSConfig specifies custom TLS configuration for the proxy connection.
	// Only applies when Type="https" (connecting TO the proxy over TLS).
	// This is separate from Options.TLSConfig, which configures TLS to the target server.
	//
	// Use case: Proxy server uses self-signed certificate
	//   TLSConfig: &tls.Config{InsecureSkipVerify: true}
	TLSConfig *tls.Config `json:"-"`

	// ResolveDNSViaProxy controls DNS resolution for SOCKS5 proxies.
	// - true (default): Target hostname is sent to SOCKS5 proxy, which resolves DNS
	// - false: DNS is resolved locally before connecting to SOCKS5 proxy
	//
	// Only applies to Type="socks5". Ignored for other proxy types.
	// HTTP proxies always resolve DNS locally (CONNECT uses hostname).
	// SOCKS4 always resolves DNS locally (requires IPv4 address).
	ResolveDNSViaProxy bool `json:"resolve_dns_via_proxy,omitempty"`
}

// Options controls how the Client establishes connections and reads responses.
type Options struct {
	Scheme    string
	Host      string
	Port      int
	ConnectIP string // Optional: specific IP to connect to (bypasses DNS)

	// TLS/SNI Configuration
	// SNI specifies custom Server Name Indication for TLS handshake.
	// Priority: TLSConfig.ServerName > SNI > Host (if DisableSNI is false)
	SNI string

	// DisableSNI completely disables SNI extension in TLS handshake.
	// Cannot be used together with SNI option (validation error).
	DisableSNI bool

	// InsecureTLS skips TLS certificate verification (for testing/development).
	// IMPORTANT (DEF-13): This flag ALWAYS overrides TLSConfig.InsecureSkipVerify,
	// even when custom TLSConfig is provided. This is intentional to support proxy
	// MITM scenarios where you need custom TLS settings AND disabled verification.
	// Example: InsecureTLS=true + custom TLSConfig → verification is DISABLED.
	InsecureTLS bool

	// Timeouts
	ConnTimeout  time.Duration
	DNSTimeout   time.Duration // DNS resolution timeout (0 = use ConnTimeout)
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Body memory limit before spilling to disk (default: 4MB)
	BodyMemLimit int64

	// Protocol selection (http/1.1 or http/2)
	Protocol string

	// HTTP/2 specific options (deprecated: this is now a placeholder for backward compatibility)
	// TLS settings (InsecureTLS, TLSConfig) are now automatically passed to HTTP/2
	HTTP2Settings *HTTP2Settings

	// Connection pooling and reuse
	ReuseConnection bool // Enable Keep-Alive and connection pooling

	// Upstream proxy configuration (v2.0.0+)
	// Use ParseProxyURL for simple cases or create ProxyConfig for advanced control.
	//
	// Simple usage:
	//   Proxy: ParseProxyURL("socks5://user:pass@proxy.com:1080")
	//
	// Advanced usage:
	//   Proxy: &ProxyConfig{
	//       Type: "socks5",
	//       Host: "proxy.com",
	//       Port: 1080,
	//       Username: "user",
	//       Password: "pass",
	//       ConnTimeout: 10 * time.Second,
	//   }
	Proxy *ProxyConfig

	// Custom TLS configuration
	CustomCACerts [][]byte // Custom root CA certificates in PEM format

	// Client certificate for mutual TLS (mTLS authentication)
	// Option 1: Provide PEM-encoded certificate and key directly
	ClientCertPEM []byte // Client certificate in PEM format
	ClientKeyPEM  []byte // Client private key in PEM format (unencrypted)

	// Option 2: Provide file paths (will be loaded automatically)
	ClientCertFile string // Path to client certificate file (.crt, .pem)
	ClientKeyFile  string // Path to client private key file (.key, .pem)

	// TLSConfig allows direct passthrough of crypto/tls.Config for full TLS control.
	// If nil, default configuration will be used based on other options (InsecureTLS, SNI, etc.).
	// Note: InsecureTLS flag will override InsecureSkipVerify if set to true (DEF-13).
	// Note: Client certificates can also be added via TLSConfig.Certificates, but using
	// ClientCertPEM/ClientKeyPEM or ClientCertFile/ClientKeyFile is more convenient.
	TLSConfig *tls.Config `json:"-"`

	// SSL/TLS Protocol Version Control (v1.2.0+)
	// These provide user-friendly version control without requiring direct TLSConfig manipulation
	// Priority: TLSConfig.MinVersion/MaxVersion > MinTLSVersion/MaxTLSVersion > defaults
	MinTLSVersion uint16 // Minimum SSL/TLS version (e.g., tls.VersionTLS12, tls.VersionSSL30)
	MaxTLSVersion uint16 // Maximum SSL/TLS version (e.g., tls.VersionTLS13)

	// SSL/TLS Renegotiation Support (v1.2.0+)
	// Controls whether TLS renegotiation is allowed (default: never)
	// Options: tls.RenegotiateNever, tls.RenegotiateOnceAsClient, tls.RenegotiateFreelyAsClient
	// WARNING: Renegotiation can have security implications, use with caution
	TLSRenegotiation tls.RenegotiationSupport

	// Cipher Suite Control (v1.2.0+)
	// Specify allowed cipher suites in order of preference
	// If nil, Go's default secure cipher suites are used
	// Use CipherSuites field in TLSConfig for more control
	CipherSuites []uint16
}

// Response represents a parsed HTTP response.
type Response struct {
	StatusLine  string
	StatusCode  int
	Method      string // HTTP method from the request (e.g., "GET", "POST", "HEAD")
	Headers     map[string][]string
	Body        *buffer.Buffer
	Raw         *buffer.Buffer
	Timings     timing.Metrics
	BodyBytes   int64
	RawBytes    int64
	HTTPVersion string // "HTTP/1.1" or "HTTP/2"
	Metrics     *timing.Metrics

	// Connection metadata - Basic network information
	ConnectedIP          string // Actual IP address connected to (after DNS resolution)
	ConnectedPort        int    // Actual port connected to
	NegotiatedProtocol   string // Negotiated protocol (e.g., "HTTP/1.1", "HTTP/2", "h2")
	ConnectionReused     bool   // Whether the connection was reused from pool

	// Enhanced connection metadata - Socket-level information
	LocalAddr    string // Local socket address (e.g., "192.168.1.100:54321")
	RemoteAddr   string // Remote socket address (e.g., "93.184.216.34:443")
	ConnectionID uint64 // Unique connection identifier for tracking

	// TLS metadata - Standard TLS information
	TLSVersion     string // TLS version used (e.g., "TLS 1.3")
	TLSCipherSuite string // TLS cipher suite used
	TLSServerName  string // TLS Server Name (SNI)

	// Enhanced TLS metadata - Session information
	TLSSessionID string // TLS session ID (hex-encoded)
	TLSResumed   bool   // Whether TLS session was resumed

	// Proxy metadata (v2.0.0+)
	ProxyUsed bool   // Whether the request was routed through an upstream proxy
	ProxyType string // Proxy protocol type: "http", "https", "socks4", "socks5" (only if ProxyUsed=true)
	ProxyAddr string // Proxy server address "host:port" (only if ProxyUsed=true)
}

// HTTP2Settings contains HTTP/2 specific configuration.
// These settings map directly to HTTP/2 SETTINGS frame parameters (RFC 7540).
type HTTP2Settings struct {
	// MaxConcurrentStreams limits the number of concurrent streams (SETTINGS_MAX_CONCURRENT_STREAMS).
	// Default: unlimited. Set to control server resource usage.
	MaxConcurrentStreams uint32

	// InitialWindowSize sets the initial flow control window size (SETTINGS_INITIAL_WINDOW_SIZE).
	// Default: 65535 bytes. Increase for high-throughput scenarios.
	InitialWindowSize uint32

	// MaxFrameSize sets the maximum frame payload size (SETTINGS_MAX_FRAME_SIZE).
	// Default: 16384 bytes. Valid range: 16384 to 16777215.
	MaxFrameSize uint32

	// MaxHeaderListSize limits the maximum size of header list (SETTINGS_MAX_HEADER_LIST_SIZE).
	// Default: unlimited. Set to protect against large header attacks.
	MaxHeaderListSize uint32

	// HeaderTableSize sets the HPACK header compression table size (SETTINGS_HEADER_TABLE_SIZE).
	// Default: 4096 bytes.
	HeaderTableSize uint32

	// DisableServerPush disables HTTP/2 server push (sets SETTINGS_ENABLE_PUSH to 0).
	// Recommended for security and to reduce unwanted traffic.
	DisableServerPush bool

	// EnableCompression enables HPACK header compression.
	// Default: true. Disable only for debugging.
	EnableCompression bool

	// Debug contains HTTP/2 debugging flags (optional, all default to false).
	// These flags enable detailed logging of HTTP/2 protocol operations.
	// Production safe - explicit opt-in with zero overhead when disabled.
	Debug struct {
		LogFrames   bool `json:"log_frames,omitempty"`   // Log all HTTP/2 frames
		LogSettings bool `json:"log_settings,omitempty"` // Log SETTINGS frames
		LogHeaders  bool `json:"log_headers,omitempty"`  // Log HEADERS frames
		LogData     bool `json:"log_data,omitempty"`     // Log DATA frames
	} `json:"debug,omitempty"`

	// Deprecated: Use DisableServerPush instead (inverted logic for clarity).
	EnableServerPush bool
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

// PoolStats returns connection pool statistics.
func (c *Client) PoolStats() transport.PoolStats {
	if c.transport == nil {
		return transport.PoolStats{}
	}
	return c.transport.PoolStats()
}

// convertProxyConfig converts client.ProxyConfig to transport.ProxyConfig.
// Returns nil if input is nil.
func convertProxyConfig(clientProxy *ProxyConfig) *transport.ProxyConfig {
	if clientProxy == nil {
		return nil
	}

	return &transport.ProxyConfig{
		Type:               clientProxy.Type,
		Host:               clientProxy.Host,
		Port:               clientProxy.Port,
		Username:           clientProxy.Username,
		Password:           clientProxy.Password,
		ConnTimeout:        clientProxy.ConnTimeout,
		ProxyHeaders:       clientProxy.ProxyHeaders,
		TLSConfig:          clientProxy.TLSConfig,
		ResolveDNSViaProxy: clientProxy.ResolveDNSViaProxy,
	}
}

// parseMethod extracts the HTTP method from a raw request.
func parseMethod(req []byte) string {
	// Find first space - method is everything before it
	idx := bytes.IndexByte(req, ' ')
	if idx <= 0 {
		return ""
	}
	return strings.ToUpper(string(req[:idx]))
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
		Proxy:           convertProxyConfig(opts.Proxy), // Convert client.ProxyConfig to transport.ProxyConfig
		CustomCACerts:   opts.CustomCACerts,
		ClientCertPEM:   opts.ClientCertPEM,
		ClientKeyPEM:    opts.ClientKeyPEM,
		ClientCertFile:  opts.ClientCertFile,
		ClientKeyFile:   opts.ClientKeyFile,
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
			c.transport.CloseConnectionWithMetadata(opts.Host, opts.Port, conn, connMetadata)
		} else {
			c.transport.ReleaseConnectionWithMetadata(opts.Host, opts.Port, conn, connMetadata)
		}
	}()

	// Parse HTTP method from request for RFC 9110 compliance
	method := parseMethod(req)

	// Initialize response
	// Calculate raw buffer size with validation (DEF-2)
	rawBufferSize := opts.BodyMemLimit
	if rawBufferSize == 0 {
		rawBufferSize = 4 * 1024 * 1024 // Default 4MB
	}
	// Add overhead for headers but cap at reasonable maximum (100MB)
	rawBufferSize += 1024 * 1024 // Add 1MB overhead
	if rawBufferSize > 100*1024*1024 {
		rawBufferSize = 100 * 1024 * 1024 // Cap at 100MB
	}

	response := &Response{
		Method:  method, // Store method for body reading logic
		Headers: make(map[string][]string),
		Body:    buffer.New(opts.BodyMemLimit),
		// Raw buffer needs extra space for headers, status line, and HTTP overhead
		Raw: buffer.New(rawBufferSize),
		// Set basic connection metadata
		ConnectedIP:        connMetadata.ConnectedIP,
		ConnectedPort:      connMetadata.ConnectedPort,
		NegotiatedProtocol: connMetadata.NegotiatedProtocol,
		ConnectionReused:   connMetadata.ConnectionReused,
		// Set enhanced socket metadata
		LocalAddr:    connMetadata.LocalAddr,
		RemoteAddr:   connMetadata.RemoteAddr,
		ConnectionID: connMetadata.ConnectionID,
		// Set TLS metadata
		TLSVersion:     connMetadata.TLSVersion,
		TLSCipherSuite: connMetadata.TLSCipherSuite,
		TLSServerName:  connMetadata.TLSServerName,
		// Set enhanced TLS metadata
		TLSSessionID: connMetadata.TLSSessionID,
		TLSResumed:   connMetadata.TLSResumed,
		// Set proxy metadata (v2.0.0+)
		ProxyUsed: connMetadata.ProxyUsed,
		ProxyType: connMetadata.ProxyType,
		ProxyAddr: connMetadata.ProxyAddr,
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
	statusCode := response.StatusCode
	method := response.Method
	transferEncoding := c.getHeaderValue(headers, "Transfer-Encoding")
	contentLength := c.getHeaderValue(headers, "Content-Length")
	connectionHeader := c.getHeaderValue(headers, "Connection")

	// RFC 9110 Section 6.4.1: Responses that MUST NOT have a message body
	// "All 1xx (Informational), 204 (No Content), and 304 (Not Modified) responses
	// do not include content."
	// "A response to a HEAD request is identical to a 200 (OK) response, except that
	// it does not include content."
	//
	// IMPORTANT: As a RAW HTTP library (like Burp Suite), we need to handle:
	// 1. RFC-compliant servers: Content-Length present but NO body → skip (prevent timeout)
	// 2. RFC-violating servers: Body actually sent → capture it (catch violations)
	//
	// Strategy: PEEK at buffered data to detect if server actually sent a body
	if method == "HEAD" ||
		(statusCode >= 100 && statusCode < 200) || // 1xx Informational
		statusCode == 204 || // No Content
		statusCode == 304 { // Not Modified

		// Check if there's actually buffered data available (peek without consuming)
		if buffered := reader.Buffered(); buffered > 0 {
			// Server sent data despite RFC saying not to
			// This is an RFC violation, but we capture it anyway (raw HTTP library behavior)
			// Fall through to normal body reading logic
		} else {
			// No buffered data = RFC-compliant server
			// Skip body reading to prevent timeout on keep-alive connections
			// (Server sent Content-Length for informational purposes only)
			return nil
		}
	}

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
		// As a RAW HTTP library, we need to handle Content-Length mismatches gracefully
		// Some servers send incorrect Content-Length headers (RFC violation)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			// Server sent less data than Content-Length indicated
			// This is a protocol violation, but we accept partial reads
			// io.CopyN already wrote the available bytes to dst and raw
			return nil
		}
		return errors.NewIOError("reading fixed body", err)
	}

	// Check if there's more data available than Content-Length specified
	// This is another common RFC violation (Content-Length too small)
	if buffered := r.Buffered(); buffered > 0 {
		// Peek to see if this looks like the start of a new HTTP response
		if peek, err := r.Peek(min(buffered, 20)); err == nil {
			// If it starts with "HTTP/", it's a new response (pipelined), don't read it
			if len(peek) >= 5 && string(peek[:5]) == "HTTP/" {
				return nil
			}
			// Otherwise, it might be extra body data. For keep-alive safety,
			// we should NOT read it (it could be the next request/response)
			// This is a trade-off: we might lose some body data, but we preserve
			// connection integrity for keep-alive scenarios
		}
	}

	return nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) readUntilClose(r *bufio.Reader, connectionHeader string, dst, raw *buffer.Buffer) error {
	_, err := io.Copy(io.MultiWriter(dst, raw), r)
	if err != nil && err != io.EOF {
		return errors.NewIOError("reading until close", err)
	}

	return nil
}
