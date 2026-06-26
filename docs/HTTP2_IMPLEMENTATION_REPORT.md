# HTTP/2 Implementation Report for go-rawhttp

**Date**: 2025-09-24  
**Author**: Technical Analysis Team  
**Status**: PENDING APPROVAL

## Executive Summary

This report presents a comprehensive implementation strategy for adding HTTP/2 support to the go-rawhttp library while maintaining its core philosophy of providing raw, low-level HTTP control. The proposed solution enables users to work with HTTP/2 using familiar HTTP/1.1-style syntax (similar to Burp Suite) while also providing direct frame manipulation capabilities for advanced users.

## 1. Implementation Overview

### 1.1 Core Approach

The implementation follows a **dual-mode approach**:

1. **High-Level Mode**: Users write HTTP/1.1-style raw requests that are automatically converted to HTTP/2
2. **Low-Level Mode**: Direct frame manipulation for advanced protocol testing

### 1.2 Key Design Principles

- **Backward Compatibility**: Existing HTTP/1.1 functionality remains unchanged
- **Minimal Dependencies**: Uses only `golang.org/x/net/http2` package
- **Modular Architecture**: Clean separation between HTTP/1.1 and HTTP/2 code
- **Performance**: Efficient frame handling and memory management
- **Simplicity**: Easy migration path for existing users

## 2. Technical Architecture

### 2.1 Package Structure

```
pkg/
├── http2/
│   ├── client.go         # Main HTTP/2 client
│   ├── converter.go      # HTTP/1.1 <-> HTTP/2 conversion
│   ├── frames.go         # Frame construction/parsing
│   ├── hpack.go          # Header compression
│   ├── stream.go         # Stream management
│   ├── transport.go      # Connection handling
│   └── errors.go         # HTTP/2 specific errors
```

### 2.2 Integration Points

```go
// Extend existing Sender interface
type Sender struct {
    httpClient  *client.Client  // Existing HTTP/1.1
    http2Client *http2.Client   // New HTTP/2 client
}

// Unified Do method
func (s *Sender) Do(ctx context.Context, request []byte, opts Options) (*RawResponse, error) {
    if opts.Protocol == "http/2" {
        return s.http2Client.Do(ctx, request, opts)
    }
    return s.httpClient.Do(ctx, request, opts)
}
```

## 3. Implementation Phases

### Phase 1: Core HTTP/2 Support (2 weeks)

**Deliverables:**
- Basic HTTP/2 client implementation
- HTTP/1.1 to HTTP/2 conversion
- HEADERS and DATA frame support
- TLS with ALPN negotiation

**Code Structure:**
```go
// pkg/http2/client.go
type Client struct {
    converter *Converter
    transport *Transport
}

func (c *Client) Do(ctx context.Context, rawReq []byte, opts Options) (*RawResponse, error) {
    // Convert HTTP/1.1 style to HTTP/2 frames
    frames, err := c.converter.TextToFrames(rawReq)
    if err != nil {
        return nil, err
    }
    
    // Establish connection
    conn, err := c.transport.Connect(ctx, opts)
    if err != nil {
        return nil, err
    }
    
    // Send frames and receive response
    return c.sendAndReceive(conn, frames)
}
```

### Phase 2: Advanced Features (1 week)

**Deliverables:**
- Stream multiplexing
- Flow control
- H2C (cleartext HTTP/2) support
- Settings frame negotiation

**Example API:**
```go
// Concurrent requests on single connection
sender := rawhttp.NewHTTP2Sender()
conn := sender.Connect(ctx, opts)

// Send multiple requests concurrently
var wg sync.WaitGroup
for _, req := range requests {
    wg.Add(1)
    go func(r []byte) {
        defer wg.Done()
        resp, _ := conn.SendRequest(ctx, r)
        // Process response
    }(req)
}
wg.Wait()
```

### Phase 3: Raw Frame Interface (1 week)

**Deliverables:**
- Direct frame manipulation API
- Frame inspection tools
- Custom frame types support

**Example Usage:**
```go
// Direct frame manipulation
frameHandler := rawhttp.NewFrameHandler(conn)

// Send custom HEADERS frame
headers := &rawhttp.HeadersFrame{
    StreamID: 1,
    Headers: map[string]string{
        ":method": "GET",
        ":path": "/test",
        ":scheme": "https",
        ":authority": "example.com",
        "x-custom": "value",
    },
    EndStream: true,
}

err := frameHandler.SendFrame(headers)

// Read response frames
for {
    frame, err := frameHandler.ReadFrame()
    if err != nil {
        break
    }
    // Process frame
}
```

### Phase 4: Testing & Documentation (1 week)

**Deliverables:**
- Comprehensive test suite
- Integration tests with popular servers
- Performance benchmarks
- User documentation and examples

## 4. Implementation Details

### 4.1 Conversion Algorithm

```go
type Converter struct {
    encoder *hpack.Encoder
    decoder *hpack.Decoder
}

func (c *Converter) TextToFrames(raw []byte) ([]Frame, error) {
    // 1. Parse HTTP/1.1 request
    req, err := parseHTTP11Request(raw)
    
    // 2. Extract pseudo-headers
    pseudoHeaders := map[string]string{
        ":method": req.Method,
        ":path": req.Path,
        ":scheme": req.Scheme,
        ":authority": req.Host,
    }
    
    // 3. Convert headers to lowercase
    headers := make(map[string]string)
    for k, v := range req.Headers {
        headers[strings.ToLower(k)] = v
    }
    
    // 4. Remove connection-specific headers
    delete(headers, "connection")
    delete(headers, "keep-alive")
    delete(headers, "transfer-encoding")
    
    // 5. Create HEADERS frame
    headerFrame := &HeadersFrame{
        StreamID: 1,
        Headers: mergeHeaders(pseudoHeaders, headers),
        EndHeaders: true,
        EndStream: len(req.Body) == 0,
    }
    
    frames := []Frame{headerFrame}
    
    // 6. Create DATA frames if body exists
    if len(req.Body) > 0 {
        dataFrame := &DataFrame{
            StreamID: 1,
            Data: req.Body,
            EndStream: true,
        }
        frames = append(frames, dataFrame)
    }
    
    return frames, nil
}
```

### 4.2 Connection Management

```go
type Transport struct {
    connections map[string]*HTTP2Connection
    mu          sync.RWMutex
}

func (t *Transport) Connect(ctx context.Context, opts Options) (*HTTP2Connection, error) {
    key := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
    
    // Check for existing connection
    t.mu.RLock()
    if conn, exists := t.connections[key]; exists {
        t.mu.RUnlock()
        return conn, nil
    }
    t.mu.RUnlock()
    
    // Create new connection
    var rawConn net.Conn
    var err error
    
    if opts.Scheme == "https" {
        // TLS with ALPN
        tlsConfig := &tls.Config{
            ServerName: opts.Host,
            NextProtos: []string{"h2"},
        }
        rawConn, err = tls.Dial("tcp", key, tlsConfig)
    } else {
        // H2C (cleartext)
        rawConn, err = net.Dial("tcp", key)
        if err == nil {
            // Send HTTP/2 preface
            _, err = rawConn.Write([]byte("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"))
        }
    }
    
    if err != nil {
        return nil, err
    }
    
    // Create HTTP/2 connection
    conn := &HTTP2Connection{
        conn: rawConn,
        framer: http2.NewFramer(rawConn, rawConn),
        streams: make(map[uint32]*Stream),
    }
    
    // Store connection
    t.mu.Lock()
    t.connections[key] = conn
    t.mu.Unlock()
    
    return conn, nil
}
```

### 4.3 Error Handling

```go
type HTTP2Error struct {
    *errors.Error
    ErrorCode   HTTP2ErrorCode
    StreamID    uint32
    FrameType   http2.FrameType
}

const (
    ErrProtocolError     HTTP2ErrorCode = 0x1
    ErrInternalError     HTTP2ErrorCode = 0x2
    ErrFlowControlError  HTTP2ErrorCode = 0x3
    ErrSettingsTimeout   HTTP2ErrorCode = 0x4
    ErrStreamClosed      HTTP2ErrorCode = 0x5
    ErrFrameSizeError    HTTP2ErrorCode = 0x6
    ErrRefusedStream     HTTP2ErrorCode = 0x7
    ErrCancel            HTTP2ErrorCode = 0x8
    ErrCompressionError  HTTP2ErrorCode = 0x9
    ErrConnectError      HTTP2ErrorCode = 0xa
    ErrEnhanceYourCalm   HTTP2ErrorCode = 0xb
    ErrInadequateSecurity HTTP2ErrorCode = 0xc
    ErrHTTP11Required    HTTP2ErrorCode = 0xd
)
```

## 5. Testing Strategy

### 5.1 Unit Tests

```go
// tests/unit/http2/converter_test.go
func TestHTTP11ToHTTP2Conversion(t *testing.T) {
    raw := []byte("GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n")
    
    converter := NewConverter()
    frames, err := converter.TextToFrames(raw)
    
    assert.NoError(t, err)
    assert.Len(t, frames, 1)
    
    headerFrame := frames[0].(*HeadersFrame)
    assert.Equal(t, "GET", headerFrame.Headers[":method"])
    assert.Equal(t, "/test", headerFrame.Headers[":path"])
    assert.Equal(t, "example.com", headerFrame.Headers[":authority"])
}
```

### 5.2 Integration Tests

```go
// tests/integration/http2_test.go
func TestHTTP2RealServer(t *testing.T) {
    sender := rawhttp.NewSender()
    
    request := []byte(`GET / HTTP/2
Host: http2.golang.org
User-Agent: go-rawhttp/2.0

`)
    
    opts := rawhttp.Options{
        Protocol: "http/2",
        Scheme: "https",
        Host: "http2.golang.org",
        Port: 443,
    }
    
    resp, err := sender.Do(context.Background(), request, opts)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
}
```

## 6. Performance Considerations

### 6.1 Memory Management

- Reuse buffers for frame encoding/decoding
- Implement buffer pools for common operations
- Limit HPACK dynamic table size

### 6.2 Concurrency

- Thread-safe stream management
- Efficient multiplexing with minimal locking
- Connection lifecycle management per request

### 6.3 Benchmarks

```go
func BenchmarkHTTP2Conversion(b *testing.B) {
    converter := NewConverter()
    raw := []byte("GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = converter.TextToFrames(raw)
    }
}
```

## 7. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Protocol Complexity | High | Extensive testing, gradual rollout |
| Performance Regression | Medium | Benchmarking, profiling |
| Breaking Changes | Low | Backward compatibility, versioning |
| Security Vulnerabilities | High | Security review, fuzzing |
| Dependency Issues | Low | Minimal dependencies, vendoring |

## 8. Success Metrics

- **Functionality**: 100% of HTTP/2 core features supported
- **Performance**: < 10% overhead compared to HTTP/1.1
- **Compatibility**: Works with 95% of HTTP/2 servers
- **Usability**: Migration requires < 5 lines of code change
- **Reliability**: 99.9% test coverage

## 9. Timeline

| Week | Phase | Deliverables |
|------|-------|--------------|
| 1-2 | Core HTTP/2 | Basic client, conversion, frames |
| 3 | Advanced | Multiplexing, flow control, H2C |
| 4 | Raw Interface | Frame API, inspection tools |
| 5 | Testing | Tests, benchmarks, documentation |

## 10. Recommendations

### Immediate Actions

1. **Approve core architecture** - Confirm the dual-mode approach
2. **Set up development branch** - `feature/http2-support`
3. **Begin Phase 1 implementation** - Core HTTP/2 functionality

### Long-term Considerations

1. **HTTP/3 Planning** - Consider future QUIC support
2. **Community Feedback** - Beta testing program
3. **Documentation** - Comprehensive guides and examples

## Conclusion

The proposed HTTP/2 implementation:

1. **Maintains Philosophy**: Preserves raw HTTP control
2. **User-Friendly**: Familiar HTTP/1.1-style interface
3. **Powerful**: Full protocol access when needed
4. **Performant**: Efficient implementation
5. **Maintainable**: Clean, modular architecture

This implementation strategy balances ease of use with powerful capabilities, allowing go-rawhttp to support modern HTTP/2 while maintaining its core value proposition of providing complete control over HTTP communication.

## Approval Request

**Requested Action**: Review and approve the HTTP/2 implementation strategy

**Key Decision Points**:
1. Dual-mode approach (high-level + low-level)
2. HTTP/1.1-style syntax for HTTP/2
3. Package structure and architecture
4. 5-week implementation timeline

**Next Steps Upon Approval**:
1. Create feature branch
2. Begin Phase 1 implementation
3. Weekly progress updates
4. Testing and review cycles