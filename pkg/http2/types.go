package http2

import (
	"bytes"
	"net"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// Options contains HTTP/2 specific configuration
type Options struct {
	// Connection settings
	EnableServerPush     bool
	EnableCompression    bool
	MaxConcurrentStreams uint32
	InitialWindowSize    uint32
	MaxFrameSize         uint32
	MaxHeaderListSize    uint32
	HeaderTableSize      uint32

	// Multiplexing options
	EnableMultiplexing bool
	ReuseConnection    bool

	// Priority settings
	Priority *PriorityParam

	// Debug options
	ShowFrameDetails bool
	TraceFrames      bool
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

// DefaultOptions returns default HTTP/2 options (aligned with Go's native HTTP/2)
func DefaultOptions() *Options {
	return &Options{
		EnableServerPush:     false,
		EnableCompression:    true,
		MaxConcurrentStreams: 100,
		InitialWindowSize:    4194304,  // 4MB (same as Go's HTTP/2)
		MaxFrameSize:         16384,    // 16KB (standard)
		MaxHeaderListSize:    10485760, // 10MB (same as Go's HTTP/2)
		HeaderTableSize:      4096,     // 4KB (standard)
		EnableMultiplexing:   false,
		ReuseConnection:      false,
		ShowFrameDetails:     false,
		TraceFrames:          false,
	}
}
