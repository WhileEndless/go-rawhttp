# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.5] - 2025-11-24

### 🔧 Fixes

**Git Tag Correction**

Fixed v2.0.4 tag pointing issue. Released as v2.0.5 to ensure proper module resolution.

- Tag v2.0.4 initially pointed to wrong commit (before /v2 migration)
- Go module cache caused resolution issues
- v2.0.5 ensures clean /v2 module path from start

**No functional changes from v2.0.4** - only version bump for proper module resolution.

## [2.0.4] - 2025-11-24 (SKIP - Use v2.0.5 instead)

### ⚠️ BREAKING CHANGE: Module Path Migration

**Module path has been updated to `/v2` to follow Go modules semantic import versioning best practices.**

**What changed:**
```go
// OLD (v2.0.3 and earlier):
import "github.com/WhileEndless/go-rawhttp"

// NEW (v2.0.4+):
import rawhttp "github.com/WhileEndless/go-rawhttp/v2"
```

**Migration steps:**
1. Update all import statements to include `/v2` suffix
2. Run `go get github.com/WhileEndless/go-rawhttp/v2`
3. Run `go mod tidy`

**Why this change:**
- Follows Go modules semantic versioning specification
- Enables proper version resolution: `go get github.com/WhileEndless/go-rawhttp/v2@latest`
- Prevents import conflicts between major versions
- Industry standard for Go libraries (e.g., `google.golang.org/grpc`, `gopkg.in/yaml.v3`)

**Impact:**
- All existing v2.0.0-2.0.3 users must update imports
- No functional changes - only import path differs
- GitHub repository URL remains unchanged

### 🐛 Bug Fixes

**HTTP/1.1 ALPN Protocol Enforcement**

Enhanced HTTP/1.1 protocol enforcement to prevent unexpected HTTP/2 negotiation with custom TLS configurations.

**Changes:**
- Force `NextProtos = ["http/1.1"]` for all HTTP/1.1 transport connections
- Applied to both default and custom TLSConfig paths
- Ensures HTTP/1.1 transport never negotiates HTTP/2 via ALPN
- Users wanting HTTP/2 must explicitly use `Protocol: "http/2"`

**Why:** When users provide custom TLSConfig without NextProtos, Go's TLS library could negotiate HTTP/2 with servers that advertise h2 support, causing protocol mismatch in HTTP/1.1 transport.

**Files:** `pkg/transport/transport.go` (line 388-392)

**Proxy Connection Optimization**

Added `Connection: keep-alive` header to HTTP proxy CONNECT requests for better connection management.

**Files:** `pkg/transport/transport.go` (line 836)

### 🧹 Code Quality

- Removed debug logging statements used during development
- Cleaned up temporary test files
- Enhanced CHANGELOG documentation with detailed fix descriptions

## [2.0.3] - 2025-11-24

### ✨ New Features

**HTTP/2 Proxy Support**

Added comprehensive proxy support for HTTP/2 protocol, matching the existing HTTP/1.1 proxy capabilities. HTTP/2 now fully supports HTTP, HTTPS, SOCKS4, and SOCKS5 proxies.

**1. HTTP/HTTPS Proxy Support via CONNECT Tunnel**

HTTP/2 can now be used through HTTP and HTTPS proxies using the standard CONNECT tunnel method (RFC 7231).

```go
sender := rawhttp.NewSender()
opts := rawhttp.Options{
    Host:     "example.com",
    Port:     443,
    Scheme:   "https",
    Protocol: "http/2",  // Now works with proxy!
    Proxy: &rawhttp.ProxyConfig{
        Type: "http",
        Host: "proxy.com",
        Port: 8080,
    },
}
resp, err := sender.Do(ctx, req, opts)
```

**How it works:**
- Establishes HTTP/1.1 CONNECT tunnel to proxy
- Upgrades to TLS inside the tunnel
- Negotiates HTTP/2 via ALPN (h2)
- All subsequent HTTP/2 frames sent through encrypted tunnel
- Proxy sees CONNECT request, tunnel contents are encrypted

**Features:**
- Full CONNECT tunnel implementation
- TLS upgrade inside tunnel
- Proxy authentication (Basic auth)
- Custom proxy headers
- HTTPS proxy support (TLS to proxy)
- Proper error handling and reporting

**Files:** `pkg/http2/transport.go` (connectViaHTTPProxy, connectViaProxy)

**2. SOCKS4 Proxy Support for HTTP/2**

Added SOCKS4 protocol support for HTTP/2, matching HTTP/1.1 implementation.

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type:     "socks4",
    Host:     "proxy.com",
    Port:     1080,
    Username: "user", // SOCKS4 user ID
}
```

**Features:**
- IPv4-only support (SOCKS4 limitation)
- User ID authentication
- Manual SOCKS4 protocol implementation
- Proper response code handling (0x5A=success, 0x5B-5D=failures)

**Files:** `pkg/http2/transport.go` (connectViaSOCKS4Proxy)

**3. SOCKS5 Proxy Support for HTTP/2**

Added SOCKS5 protocol support for HTTP/2, matching HTTP/1.1 implementation.

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type:     "socks5",
    Host:     "proxy.com",
    Port:     1080,
    Username: "user",
    Password: "pass",
}
```

**Features:**
- IPv4 and IPv6 support
- Username/password authentication (RFC 1929)
- Uses golang.org/x/net/proxy library
- DNS resolution via proxy supported

**Files:** `pkg/http2/transport.go` (connectViaSOCKS5Proxy)

**4. Proxy Configuration Pass-through**

Enhanced the main library interface to automatically pass proxy configuration from Options to HTTP/2 client.

**Changes:**
- `rawhttp.go` - Added convertToHTTP2Options() to pass proxy config
- Automatic mapping of ProxyConfig from client.Options to http2.Options
- Maintains consistency between HTTP/1.1 and HTTP/2 proxy handling

**Files:** `rawhttp.go` (convertToHTTP2Options)

### 🐛 Bug Fixes

**Connection Pooling with Proxy**

Fixed critical connection pooling bug where connections were incorrectly shared across different proxy configurations.

**Issue:**
- Connection pool used simple "host:port" key format
- Different proxies to same target shared connections incorrectly
- Proxied and direct connections to same target shared pool
- Connection reuse broken when using proxies

**Impact:**
- Connections through proxy A were reused for proxy B
- Direct connections mixed with proxied connections
- Incorrect routing of requests
- Potential security and privacy issues

**Fix:**

1. **HTTP/2 Pooling** (`pkg/http2/transport.go`):
   - Modified pool key to include proxy information
   - New format: `"proxy_type:proxy_host:proxy_port->target_host:target_port"`
   - Example: `"http:127.0.0.1:8080->example.com:443"`
   - Direct connections still use: `"example.com:443"`

2. **HTTP/1.1 Pooling** (`pkg/transport/transport.go`):
   - Applied same proxy-aware pool key strategy
   - Maintains consistency with HTTP/2 implementation
   - Ensures proper connection isolation

**Pool Key Examples:**
```
With HTTP proxy:  "http:proxy.com:8080->example.com:443"
With SOCKS5:      "socks5:proxy.com:1080->example.com:443"
Direct (no proxy): "example.com:443"
```

**Testing:**
- Added comprehensive pooling test suite (`cmd/pooling_test/main.go`)
- Tests HTTP/1.1 connection reuse with proxy
- Tests HTTP/2 connection reuse with proxy
- Verifies different proxy configs use different connections

**Files:** `pkg/http2/transport.go`, `pkg/transport/transport.go`

**Connection Liveness Check False-Positive**

Fixed issue where healthy pooled connections were incorrectly marked as dead, preventing connection reuse.

**Issue:**
- `isConnectionAlive()` used `SetReadDeadline(immediate)` to check connection status
- Any buffered data (HTTP/2 control frames, TLS records) triggered false-positive "connection closed"
- Resulted in healthy connections being discarded from pool
- Connection pooling effectively disabled despite `ReuseConnection: true`

**Fix:**
- Skip liveness check for recently used connections (<5 seconds)
- Recent connections are highly likely to be alive
- Only check stale connections (unused for >5 seconds)
- Reduces false-positives while maintaining connection health

**Files:** `pkg/transport/transport.go` (line 518-530)

**HTTP/1.1 ALPN Protocol Enforcement**

Fixed issue where HTTP/1.1 transport could negotiate HTTP/2 with servers when user's custom TLSConfig didn't explicitly set NextProtos.

**Issue:**
- User provides custom `TLSConfig` without `NextProtos`
- Server advertises HTTP/2 support via ALPN
- Go TLS library negotiates HTTP/2 by default
- HTTP/1.1 transport receives HTTP/2 protocol unexpectedly

**Fix:**
- Force `NextProtos = ["http/1.1"]` for all HTTP/1.1 transport connections
- Applied to both default and custom TLSConfig paths
- Ensures HTTP/1.1 transport never negotiates HTTP/2
- Users wanting HTTP/2 must explicitly use `Protocol: "http/2"`

**Files:** `pkg/transport/transport.go` (line 388-399)

### 🧪 Testing

**New Test Program:**
- `cmd/pooling_test/main.go` - Comprehensive connection pooling tests with proxy
  - Test 1: HTTP/1.1 connection reuse verification
  - Test 2: HTTP/2 connection reuse verification
  - Test 3: Proxy isolation verification (different proxies = different connections)

### 📝 Technical Details

**Proxy Support Implementation:**
- HTTP/2 CONNECT tunnel: Full RFC 7231 compliance
- SOCKS4: Manual implementation, IPv4-only
- SOCKS5: Uses golang.org/x/net/proxy library
- TLS upgrade inside proxy tunnel for HTTPS
- ALPN negotiation (h2) after tunnel establishment

**Connection Pooling:**
- Pool key now includes full proxy context
- Default ports applied when not specified (http=8080, https=443, socks=1080)
- Backward compatible: direct connections unaffected
- Thread-safe pool key generation

**Commits:**
- `2772aae` - feat: Add HTTP/2 proxy support via CONNECT tunnel
- `70d3ef6` - feat: Add SOCKS4/SOCKS5 proxy support for HTTP/2
- `248037a` - fix: Implement proxy-aware connection pooling for HTTP/1.1 and HTTP/2

### Migration Guide

**No Breaking Changes** - All changes are backward compatible.

**New HTTP/2 Proxy Support:**
```go
// HTTP/2 now works through any proxy type
opts := rawhttp.Options{
    Protocol: "http/2",
    Proxy: &rawhttp.ProxyConfig{
        Type: "http",      // or "https", "socks4", "socks5"
        Host: "proxy.com",
        Port: 8080,
    },
}
```

**Connection Pooling:**
- Automatically fixed - no code changes needed
- Different proxies now properly use separate connections
- Pool statistics accurate for proxy scenarios

**Verification:**
```go
resp, _ := sender.Do(ctx, req, opts)
fmt.Printf("Proxy Used: %v (%s)\n", resp.ProxyUsed, resp.ProxyAddr)
fmt.Printf("Protocol: %s\n", resp.NegotiatedProtocol)
fmt.Printf("Connection Reused: %v\n", resp.ConnectionReused)
```

---

## [2.0.1] - 2025-11-23

### 🔧 Fixed

**Protocol Negotiation with Proxy**

- Improved protocol detection when using upstream proxies
- When proxy is configured without explicit Protocol setting, HTTP/1.1 is now preferred (since HTTP/2 transport doesn't support proxies)
- When TLSConfig.NextProtos is set without "h2", HTTP/1.1 is automatically selected
- This resolves issues where users expected HTTP/1.1 but HTTP/2 was being attempted

**Changes:**
- Enhanced `detectProtocol()` in rawhttp.go to respect proxy configuration
- Added automatic HTTP/1.1 selection when proxy is used without Protocol field
- Added automatic HTTP/1.1 selection when NextProtos excludes "h2"
- Improved documentation in HTTP/2 transport about ALPN behavior

**Impact:**
- No breaking changes
- Better automatic protocol selection for proxy scenarios
- Explicit Protocol setting still takes highest priority

**Test Coverage:**
- Added comprehensive protocol detection tests with proxy scenarios
- All existing tests pass without modification

## [2.0.0] - 2025-11-23

### 🔴 BREAKING CHANGES

**Proxy Configuration API Redesign**

The proxy configuration has been completely redesigned for better clarity, extensibility, and developer experience. The simple `ProxyURL` string field has been removed in favor of a dedicated `ProxyConfig` struct and `ParseProxyURL()` helper function.

#### Breaking Change: `ProxyURL` Field Removed

**What changed:**
```go
// OLD (v1.x) - REMOVED
type Options struct {
    ProxyURL string
}

// NEW (v2.0.0)
type Options struct {
    Proxy *ProxyConfig
}
```

**Migration:**

Before (v1.x):
```go
opts := rawhttp.Options{
    ProxyURL: "socks5://user:pass@proxy:1080",
}
```

After (v2.0.0) - Simple:
```go
opts := rawhttp.Options{
    Proxy: rawhttp.ParseProxyURL("socks5://user:pass@proxy:1080"),
}
```

After (v2.0.0) - Advanced:
```go
opts := rawhttp.Options{
    Proxy: &rawhttp.ProxyConfig{
        Type:        "socks5",
        Host:        "proxy.com",
        Port:        1080,
        Username:    "user",
        Password:    "pass",
        ConnTimeout: 10 * time.Second,
        ProxyHeaders: map[string]string{"X-Custom": "value"},
    },
}
```

**See:** `docs/MIGRATION_V2.md` for complete migration guide.

### ✨ New Features

#### 1. SOCKS4 Proxy Support

Added support for SOCKS4 protocol (RFC 1928) for legacy proxy compatibility.

```go
opts.Proxy = rawhttp.ParseProxyURL("socks4://user@proxy.com:1080")
```

**Features:**
- IPv4-only support (SOCKS4 limitation)
- User ID authentication
- Local DNS resolution (SOCKS4 requires IPv4 addresses)
- Proper error codes (0x5A=success, 0x5B/5C/5D=various failures)

**Files:** `pkg/transport/transport.go` (SOCKS4 implementation)
**Tests:** `tests/unit/proxy_parser_test.go`

#### 2. ProxyConfig Struct

New dedicated struct for advanced proxy configuration with fine-grained control.

```go
type ProxyConfig struct {
    Type               string            // "http", "https", "socks4", "socks5"
    Host               string            // Proxy hostname
    Port               int               // Proxy port (defaults: http=8080, https=443, socks=1080)
    Username           string            // Authentication username
    Password           string            // Authentication password
    ConnTimeout        time.Duration     // Proxy-specific connection timeout
    ProxyHeaders       map[string]string // Custom headers (HTTP/HTTPS only)
    TLSConfig          *tls.Config       // Custom TLS config for HTTPS proxy
    ResolveDNSViaProxy bool              // DNS via proxy (SOCKS5 only)
}
```

**Use cases:**
- Corporate proxies requiring custom headers
- Different timeouts for proxy vs target
- HTTPS proxies with self-signed certificates
- Fine-grained control over SOCKS5 DNS resolution

**Files:** `pkg/client/client.go`, `pkg/transport/transport.go`

#### 3. ParseProxyURL Helper Function

Convenient helper to parse proxy URLs into ProxyConfig.

```go
func ParseProxyURL(proxyURL string) (*ProxyConfig, error)
```

**Supported formats:**
- `http://proxy:8080` - HTTP proxy
- `http://user:pass@proxy:8080` - HTTP with auth
- `https://proxy:443` - HTTPS proxy
- `socks4://proxy:1080` - SOCKS4 proxy
- `socks4://user@proxy:1080` - SOCKS4 with user ID
- `socks5://proxy:1080` - SOCKS5 proxy
- `socks5://user:pass@proxy:1080` - SOCKS5 with auth

**Auto-applies default ports** when not specified.

**Files:** `pkg/client/proxy_parser.go`
**Tests:** `tests/unit/proxy_parser_test.go` (8 test suites, all passing)

#### 4. Custom Headers for HTTP Proxies

Add custom headers to HTTP CONNECT requests.

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type: "http",
    Host: "proxy.com",
    Port: 8080,
    ProxyHeaders: map[string]string{
        "X-Request-ID":     "12345",
        "X-Department":     "Engineering",
        "Proxy-Connection": "keep-alive",
    },
}
```

**Use cases:**
- Corporate proxy requirements
- Request tracking
- Custom proxy authentication schemes
- Debugging proxy traffic

**Files:** `pkg/transport/transport.go` (connectViaHTTPProxy)

#### 5. Proxy-Specific Timeout

Configure timeout specifically for proxy connection, separate from target timeout.

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type:        "socks5",
    Host:        "slow-proxy.com",
    Port:        1080,
    ConnTimeout: 30 * time.Second, // 30s for slow proxy
}
opts.ConnTimeout = 10 * time.Second // 10s for target server
```

**Files:** `pkg/transport/transport.go` (connectViaProxy)

#### 6. Proxy Metadata in Response

Response now includes detailed proxy information.

```go
type Response struct {
    // ... existing fields ...

    // Proxy metadata (v2.0.0+)
    ProxyUsed bool   // Whether request went through proxy
    ProxyType string // "http", "https", "socks4", "socks5"
    ProxyAddr string // "proxy.com:8080"
}
```

**Example:**
```go
resp, _ := sender.Do(ctx, req, opts)
if resp.ProxyUsed {
    fmt.Printf("Proxied via %s at %s\n", resp.ProxyType, resp.ProxyAddr)
}
```

**Files:** `pkg/client/client.go`, `pkg/transport/transport.go`

#### 7. ProxyError Type

Dedicated error type for proxy-specific failures.

```go
type ProxyError struct {
    ProxyType string // "http", "https", "socks4", "socks5"
    ProxyAddr string // "proxy.com:8080"
    Operation string // "connect", "auth", "handshake", "tunnel"
    Err       error  // Underlying error
    Timestamp time.Time
}
```

**Example:**
```go
resp, err := sender.Do(ctx, req, opts)
if err != nil {
    var proxyErr *rawhttp.ProxyError
    if errors.As(err, &proxyErr) {
        fmt.Printf("Proxy %s at %s failed during %s: %v\n",
            proxyErr.ProxyType, proxyErr.ProxyAddr,
            proxyErr.Operation, proxyErr.Err)
    }
}
```

**Files:** `pkg/errors/errors.go`

### 🐛 Bug Fixes

None (this is a feature release with breaking changes).

### 📝 Documentation

- **Added:** `docs/MIGRATION_V2.md` - Complete migration guide from v1.x
- **Added:** `examples/proxy_comprehensive.go` - All proxy types with examples
- **Updated:** `README.md` - Proxy section rewritten with v2.0 API
- **Updated:** `docs/PROXY_IMPLEMENTATION_PLAN_V2.md` - Implementation plan

### 🧪 Testing

**New Tests:**
- `tests/unit/proxy_parser_test.go` - ParseProxyURL tests (8 suites, all passing)
- Tests for HTTP proxy custom headers
- Tests for SOCKS4 protocol
- Tests for ProxyConfig advanced options
- Tests for ProxyError handling

**Test Coverage:**
- All existing tests passing (48 unit tests, integration tests)
- No regressions detected
- Proxy tests: 100% coverage

### 🚀 Performance

No performance impact. Proxy connections use the same underlying protocol implementations.

### 📋 Migration Checklist

If you're migrating from v1.x:

✅ Search codebase for `ProxyURL`
✅ Replace with `Proxy: rawhttp.ParseProxyURL(...)`
✅ Run tests to verify proxy functionality
✅ Consider using advanced ProxyConfig features if needed
✅ Update to v2.0.0

See `docs/MIGRATION_V2.md` for detailed guide.

---

## [1.1.6] - 2025-11-21

### Fixed - Critical Bugs and Stability Improvements

This release addresses multiple critical bugs discovered during comprehensive code analysis:

#### 🔴 Critical Bugs Fixed (BUG-1,2,3,4,6,8)

**BUG-1: HTTP/2 Time Calculation Always Returns Zero**
- **Issue**: `time.Since(time.Now())` always returned ~0 duration
- **Impact**: HTTP/2 metrics completely useless
- **Fix**: Added `TotalTime` field to HTTP/2 response, capture actual start time
- **Files**: `pkg/http2/types.go`, `pkg/http2/client.go`, `rawhttp.go`

**BUG-2: Goroutine Leaks in Transport Layers**
- **Issue**: Background goroutines (cleanup, health checker) never stopped
- **Impact**: Memory leaks in long-running applications
- **Fix**: Added lifecycle management (stopChan, WaitGroup, Close() methods)
- **Files**: `pkg/transport/transport.go`, `pkg/http2/transport.go`

**BUG-3: Race Condition in HTTP/2 Options**
- **Issue**: Concurrent requests mutated shared `c.transport.options`
- **Impact**: Data races, options from one request used in another
- **Fix**: Pass options down call chain instead of mutating shared state
- **Files**: `pkg/http2/transport.go`, `pkg/http2/client.go`

**BUG-4: Concurrent Write Panic in PING**
- **Issue**: `WritePing` called without `conn.mu` lock
- **Impact**: Application crashes during health checks
- **Fix**: Added lock before WritePing
- **Files**: `pkg/http2/transport.go`

**BUG-6: Buffer Write After Close**
- **Issue**: Temp file created but not stored immediately, causing leak
- **Impact**: File descriptor leaks
- **Fix**: Store file reference immediately after creation
- **Files**: `pkg/buffer/buffer.go`

**BUG-8: Hardcoded Context Timeout Message**
- **Issue**: Error message always showed "30 seconds" regardless of actual timeout
- **Impact**: Misleading error messages
- **Fix**: Calculate actual elapsed time from context deadline
- **Files**: `pkg/http2/client.go`

**BUG-9: isConnectionAlive Documentation**
- **Issue**: Conservative approach may mark good connections as dead
- **Impact**: Unnecessary connection recreation (acceptable)
- **Fix**: Added documentation explaining behavior
- **Files**: `pkg/transport/transport.go`

#### ⚡ Improvements (DEF-1,2,3,4,5,6,7,9,13,14,15)

**DEF-1: Conflicting Options Validation**
- Added validation: `DisableSNI=true` && `SNI != ""` is an error
- Prevents configuration mistakes
- **Files**: `pkg/transport/transport.go`

**DEF-2: Excessive Memory Allocation**
- Fixed raw buffer calculation: was always allocating 1GB per response
- Now: `BodyMemLimit + 1MB overhead`, capped at 100MB max
- Prevents OOM in high-concurrency scenarios
- **Files**: `pkg/client/client.go`

**DEF-3: Magic Numbers Centralized**
- Created `pkg/constants` package for all magic numbers
- Timeouts, limits, buffer sizes now in one place
- **Files**: `pkg/constants/constants.go` (new)

**DEF-4: SNI Code Duplication Eliminated**
- Created shared `ConfigureSNI()` helper function
- Eliminates 30+ lines of duplicate code between HTTP/1.1 and HTTP/2
- Same SNI priority logic: TLSConfig.ServerName > SNI > Host
- **Files**: `pkg/transport/transport.go`, `pkg/http2/transport.go`

**DEF-5: HTTP/2 Connection Pool Statistics**
- Added `GetPoolStats()` method to HTTP/2 client and transport
- Provides: active connections, stream counts, last activity, ready state
- Useful for monitoring and debugging connection pooling
- **Files**: `pkg/http2/types.go`, `pkg/http2/transport.go`, `pkg/http2/client.go`

**DEF-6: Stream ID Exhaustion Check**
- Added check: stream IDs must not exceed 2^31-1 (RFC 7540)
- Returns clear error instead of wrapping around
- **Files**: `pkg/http2/stream.go`

**DEF-7: SETTINGS Handshake Timeout**
- Added SetReadDeadline (10s) to prevent indefinite blocking
- Prevents hung connections during SETTINGS handshake
- **Files**: `pkg/http2/transport.go`

**DEF-9: MaxFrameSize Validation**
- Added RFC 7540 compliance validation (16384 to 16777215)
- Validates both min and max bounds
- **Files**: `pkg/http2/types.go`

**DEF-13: InsecureTLS Override Documentation**
- Added comprehensive documentation explaining InsecureTLS behavior
- Clearly states: InsecureTLS ALWAYS overrides TLSConfig.InsecureSkipVerify
- Documents SNI priority order and validation rules
- **Files**: `pkg/client/client.go`, `pkg/transport/transport.go`, `pkg/http2/types.go`

**DEF-14: CA Certificate Validation**
- Improved error message: shows certificate index on parse failure
- Helps identify which cert in array is malformed
- **Files**: `pkg/transport/transport.go`

**DEF-15: HTTP/2 ALPN Fallback**
- Automatic fallback to HTTP/1.1 when server doesn't support HTTP/2
- Detects ALPN negotiation failure and retries with HTTP/1.1
- Improves compatibility with HTTP/1.1-only servers
- **Files**: `rawhttp.go`

### Technical Details

**Bugs Fixed**: 7 (BUG-1,2,3,4,6,8,9)
**Improvements**: 11 (DEF-1,2,3,4,5,6,7,9,13,14,15)
**Files Modified**: 14 files (10 modified, 1 new: pkg/constants)
**Lines Changed**: ~400+ additions, ~100 deletions
**Test Coverage**: All unit and integration tests passing (50+ tests)
**Breaking Changes**: None - Full backward compatibility maintained

**Commits**:
- `3b202cb` - fix: Critical bug fixes - Time calculation and goroutine leaks (BUG-1, BUG-2)
- `a917171` - fix: Eliminate race condition in HTTP/2 options (BUG-3)
- `1db41dc` - fix: Multiple bug fixes and improvements (BUG-4,6,8,9 + DEF-1,3,6,7,9,14)
- `4fa656d` - fix: Excessive memory allocation and complete v1.1.6 documentation (DEF-2)
- `98cf5c2` - refactor: Eliminate SNI code duplication (DEF-4)
- `c8c30b1` - feat: Add HTTP/2 connection pool statistics (DEF-5)
- `3a0361d` - docs: Document InsecureTLS override behavior (DEF-13)
- `95b3b52` - feat: Implement automatic HTTP/2 to HTTP/1.1 fallback (DEF-15)

### Migration Guide

**No breaking changes** - All changes are backward compatible.

**New Features Available:**
```go
// DEF-5: Monitor HTTP/2 connection pool
stats := http2Client.GetPoolStats()
fmt.Printf("Active connections: %d\n", stats.ActiveConnections)
fmt.Printf("Total streams: %d\n", stats.TotalStreams)

// DEF-15: HTTP/2 with automatic HTTP/1.1 fallback
opts := rawhttp.Options{
    Protocol: "http/2",  // Will fallback to HTTP/1.1 if server doesn't support h2
    // ... other options
}
```

**Improvements You Get Automatically:**
- HTTP/2 timing metrics now work correctly (BUG-1)
- No more goroutine/memory leaks (BUG-2)
- Thread-safe concurrent HTTP/2 requests (BUG-3)
- No more crashes during PING health checks (BUG-4)
- Better error messages for timeouts and cert validation (BUG-8, DEF-14)
- Automatic HTTP/2→HTTP/1.1 fallback (DEF-15)

---

## [1.1.5] - 2025-11-21

### Fixed - Critical TLS and HTTP/2 Issues

#### 🔴 Bug #1: TLS InsecureSkipVerify Ignored with Custom TLSConfig (HTTP/1.1)

**Severity:** CRITICAL - Blocks proxy MITM scenarios and self-signed certificate handling

**Issue:**
- When users provided custom `TLSConfig` alongside `InsecureTLS: true`, the `InsecureTLS` flag was completely ignored
- This prevented accepting self-signed certificates in proxy scenarios where users need both custom TLS settings (versions, ciphers) AND certificate validation bypass
- Critical for: proxy applications, testing with self-signed certificates, development environments

**Root Cause:**
- `pkg/transport/transport.go:286-317` - `upgradeTLS` function cloned custom `TLSConfig` but didn't apply `InsecureTLS` flag override
- The function checked `if config.TLSConfig != nil` and used it directly without checking `config.InsecureTLS`

**Fix Applied:**
- Added `InsecureTLS` flag override after cloning custom `TLSConfig` (lines 290-295)
- Now `InsecureTLS: true` takes priority and sets `InsecureSkipVerify: true` even with custom TLS config
- Maintains backward compatibility: users can still provide `InsecureSkipVerify` directly in `TLSConfig`

**Impact:**
- ✅ Fixes proxy MITM functionality with custom TLS settings
- ✅ Enables HTTP→HTTPS scheme override with self-signed certificates
- ✅ Resolves testing/development workflow blocks
- ✅ Maintains flexibility: both methods now work correctly

**Code Location:** `pkg/transport/transport.go:290-295`

#### 🔴 Bug #2: Port Double Formatting in HTTP/2 Error Messages

**Severity:** HIGH - Confusing error messages, breaks error parsing

**Issue:**
- HTTP/2 client incorrectly passed `fmt.Sprintf("%s:%d", host, port)` to `errors.NewConnectionError()`
- `NewConnectionError` expects separate `host string, port int` parameters and formats them internally
- Result: Error messages showed `"127.0.0.1:8080:8080"` instead of `"127.0.0.1:8080"`

**Root Cause:**
- `pkg/http2/client.go:68, 143` - Passed pre-formatted address string as first parameter
- `errors.NewConnectionError()` then formatted it again: `fmt.Sprintf("%s:%d", alreadyFormattedAddr, port)`

**Fix Applied:**
- Changed HTTP/2 client calls from:
  ```go
  errors.NewConnectionError(fmt.Sprintf("%s:%d", host, port), port, err)
  ```
- To correct format:
  ```go
  errors.NewConnectionError(host, port, err)
  ```

**Impact:**
- ✅ Fixes confusing error messages
- ✅ Consistent error formatting between HTTP/1.1 and HTTP/2
- ✅ Enables proper error message parsing
- ✅ Improves debugging experience

**Code Locations:**
- `pkg/http2/client.go:68`
- `pkg/http2/client.go:143`

#### 🔴 Bug #3: HTTP/2 Completely Ignores TLS Configuration

**Severity:** CRITICAL - HTTP/2 unusable with self-signed certificates

**Issue:**
- HTTP/2 transport created hardcoded `tls.Config` with no `InsecureSkipVerify` support
- HTTP/2 `Options` struct lacked `InsecureTLS` and `TLSConfig` fields
- `rawhttp.Do()` didn't pass TLS configuration from main options to HTTP/2 client
- Result: HTTP/2 connections ALWAYS fail with self-signed certificates, regardless of settings

**Root Cause:**
- `pkg/http2/types.go:15-61` - Options struct missing TLS configuration fields
- `pkg/http2/transport.go:213-217` - Hardcoded TLS config without InsecureSkipVerify
- `rawhttp.go:87-103` - Didn't convert or pass TLS settings to HTTP/2

**Fixes Applied:**

1. **Added TLS fields to HTTP/2 Options** (`pkg/http2/types.go:43-44`):
   ```go
   InsecureTLS bool         // Skip TLS certificate verification
   TLSConfig   *tls.Config  // Custom TLS configuration
   ```

2. **Updated HTTP/2 transport TLS handling** (`pkg/http2/transport.go:215-250`):
   - Use custom `TLSConfig` if provided (with clone)
   - Ensure HTTP/2 ALPN (`h2`) is always included
   - Apply `InsecureTLS` flag override
   - Fallback to default config with `InsecureTLS` support

3. **Created TLS config pass-through** (`rawhttp.go:126-154`):
   - Added `convertToHTTP2Options()` function
   - Converts `client.Options` → `http2.Options`
   - Automatically passes `InsecureTLS` and `TLSConfig` to HTTP/2

4. **Enhanced HTTP/2 Client API** (`pkg/http2/client.go:44-59`):
   - Added `DoWithOptions()` method for dynamic TLS config
   - Maintained backward compatibility with `Do()` method

**Impact:**
- ✅ HTTP/2 now fully supports self-signed certificates
- ✅ HTTP/2 respects both `InsecureTLS` flag and custom `TLSConfig`
- ✅ Consistent TLS behavior between HTTP/1.1 and HTTP/2
- ✅ Enables HTTP/2 usage in proxy, testing, and development scenarios
- ✅ Maintains backward compatibility

**Code Locations:**
- `pkg/http2/types.go:43-44`
- `pkg/http2/transport.go:215-250`
- `pkg/http2/client.go:44-59`
- `rawhttp.go:126-154`

### Added

- Comprehensive test suite for TLS `InsecureTLS` override functionality
- Port formatting validation tests for both HTTP/1.1 and HTTP/2
- HTTP/2 TLS configuration integration tests
- Error message formatting consistency tests

### Changed

- HTTP/2 `Options` struct now includes `InsecureTLS` and `TLSConfig` fields
- HTTP/2 transport now uses configurable TLS settings instead of hardcoded config
- `rawhttp.Do()` now automatically passes TLS configuration to HTTP/2 client
- Error message formatting is now consistent across HTTP/1.1 and HTTP/2

### Testing

**New Test Files:**
- `tests/unit/insecure_tls_override_test.go` - TLS InsecureTLS override tests
- `tests/unit/port_formatting_test.go` - Port formatting validation
- `tests/unit/http2_port_formatting_test.go` - HTTP/2 specific port tests

**Test Coverage:**
- ✅ InsecureTLS with custom TLSConfig (Bug #1)
- ✅ InsecureTLS without custom TLSConfig (backward compatibility)
- ✅ Custom TLSConfig with InsecureSkipVerify
- ✅ Port formatting in error messages (Bug #2)
- ✅ HTTP/2 port formatting consistency
- ✅ HTTP/2 TLS configuration (Bug #3)
- ✅ HTTP/2 vs HTTP/1.1 error message consistency

All tests pass ✓ (139 total tests)

### Migration Guide

**No Breaking Changes** - All fixes maintain backward compatibility.

**HTTP/2 TLS Configuration (New Feature):**
```go
sender := rawhttp.NewSender()

opts := rawhttp.Options{
    Scheme:      "https",
    Host:        "example.com",
    Port:        443,
    Protocol:    "http/2",
    InsecureTLS: true, // Now works with HTTP/2!
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS13,
        // InsecureSkipVerify will be set automatically if InsecureTLS is true
    },
}

resp, err := sender.Do(context.Background(), req, opts)
```

**Priority of TLS Settings:**
1. If `TLSConfig` is provided, it is cloned and used
2. If `InsecureTLS: true`, it overrides `TLSConfig.InsecureSkipVerify`
3. If neither provided, default TLS config is used

### References

- Bug Report: Internal analysis of proxy MITM failures and HTTP/2 limitations
- Related Issues: Self-signed certificate handling, HTTP→HTTPS scheme override

---

## [1.1.4] - 2025-11-20

### Fixed - RFC 9110 Compliance

#### 🔴 Missing Special Case Handling for Responses Without Message Bodies

**Severity:** MEDIUM-HIGH - Causes 10-second timeouts for compliant servers

**Issue:**
- The library did not implement RFC 9110 Section 6.4.1 requirements for responses that MUST NOT contain a message body
- This caused incorrect attempts to read response bodies for:
  - HEAD method responses
  - 1xx (Informational) status codes
  - 204 No Content responses
  - 304 Not Modified responses
- Result: Blocking reads that timeout (typically 10 seconds) when servers correctly send no body

**Root Cause:**
- `pkg/client/client.go:428-472` - `readBody` function did not check for RFC 9110 special cases
- The function saw `Content-Length` headers and attempted to read bodies that would never arrive
- RFC allows servers to send `Content-Length` for informational purposes even when no body is present

**Impact:**
- ✅ Eliminates 10-second timeouts for HEAD requests
- ✅ Fixes 204 No Content API endpoint handling
- ✅ Resolves HTTP compliance testing failures
- ✅ Improves user experience for proxies and web crawlers

**Fixes Applied:**
1. **Added Method field to Response struct** (`pkg/client/client.go:65`) - Store request method for body reading logic
2. **Added parseMethod helper** (`pkg/client/client.go:170-177`) - Extract HTTP method from raw request
3. **RFC 9110 compliance check** (`pkg/client/client.go:435-459`) - Skip body reading for special cases

**Strategy (Burp Suite-like approach):**
- PEEK at buffered data to detect if server actually sent a body
- If buffered data present: read body (captures RFC violations from buggy servers)
- If no buffered data: skip body (prevents timeout on RFC-compliant servers)
- This allows us to handle both compliant and non-compliant servers without timeouts

**Triggering Scenarios:**
- HEAD requests to any server (most common)
- REST API DELETE/PUT returning 204 No Content
- Conditional GET requests receiving 304 Not Modified
- Any 1xx informational responses (rare but valid)

**Testing:**
- Verified no timeout on RFC-compliant servers
- Verified body capture on non-compliant servers
- All existing tests continue to pass

### Changed

- Enhanced `Response` struct with `Method` field for RFC compliance checking
- Improved body reading logic to handle both compliant and non-compliant servers

### References

- [RFC 9110 Section 6.4.1 - Control Data](https://www.rfc-editor.org/rfc/rfc9110.html#section-6.4.1)
- [RFC 9110 Section 9.3.2 - HEAD Method](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.2)
- [RFC 9110 Section 15.3.5 - 204 No Content](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.5)
- [RFC 9110 Section 15.4.5 - 304 Not Modified](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.4.5)

## [1.1.3] - 2025-11-15

### Fixed - Enhanced Error Handling

#### 🛡️ Additional Defensive Checks for TLS Error Paths

**Severity:** LOW - Preventive hardening

**Issue:**
- Added extra defensive nil pointer checks in HTTP/2 transport error paths
- Improved robustness for edge cases in TLS connection failures

**Changes:**
- `pkg/http2/transport.go:179-182` - Added defensive nil check before closing connection on settings send failure
- Enhanced test coverage with comprehensive TLS error scenario tests

**Testing:**
- Added `TestHTTPSToPlainHTTPServer` - Verifies HTTPS to plain HTTP server doesn't panic
- Added `TestTLSHandshakeTimeout` - Verifies timeout handling doesn't panic
- Added `TestContextCancellationDuringTLS` - Verifies context cancellation handling
- All tests confirm proper error handling without panics

**Impact:**
- ✅ Additional safety layer for HTTP/2 transport error paths
- ✅ Comprehensive test coverage for TLS error scenarios
- ✅ Proactive protection against potential edge cases
- ✅ No performance impact (checks only in error paths)

**Note:** This release builds upon the critical fixes in v1.1.2, adding an extra layer of defensive programming to ensure maximum robustness in production environments.

### Changed

- Enhanced defensive programming in HTTP/2 transport layer
- Added comprehensive TLS error handling test suite

## [1.1.2] - 2025-11-14

### Fixed - Critical Stability Issues

#### 🔴 TLS Handshake Resource Leak and Nil Pointer Dereference

**Severity:** CRITICAL - Application crash and resource leak

**Issue:**
- Failed TLS handshakes didn't close the underlying TCP connection, causing file descriptor leaks
- Nil pointer dereference panic when attempting to close connection after TLS upgrade failure
- Incorrect connection cleanup in HTTP/2 ALPN negotiation failure path

**Root Cause:**
- `pkg/transport/transport.go:324-326` - `upgradeTLS` returned nil without closing original TCP connection
- `pkg/transport/transport.go:193` - Attempted to call `.Close()` on nil connection pointer
- `pkg/http2/transport.go:243` - Closed wrong connection reference in ALPN negotiation failure

**Impact:**
- ✅ Prevents application crashes from TLS handshake failures
- ✅ Eliminates resource leaks (file descriptors, memory)
- ✅ Improves stability when connecting to servers with certificate issues
- ✅ Mitigates potential DoS vulnerability from repeated TLS failures

**Fixes Applied:**
1. **Primary Fix (transport.go:325)**: Close original TCP connection before returning error from `upgradeTLS`
2. **Defensive Check (transport.go:194)**: Add nil check before closing connection in error path
3. **HTTP/2 Fix (http2/transport.go:243)**: Close TLS connection instead of underlying TCP connection

**Triggering Scenarios:**
- TLS handshake timeout
- Certificate validation failure (expired, self-signed, hostname mismatch)
- SNI issues
- Protocol negotiation failure (unsupported TLS version/cipher suite)
- Connection reset during handshake
- Context cancellation during TLS handshake

**Testing:**
- Verified no panic on TLS handshake failures
- Verified proper connection cleanup on all error paths
- Verified no file descriptor leaks using lsof
- Tested against various badssl.com endpoints (expired.badssl.com, self-signed.badssl.com, etc.)

### Changed

- Enhanced error handling robustness in transport layer
- Improved connection cleanup in TLS failure scenarios
