# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with go-rawhttp.

## Common Issues

### 1. Connection Issues

#### DNS Resolution Failures
```
Error: [dns] DNS lookup failed for host example.com: no such host
```

**Causes:**
- Invalid hostname
- Network connectivity issues
- DNS server problems

**Solutions:**
```go
// Use custom IP to bypass DNS
opts := rawhttp.Options{
    Scheme:    "https",
    Host:      "example.com",
    Port:      443,
    ConnectIP: "1.2.3.4", // Direct IP connection
}

// Or check DNS resolution separately
import "net"
ips, err := net.LookupIP("example.com")
if err != nil {
    log.Printf("DNS lookup failed: %v", err)
}
```

#### Connection Refused
```
Error: [connection] failed to connect to example.com:443: connection refused
```

**Causes:**
- Wrong port number
- Service not running
- Firewall blocking connection

**Solutions:**
```go
// Check if port is correct
opts.Port = 80  // for HTTP
opts.Port = 443 // for HTTPS

// Increase connection timeout
opts.ConnTimeout = 30 * time.Second

// Test connectivity with telnet/nc first:
// telnet example.com 443
```

#### TLS Handshake Failures
```
Error: [tls] TLS handshake failed for example.com:443: certificate verify failed
```

**Solutions:**
```go
// Skip certificate verification (development only!)
opts.InsecureTLS = true

// Custom SNI hostname
opts.SNI = "custom.example.com"

// Disable SNI entirely
opts.DisableSNI = true
```

### 2. Timeout Issues

#### Connection Timeouts
```go
// Adjust connection timeout
opts.ConnTimeout = 30 * time.Second

// Use context for overall timeout
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

resp, err := sender.Do(ctx, request, opts)
```

#### Read Timeouts
```go
// Increase read timeout for slow servers
opts.ReadTimeout = 120 * time.Second

// For large downloads
opts.ReadTimeout = 300 * time.Second
```

### 3. Memory Issues

#### Large Response Handling
```go
// Configure memory limits
opts.BodyMemLimit = 10 * 1024 * 1024 // 10MB before disk spill

// Check if response spilled to disk
if resp.Body.IsSpilled() {
    fmt.Printf("Response spilled to: %s", resp.Body.Path())
}

// Always close responses to free memory
defer resp.Body.Close()
defer resp.Raw.Close()
```

#### Memory Leaks
```go
// Always close responses
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    return err
}
defer func() {
    if resp != nil {
        resp.Body.Close()
        resp.Raw.Close()
    }
}()
```

### 4. HTTP/2 Specific Issues

#### FRAME_SIZE_ERROR
```
Error: [connection] failed to send settings: server sent GOAWAY during handshake: last stream 0, error PROTOCOL_ERROR
```

**Causes:**
- Incorrect HTTP/2 frame encoding
- HPACK compression issues
- Wrong SETTINGS parameters

**Solutions:**
```go
// Use production-ready HTTP/2 settings
opts.HTTP2Settings = &rawhttp.HTTP2Settings{
    EnableServerPush:     false,    // Recommended for security
    EnableCompression:    true,     // Enable HPACK compression
    InitialWindowSize:    4194304,  // 4MB - production optimized
    MaxFrameSize:         16384,    // 16KB - RFC compliant
    MaxHeaderListSize:    10485760, // 10MB header limit
    HeaderTableSize:      4096,     // 4KB HPACK table
}

// Disable connection reuse if having issues
// Each request creates a new connection

// Force specific protocol version
opts.Protocol = "http/2"
```

#### Connection Reuse Issues
```go
// Some servers don't support connection reuse well
// Use sequential requests for compatibility
// Note: Connection reuse not currently supported

// For concurrent scenarios, use separate connections
for i := 0; i < 10; i++ {
    go func(id int) {
        client := rawhttp.NewSender() // Separate client per goroutine
        // ... make request
    }(i)
}
```

#### ALPN Negotiation Failures
```
Error: [connection] server does not support HTTP/2 (negotiated: http/1.1)
```

**Solutions:**
```go
// Force HTTP/2 if server supports it
opts.Protocol = "http/2"

// Use H2C for cleartext HTTP/2
opts.Scheme = "http"
opts.Protocol = "http/2"
```

### 5. Protocol Issues

#### Invalid HTTP Requests
```go
// Ensure proper HTTP format
request := "GET / HTTP/1.1\r\n" +
    "Host: example.com\r\n" +
    "Connection: close\r\n" +
    "\r\n" // Empty line required

// Check request format
fmt.Printf("Request:\n%q\n", request)
```

#### Malformed Responses
```
Error: [protocol] invalid status line format
```

**Debug response:**
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    if resp != nil {
        // Check raw response
        raw, _ := resp.Raw.Reader()
        if raw != nil {
            data, _ := io.ReadAll(raw)
            fmt.Printf("Raw response:\n%s\n", string(data))
            raw.Close()
        }
    }
    return err
}
```

## Debugging Techniques

### 1. Enable Verbose Logging
```go
import "log"

// Log request details
log.Printf("Sending request to %s:%d", opts.Host, opts.Port)
log.Printf("Request: %q", string(request))

// Log response details
if resp != nil {
    log.Printf("Response: %d %s", resp.StatusCode, resp.StatusLine)
    log.Printf("Timings: %s", resp.Timings.String())
}
```

### 2. Network Analysis
```bash
# Test connectivity
telnet example.com 443

# Capture network traffic
sudo tcpdump -i any -s 0 -w capture.pcap host example.com

# Analyze with Wireshark or tcpdump
tcpdump -r capture.pcap -A
```

### 3. Compare with curl
```bash
# Test the same request with curl
curl -v -H "Host: example.com" https://example.com/path

# Save curl output for comparison
curl -v --trace-ascii trace.txt https://example.com/
```

### 4. Error Type Analysis
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    if rawErr, ok := err.(*rawhttp.Error); ok {
        fmt.Printf("Error Type: %s\n", rawErr.Type)
        fmt.Printf("Message: %s\n", rawErr.Message)
        fmt.Printf("Host: %s\n", rawErr.Host)
        fmt.Printf("Port: %d\n", rawErr.Port)
        fmt.Printf("Timestamp: %s\n", rawErr.Timestamp)
        
        // Check underlying cause
        if rawErr.Cause != nil {
            fmt.Printf("Cause: %v\n", rawErr.Cause)
        }
    }
}
```

## Performance Troubleshooting

### 1. Slow Connections
```go
// Analyze timing breakdown
fmt.Printf("DNS: %v\n", resp.Timings.DNS)
fmt.Printf("TCP: %v\n", resp.Timings.TCP)
fmt.Printf("TLS: %v\n", resp.Timings.TLS)
fmt.Printf("TTFB: %v\n", resp.Timings.TTFB)

// Identify bottlenecks
if resp.Timings.DNS > 5*time.Second {
    fmt.Println("DNS resolution is slow")
}
if resp.Timings.TCP > 10*time.Second {
    fmt.Println("TCP connection is slow")
}
```

### 2. HTTP/2 Performance Optimization
```go
// Note: Sequential requests create new connections

// Optimize window sizes for better throughput
opts.HTTP2Settings = &rawhttp.HTTP2Settings{
    InitialWindowSize:    4194304,  // 4MB - good for high bandwidth
    MaxFrameSize:         16384,    // 16KB - RFC compliant
    MaxHeaderListSize:    10485760, // 10MB - handles large headers
    EnableCompression:    true,     // Reduce header overhead
}

// Monitor HTTP/2 specific metrics
fmt.Printf("Protocol: %s\n", resp.HTTPVersion)
fmt.Printf("Response size: %d bytes\n", resp.BodyBytes)
```

### 3. Memory Usage
```go
// Monitor memory usage
import "runtime"

var m runtime.MemStats
runtime.ReadMemStats(&m)
fmt.Printf("Memory usage: %d KB\n", m.Alloc/1024)

// Use smaller buffer limits for memory-constrained environments
opts.BodyMemLimit = 1024 * 1024 // 1MB

// Check for memory leaks in production
func TestMemoryUsage() {
    runtime.GC()
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    // Make multiple requests
    for i := 0; i < 100; i++ {
        resp, err := sender.Do(ctx, request, opts)
        if err == nil {
            resp.Body.Close()
            resp.Raw.Close()
        }
    }
    
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    // Memory growth should be minimal
    memDiff := int64(m2.Alloc) - int64(m1.Alloc)
    if memDiff > 1024*1024 { // > 1MB
        fmt.Printf("Warning: Memory leak detected: %d bytes\n", memDiff)
    }
}
```

### 4. Production Performance Monitoring
```go
// Track request rates and success rates
type Metrics struct {
    TotalRequests   int64
    SuccessRequests int64
    ErrorRequests   int64
    TotalTime       time.Duration
    mu              sync.Mutex
}

func (m *Metrics) Record(success bool, duration time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.TotalRequests++
    m.TotalTime += duration
    
    if success {
        m.SuccessRequests++
    } else {
        m.ErrorRequests++
    }
}

func (m *Metrics) Stats() (successRate float64, avgTime time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if m.TotalRequests == 0 {
        return 0, 0
    }
    
    successRate = float64(m.SuccessRequests) / float64(m.TotalRequests)
    avgTime = m.TotalTime / time.Duration(m.TotalRequests)
    return
}
```

## Best Practices

### 1. Error Handling
```go
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    // Always check error type
    switch rawhttp.GetErrorType(err) {
    case rawhttp.ErrorTypeDNS:
        // Retry with different DNS or IP
    case rawhttp.ErrorTypeConnection:
        // Retry with different port or timeout
    case rawhttp.ErrorTypeTimeout:
        // Retry with longer timeout
    default:
        // Log and return
        return fmt.Errorf("request failed: %w", err)
    }
}

// Always close resources
defer resp.Body.Close()
defer resp.Raw.Close()
```

### 2. Resource Management
```go
// Create sender once, reuse multiple times
sender := rawhttp.NewSender()

// Configure reasonable limits
opts := rawhttp.Options{
    ConnTimeout:  10 * time.Second,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 10 * time.Second,
    BodyMemLimit: 5 * 1024 * 1024, // 5MB
}

// Use contexts for timeout control
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
```

### 3. Testing
```go
func TestHTTPRequest(t *testing.T) {
    sender := rawhttp.NewSender()
    request := []byte("GET / HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n")
    
    resp, err := sender.Do(context.Background(), request, 
        rawhttp.DefaultOptions("https", "httpbin.org", 443))
    
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()
    defer resp.Raw.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

## Getting Help

1. Check the examples in the `examples/` directory
2. Review the API documentation in `README.md`
3. Search for similar issues in the repository
4. Create a minimal reproduction case
5. Include relevant debug output when reporting issues