# HTTP/2 Raw Request Handling Documentation

## Overview

This document describes how to extend the go-rawhttp library to support HTTP/2 protocol while maintaining the same raw request editing capabilities similar to Burp Suite's approach.

## 1. HTTP/2 Protocol Fundamentals

### 1.1 Key Differences from HTTP/1.1

HTTP/2 is a binary protocol that operates fundamentally differently from HTTP/1.1:

- **Binary Framing**: All communication occurs through binary frames, not text
- **Multiplexing**: Multiple requests/responses can be interleaved on a single connection
- **Header Compression**: Uses HPACK algorithm to compress headers
- **Pseudo-Headers**: Special headers prefixed with `:` replace the request line

### 1.2 HTTP/2 Frame Structure

```
+-----------------------------------------------+
|                 Length (24 bits)              |
+---------------+---------------+---------------+
|   Type (8)    |   Flags (8)   |
+-+-------------+---------------+---------------+
|R|         Stream Identifier (31 bits)         |
+=+=============================================================+
|                   Frame Payload (0...)                       |
+---------------------------------------------------------------+
```

### 1.3 Pseudo-Headers

HTTP/2 replaces the HTTP/1.1 request line with pseudo-headers:

```
:method = GET
:path = /resource
:scheme = https
:authority = example.com
```

## 2. Burp Suite's Approach to HTTP/2

Burp Suite handles HTTP/2 in a user-friendly way:

1. **Display Format**: Shows HTTP/2 requests in HTTP/1.1-style format for familiarity
2. **Editing**: Users edit requests as if they were HTTP/1.1
3. **Conversion**: Automatically converts between formats when sending
4. **Visibility**: Optionally shows actual HTTP/2 frames for advanced users

### Example Burp Suite Representation:

**User sees (HTTP/1.1 style):**
```http
GET /api/data HTTP/2
Host: api.example.com
User-Agent: Mozilla/5.0
Accept: application/json
```

**Actually sent as HTTP/2:**
```
HEADERS frame:
  :method = GET
  :path = /api/data
  :scheme = https
  :authority = api.example.com
  user-agent = Mozilla/5.0
  accept = application/json
```

## 3. Proposed Implementation Architecture

### 3.1 Module Structure

```
pkg/
├── http2/
│   ├── client.go        # HTTP/2 client implementation
│   ├── converter.go     # HTTP/1.1 <-> HTTP/2 conversion
│   ├── frames.go        # Frame construction and parsing
│   ├── hpack.go         # HPACK compression/decompression
│   ├── stream.go        # Stream management
│   └── transport.go     # HTTP/2 transport layer
```

### 3.2 Core Components

#### 3.2.1 Converter Component

Handles bidirectional conversion between HTTP/1.1-style raw text and HTTP/2 frames:

```go
type HTTP2Converter struct {
    hpackEncoder *hpack.Encoder
    hpackDecoder *hpack.Decoder
}

// Convert HTTP/1.1-style raw request to HTTP/2 frames
func (c *HTTP2Converter) TextToFrames(rawRequest []byte) ([]Frame, error)

// Convert HTTP/2 frames to HTTP/1.1-style representation
func (c *HTTP2Converter) FramesToText(frames []Frame) ([]byte, error)
```

#### 3.2.2 Raw Frame Handler

Direct frame manipulation capabilities:

```go
type RawFrameHandler struct {
    framer *http2.Framer
}

// Send raw frames
func (h *RawFrameHandler) SendRawFrames(frames []Frame) error

// Receive and parse frames
func (h *RawFrameHandler) ReceiveFrames() ([]Frame, error)
```

#### 3.2.3 Stream Manager

Manages HTTP/2 stream lifecycle:

```go
type StreamManager struct {
    streams   map[uint32]*Stream
    nextID    uint32
    maxConcurrent int
}

// Create new stream for request
func (m *StreamManager) NewStream() *Stream

// Handle stream flow control
func (m *StreamManager) UpdateWindow(streamID uint32, increment int32)
```

### 3.3 API Design

Extend existing rawhttp API to support HTTP/2:

```go
type Options struct {
    // Existing fields...
    
    // HTTP/2 specific options
    Protocol           string  // "http/1.1" or "http/2"
    HTTP2Settings      *HTTP2Settings
    EnableServerPush   bool
    MaxConcurrentStreams int
}

type HTTP2Settings struct {
    HeaderTableSize      uint32
    EnablePush           bool
    MaxConcurrentStreams uint32
    InitialWindowSize    uint32
    MaxFrameSize         uint32
    MaxHeaderListSize    uint32
}
```

## 4. Conversion Algorithm

### 4.1 HTTP/1.1 to HTTP/2 Conversion

```
Input: HTTP/1.1-style raw request
Output: HTTP/2 frames

1. Parse request line to extract method, path, version
2. Extract Host header for :authority
3. Determine :scheme from connection type
4. Create pseudo-headers:
   - :method = <method>
   - :path = <path>
   - :scheme = <scheme>
   - :authority = <host>
5. Convert remaining headers to lowercase
6. Remove connection-specific headers (Connection, Keep-Alive, etc.)
7. Compress headers using HPACK
8. Create HEADERS frame with compressed headers
9. If body exists, create DATA frame(s)
10. Set appropriate flags (END_STREAM, END_HEADERS)
```

### 4.2 HTTP/2 to HTTP/1.1 Conversion

```
Input: HTTP/2 frames
Output: HTTP/1.1-style representation

1. Decompress headers using HPACK
2. Extract pseudo-headers
3. Construct request/status line from pseudo-headers
4. Format regular headers
5. Combine request line and headers
6. Append body from DATA frames
7. Format as HTTP/1.1-style text
```

## 5. Implementation Examples

### 5.1 Raw HTTP/2 Request Example

```go
// User provides HTTP/1.1-style raw request
rawRequest := []byte(`GET /api/users HTTP/2
Host: api.example.com
Authorization: Bearer token123
Accept: application/json

`)

// Create HTTP/2 sender
sender := rawhttp.NewHTTP2Sender()

// Options for HTTP/2
opts := rawhttp.Options{
    Protocol: "http/2",
    Scheme:   "https",
    Host:     "api.example.com",
    Port:     443,
    HTTP2Settings: &rawhttp.HTTP2Settings{
        MaxConcurrentStreams: 100,
        InitialWindowSize:    65535,
    },
}

// Send request (automatic conversion to HTTP/2)
resp, err := sender.Do(ctx, rawRequest, opts)
```

### 5.2 Direct Frame Manipulation

```go
// Advanced users can work with raw frames
frames := []rawhttp.Frame{
    &rawhttp.HeadersFrame{
        StreamID: 1,
        Headers: map[string]string{
            ":method": "GET",
            ":path": "/api/data",
            ":scheme": "https",
            ":authority": "api.example.com",
        },
        EndStream: false,
        EndHeaders: true,
    },
    &rawhttp.DataFrame{
        StreamID: 1,
        Data: []byte("request body"),
        EndStream: true,
    },
}

resp, err := sender.SendFrames(ctx, frames, opts)
```

## 6. Connection Management

### 6.1 H2C (HTTP/2 Cleartext)

For non-TLS HTTP/2 connections:

```go
// HTTP/1.1 Upgrade method
GET / HTTP/1.1
Host: example.com
Connection: Upgrade, HTTP2-Settings
Upgrade: h2c
HTTP2-Settings: <base64url encoding of HTTP/2 SETTINGS>

// Prior knowledge method
// Send HTTP/2 connection preface directly
"PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
```

### 6.2 ALPN Negotiation (TLS)

For TLS connections:

```go
tlsConfig := &tls.Config{
    NextProtos: []string{"h2", "http/1.1"},
    // Other TLS settings...
}
```

## 7. Error Handling

HTTP/2 specific errors:

```go
type HTTP2Error struct {
    Type      HTTP2ErrorType
    Code      uint32  // HTTP/2 error code
    StreamID  uint32
    Message   string
}

type HTTP2ErrorType int

const (
    ErrorTypeProtocol HTTP2ErrorType = iota
    ErrorTypeStream
    ErrorTypeCompression
    ErrorTypeFlowControl
    ErrorTypeSettings
    ErrorTypeFrameSize
)
```

## 8. Performance Considerations

1. **Connection Reuse**: HTTP/2 encourages single connection per host
2. **Stream Multiplexing**: Handle multiple concurrent requests
3. **Header Compression**: Maintain HPACK dynamic table
4. **Flow Control**: Respect window sizes for streams and connection
5. **Priority**: Optional stream prioritization

## 9. Testing Strategy

### 9.1 Unit Tests

- Frame construction/parsing
- HPACK compression/decompression
- Conversion algorithms
- Stream state management

### 9.2 Integration Tests

- Real HTTP/2 server communication
- H2C upgrade scenarios
- ALPN negotiation
- Error conditions
- Flow control scenarios

### 9.3 Compatibility Tests

- Test against popular HTTP/2 servers (nginx, Apache, Caddy)
- Verify Burp Suite-style conversion accuracy
- Edge cases in header handling

## 10. Migration Path

For existing rawhttp users:

1. **Backward Compatibility**: HTTP/1.1 remains default
2. **Opt-in HTTP/2**: Explicitly set `Protocol: "http/2"`
3. **Gradual Adoption**: Start with simple GET requests
4. **Advanced Features**: Add streaming, server push later

## 11. Known Limitations

1. **Protocol Complexity**: HTTP/2 is significantly more complex than HTTP/1.1
2. **Debugging**: Binary protocol is harder to debug than text
3. **Middleboxes**: Some proxies/firewalls may not support HTTP/2
4. **Resource Usage**: Maintains state for streams and HPACK tables

## 12. Security Considerations

1. **HPACK Bombing**: Limit dynamic table size
2. **Stream Flooding**: Limit concurrent streams
3. **Flow Control Abuse**: Enforce window limits
4. **Header Size**: Limit maximum header size
5. **Connection Reuse**: Consider security implications

## Conclusion

This implementation approach provides:

1. **User-Friendly Interface**: HTTP/1.1-style editing for HTTP/2
2. **Full Control**: Direct frame manipulation when needed
3. **Compatibility**: Works with standard HTTP/2 servers
4. **Performance**: Leverages HTTP/2 benefits
5. **Flexibility**: Supports both high-level and low-level operations

The design maintains the rawhttp philosophy of providing complete control over HTTP communication while making HTTP/2 accessible to users familiar with HTTP/1.1.