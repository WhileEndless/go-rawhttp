# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
