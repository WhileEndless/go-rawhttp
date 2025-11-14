# go-rawhttp

[![Version](https://img.shields.io/badge/version-1.1.2-blue.svg)](https://github.com/WhileEndless/go-rawhttp)
[![Go](https://img.shields.io/badge/go-1.19+-00ADD8.svg)](https://golang.org/)

A high-performance, modular HTTP client library for Go that provides raw socket-based HTTP communication with support for both HTTP/1.1 and HTTP/2 protocols, offering comprehensive features and fine-grained control.

## Features

### 🚀 Protocol Support
✅ **HTTP/1.1 & HTTP/2 Support** - Full support for both protocols with seamless switching  
✅ **Raw Request Editing** - Write requests in familiar HTTP/1.1 format, even for HTTP/2  
✅ **H2C Support** - HTTP/2 over cleartext connections with upgrade mechanism  
✅ **ALPN Negotiation** - Automatic HTTP/2 detection via ALPN in TLS handshake  

### ⚡ HTTP/2 Advanced Features  
✅ **Stream Multiplexing** - Multiple requests on single connection (when enabled)  
✅ **HPACK Compression** - Header compression with dynamic table management  
✅ **Flow Control** - RFC 7540 compliant window updates and flow management  
  
✅ **Server Push** - HTTP/2 server push support (configurable)  
✅ **Priority Handling** - Stream priority and dependency management  

### 🛡️ Production Ready
✅ **Memory Efficient** - No memory leaks, automatic cleanup, disk spilling for large responses
✅ **Connection Management** - Proper resource cleanup, health monitoring, idle timeouts
✅ **Connection Pooling** - Keep-Alive support with automatic connection reuse and observability
✅ **Proxy Support** - HTTP, HTTPS, and SOCKS5 upstream proxy support with authentication
✅ **Custom TLS** - Direct TLS config passthrough for full control (TLS versions, cipher suites, client certs)
✅ **Connection Metadata** - Detailed socket-level and TLS session info (addresses, session IDs, resumption)
✅ **Error Recovery** - Structured error classification with operation tracking for smart retry logic
✅ **Performance Monitoring** - Standardized DNS, TCP, TLS, and TTFB timing measurements
✅ **Thread Safety** - Concurrent request handling with proper synchronization

### 🔧 Developer Experience
✅ **Multiple Transfer Encodings** - Chunked encoding, Content-Length, and connection-close handling  
✅ **Structured Error Handling** - Rich error types with context information  
✅ **Modular Architecture** - Clean separation between protocols and components  
✅ **Comprehensive Testing** - Unit, integration, and production readiness tests  
✅ **Minimal Dependencies** - Uses only Go standard library and golang.org/x/net/http2  

## Installation

```bash
go get github.com/WhileEndless/go-rawhttp
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/WhileEndless/go-rawhttp"
)

func main() {
    // Create a new sender
    sender := rawhttp.NewSender()
    
    // Prepare raw HTTP request
    request := []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")
    
    // Use default options
    opts := rawhttp.DefaultOptions("https", "example.com", 443)
    
    // Send request
    resp, err := sender.Do(context.Background(), request, opts)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    defer resp.Raw.Close()
    
    fmt.Printf("Status: %d\n", resp.StatusCode)
    fmt.Printf("Body Size: %d bytes\n", resp.BodyBytes)
    fmt.Printf("Timings: %s\n", resp.Timings.String())
}
```

## Recent Enhancements

### v1.1.2 (2025-11-14)

🔴 **CRITICAL BUG FIX** - Resource leak and nil pointer dereference in TLS handshake

#### Fixed
- **TLS Handshake Resource Leak**: Fixed critical bug where failed TLS handshakes didn't close the underlying TCP connection, causing file descriptor leaks
- **Nil Pointer Dereference**: Fixed panic when TLS upgrade fails and connection cleanup is attempted
- **HTTP/2 ALPN Cleanup**: Fixed incorrect connection closure in HTTP/2 ALPN negotiation failure path
- **Defensive Error Handling**: Added nil checks in error paths to prevent panics

#### Impact
- Prevents application crashes from TLS handshake failures
- Eliminates resource leaks (file descriptors, memory)
- Improves stability when connecting to servers with certificate issues
- Mitigates potential DoS vulnerability from repeated TLS failures

#### Technical Details
See commit history for detailed analysis and test cases covering:
- TLS handshake timeout scenarios
- Certificate validation failures
- SNI issues and protocol negotiation failures
- Connection reset during handshake
- Context cancellation during TLS handshake

All existing functionality remains unchanged. This is a critical stability update recommended for all users.

---

### v1.1.1 (2025-11-14)

#### HTTP/2 Debug Flags ⭐⭐

Optional debugging for HTTP/2 protocol issues:

```go
opts := rawhttp.Options{
    Protocol: "http/2",
    HTTP2Settings: &rawhttp.HTTP2Settings{
        // Production settings...
        MaxConcurrentStreams: 100,
        InitialWindowSize:    4194304,
    },
}

// Enable selective debugging (explicit opt-in)
opts.HTTP2Settings.Debug.LogFrames = true    // Log all frames
opts.HTTP2Settings.Debug.LogSettings = true  // Log SETTINGS frames
opts.HTTP2Settings.Debug.LogHeaders = false  // Don't log HEADERS
opts.HTTP2Settings.Debug.LogData = false     // Don't log DATA

resp, _ := sender.Do(ctx, req, opts)
```

**Features:**
- ✅ **Zero overhead when disabled** - All flags default to false
- ✅ **Production safe** - Explicit opt-in required
- ✅ **Selective logging** - Log only what you need (frames, settings, headers, data)
- ✅ **Backward compatible** - Old ShowFrameDetails/TraceFrames still supported (deprecated)

### v1.1.0 (2025-11-14)

go-rawhttp includes 6 powerful low-level transport enhancements designed for production use:

### 1. TLS Configuration Passthrough ⭐⭐⭐⭐⭐

Direct access to `crypto/tls.Config` for maximum flexibility:

```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "example.com",
    Port:   443,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS13,          // Enforce TLS 1.3+
        CipherSuites: []uint16{...},            // Custom cipher suites
        Certificates: []tls.Certificate{...},   // Client certificates
    },
}
```

### 2. Standardized Timing Metrics ⭐⭐⭐⭐

Industry-standard field names with backward compatibility:

```go
resp, _ := sender.Do(ctx, req, opts)
fmt.Printf("DNS: %v\n", resp.Metrics.DNSLookup)       // New: Clear naming
fmt.Printf("TCP: %v\n", resp.Metrics.TCPConnect)      // New: Clear naming
fmt.Printf("TLS: %v\n", resp.Metrics.TLSHandshake)    // New: Clear naming
fmt.Printf("TTFB: %v\n", resp.Metrics.TTFB)           // Unchanged
fmt.Printf("Total: %v\n", resp.Metrics.TotalTime)     // New: Consistent naming

// Old field names still work (deprecated but supported)
fmt.Printf("DNS: %v\n", resp.Metrics.DNS)             // Backward compatible
```

### 3. HTTP/2 Settings Exposure ⭐⭐⭐⭐

RFC 7540 compliant HTTP/2 configuration:

```go
opts := rawhttp.Options{
    HTTP2Settings: &rawhttp.HTTP2Settings{
        MaxConcurrentStreams: 100,                     // SETTINGS_MAX_CONCURRENT_STREAMS
        InitialWindowSize:    4194304,                 // SETTINGS_INITIAL_WINDOW_SIZE (4MB)
        MaxFrameSize:         16384,                   // SETTINGS_MAX_FRAME_SIZE (16KB)
        MaxHeaderListSize:    10485760,                // SETTINGS_MAX_HEADER_LIST_SIZE (10MB)
        DisableServerPush:    true,                    // Security: disable server push
        EnableCompression:    true,                    // HPACK compression
    },
}
```

### 4. Enhanced Connection Metadata ⭐⭐⭐

Socket-level and TLS session information:

```go
resp, _ := sender.Do(ctx, req, opts)

// Socket-level info
fmt.Printf("Local: %s\n", resp.LocalAddr)              // "192.168.1.100:54321"
fmt.Printf("Remote: %s\n", resp.RemoteAddr)            // "93.184.216.34:443"
fmt.Printf("Conn ID: %d\n", resp.ConnectionID)         // Unique identifier

// TLS session info
fmt.Printf("Session ID: %s\n", resp.TLSSessionID)      // Hex-encoded session ID
fmt.Printf("Resumed: %v\n", resp.TLSResumed)           // Session resumption flag
```

### 5. Error Type Classification ⭐⭐⭐

Operation tracking for smart retry logic:

```go
resp, err := sender.Do(ctx, req, opts)
if err != nil {
    if transportErr, ok := err.(*rawhttp.TransportError); ok {
        fmt.Printf("Phase: %s\n", transportErr.Type)   // dns, tcp, tls, http
        fmt.Printf("Op: %s\n", transportErr.Op)        // lookup, dial, handshake, read, write
        fmt.Printf("Addr: %s\n", transportErr.Addr)    // example.com:443

        // Smart retry logic
        if transportErr.Type == rawhttp.ErrorTypeDNS {
            // Retry DNS errors
        } else if transportErr.Type == rawhttp.ErrorTypeTLS {
            // Don't retry TLS errors
        }
    }
}
```

### 6. Connection Pool Observability ⭐⭐

Monitor pool health and performance:

```go
sender := rawhttp.NewSender()

// Make requests with pooling enabled
opts := rawhttp.Options{
    ReuseConnection: true,
    // ... other options
}

// Get pool statistics
stats := sender.PoolStats()
fmt.Printf("Active: %d\n", stats.ActiveConns)          // In use
fmt.Printf("Idle: %d\n", stats.IdleConns)              // Available
fmt.Printf("Reused: %d\n", stats.TotalReused)          // Lifetime count

// Detect issues
if stats.ActiveConns > 100 {
    fmt.Println("Warning: Possible connection leak")
}
```

All enhancements are **100% backward compatible** - existing code continues to work unchanged.

## Architecture

The library is designed with a modular architecture that makes it easy to extend and maintain:

```
github.com/WhileEndless/go-rawhttp/
├── rawhttp.go              # Main API
├── pkg/
│   ├── client/             # HTTP/1.1 client implementation
│   ├── http2/              # HTTP/2 protocol support
│   │   ├── client.go       # HTTP/2 client
│   │   ├── converter.go    # HTTP/1.1 <-> HTTP/2 conversion
│   │   ├── frames.go       # Frame handling
│   │   ├── stream.go       # Stream management
│   │   └── transport.go    # Connection management
│   ├── transport/          # Network transport layer
│   ├── buffer/             # Memory-efficient buffering
│   ├── errors/             # Structured error handling
│   └── timing/             # Performance measurement
├── tests/
│   ├── unit/               # Unit tests
│   └── integration/        # Integration tests
└── examples/               # Usage examples
```

## API Reference

### Core Types

#### Sender
```go
type Sender struct {
    // private fields
}

func NewSender() *Sender
func (s *Sender) Do(ctx context.Context, req []byte, opts Options) (*Response, error)
```

#### Options
```go
type Options struct {
    Scheme       string        // "http" or "https"
    Host         string        // Target hostname
    Port         int           // Target port
    ConnectIP    string        // Optional: specific IP to connect to
    SNI          string        // Optional: custom SNI hostname
    DisableSNI   bool          // Disable SNI extension
    InsecureTLS  bool          // Skip TLS certificate verification
    ConnTimeout  time.Duration // Connection timeout (default: 10s)
    DNSTimeout   time.Duration // DNS resolution timeout (0 = use ConnTimeout)
    ReadTimeout  time.Duration // Read timeout
    WriteTimeout time.Duration // Write timeout
    BodyMemLimit int64         // Memory limit before spilling to disk (default: 4MB)

    // Protocol selection
    Protocol     string        // "http/1.1" or "http/2" (auto-detected if not set)

    // HTTP/2 specific options
    HTTP2Settings *HTTP2Settings

    // Connection pooling and reuse
    ReuseConnection bool        // Enable Keep-Alive and connection pooling

    // Upstream proxy support
    ProxyURL     string        // Upstream proxy URL (e.g., "http://proxy:8080" or "socks5://user:pass@proxy:1080")

    // Custom TLS configuration
    CustomCACerts [][]byte     // Custom root CA certificates in PEM format
}

type HTTP2Settings struct {
    // Connection and Protocol Settings
    EnableServerPush     bool   // Enable HTTP/2 server push (default: false - recommended for security)
    EnableCompression    bool   // Enable HPACK header compression (default: true)
    EnableMultiplexing   bool   // Enable HTTP/2 stream multiplexing (default: false)

    // Performance and Resource Limits
    MaxConcurrentStreams uint32 // Max concurrent streams per connection (default: 100)
    InitialWindowSize    uint32 // Flow control window size bytes (default: 4194304 - 4MB, production optimized)
    MaxFrameSize         uint32 // Maximum HTTP/2 frame size bytes (default: 16384 - 16KB, RFC compliant)
    MaxHeaderListSize    uint32 // Maximum header list size bytes (default: 10485760 - 10MB)
    HeaderTableSize      uint32 // HPACK dynamic table size bytes (default: 4096 - 4KB)

    // Debugging and Monitoring (NEW in v1.1.1)
    Debug struct {
        LogFrames   bool // Log all HTTP/2 frames (default: false)
        LogSettings bool // Log SETTINGS frames (default: false)
        LogHeaders  bool // Log HEADERS frames (default: false)
        LogData     bool // Log DATA frames (default: false)
    }

    // Deprecated: Use Debug.LogFrames instead
    ShowFrameDetails     bool   // Log detailed frame information (deprecated)
    TraceFrames          bool   // Trace all HTTP/2 frames (deprecated)
}
```

#### Response
```go
type Response struct {
    StatusLine  string                // HTTP status line
    StatusCode  int                   // HTTP status code
    Headers     map[string][]string   // Response headers
    Body        *Buffer               // Response body
    Raw         *Buffer               // Complete raw response (status line + headers + body)
    Timings     Metrics              // Performance timings
    BodyBytes   int64                // Body size in bytes
    RawBytes    int64                // Total response size in bytes
    HTTPVersion string               // "HTTP/1.1" or "HTTP/2"
    Metrics     *timing.Metrics      // Detailed timing metrics (same as Timings for compatibility)

    // Connection metadata
    ConnectedIP        string         // Actual IP address connected to (after DNS resolution)
    ConnectedPort      int            // Actual port connected to
    NegotiatedProtocol string         // Negotiated protocol (e.g., "HTTP/1.1", "HTTP/2", "h2")
    TLSVersion         string         // TLS version used (e.g., "TLS 1.3")
    TLSCipherSuite     string         // TLS cipher suite used
    TLSServerName      string         // TLS Server Name (SNI)
    ConnectionReused   bool           // Whether the connection was reused from pool
}
```

#### Error Handling
```go
type Error struct {
    Type      ErrorType `json:"type"`
    Message   string    `json:"message"`
    Cause     error     `json:"cause,omitempty"`
    Host      string    `json:"host,omitempty"`
    Port      int       `json:"port,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

// Error types
const (
    ErrorTypeDNS        = "dns"
    ErrorTypeConnection = "connection"
    ErrorTypeTLS        = "tls"
    ErrorTypeTimeout    = "timeout"
    ErrorTypeProtocol   = "protocol"
    ErrorTypeIO         = "io"
    ErrorTypeValidation = "validation"
)
```

### Helper Functions

```go
// Create default options
func DefaultOptions(scheme, host string, port int) Options

// Error checking
func IsTimeoutError(err error) bool
func IsTemporaryError(err error) bool
func GetErrorType(err error) string

// Buffer creation
func NewBuffer(limit int64) *Buffer
```

## Examples

### Basic HTTP Request
```go
sender := rawhttp.NewSender()
request := []byte("GET /api/users HTTP/1.1\r\nHost: api.example.com\r\nConnection: close\r\n\r\n")

resp, err := sender.Do(context.Background(), request, rawhttp.DefaultOptions("http", "api.example.com", 80))
```

### HTTPS POST with JSON
```go
jsonData := `{"name": "test", "value": 42}`
request := fmt.Sprintf(
    "POST /api/data HTTP/1.1\r\n" +
    "Host: api.example.com\r\n" +
    "Content-Type: application/json\r\n" +
    "Content-Length: %d\r\n" +
    "Connection: close\r\n\r\n" +
    "%s", len(jsonData), jsonData)

opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "api.example.com",
    Port:        443,
    ConnTimeout: 15 * time.Second,
    ReadTimeout: 60 * time.Second,
}

resp, err := sender.Do(context.Background(), []byte(request), opts)
```

### Error Handling
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    if rawErr, ok := err.(*rawhttp.Error); ok {
        switch rawErr.Type {
        case rawhttp.ErrorTypeDNS:
            log.Printf("DNS resolution failed for %s", rawErr.Host)
        case rawhttp.ErrorTypeConnection:
            log.Printf("Connection failed to %s:%d", rawErr.Host, rawErr.Port)
        case rawhttp.ErrorTypeTLS:
            log.Printf("TLS handshake failed")
        case rawhttp.ErrorTypeTimeout:
            log.Printf("Request timed out")
        }
    }
    return err
}
```

### Performance Metrics
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    return err
}

fmt.Printf("Connection Time: %v\n", resp.Timings.GetConnectionTime())
fmt.Printf("Server Time: %v\n", resp.Timings.GetServerTime())
fmt.Printf("Total Time: %v\n", resp.Timings.Total)
fmt.Printf("DNS: %v, TCP: %v, TLS: %v, TTFB: %v\n",
    resp.Timings.DNS, resp.Timings.TCP, resp.Timings.TLS, resp.Timings.TTFB)
```

### HTTP/2 Examples

#### Simple HTTP/2 Request
```go
// Method 1: Auto-detection from request line
request := []byte("GET /api HTTP/2\r\nHost: example.com\r\nAccept: application/json\r\n\r\n")

// Method 2: Explicit protocol setting
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "example.com",
    Port:     443,
    Protocol: "http/2", // Force HTTP/2
}

resp, err := sender.Do(ctx, request, opts)
fmt.Printf("Protocol: %s\n", resp.HTTPVersion) // "HTTP/2"
```

#### HTTP/2 Production Configuration
```go
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "api.example.com",
    Port:     443,
    Protocol: "http/2",
    ConnTimeout:  15 * time.Second,
    ReadTimeout:  30 * time.Second,
    HTTP2Settings: &rawhttp.HTTP2Settings{
        MaxConcurrentStreams: 100,
        InitialWindowSize:    4194304,  // 4MB - Production optimized
        EnableServerPush:     false,     // Disabled for better control
        EnableCompression:    true,      // HPACK compression enabled
        HeaderTableSize:      4096,      // Standard HPACK table size
        MaxFrameSize:         16384,     // 16KB frames
        MaxHeaderListSize:    10485760,  // 10MB header limit
    },
}

// Multiple sequential requests will reuse the same HTTP/2 connection
// Automatic cleanup and flow control management
for i := 0; i < 5; i++ {
    request := fmt.Sprintf("GET /api/data/%d HTTP/2\r\nHost: api.example.com\r\nAuthorization: Bearer token\r\n\r\n", i)
    
    resp, err := sender.Do(ctx, []byte(request), opts)
    if err != nil {
        log.Printf("Request %d failed: %v", i, err)
        continue
    }
    
    fmt.Printf("Request %d: %s status %d, %d bytes, took %v\n", 
        i, resp.HTTPVersion, resp.StatusCode, resp.BodyBytes, resp.Timings.Total)
    
    // Resources automatically cleaned up
    resp.Body.Close()
    resp.Raw.Close()
}
```

#### H2C (HTTP/2 Cleartext)
```go
opts := rawhttp.Options{
    Scheme:   "http", // Note: http, not https
    Host:     "localhost",
    Port:     8080,
    Protocol: "http/2",
}

// Sends HTTP/2 over cleartext connection with H2C upgrade
resp, err := sender.Do(ctx, request, opts)
```

#### HTTP/2 Debug Logging (NEW in v1.1.1)
```go
// Enable selective HTTP/2 debug logging for troubleshooting
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "example.com",
    Port:     443,
    Protocol: "http/2",
    HTTP2Settings: &rawhttp.HTTP2Settings{
        MaxConcurrentStreams: 100,
        InitialWindowSize:    4194304,
        // Enable debug logging (production safe - zero overhead when disabled)
        Debug: struct {
            LogFrames   bool
            LogSettings bool
            LogHeaders  bool
            LogData     bool
        }{
            LogFrames:   true,  // Log all HTTP/2 frames
            LogSettings: true,  // Log SETTINGS frames
            LogHeaders:  false, // Don't log HEADERS frames
            LogData:     false, // Don't log DATA frames
        },
    },
}

resp, err := sender.Do(ctx, request, opts)
// Debug output will be logged to stderr during execution
```

### Large Response Handling
```go
opts := rawhttp.Options{
    Scheme:       "https",
    Host:         "example.com",
    Port:         443,
    BodyMemLimit: 1024 * 1024, // 1MB memory limit
}

resp, err := sender.Do(ctx, request, opts)
if err != nil {
    return err
}
defer resp.Body.Close()
defer resp.Raw.Close()

// Check if response spilled to disk
if resp.Body.IsSpilled() {
    fmt.Printf("Large response spilled to: %s\n", resp.Body.Path())
}

// Read response data
reader, err := resp.Body.Reader()
if err != nil {
    return err
}
defer reader.Close()

data, err := io.ReadAll(reader)
if err != nil {
    return err
}
```

### Connection Pooling (Keep-Alive)
```go
sender := rawhttp.NewSender()

opts := rawhttp.Options{
    Scheme:          "https",
    Host:            "api.example.com",
    Port:            443,
    ReuseConnection: true, // Enable connection pooling
}

// Multiple requests reuse the same connection
for i := 0; i < 10; i++ {
    request := fmt.Sprintf("GET /api/endpoint/%d HTTP/1.1\r\nHost: api.example.com\r\nConnection: keep-alive\r\n\r\n", i)

    resp, err := sender.Do(context.Background(), []byte(request), opts)
    if err != nil {
        log.Printf("Request %d failed: %v", i, err)
        continue
    }

    // Check if connection was reused
    if resp.ConnectionReused {
        log.Printf("Request %d: Reused connection to %s:%d", i, resp.ConnectedIP, resp.ConnectedPort)
    }

    resp.Body.Close()
    resp.Raw.Close()
}
```

### Upstream Proxy Support
```go
// HTTP proxy
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "target.example.com",
    Port:     443,
    ProxyURL: "http://proxy.example.com:8080", // HTTP proxy
}

// HTTP proxy with authentication
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "target.example.com",
    Port:     443,
    ProxyURL: "http://user:password@proxy.example.com:8080",
}

// HTTPS proxy
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "target.example.com",
    Port:     443,
    ProxyURL: "https://secure-proxy.example.com:8443",
}

// SOCKS5 proxy
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "target.example.com",
    Port:     443,
    ProxyURL: "socks5://user:password@socks-proxy.example.com:1080",
}

resp, err := sender.Do(ctx, request, opts)
```

### Custom CA Certificates
```go
// Load custom CA certificate
caCert, err := os.ReadFile("custom-ca.pem")
if err != nil {
    log.Fatal(err)
}

opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "internal.example.com",
    Port:          443,
    CustomCACerts: [][]byte{caCert}, // Add custom CA
}

resp, err := sender.Do(ctx, request, opts)
if err != nil {
    log.Fatal(err)
}

// Check TLS information
fmt.Printf("TLS Version: %s\n", resp.TLSVersion)
fmt.Printf("Cipher Suite: %s\n", resp.TLSCipherSuite)
fmt.Printf("Server Name: %s\n", resp.TLSServerName)
```

### Connection Metadata
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    return err
}
defer resp.Body.Close()
defer resp.Raw.Close()

// Access detailed connection information
fmt.Printf("Connected to: %s:%d\n", resp.ConnectedIP, resp.ConnectedPort)
fmt.Printf("Negotiated Protocol: %s\n", resp.NegotiatedProtocol)
fmt.Printf("TLS Version: %s\n", resp.TLSVersion)
fmt.Printf("TLS Cipher: %s\n", resp.TLSCipherSuite)
fmt.Printf("Connection Reused: %v\n", resp.ConnectionReused)

// Verify actual connected IP (useful for debugging DNS)
if resp.ConnectedIP != expectedIP {
    log.Printf("Warning: Connected to %s instead of %s", resp.ConnectedIP, expectedIP)
}
```

## Testing

The library includes comprehensive tests:

```bash
# Run all tests
go test ./...

# Run unit tests only
go test ./tests/unit/...

# Run integration tests only
go test ./tests/integration/...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...
```

## Protocol Support

### HTTP/1.1
- Full RFC compliance
- Chunked transfer encoding
- Keep-alive connections
- Custom headers
- Raw socket control

### HTTP/2
- ALPN negotiation
- Multiplexing support
- HPACK header compression
- Flow control
- Server push (optional)
- H2C (cleartext HTTP/2)
- Automatic HTTP/1.1-style formatting

The library automatically handles protocol differences while maintaining the same simple API. Write your requests in familiar HTTP/1.1 format, and the library handles the conversion to HTTP/2 frames when needed.

## Use Cases

- **Security Testing Tools** - Penetration testing and vulnerability scanners
- **HTTP Load Testing** - High-performance load testing tools  
- **Web Scrapers** - Fine-grained control over HTTP requests
- **API Testing** - Detailed HTTP testing with timing metrics
- **Proxy Development** - Raw HTTP manipulation for proxy servers
- **Research Tools** - HTTP protocol research and experimentation

## Performance

### 🏆 Benchmark Results
- **50 Concurrent Requests**: 100% success rate, 20+ req/sec throughput
- **Memory Efficiency**: No memory leaks detected, negative growth after cleanup  
- **Universal Compatibility**: 100% success rate with major HTTP/2 servers (Google, Cloudflare, GitHub, etc.)
- **Production Stability**: Sustained performance over extended periods

### ⚡ Technical Performance  
- **Zero Allocations** for small requests (< 4MB)
- **Memory Efficient** with automatic disk spilling for large responses
- **Low Latency** with direct socket communication
- **Flow Control Compliant** - Proper HTTP/2 window management prevents bottlenecks
- **Resource Cleanup** - Guaranteed cleanup of connections, file descriptors, and memory
- **Thread Safe Operations** - Concurrent request handling without race conditions
- **Detailed Metrics** for comprehensive performance analysis

## Limitations

- **No Redirect Following** - Manual redirect handling required
- **No Cookie Management** - Manual cookie handling required
- **WebSocket Support** - Not currently supported

## Documentation

- **[API Reference](docs/API.md)** - Complete API documentation
- **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Common issues and solutions
- **[Migration Guide](docs/MIGRATION.md)** - Migrating from other HTTP clients
- **[Examples](examples/)** - Working code examples

## Testing

The library includes comprehensive tests:

```bash
# Run all tests
go test ./...

# Run unit tests only
go test ./tests/unit/...

# Run integration tests only  
go test ./tests/integration/...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Test examples
go run examples/basic.go
go run examples/https_post.go
go run examples/advanced_usage.go
```

## Project Structure

```
github.com/WhileEndless/go-rawhttp/
├── rawhttp.go              # Main API
├── pkg/                    # Internal packages
│   ├── client/             # HTTP client implementation
│   ├── transport/          # Network transport layer
│   ├── buffer/             # Memory-efficient buffering
│   ├── errors/             # Structured error handling
│   └── timing/             # Performance measurement
├── tests/                  # Test suite
│   ├── unit/               # Unit tests
│   └── integration/        # Integration tests
├── examples/               # Usage examples
└── docs/                   # Documentation
```

## Roadmap

### Planned Features
- **WebSocket Support** - WebSocket protocol support
- **Advanced HTTP/2 Features** - Server push optimization and priority handling
- **Custom DNS Resolvers** - Support for custom DNS resolution strategies
- **Advanced Metrics** - Extended timing and performance metrics

### Performance Enhancements
- **Memory Optimization** - Further reduce memory footprint for high-volume scenarios
- **Concurrent Connection Limits** - Configurable connection limits and queuing
- **Protocol Detection** - Enhanced automatic protocol detection and fallback

Want to contribute to any of these features? Check out the [Contributing](#contributing) section!

## Contributing

Contributions are welcome! Please:

1. Read the documentation in `docs/`
2. Run the test suite: `go test ./...`
3. Add tests for new features
4. Follow Go best practices
5. Update documentation as needed