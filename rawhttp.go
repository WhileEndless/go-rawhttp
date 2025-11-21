// Package rawhttp provides a high-performance, low-level HTTP client library
// for Go that supports both HTTP/1.1 and HTTP/2 protocols with raw socket-based
// communication and comprehensive features for fine-grained control.
package rawhttp

import (
	"context"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/buffer"
	"github.com/WhileEndless/go-rawhttp/pkg/client"
	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/http2"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
	"github.com/WhileEndless/go-rawhttp/pkg/transport"
)

// Version is the current version of the rawhttp library
const Version = "1.2.0"

// GetVersion returns the current version of the library
func GetVersion() string {
	return Version
}

// Re-export key types for easier usage
type (
	// Options controls how the Sender establishes connections and reads responses.
	Options = client.Options

	// Response represents a parsed HTTP response.
	Response = client.Response

	// Buffer provides memory-efficient storage with disk spilling.
	Buffer = buffer.Buffer

	// Metrics captures detailed timing information for a request.
	Metrics = timing.Metrics

	// Error represents a structured error with context information.
	Error = errors.Error

	// TransportError is an alias for Error (transport error naming convention).
	TransportError = errors.TransportError

	// HTTP2Settings contains HTTP/2 specific configuration
	HTTP2Settings = http2.Options

	// PoolStats provides connection pool statistics
	PoolStats = transport.PoolStats
)

// Re-export error types for convenience
const (
	ErrorTypeDNS        = errors.ErrorTypeDNS
	ErrorTypeConnection = errors.ErrorTypeConnection
	ErrorTypeTLS        = errors.ErrorTypeTLS
	ErrorTypeTimeout    = errors.ErrorTypeTimeout
	ErrorTypeProtocol   = errors.ErrorTypeProtocol
	ErrorTypeIO         = errors.ErrorTypeIO
	ErrorTypeValidation = errors.ErrorTypeValidation
)

// Sender implements raw HTTP transport for both HTTP/1.1 and HTTP/2.
type Sender struct {
	client      *client.Client
	http2Client *http2.Client
}

// NewSender returns a new Sender instance with HTTP/1.1 and HTTP/2 support.
func NewSender() *Sender {
	return &Sender{
		client:      client.New(),
		http2Client: http2.NewClient(nil),
	}
}

// PoolStats returns HTTP/1.1 connection pool statistics.
// Note: HTTP/2 connection pooling uses separate mechanisms.
func (s *Sender) PoolStats() PoolStats {
	return s.client.PoolStats()
}

// Do executes the HTTP request using raw sockets.
// Automatically detects protocol from request or options.
func (s *Sender) Do(ctx context.Context, req []byte, opts Options) (*Response, error) {
	// Detect protocol from request line or options
	protocol := s.detectProtocol(req, opts)

	if protocol == "http/2" {
		// Convert client.Options to http2.Options
		http2Opts := s.convertToHTTP2Options(opts)

		// Use HTTP/2 client with options
		resp, err := s.http2Client.DoWithOptions(ctx, req, opts.Host, opts.Port, opts.Scheme, http2Opts)
		if err != nil {
			// DEF-15: Automatic fallback to HTTP/1.1 if server doesn't support HTTP/2
			// Check if error is due to ALPN negotiation failure
			if strings.Contains(err.Error(), "does not support HTTP/2") {
				// Fallback to HTTP/1.1 automatically
				return s.client.Do(ctx, req, opts)
			}
			return nil, err
		}

		// Convert HTTP/2 response to common Response format
		return s.convertHTTP2Response(resp), nil
	}

	// Use HTTP/1.1 client (default)
	return s.client.Do(ctx, req, opts)
}

// detectProtocol determines whether to use HTTP/1.1 or HTTP/2
func (s *Sender) detectProtocol(req []byte, opts Options) string {
	// Check options first
	if opts.Protocol != "" {
		return strings.ToLower(opts.Protocol)
	}

	// Check request line for HTTP/2 indicator
	reqStr := string(req)
	if strings.Contains(reqStr, "HTTP/2") {
		return "http/2"
	}

	// Default to HTTP/1.1
	return "http/1.1"
}

// convertToHTTP2Options converts client.Options to http2.Options
func (s *Sender) convertToHTTP2Options(opts Options) *http2.Options {
	var h2opts *http2.Options

	if opts.HTTP2Settings == nil {
		// Create default HTTP/2 settings
		h2opts = http2.DefaultOptions()
	} else {
		// Convert client.HTTP2Settings to http2.Options
		// Copy field by field to maintain compatibility
		h2opts = &http2.Options{
			MaxConcurrentStreams: opts.HTTP2Settings.MaxConcurrentStreams,
			InitialWindowSize:    opts.HTTP2Settings.InitialWindowSize,
			MaxFrameSize:         opts.HTTP2Settings.MaxFrameSize,
			MaxHeaderListSize:    opts.HTTP2Settings.MaxHeaderListSize,
			HeaderTableSize:      opts.HTTP2Settings.HeaderTableSize,
			DisableServerPush:    opts.HTTP2Settings.DisableServerPush,
			EnableCompression:    opts.HTTP2Settings.EnableCompression,
		}
		// Copy Debug fields manually due to different struct tags
		h2opts.Debug.LogFrames = opts.HTTP2Settings.Debug.LogFrames
		h2opts.Debug.LogSettings = opts.HTTP2Settings.Debug.LogSettings
		h2opts.Debug.LogHeaders = opts.HTTP2Settings.Debug.LogHeaders
		h2opts.Debug.LogData = opts.HTTP2Settings.Debug.LogData
	}

	// Always override TLS settings from main options
	// This ensures TLS configuration is passed through to HTTP/2
	h2opts.InsecureTLS = opts.InsecureTLS
	h2opts.TLSConfig = opts.TLSConfig
	h2opts.SNI = opts.SNI
	h2opts.DisableSNI = opts.DisableSNI

	// Pass client certificate configuration for mTLS
	h2opts.ClientCertPEM = opts.ClientCertPEM
	h2opts.ClientKeyPEM = opts.ClientKeyPEM
	h2opts.ClientCertFile = opts.ClientCertFile
	h2opts.ClientKeyFile = opts.ClientKeyFile

	// Pass SSL/TLS version control and cipher suites (v1.2.0+)
	h2opts.MinTLSVersion = opts.MinTLSVersion
	h2opts.MaxTLSVersion = opts.MaxTLSVersion
	h2opts.TLSRenegotiation = opts.TLSRenegotiation
	h2opts.CipherSuites = opts.CipherSuites

	return h2opts
}

// convertHTTP2Response converts HTTP/2 response to common Response format
func (s *Sender) convertHTTP2Response(resp *http2.Response) *Response {
	// Create buffer for raw response
	rawBuf := buffer.New(10 * 1024 * 1024) // 10MB default

	// Format response as HTTP/1.1-style text
	rawText := s.http2Client.FormatResponse(resp)
	rawBuf.Write(rawText)

	// Convert headers to HTTP/1.1 format
	headers := make(map[string][]string)
	for k, v := range resp.Headers {
		headers[k] = v
	}

	return &Response{
		StatusCode:  resp.Status,
		Headers:     headers,
		Body:        buffer.NewWithData(resp.Body),
		Raw:         rawBuf,
		HTTPVersion: resp.HTTPVersion,
		Metrics: &timing.Metrics{
			Total: resp.TotalTime, // Use actual timing from HTTP/2 client
		},
	}
}

// NewBuffer creates a new buffer with the specified memory limit.
func NewBuffer(limit int64) *Buffer {
	return buffer.New(limit)
}

// IsTimeoutError checks if an error is a timeout error.
func IsTimeoutError(err error) bool {
	return errors.IsTimeoutError(err)
}

// IsTemporaryError checks if an error is temporary.
func IsTemporaryError(err error) bool {
	return errors.IsTemporaryError(err)
}

// GetErrorType returns the error type if it's a structured error.
func GetErrorType(err error) string {
	return string(errors.GetErrorType(err))
}

// DefaultOptions returns default options for common use cases.
func DefaultOptions(scheme, host string, port int) Options {
	return Options{
		Scheme:      scheme,
		Host:        host,
		Port:        port,
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
	}
}
