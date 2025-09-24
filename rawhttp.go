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
)

// Version is the current version of the rawhttp library
const Version = "1.0.0"

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

	// HTTP2Settings contains HTTP/2 specific configuration
	HTTP2Settings = client.HTTP2Settings
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

// Do executes the HTTP request using raw sockets.
// Automatically detects protocol from request or options.
func (s *Sender) Do(ctx context.Context, req []byte, opts Options) (*Response, error) {
	// Detect protocol from request line or options
	protocol := s.detectProtocol(req, opts)

	if protocol == "http/2" {
		// Use HTTP/2 client
		resp, err := s.http2Client.Do(ctx, req, opts.Host, opts.Port, opts.Scheme)
		if err != nil {
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
			Total: time.Since(time.Now()), // Simplified for now
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
