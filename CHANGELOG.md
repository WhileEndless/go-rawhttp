# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
