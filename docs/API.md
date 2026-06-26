# API Documentation

Complete API reference for go-rawhttp library.

## Core API

### Sender

The main client for making HTTP requests.

```go
type Sender struct {
    // private fields
}
```

#### Functions

##### NewSender
```go
func NewSender() *Sender
```
Creates a new Sender instance with default pool configuration.

**Example:**
```go
sender := rawhttp.NewSender()
```

##### NewSenderWithPoolConfig (v2.1.0+)
```go
func NewSenderWithPoolConfig(config PoolConfig) *Sender
```
Creates a new Sender with custom connection pool configuration.

**Parameters:**
- `config`: Pool configuration options

**Example:**
```go
sender := rawhttp.NewSenderWithPoolConfig(rawhttp.PoolConfig{
    MaxIdleConnsPerHost: 5,
    MaxConnsPerHost:     10,
    MaxIdleTime:         90 * time.Second,
})
```

##### PoolStats
```go
func (s *Sender) PoolStats() PoolStats
```
Returns current connection pool statistics.

**Example:**
```go
stats := sender.PoolStats()
fmt.Printf("Active: %d, Idle: %d\n", stats.ActiveConns, stats.IdleConns)
```

##### Do
```go
func (s *Sender) Do(ctx context.Context, req []byte, opts Options) (*Response, error)
```
Executes an HTTP request using raw sockets.

**Parameters:**
- `ctx`: Context for cancellation and timeout
- `req`: Raw HTTP request bytes (including headers and body)
- `opts`: Configuration options for the request

**Returns:**
- `*Response`: HTTP response (may be partial on error)
- `error`: Structured error or nil on success

**Example:**
```go
request := []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")
resp, err := sender.Do(context.Background(), request, opts)
```

### Options

Configuration for HTTP requests.

```go
type Options struct {
    Scheme       string        // "http" or "https"
    Host         string        // Target hostname
    Port         int           // Target port (80 for HTTP, 443 for HTTPS)
    ConnectIP    string        // Optional: specific IP to connect to
    SNI          string        // Optional: custom SNI hostname for TLS
    DisableSNI   bool          // Disable SNI extension
    InsecureTLS  bool          // Skip TLS certificate verification
    ConnTimeout  time.Duration // Connection timeout (default: 10s)
    DNSTimeout   time.Duration // DNS resolution timeout (0 = use ConnTimeout, default: 5s)
    ReadTimeout  time.Duration // Read timeout (0 = no timeout)
    WriteTimeout time.Duration // Write timeout (0 = no timeout)
    BodyMemLimit int64         // Memory limit before spilling to disk (default: 4MB)

    // Protocol selection
    Protocol     string        // "http/1.1" or "http/2" (auto-detected if not set)

    // HTTP/2 specific options
    HTTP2Settings   *HTTP2Settings // HTTP/2 configuration

    // TLS Configuration
    TLSConfig      *tls.Config // Custom TLS configuration (full control)
    CustomCACerts  [][]byte    // Custom root CA certificates in PEM format

    // Client certificate for mutual TLS (mTLS authentication) - v1.2.0+
    // Option 1: Provide PEM-encoded certificate and key directly
    ClientCertPEM  []byte      // Client certificate in PEM format
    ClientKeyPEM   []byte      // Client private key in PEM format (unencrypted)

    // Option 2: Provide file paths (will be loaded automatically)
    ClientCertFile string      // Path to client certificate file (.crt, .pem)
    ClientKeyFile  string      // Path to client private key file (.key, .pem)

    // SSL/TLS Protocol Version Control - v1.2.0+
    MinTLSVersion    uint16                   // Minimum SSL/TLS version (e.g., tls.VersionTLS12)
    MaxTLSVersion    uint16                   // Maximum SSL/TLS version (e.g., tls.VersionTLS13)
    TLSRenegotiation tls.RenegotiationSupport // TLS renegotiation (default: RenegotiateNever)
    CipherSuites     []uint16                 // Allowed cipher suites (default: Go secure defaults)

    // Proxy configuration
    ProxyURL       string      // Upstream proxy URL (e.g., "http://proxy:8080")

    // Connection pooling
    ReuseConnection bool       // Enable Keep-Alive and connection pooling
}
```

#### Helper Functions

##### DefaultOptions
```go
func DefaultOptions(scheme, host string, port int) Options
```
Creates default options for common use cases.

**Example:**
```go
opts := rawhttp.DefaultOptions("https", "api.example.com", 443)
```

### Response

HTTP response with parsed components and metadata.

```go
type Response struct {
    StatusLine string                // Full HTTP status line
    StatusCode int                   // HTTP status code (200, 404, etc.)
    Headers    map[string][]string   // Response headers (canonical keys)
    Body       *Buffer               // Response body
    Raw        *Buffer               // Complete raw response (headers + body)
    Timings    Metrics              // Performance timing information
    BodyBytes  int64                // Body size in bytes
    RawBytes   int64                // Total response size in bytes
    HTTPVersion string               // "HTTP/1.1" or "HTTP/2"
    Metrics     *Metrics             // Same as Timings for compatibility
}
```

**Important:** Always close `Body` and `Raw` buffers:
```go
defer resp.Body.Close()
defer resp.Raw.Close()
```

## Buffer API

Memory-efficient storage with automatic disk spilling.

### Buffer

```go
type Buffer struct {
    // private fields
}
```

#### Functions

##### NewBuffer
```go
func NewBuffer(limit int64) *Buffer
```
Creates a new buffer with specified memory limit.

##### Write
```go
func (b *Buffer) Write(p []byte) (int, error)
```
Writes data to buffer, spilling to disk if over limit.

##### Bytes
```go
func (b *Buffer) Bytes() []byte
```
Returns in-memory data (nil if spilled to disk).

##### Reader
```go
func (b *Buffer) Reader() (io.ReadCloser, error)
```
Returns a reader for the buffer contents.

##### Size
```go
func (b *Buffer) Size() int64
```
Returns total bytes written.

##### IsSpilled
```go
func (b *Buffer) IsSpilled() bool
```
Returns true if data has spilled to disk.

##### Path
```go
func (b *Buffer) Path() string
```
Returns filesystem path of spilled data (empty if in memory).

##### Close
```go
func (b *Buffer) Close() error
```
Closes and removes temporary files.

##### Reset
```go
func (b *Buffer) Reset() error
```
Resets buffer for reuse.

**Example:**
```go
buf := rawhttp.NewBuffer(1024 * 1024) // 1MB limit
defer buf.Close()

buf.Write(data)
if buf.IsSpilled() {
    fmt.Printf("Data spilled to: %s", buf.Path())
}

reader, err := buf.Reader()
if err != nil {
    return err
}
defer reader.Close()

content, err := io.ReadAll(reader)
```

## Timing API

Performance measurement utilities.

### Metrics

```go
type Metrics struct {
    DNS   time.Duration `json:"dns"`   // DNS resolution time
    TCP   time.Duration `json:"tcp"`   // TCP connection time
    TLS   time.Duration `json:"tls"`   // TLS handshake time
    TTFB  time.Duration `json:"ttfb"`  // Time to first byte
    Total time.Duration `json:"total"` // Total request time
}
```

#### Methods

##### GetConnectionTime
```go
func (m Metrics) GetConnectionTime() time.Duration
```
Returns total connection establishment time (DNS + TCP + TLS).

##### GetServerTime
```go
func (m Metrics) GetServerTime() time.Duration
```
Returns server processing time (TTFB).

##### GetNetworkTime
```go
func (m Metrics) GetNetworkTime() time.Duration
```
Returns network time excluding server processing.

##### String
```go
func (m Metrics) String() string
```
Returns human-readable representation.

**Example:**
```go
fmt.Printf("Connection: %v\n", resp.Timings.GetConnectionTime())
fmt.Printf("Server: %v\n", resp.Timings.GetServerTime())
fmt.Printf("Details: %s\n", resp.Timings.String())
```

## Error API

Structured error handling with context information.

### Error Types

```go
const (
    ErrorTypeDNS        = "dns"        // DNS resolution errors
    ErrorTypeConnection = "connection" // TCP connection errors
    ErrorTypeTLS        = "tls"        // TLS handshake errors
    ErrorTypeTimeout    = "timeout"    // Timeout errors
    ErrorTypeProtocol   = "protocol"   // HTTP protocol errors
    ErrorTypeIO         = "io"         // I/O errors
    ErrorTypeValidation = "validation" // Input validation errors
)
```

### Error

```go
type Error struct {
    Type      ErrorType `json:"type"`      // Error category
    Message   string    `json:"message"`   // Human-readable message
    Cause     error     `json:"cause"`     // Underlying error
    Host      string    `json:"host"`      // Target host (if applicable)
    Port      int       `json:"port"`      // Target port (if applicable)
    Timestamp time.Time `json:"timestamp"` // When error occurred
}
```

#### Methods

##### Error
```go
func (e *Error) Error() string
```
Implements error interface.

##### Unwrap
```go
func (e *Error) Unwrap() error
```
Returns underlying error for error wrapping.

##### Is
```go
func (e *Error) Is(target error) bool
```
Checks if error matches target type.

#### Helper Functions

##### IsTimeoutError
```go
func IsTimeoutError(err error) bool
```
Checks if error is timeout-related.

##### IsTemporaryError
```go
func IsTemporaryError(err error) bool
```
Checks if error is temporary/retryable.

##### GetErrorType
```go
func GetErrorType(err error) string
```
Returns error type string.

**Example:**
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    if rawErr, ok := err.(*rawhttp.Error); ok {
        switch rawErr.Type {
        case rawhttp.ErrorTypeDNS:
            log.Printf("DNS error for %s: %v", rawErr.Host, rawErr.Cause)
        case rawhttp.ErrorTypeConnection:
            log.Printf("Connection error to %s:%d: %v", rawErr.Host, rawErr.Port, rawErr.Cause)
        case rawhttp.ErrorTypeTimeout:
            log.Printf("Timeout error: %v", rawErr.Message)
        }
    }
    
    // Check specific conditions
    if rawhttp.IsTimeoutError(err) {
        // Implement retry logic
    }
    
    return err
}
```

## TLS Configuration

### SSL/TLS Version Control

Control SSL/TLS protocol versions for security and compatibility.

#### Basic Version Control

```go
import "github.com/WhileEndless/go-rawhttp/pkg/tlsconfig"

// Force TLS 1.3 only (most secure)
opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "modern-server.com",
    Port:          443,
    MinTLSVersion: tlsconfig.VersionTLS13,
    MaxTLSVersion: tlsconfig.VersionTLS13,
}

// TLS 1.2+ (recommended for production)
opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "api.example.com",
    Port:          443,
    MinTLSVersion: tlsconfig.VersionTLS12,
    MaxTLSVersion: tlsconfig.VersionTLS13,
}

// Legacy SSL 3.0 support (use with extreme caution)
opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "legacy-system.com",
    Port:          443,
    MinTLSVersion: tlsconfig.VersionSSL30,
    InsecureTLS:   true, // Required for deprecated versions
}
```

#### Using Security Profiles

```go
import "github.com/WhileEndless/go-rawhttp/pkg/tlsconfig"

// Apply pre-configured secure profile
tlsConf := &tls.Config{}
tlsconfig.ApplyVersionProfile(tlsConf, tlsconfig.ProfileSecure)
// Result: TLS 1.2-1.3 with secure defaults

opts := rawhttp.Options{
    Scheme:    "https",
    Host:      "example.com",
    Port:      443,
    TLSConfig: tlsConf,
}

// Available profiles:
// - ProfileModern:     TLS 1.3 only
// - ProfileSecure:     TLS 1.2-1.3 (recommended)
// - ProfileCompatible: TLS 1.0-1.3
// - ProfileLegacy:     SSL 3.0-TLS 1.3 (includes insecure versions)
```

#### Cipher Suite Control

```go
import "github.com/WhileEndless/go-rawhttp/pkg/tlsconfig"

// Use secure TLS 1.2 cipher suites
opts := rawhttp.Options{
    Scheme:       "https",
    Host:         "api.example.com",
    Port:         443,
    CipherSuites: tlsconfig.CipherSuitesTLS12Secure,
}

// Custom cipher suite selection
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "example.com",
    Port:   443,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
    },
}

// Automatic cipher suite selection based on TLS version
tlsConf := &tls.Config{}
tlsconfig.ApplyCipherSuites(tlsConf, tlsconfig.VersionTLS12)
```

#### TLS Renegotiation

```go
// Disable renegotiation (default, most secure)
opts := rawhttp.Options{
    Scheme:           "https",
    Host:             "example.com",
    Port:             443,
    TLSRenegotiation: tls.RenegotiateNever,
}

// Allow renegotiation once
opts := rawhttp.Options{
    Scheme:           "https",
    Host:             "example.com",
    Port:             443,
    TLSRenegotiation: tls.RenegotiateOnceAsClient,
}
```

**Warning**: TLS renegotiation can have security implications. Use `RenegotiateNever` unless specifically required.

### Client Certificates (mTLS)

Mutual TLS (mTLS) allows clients to authenticate themselves to the server using client certificates.

#### Using PEM Byte Arrays

```go
clientCertPEM := []byte(`-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCFcV9...
-----END CERTIFICATE-----`)

clientKeyPEM := []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhk...
-----END PRIVATE KEY-----`)

opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "mtls-server.example.com",
    Port:          443,
    ClientCertPEM: clientCertPEM,
    ClientKeyPEM:  clientKeyPEM,
}

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    log.Fatalf("mTLS request failed: %v", err)
}
defer resp.Body.Close()
```

#### Using File Paths

```go
opts := rawhttp.Options{
    Scheme:         "https",
    Host:           "mtls-server.example.com",
    Port:           443,
    ClientCertFile: "/path/to/client.crt",
    ClientKeyFile:  "/path/to/client.key",
}

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, opts)
```

#### mTLS with Custom CA

Combine client certificates with custom CA trust for self-signed server certificates:

```go
caCertPEM, _ := os.ReadFile("server-ca.pem")
clientCertPEM, _ := os.ReadFile("client.crt")
clientKeyPEM, _ := os.ReadFile("client.key")

opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "self-signed-mtls.example.com",
    Port:          8443,
    CustomCACerts: [][]byte{caCertPEM},  // Trust server's CA
    ClientCertPEM: clientCertPEM,        // Client authentication
    ClientKeyPEM:  clientKeyPEM,
}

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    log.Fatalf("mTLS handshake failed: %v", err)
}
defer resp.Body.Close()

fmt.Printf("TLS Version: %s\n", resp.TLSVersion)
fmt.Printf("Cipher Suite: %s\n", resp.TLSCipherSuite)
```

#### mTLS with HTTP/2

Client certificates work seamlessly with HTTP/2:

```go
opts := rawhttp.Options{
    Scheme:        "https",
    Host:          "h2-mtls.example.com",
    Port:          443,
    Protocol:      "http/2",
    ClientCertPEM: clientCertPEM,
    ClientKeyPEM:  clientKeyPEM,
}

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, opts)
// HTTP/2 with mTLS authentication
```

**Notes:**
- Client certificates can also be provided via `TLSConfig.Certificates`
- PEM data is preferred over file paths for better control
- Private keys must be unencrypted (no password protection)
- Supports both HTTP/1.1 and HTTP/2 protocols
- Certificates are validated during TLS handshake

## Advanced Usage

### Custom Transport Configuration

For advanced users who need custom transport behavior:

```go
import (
    "github.com/WhileEndless/go-rawhttp/pkg/client"
    "github.com/WhileEndless/go-rawhttp/pkg/transport"
)

// Create custom transport
customResolver := &net.Resolver{
    PreferGo: true,
    Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
        // Custom DNS resolution logic
    },
}

transport := transport.NewWithResolver(customResolver)
httpClient := client.NewWithTransport(transport)

// Use with rawhttp
sender := &rawhttp.Sender{Client: httpClient}
```

### Protocol Extension

The modular architecture allows for future protocol extensions:

```go
// Future HTTP/2 support example
type HTTP2Sender struct {
    // HTTP/2 specific implementation
}

func (s *HTTP2Sender) Do(ctx context.Context, req []byte, opts Options) (*Response, error) {
    // HTTP/2 implementation
}
```

## Thread Safety

- `Sender` is safe for concurrent use
- `Response` objects are NOT safe for concurrent use
- Each goroutine should handle its own `Response` objects
- `Buffer` objects are NOT safe for concurrent use

**Example:**
```go
sender := rawhttp.NewSender() // Create once

// Safe: multiple goroutines can use same sender
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        resp, err := sender.Do(ctx, request, opts)
        if err != nil {
            return
        }
        defer resp.Body.Close()
        defer resp.Raw.Close()
        
        // Process response...
    }(i)
}
wg.Wait()
```

## Constants and Defaults

```go
const (
    DefaultMemoryLimit = 4 * 1024 * 1024 // 4MB
    DefaultConnTimeout = 10 * time.Second
    DefaultReadTimeout = 30 * time.Second
    MaxHeaderBytes     = 64 * 1024        // 64KB
)
```

### PoolConfig (v2.1.0+)

Configuration for connection pool behavior.

```go
type PoolConfig struct {
    MaxIdleConnsPerHost int           // Max idle connections per host (default: 2)
    MaxConnsPerHost     int           // Max total connections per host (0 = unlimited)
    MaxIdleTime         time.Duration // Idle connection timeout (default: 90s)
    WaitTimeout         time.Duration // Wait for available connection (0 = no wait)
}
```

**Fields:**
- `MaxIdleConnsPerHost`: Maximum number of idle connections to keep per host. Default: 2
- `MaxConnsPerHost`: Maximum total connections (idle + active) per host. 0 means unlimited. Default: 0
- `MaxIdleTime`: How long an idle connection can remain in the pool. Default: 90 seconds
- `WaitTimeout`: How long to wait for an available connection when pool is exhausted. Default: 0 (no wait)

**Example:**
```go
config := rawhttp.PoolConfig{
    MaxIdleConnsPerHost: 5,
    MaxConnsPerHost:     10,
    MaxIdleTime:         90 * time.Second,
    WaitTimeout:         5 * time.Second,
}
sender := rawhttp.NewSenderWithPoolConfig(config)
```

### PoolStats (v2.1.0+)

Read-only statistics about the connection pool.

```go
type PoolStats struct {
    ActiveConns  int                      // Currently in use (checked out)
    IdleConns    int                      // Idle in pool (available)
    TotalReused  int                      // Lifetime reuse count
    TotalCreated int                      // Lifetime creation count
    WaitTimeouts int                      // Lifetime wait timeout count
    HostStats    map[string]HostPoolStats // Per-host statistics
}

type HostPoolStats struct {
    ActiveConns int // Active connections for this host
    IdleConns   int // Idle connections for this host
}
```

**Example:**
```go
stats := sender.PoolStats()
fmt.Printf("Active: %d, Idle: %d\n", stats.ActiveConns, stats.IdleConns)
fmt.Printf("Total Created: %d, Total Reused: %d\n", stats.TotalCreated, stats.TotalReused)

for host, hostStats := range stats.HostStats {
    fmt.Printf("%s: Active=%d, Idle=%d\n", host, hostStats.ActiveConns, hostStats.IdleConns)
}
```

### HTTP2Settings

Configuration for HTTP/2 protocol features.

```go
type HTTP2Settings struct {
    EnableServerPush     bool   // Enable server push (default: false)
    EnableCompression    bool   // Enable HPACK compression (default: true) 
    MaxConcurrentStreams uint32 // Max concurrent streams (default: 100)
    InitialWindowSize    uint32 // Flow control window size (default: 65535)
    MaxFrameSize         uint32 // Maximum frame size (default: 16384)
    MaxHeaderListSize    uint32 // Maximum header list size (default: 8192)
    HeaderTableSize      uint32 // HPACK table size (default: 4096)
}
```

**Example:**
```go
http2Settings := &rawhttp.HTTP2Settings{
    EnableCompression:    true,
    MaxConcurrentStreams: 200,
    InitialWindowSize:    1048576, // 1MB
    EnableServerPush:     false,
}

opts := rawhttp.Options{
    Scheme:        "https", 
    Host:          "api.example.com",
    Port:          443,
    Protocol:      "http/2",
    HTTP2Settings: http2Settings,
}
```

## Compatibility

- Go 1.21+
- HTTP/1.1 and HTTP/2 support
- ALPN negotiation for HTTPS
- H2C support for HTTP/2 over cleartext
- Linux, macOS, Windows
- Minimal dependencies: golang.org/x/net/http2