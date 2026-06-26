# Migration Guide

This guide helps you migrate from other HTTP clients to go-rawhttp and between versions of go-rawhttp.

## From net/http

### Basic HTTP Request

**Before (net/http):**
```go
import "net/http"

resp, err := http.Get("https://api.example.com/users")
if err != nil {
    return err
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
if err != nil {
    return err
}
```

**After (go-rawhttp):**
```go
import "github.com/WhileEndless/go-rawhttp"

sender := rawhttp.NewSender()
request := []byte("GET /users HTTP/1.1\r\nHost: api.example.com\r\nConnection: close\r\n\r\n")
opts := rawhttp.DefaultOptions("https", "api.example.com", 443)

resp, err := sender.Do(context.Background(), request, opts)
if err != nil {
    return err
}
defer resp.Body.Close()
defer resp.Raw.Close()

reader, err := resp.Body.Reader()
if err != nil {
    return err
}
defer reader.Close()

body, err := io.ReadAll(reader)
if err != nil {
    return err
}
```

### POST Request with JSON

**Before (net/http):**
```go
import (
    "bytes"
    "encoding/json"
    "net/http"
)

data := map[string]string{"name": "test"}
jsonData, _ := json.Marshal(data)

resp, err := http.Post("https://api.example.com/users", 
    "application/json", bytes.NewBuffer(jsonData))
```

**After (go-rawhttp):**
```go
import "github.com/WhileEndless/go-rawhttp"

data := `{"name": "test"}`
request := fmt.Sprintf(
    "POST /users HTTP/1.1\r\n"+
    "Host: api.example.com\r\n"+
    "Content-Type: application/json\r\n"+
    "Content-Length: %d\r\n"+
    "Connection: close\r\n\r\n"+
    "%s", len(data), data)

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), []byte(request), 
    rawhttp.DefaultOptions("https", "api.example.com", 443))
```

### Custom Headers

**Before (net/http):**
```go
req, _ := http.NewRequest("GET", "https://api.example.com/users", nil)
req.Header.Set("Authorization", "Bearer token123")
req.Header.Set("User-Agent", "MyApp/1.0")

client := &http.Client{}
resp, err := client.Do(req)
```

**After (go-rawhttp):**
```go
request := []byte(
    "GET /users HTTP/1.1\r\n" +
    "Host: api.example.com\r\n" +
    "Authorization: Bearer token123\r\n" +
    "User-Agent: MyApp/1.0\r\n" +
    "Connection: close\r\n\r\n")

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, 
    rawhttp.DefaultOptions("https", "api.example.com", 443))
```

### Timeout Configuration

**Before (net/http):**
```go
client := &http.Client{
    Timeout: 30 * time.Second,
}
resp, err := client.Get("https://api.example.com/users")
```

**After (go-rawhttp):**
```go
opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "api.example.com",
    Port:        443,
    ConnTimeout: 10 * time.Second,
    ReadTimeout: 30 * time.Second,
}

sender := rawhttp.NewSender()
resp, err := sender.Do(context.Background(), request, opts)
```

## From FastHTTP

### Basic Request

**Before (fasthttp):**
```go
import "github.com/valyala/fasthttp"

req := fasthttp.AcquireRequest()
defer fasthttp.ReleaseRequest(req)
resp := fasthttp.AcquireResponse()
defer fasthttp.ReleaseResponse(resp)

req.SetRequestURI("https://api.example.com/users")
req.Header.SetMethod("GET")

err := fasthttp.Do(req, resp)
```

**After (go-rawhttp):**
```go
sender := rawhttp.NewSender()
request := []byte("GET /users HTTP/1.1\r\nHost: api.example.com\r\nConnection: close\r\n\r\n")

resp, err := sender.Do(context.Background(), request, 
    rawhttp.DefaultOptions("https", "api.example.com", 443))
if err != nil {
    return err
}
defer resp.Body.Close()
defer resp.Raw.Close()
```

## From Previous go-rawhttp Versions

### Version 1.x to 2.x (Current)

#### Breaking Changes

1. **Package Structure**: Moved from single-file to modular structure
2. **Error Handling**: Introduced structured errors
3. **Buffer API**: Enhanced with disk spilling

#### Migration Steps

**Old API (v1.x):**
```go
import "github.com/WhileEndless/go-rawhttp"

sender := rawhttp.NewSender()
resp, err := sender.Do(ctx, request, rawhttp.Options{...})
if err != nil {
    // Simple error handling
    log.Printf("Request failed: %v", err)
}

// Old buffer access
data := resp.Body.Bytes()
```

**New API (v2.x):**
```go
import "github.com/WhileEndless/go-rawhttp"

sender := rawhttp.NewSender()
resp, err := sender.Do(ctx, request, rawhttp.Options{...})
if err != nil {
    // Structured error handling
    if rawErr, ok := err.(*rawhttp.Error); ok {
        switch rawErr.Type {
        case rawhttp.ErrorTypeDNS:
            log.Printf("DNS error: %v", rawErr)
        case rawhttp.ErrorTypeConnection:
            log.Printf("Connection error: %v", rawErr)
        }
    }
}
defer resp.Body.Close()
defer resp.Raw.Close()

// New buffer access
reader, err := resp.Body.Reader()
if err != nil {
    return err
}
defer reader.Close()
data, err := io.ReadAll(reader)
```

## Key Differences and Considerations

### Performance Characteristics

| Feature | net/http | fasthttp | go-rawhttp |
|---------|----------|----------|------------|
| Connection Pooling | ✅ | ✅ | N/A |
| Raw Socket Access | ❌ | ❌ | ✅ |
| Memory Efficiency | Medium | High | High |
| Timing Metrics | Basic | Basic | Detailed |
| Error Details | Basic | Basic | Structured |
| Protocol Support | HTTP/1.1, HTTP/2 | HTTP/1.1 | HTTP/1.1 |

### When to Choose go-rawhttp

**Choose go-rawhttp when:**
- You need raw socket control
- Detailed timing metrics are required
- Custom protocol handling is needed
- Memory efficiency for large responses is important
- Structured error handling is valuable
- Testing/debugging HTTP implementations

**Consider alternatives when:**
- You need advanced connection management
- HTTP/2 support is required
- Simple HTTP requests are sufficient
- Performance is critical over control

### Migration Checklist

#### From net/http
- [ ] Convert URL-based requests to raw HTTP format
- [ ] Replace `http.Client` with `rawhttp.Sender`
- [ ] Update header handling (manual formatting required)
- [ ] Add explicit connection closing (`Connection: close`)
- [ ] Update response body reading (use `resp.Body.Reader()`)
- [ ] Add proper resource cleanup (`defer resp.Body.Close()`)

#### From fasthttp
- [ ] Replace request/response acquisition with direct creation
- [ ] Convert URI setting to raw HTTP requests
- [ ] Update header access methods
- [ ] Modify timeout configuration
- [ ] Add structured error handling

### Common Pitfalls

1. **Forgetting Connection Headers**
```go
// Wrong: might hang waiting for more requests
request := "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"

// Right: explicitly close connection
request := "GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
```

2. **Not Closing Resources**
```go
// Wrong: memory leak
resp, err := sender.Do(ctx, request, opts)
return resp.StatusCode

// Right: proper cleanup
resp, err := sender.Do(ctx, request, opts)
if err != nil {
    return 0, err
}
defer resp.Body.Close()
defer resp.Raw.Close()
return resp.StatusCode, nil
```

3. **Incorrect Content-Length**
```go
data := "test data"
// Wrong: incorrect content length
request := fmt.Sprintf("POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 10\r\n\r\n%s", data)

// Right: calculate correct length
request := fmt.Sprintf("POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: %d\r\n\r\n%s", 
    len(data), data)
```

4. **Ignoring Error Types**
```go
// Basic: miss retry opportunities
if err != nil {
    return err
}

// Better: handle specific errors
if err != nil {
    if rawhttp.IsTimeoutError(err) {
        // Retry with longer timeout
        return retryWithTimeout(request, opts)
    }
    return err
}
```

### Testing Migration

Create a test suite to verify migration:

```go
func TestMigration(t *testing.T) {
    // Test basic functionality
    sender := rawhttp.NewSender()
    request := []byte("GET / HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n")
    
    resp, err := sender.Do(context.Background(), request, 
        rawhttp.DefaultOptions("https", "httpbin.org", 443))
    
    if err != nil {
        t.Fatalf("Basic request failed: %v", err)
    }
    defer resp.Body.Close()
    defer resp.Raw.Close()
    
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
    
    // Test error handling
    _, err = sender.Do(context.Background(), request, 
        rawhttp.DefaultOptions("https", "nonexistent.example", 443))
    
    if err == nil {
        t.Error("Expected error for nonexistent host")
    }
    
    if !rawhttp.IsTimeoutError(err) && rawhttp.GetErrorType(err) != rawhttp.ErrorTypeDNS {
        t.Errorf("Unexpected error type: %s", rawhttp.GetErrorType(err))
    }
}
```

This migration guide should help you transition smoothly to go-rawhttp while taking advantage of its advanced features.