# go-rawhttp

[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/WhileEndless/go-rawhttp)
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
✅ **Proxy Support** - HTTP, HTTPS, SOCKS4, and SOCKS5 upstream proxy support for both HTTP/1.1 and HTTP/2 with authentication and advanced features
✅ **Custom TLS** - Direct TLS config passthrough for full control (TLS versions, cipher suites, mTLS client certificates)
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

    rawhttp "github.com/WhileEndless/go-rawhttp"
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

## Command-Line Tool (`rawhttp` CLI)

The repository ships a **curl-compatible command-line client** built on this
library, in [`cmd/rawhttp`](cmd/rawhttp/) (its own Go module, so the rendering
dependencies stay out of the library). See
[`cmd/rawhttp/README.md`](cmd/rawhttp/README.md) for the full reference.

### Install (no root required, macOS + Linux)

```sh
make build                 # compile ./rawhttp
make install               # install to ~/.local/bin (override with BINDIR=...)
```

> Because `cmd/rawhttp` is a separate module, `go build ./cmd/rawhttp` from the
> repo root does **not** work — use `make build` (or `cd cmd/rawhttp && go build`).

### Highlights

- **curl-compatible flags**: `-X -H -d --data-binary -F -A -e -b -u -G -I -L
  -o -O -s -v -i -k -m -x -w --http1.1 --http2 --resolve --cert --key --cacert`.
- **Library superpowers**: `--raw-request` (send a request verbatim),
  `--sni`/`--disable-sni`, `--connect-ip`/`--connect-to`/`--resolve`, `--reuse`,
  `--tls-min`/`--tls-max`, `--timings`, every proxy type, and mTLS.
- **Multi-connection download manager** (`--download` / `-j`): IDM-style
  segmented range downloads with a live progress bar (percent, speed, ETA) and
  automatic single-stream fallback.
- **Beautify + syntax-highlight + color** (on by default for a terminal):
  headers and structured fields (query, cookies, auth, form) are colorized; the
  body is beautified and highlighted by content type (JSON/XML/HTML/JS/CSS,
  including embedded `<script>`/`<style>` in HTML); `gzip`/`deflate`/`br` is
  decompressed; binary/image bodies are summarized (with image dimensions).
  Output to a pipe or file stays raw. Toggle with `--color`/`--no-color`,
  `--beautify`/`--no-beautify` (and `NO_COLOR`/`FORCE_COLOR`).
- **Structured output**: `--json` / `--xml` emit the whole transaction (request,
  response, connection, TLS, proxy, timing statistics, and any error) as one
  document, on success or failure, to stdout or `-o file`.

```sh
rawhttp -s https://example.com                          # GET (pretty in a terminal)
rawhttp -v https://example.com                          # colorized request/response trace
rawhttp -d 'a=1&b=2' https://httpbin.org/post           # POST form
rawhttp -L -o page.html https://example.com             # follow redirects, save to file
rawhttp -j 8 -o file.bin https://cdn.example.com/file   # 8-connection download
rawhttp --http2 -k https://example.com                  # force HTTP/2
```

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

    // TLS Configuration
    SNI          string        // Optional: custom SNI hostname (overrides Host for TLS handshake)
    DisableSNI   bool          // Disable SNI extension completely
    InsecureTLS  bool          // Skip TLS certificate verification (works with custom TLSConfig in v1.1.5+)
    TLSConfig    *tls.Config   // Custom TLS configuration (full control over TLS settings)

    ConnTimeout  time.Duration // Connection timeout (default: 10s)
    DNSTimeout   time.Duration // DNS resolution timeout (0 = use ConnTimeout)
    ReadTimeout  time.Duration // Read timeout
    WriteTimeout time.Duration // Write timeout
    BodyMemLimit int64         // Memory limit before spilling to disk (default: 4MB)

    // Protocol selection
    Protocol     string        // "http/1.1" or "http/2" (auto-detected if not set)

    // HTTP/2 specific options (v1.1.5+ supports full TLS configuration)
    HTTP2Settings *HTTP2Settings

    // Connection pooling and reuse
    ReuseConnection bool        // Enable Keep-Alive and connection pooling

    // Upstream proxy support
    Proxy *ProxyConfig          // Upstream proxy configuration (replaces ProxyURL in v2.0.0)

    // Custom TLS configuration
    CustomCACerts  [][]byte     // Custom root CA certificates in PEM format

    // Client certificate for mutual TLS (mTLS authentication) - v1.2.0+
    // Option 1: Provide PEM-encoded certificate and key directly
    ClientCertPEM  []byte       // Client certificate in PEM format
    ClientKeyPEM   []byte       // Client private key in PEM format (unencrypted)

    // Option 2: Provide file paths (will be loaded automatically)
    ClientCertFile string       // Path to client certificate file (.crt, .pem)
    ClientKeyFile  string       // Path to client private key file (.key, .pem)

    // SSL/TLS Protocol Version Control - v1.2.0+
    MinTLSVersion    uint16                   // Minimum SSL/TLS version (e.g., tls.VersionTLS12)
    MaxTLSVersion    uint16                   // Maximum SSL/TLS version (e.g., tls.VersionTLS13)
    TLSRenegotiation tls.RenegotiationSupport // TLS renegotiation (default: RenegotiateNever)
    CipherSuites     []uint16                 // Allowed cipher suites (default: Go secure defaults)
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

    // Debugging and Monitoring
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

type ProxyConfig struct {
    Type               string            // Proxy type: "http", "https", "socks4", "socks5"
    Host               string            // Proxy hostname
    Port               int               // Proxy port
    Username           string            // Optional: proxy authentication username
    Password           string            // Optional: proxy authentication password
    ConnTimeout        time.Duration     // Optional: proxy connection timeout (uses ConnTimeout if not set)
    ProxyHeaders       map[string]string // Optional: custom headers for HTTP/HTTPS proxies
    TLSConfig          *tls.Config       // Optional: custom TLS config for HTTPS proxies
    ResolveDNSViaProxy bool              // Optional: resolve DNS via SOCKS proxy (default: true for SOCKS5)
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

#### HTTP/2 Debug Logging
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

### Advanced Connection Pool Configuration

For high-concurrency scenarios, customize pool behavior:

```go
import rawhttp "github.com/WhileEndless/go-rawhttp"

// Create sender with custom pool settings
sender := rawhttp.NewSenderWithPoolConfig(rawhttp.PoolConfig{
    MaxIdleConnsPerHost: 5,             // Keep up to 5 idle connections per host (default: 2)
    MaxConnsPerHost:     10,            // Limit total connections per host (0 = unlimited)
    MaxIdleTime:         90 * time.Second, // Connection idle timeout
    WaitTimeout:         5 * time.Second,  // Wait for available connection (0 = no wait)
})

opts := rawhttp.Options{
    Scheme:          "https",
    Host:            "api.example.com",
    Port:            443,
    ReuseConnection: true,
}

// Monitor pool statistics
stats := sender.PoolStats()
fmt.Printf("Pool Stats:\n")
fmt.Printf("  Active: %d\n", stats.ActiveConns)
fmt.Printf("  Idle: %d\n", stats.IdleConns)
fmt.Printf("  Total Created: %d\n", stats.TotalCreated)
fmt.Printf("  Total Reused: %d\n", stats.TotalReused)
fmt.Printf("  Wait Timeouts: %d\n", stats.WaitTimeouts)

// Per-host statistics
for host, hostStats := range stats.HostStats {
    fmt.Printf("  %s: Active=%d, Idle=%d\n", host, hostStats.ActiveConns, hostStats.IdleConns)
}
```

### Upstream Proxy Support

⚠️ **BREAKING CHANGE**: `ProxyURL` field has been removed in v2.0.0. Use `Proxy` with `ParseProxyURL()` helper.

See [MIGRATION_V2.md](docs/MIGRATION_V2.md) for full migration guide.

#### Simple Proxy Usage (ParseProxyURL)
```go
import rawhttp "github.com/WhileEndless/go-rawhttp"

// HTTP proxy - simple and convenient!
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("http://127.0.0.1:8080"),
}

// HTTP proxy with authentication
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("http://user:pass@proxy.example.com:8080"),
}

// HTTPS proxy (TLS to proxy)
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("https://secure-proxy.example.com:8443"),
}

// SOCKS5 proxy (DNS via proxy by default)
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("socks5://user:pass@proxy.example.com:1080"),
}

// SOCKS4 proxy (legacy support, IPv4 only)
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("socks4://user@proxy.example.com:1080"),
}

resp, err := sender.Do(ctx, request, opts)
```

#### Advanced Proxy Configuration
```go
// Custom headers for HTTP/HTTPS proxies
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy: &rawhttp.ProxyConfig{
        Type:     "http",
        Host:     "corporate-proxy.example.com",
        Port:     8080,
        Username: "employee",
        Password: "secret",

        // NEW: Advanced features in v2.0.0
        ConnTimeout: 5 * time.Second,           // Proxy-specific timeout
        ProxyHeaders: map[string]string{        // Custom headers
            "X-Employee-ID": "12345",
            "X-Department":  "Engineering",
        },
    },
}

// HTTPS proxy with custom TLS configuration
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy: &rawhttp.ProxyConfig{
        Type:     "https",
        Host:     "secure-proxy.example.com",
        Port:     8443,
        Username: "user",
        Password: "pass",

        // Custom TLS for connecting TO the proxy
        TLSConfig: &tls.Config{
            InsecureSkipVerify: true, // Accept self-signed proxy cert
            MinVersion:         tls.VersionTLS12,
        },
    },
}

// Check proxy metadata in response
resp, err := sender.Do(ctx, request, opts)
if resp != nil {
    fmt.Printf("Proxy Used: %v\n", resp.ProxyUsed)
    fmt.Printf("Proxy Type: %s\n", resp.ProxyType)
    fmt.Printf("Proxy Addr: %s\n", resp.ProxyAddr)
}

// Handle proxy errors
if err != nil {
    if proxyErr, ok := err.(*rawhttp.ProxyError); ok {
        fmt.Printf("Proxy %s at %s failed: %v\n",
            proxyErr.ProxyType, proxyErr.ProxyAddr, proxyErr.Err)
    }
}
```

**Common Question**: Can HTTP proxy handle HTTPS targets?

**YES!** `http://` proxy can proxy HTTPS requests. The proxy type (http/https) determines how you connect TO the proxy. The target scheme (http/https) determines traffic THROUGH the proxy.

Example: `Proxy: ParseProxyURL("http://proxy:8080")` with `Scheme: "https"` works perfectly - cleartext connection to proxy, encrypted connection to target.

**See also**: `examples/proxy_comprehensive.go` for complete examples

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

### Client Certificates (mTLS)

Mutual TLS (mTLS) allows clients to authenticate themselves to the server using client certificates. go-rawhttp supports client certificates through two convenient methods:

#### Option 1: Using PEM Byte Arrays
```go
// Your client certificate and key in PEM format
clientCertPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCFcV9...
-----END CERTIFICATE-----`)

clientKeyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhk...
-----END PRIVATE KEY-----`)

sender := rawhttp.NewSender()

opts := rawhttp.Options{
    Scheme: "https",
    Host:   "mtls-server.example.com",
    Port:   443,

    // Client certificate for mTLS authentication
    ClientCertPEM: clientCertPEM,
    ClientKeyPEM:  clientKeyPEM,
}

resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    log.Fatalf("Request failed: %v", err)
}
defer resp.Body.Close()

fmt.Printf("Status: %d\n", resp.StatusCode)
```

#### Option 2: Using File Paths
```go
sender := rawhttp.NewSender()

opts := rawhttp.Options{
    Scheme: "https",
    Host:   "mtls-server.example.com",
    Port:   443,

    // Client certificate from files (loaded automatically)
    ClientCertFile: "/path/to/client.crt",
    ClientKeyFile:  "/path/to/client.key",
}

resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    log.Fatalf("Request failed: %v", err)
}
defer resp.Body.Close()

fmt.Printf("Status: %d\n", resp.StatusCode)
```

#### mTLS with Custom CA (Self-Signed Server)
```go
// Combine client certificates with custom CA trust
caCertPEM, _ := os.ReadFile("server-ca.pem")
clientCertPEM, _ := os.ReadFile("client.crt")
clientKeyPEM, _ := os.ReadFile("client.key")

sender := rawhttp.NewSender()

opts := rawhttp.Options{
    Scheme: "https",
    Host:   "self-signed-mtls.example.com",
    Port:   8443,

    // Custom CA to trust server's self-signed cert
    CustomCACerts: [][]byte{caCertPEM},

    // Client certificate for mTLS authentication
    ClientCertPEM: clientCertPEM,
    ClientKeyPEM:  clientKeyPEM,
}

resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    log.Fatalf("Request failed: %v", err)
}
defer resp.Body.Close()

fmt.Printf("Mutual TLS handshake successful!\n")
fmt.Printf("TLS Version: %s\n", resp.TLSVersion)
fmt.Printf("Cipher Suite: %s\n", resp.TLSCipherSuite)
```

#### mTLS with HTTP/2
```go
// Client certificates work seamlessly with HTTP/2
opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "h2-mtls.example.com",
    Port:          443,
    Protocol:      "http/2",
    ClientCertPEM: clientCertPEM,
    ClientKeyPEM:  clientKeyPEM,
}

resp, err := sender.Do(context.Background(), request, opts)
// Supports full HTTP/2 with mTLS authentication
```

**Note**: Client certificates can also be provided via `TLSConfig.Certificates`, but using `ClientCertPEM`/`ClientKeyPEM` or `ClientCertFile`/`ClientKeyFile` is more convenient.

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

### TLS Configuration

#### InsecureTLS with Custom TLS Config
```go
// v1.1.5 Fix: InsecureTLS now works WITH custom TLSConfig
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "self-signed.example.com",
    Port:        443,
    InsecureTLS: true, // ✅ Now properly overrides InsecureSkipVerify
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
        MaxVersion: tls.VersionTLS13,
        // Custom cipher suites, ALPN, etc.
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        },
    },
}

resp, err := sender.Do(ctx, request, opts)
// ✅ Works correctly: accepts self-signed cert AND uses custom TLS settings
```

#### HTTP/2 with TLS Configuration
```go
// v1.1.5 Fix: HTTP/2 now supports InsecureTLS and custom TLSConfig
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "example.com",
    Port:        443,
    Protocol:    "http/2",
    InsecureTLS: true, // ✅ Now works with HTTP/2!
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
        NextProtos: []string{"h2"}, // HTTP/2 ALPN
    },
}

resp, err := sender.Do(ctx, request, opts)
// ✅ HTTP/2 with self-signed certificates now works
```

### SNI Configuration

#### Custom SNI Hostname
```go
// Useful for CDNs, virtual hosting, IP-to-hostname mapping
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "151.101.1.69", // CDN edge IP
    Port:        443,
    SNI:         "example.com", // ✅ Custom SNI for virtual host
    InsecureTLS: true,
}

resp, err := sender.Do(ctx, request, opts)
// Connects to IP but sends SNI: example.com
```

#### Disable SNI Completely
```go
// For legacy servers or special testing scenarios
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "legacy-server.example.com",
    Port:        443,
    DisableSNI:  true, // ✅ No SNI extension sent
    InsecureTLS: true,
}

resp, err := sender.Do(ctx, request, opts)
```

#### SNI Priority Order
```go
// Priority: TLSConfig.ServerName > SNI option > Host field
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "fallback.example.com", // Priority 3 (lowest)
    Port:   443,
    TLSConfig: &tls.Config{
        ServerName:         "priority1.example.com", // Priority 1 (used)
        InsecureSkipVerify: true,
    },
    SNI: "priority2.example.com", // Priority 2 (ignored when TLSConfig.ServerName set)
}

resp, err := sender.Do(ctx, request, opts)
// TLS handshake uses ServerName: priority1.example.com
```

### Proxy MITM Scenarios

#### Basic MITM with Self-Signed Certificates
```go
// Common scenario: intercepting proxy with self-signed certificates
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "api.example.com",
    Port:        443,
    InsecureTLS: true, // ✅ Accept proxy's self-signed cert
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
        // Custom cipher suites for proxy compatibility
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
    },
}

resp, err := sender.Do(ctx, request, opts)
// ✅ v1.1.5: InsecureTLS works with custom TLS config
```

#### HTTP/2 MITM with SNI
```go
// Advanced: HTTP/2 through MITM proxy with custom SNI
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "151.101.1.69", // CDN IP
    Port:        443,
    Protocol:    "http/2",
    SNI:         "cdn.example.com", // ✅ Custom SNI for CDN
    InsecureTLS: true,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
        NextProtos: []string{"h2"},
    },
}

resp, err := sender.Do(ctx, request, opts)
// ✅ v1.1.5: HTTP/2 + TLS config + SNI all working together
```

### Complete Examples

See the `examples/` directory for complete working examples:

- **`proxy_comprehensive.go`** - Comprehensive proxy examples for v2.0.0 (HTTP, HTTPS, SOCKS4, SOCKS5)
- **`tls_custom_config.go`** - TLS configuration with InsecureTLS and custom TLSConfig
- **`sni_configuration.go`** - SNI configuration examples (custom SNI, disable SNI, priority order)
- **`proxy_mitm.go`** - Proxy MITM scenarios showcasing v1.1.5 bug fixes
- **`mtls_client_cert_example.go`** - Client certificates for mutual TLS (mTLS) authentication
- **`http2_basic.go`** - Basic HTTP/2 usage examples
- **`http2_advanced.go`** - Advanced HTTP/2 features
- **`http2_connection_pooling.go`** - HTTP/2 connection pooling and multiplexing

Run examples:
```bash
go run examples/proxy_comprehensive.go
go run examples/tls_custom_config.go
go run examples/sni_configuration.go
go run examples/proxy_mitm.go
go run examples/mtls_client_cert_example.go
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
- **[CLI Reference](cmd/rawhttp/README.md)** - The `rawhttp` command-line tool (curl-compatible, beautify/color, download manager)
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
│   ├── http2/              # HTTP/2 implementation
│   ├── transport/          # Network transport layer
│   ├── buffer/             # Memory-efficient buffering
│   ├── errors/             # Structured error handling
│   └── timing/             # Performance measurement
├── cmd/rawhttp/            # curl-compatible CLI (separate Go module)
├── tests/                  # Test suite
│   ├── unit/               # Unit tests
│   └── integration/        # Integration tests
├── examples/               # Usage examples
├── docs/                   # Documentation
└── Makefile                # Build/install the CLI (make build / make install)
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