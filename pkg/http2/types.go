package http2

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/timing"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// ProxyConfig re-export from client package for HTTP/2 support
type ProxyConfig struct {
	Type         string
	Host         string
	Port         int
	Username     string
	Password     string
	ConnTimeout  time.Duration
	ProxyHeaders map[string]string
	TLSConfig    *tls.Config
	ResolveDNSViaProxy bool
}

// Options contains HTTP/2 specific configuration.
// These settings map to HTTP/2 SETTINGS frame parameters (RFC 7540).
type Options struct {
	// MaxConcurrentStreams limits concurrent streams (SETTINGS_MAX_CONCURRENT_STREAMS)
	MaxConcurrentStreams uint32

	// InitialWindowSize sets flow control window (SETTINGS_INITIAL_WINDOW_SIZE)
	InitialWindowSize uint32

	// MaxFrameSize sets maximum frame payload (SETTINGS_MAX_FRAME_SIZE)
	MaxFrameSize uint32

	// MaxHeaderListSize limits header list size (SETTINGS_MAX_HEADER_LIST_SIZE)
	MaxHeaderListSize uint32

	// HeaderTableSize sets HPACK table size (SETTINGS_HEADER_TABLE_SIZE)
	HeaderTableSize uint32

	// DisableServerPush disables server push (SETTINGS_ENABLE_PUSH = 0)
	DisableServerPush bool

	// EnableCompression enables HPACK header compression
	EnableCompression bool

	// Multiplexing options
	EnableMultiplexing bool
	ReuseConnection    bool

	// TLS configuration
	// InsecureTLS skips TLS certificate verification (for testing/development).
	// IMPORTANT (DEF-13): This flag ALWAYS overrides TLSConfig.InsecureSkipVerify,
	// even when custom TLSConfig is provided. This is intentional to support proxy
	// MITM scenarios where you need custom TLS settings AND disabled verification.
	// Example: InsecureTLS=true + custom TLSConfig → verification is DISABLED.
	InsecureTLS bool

	// TLSConfig provides custom TLS configuration for fine-grained control.
	// When set, this config is cloned and used instead of defaults.
	// Note: InsecureTLS flag will override InsecureSkipVerify if set to true.
	TLSConfig *tls.Config

	// Client certificate for mutual TLS (mTLS authentication)
	// Option 1: Provide PEM-encoded certificate and key directly
	ClientCertPEM []byte // Client certificate in PEM format
	ClientKeyPEM  []byte // Client private key in PEM format (unencrypted)

	// Option 2: Provide file paths (will be loaded automatically)
	ClientCertFile string // Path to client certificate file (.crt, .pem)
	ClientKeyFile  string // Path to client private key file (.key, .pem)

	// SSL/TLS Protocol Version Control (v1.2.0+)
	MinTLSVersion    uint16                   // Minimum SSL/TLS version
	MaxTLSVersion    uint16                   // Maximum SSL/TLS version
	TLSRenegotiation tls.RenegotiationSupport // TLS renegotiation support
	CipherSuites     []uint16                 // Allowed cipher suites

	// SNI specifies custom Server Name Indication for TLS handshake.
	// Priority: TLSConfig.ServerName > SNI > Host (if DisableSNI is false)
	SNI string

	// DisableSNI completely disables SNI extension in TLS handshake.
	// Cannot be used together with SNI option (validation error).
	DisableSNI bool

	// Proxy specifies upstream proxy configuration for HTTP/2 connections.
	// When set, HTTP/2 will use HTTP CONNECT tunnel through the proxy.
	// All proxy types supported: http, https, socks4, socks5
	Proxy *ProxyConfig

	// Priority settings
	Priority *PriorityParam

	// Debug contains HTTP/2 debugging flags (optional, all default to false).
	// These flags enable detailed logging of HTTP/2 protocol operations.
	// Production safe - explicit opt-in with zero overhead when disabled.
	Debug struct {
		LogFrames   bool // Log all HTTP/2 frames
		LogSettings bool // Log SETTINGS frames
		LogHeaders  bool // Log HEADERS frames
		LogData     bool // Log DATA frames
	}

	// Deprecated: Use Debug.LogFrames instead
	ShowFrameDetails bool
	// Deprecated: Use Debug.LogFrames instead
	TraceFrames bool

	// Deprecated: Use DisableServerPush instead (inverted logic)
	EnableServerPush bool
}

// PriorityParam represents stream priority settings
type PriorityParam struct {
	StreamDependency uint32
	Exclusive        bool
	Weight           uint8
}

// Frame represents a generic HTTP/2 frame
type Frame interface {
	Type() http2.FrameType
	StreamID() uint32
	Flags() http2.Flags
	Payload() []byte
}

// HeadersFrame represents an HTTP/2 HEADERS frame
type HeadersFrame struct {
	StreamId   uint32
	Headers    map[string]string
	EndStream  bool
	EndHeaders bool
	Priority   *PriorityParam
	PadLength  uint8
}

func (f *HeadersFrame) Type() http2.FrameType { return http2.FrameHeaders }
func (f *HeadersFrame) StreamID() uint32      { return f.StreamId }
func (f *HeadersFrame) Flags() http2.Flags {
	var flags http2.Flags
	if f.EndStream {
		flags |= http2.FlagHeadersEndStream
	}
	if f.EndHeaders {
		flags |= http2.FlagHeadersEndHeaders
	}
	if f.Priority != nil {
		flags |= http2.FlagHeadersPriority
	}
	if f.PadLength > 0 {
		flags |= http2.FlagHeadersPadded
	}
	return flags
}
func (f *HeadersFrame) Payload() []byte { return nil } // Handled by HPACK encoder

// DataFrame represents an HTTP/2 DATA frame
type DataFrame struct {
	StreamId  uint32
	Data      []byte
	EndStream bool
	PadLength uint8
}

func (f *DataFrame) Type() http2.FrameType { return http2.FrameData }
func (f *DataFrame) StreamID() uint32      { return f.StreamId }
func (f *DataFrame) Flags() http2.Flags {
	var flags http2.Flags
	if f.EndStream {
		flags |= http2.FlagDataEndStream
	}
	if f.PadLength > 0 {
		flags |= http2.FlagDataPadded
	}
	return flags
}
func (f *DataFrame) Payload() []byte { return f.Data }

// Stream represents an HTTP/2 stream
type Stream struct {
	ID              uint32
	State           StreamState
	Request         *Request
	Response        *Response
	WindowSize      int32
	PeerWindowSize  int32
	Priority        *PriorityParam
	HeadersReceived bool
	DataReceived    bool
	Closed          bool
}

// StreamState represents the state of an HTTP/2 stream
type StreamState int

const (
	StateIdle StreamState = iota
	StateReservedLocal
	StateReservedRemote
	StateOpen
	StateHalfClosedLocal
	StateHalfClosedRemote
	StateClosed
)

// Request represents an HTTP/2 request
type Request struct {
	Method    string
	Path      string
	Authority string
	Scheme    string
	Headers   map[string]string
	Body      []byte
	RawText   []byte // Original HTTP/1.1-style raw request
}

// Response represents an HTTP/2 response
type Response struct {
	Status      int
	StatusText  string
	Headers     map[string][]string
	Body        []byte
	Frames      []Frame
	RawFrames   [][]byte
	HTTPVersion string

	// HTTP/2 specific metadata
	StreamID   uint32
	ServerPush []*PushPromise
	HPACKStats *HPACKStats
	FrameStats *FrameStats

	// Timing information
	TotalTime time.Duration // Total time taken for the request
	Metrics   *timing.Metrics // Detailed timing metrics (DNS, TCP, TLS, TTFB)

	// Connection metadata (added for compatibility with HTTP/1.1 Response)
	ConnectedIP        string // Actual IP address connected to
	ConnectedPort      int    // Actual port connected to
	NegotiatedProtocol string // Negotiated protocol via ALPN (e.g., "h2")
	TLSVersion         string // TLS version used (e.g., "TLS 1.3")
	TLSCipherSuite     string // TLS cipher suite used
	TLSServerName      string // TLS Server Name (SNI)
	ConnectionReused   bool   // Whether connection was reused from pool

	// Proxy metadata (added for consistency with HTTP/1.1)
	ProxyUsed bool   // Whether an upstream proxy was used
	ProxyType string // Proxy type (http, https, socks4, socks5)
	ProxyAddr string // Proxy server address
}

// PushPromise represents a server push promise
type PushPromise struct {
	PromisedStreamID uint32
	Headers          map[string]string
	Response         *Response
}

// HPACKStats contains HPACK compression statistics
type HPACKStats struct {
	CompressedSize   int
	UncompressedSize int
	TableSize        int
	TableEntries     int
}

// FrameStats contains frame-level statistics
type FrameStats struct {
	FramesSent     int
	FramesReceived int
	BytesSent      int
	BytesReceived  int
	StreamsOpened  int
	StreamsClosed  int
}

// ConnectionPoolStats contains HTTP/2 connection pool statistics (DEF-5)
type ConnectionPoolStats struct {
	// Active connections currently in the pool
	ActiveConnections int

	// Total number of streams across all connections
	TotalStreams int

	// Connection details (map of address to connection stats)
	Connections map[string]ConnectionStats
}

// ConnectionStats contains statistics for a single HTTP/2 connection
type ConnectionStats struct {
	Address        string    // Connection address (host:port)
	StreamsActive  int       // Currently active streams on this connection
	StreamsTotal   int       // Total streams created on this connection
	LastActivity   time.Time // Last activity timestamp
	Ready          bool      // True if connection is ready for use
}

// Connection represents an HTTP/2 connection
type Connection struct {
	Conn           net.Conn // Underlying network connection
	Framer         *http2.Framer
	Encoder        *hpack.Encoder
	EncoderBuf     *bytes.Buffer // Buffer used by the encoder
	Decoder        *hpack.Decoder
	Streams        map[uint32]*Stream
	NextStreamID   uint32
	MaxConcurrent  uint32
	WindowSize     int32
	PeerWindowSize int32
	Settings       map[http2.SettingID]uint32
	PeerSettings   map[http2.SettingID]uint32
	Closed         bool
	Ready          bool         // True when SETTINGS handshake is complete
	LastActivity   time.Time    // For idle timeout tracking
	PoolKey        string       // Pool key for this connection (v2.0.3+)
	Reused         bool         // True if connection was reused from pool (v2.0.3+)
	mu             sync.RWMutex // Protects connection state
}

// Close properly closes the HTTP/2 connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Closed {
		return nil
	}

	c.Closed = true

	// Send GOAWAY frame if framer is available
	if c.Framer != nil {
		c.Framer.WriteGoAway(0, http2.ErrCodeNo, nil)
	}

	// Close the underlying connection
	if c.Conn != nil {
		return c.Conn.Close()
	}

	return nil
}

// DefaultOptions returns default HTTP/2 options (aligned with Go's native HTTP/2).
// All SETTINGS values are set to recommended defaults per RFC 7540.
func DefaultOptions() *Options {
	opts := &Options{
		MaxConcurrentStreams: 100,
		InitialWindowSize:    4194304,  // 4MB (same as Go's HTTP/2)
		MaxFrameSize:         16384,    // 16KB (RFC 7540 default)
		MaxHeaderListSize:    10485760, // 10MB (same as Go's HTTP/2)
		HeaderTableSize:      4096,     // 4KB (RFC 7540 default)
		DisableServerPush:    true,     // Disabled by default for security
		EnableCompression:    true,
		EnableMultiplexing:   false,
		ReuseConnection:      false,
		ShowFrameDetails:     false, // Deprecated
		TraceFrames:          false, // Deprecated
		EnableServerPush:     false, // Deprecated: kept for backward compatibility
	}
	// Debug flags default to false (zero values)
	opts.Debug.LogFrames = false
	opts.Debug.LogSettings = false
	opts.Debug.LogHeaders = false
	opts.Debug.LogData = false
	return opts
}

// ValidateOptions validates HTTP/2 options for RFC 7540 compliance (DEF-9)
func ValidateOptions(opts *Options) error {
	if opts == nil {
		return nil // nil options are OK, defaults will be used
	}

	// Validate MaxFrameSize (RFC 7540 Section 6.5.2)
	// MUST be between 16384 (2^14) and 16777215 (2^24-1)
	if opts.MaxFrameSize > 0 && (opts.MaxFrameSize < 16384 || opts.MaxFrameSize > 16777215) {
		return fmt.Errorf("MaxFrameSize must be between 16384 and 16777215 (RFC 7540), got %d", opts.MaxFrameSize)
	}

	// Validate InitialWindowSize (RFC 7540 Section 6.5.2)
	// MUST NOT exceed 2^31-1
	if opts.InitialWindowSize > (1<<31 - 1) {
		return fmt.Errorf("InitialWindowSize must not exceed 2147483647 (2^31-1), got %d", opts.InitialWindowSize)
	}

	return nil
}
