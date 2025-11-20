# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Improved resource cleanup consistency across all error paths

### Security

- Mitigated potential DoS vulnerability from TLS handshake failures
- Prevented resource exhaustion from leaked file descriptors

## [1.1.1] - 2025-11-14

### Added

#### HTTP/2 Debug Flags

- Optional selective debugging for HTTP/2 protocol issues
- New `Debug` struct in `HTTP2Settings` with granular logging flags
- Zero overhead when disabled (all flags default to false)
- Backward compatible with deprecated `ShowFrameDetails` and `TraceFrames` flags

## [1.1.0] - 2025-11-14

### Added - Low-Level Transport Enhancements

This release focuses on production-ready, low-level transport features while maintaining 100% backward compatibility.

#### 🔒 TLS Configuration Passthrough (Phase 1 - Critical)
- **Direct TLS config passthrough** via `Options.TLSConfig` field
- Full control over TLS versions, cipher suites, and client certificates
- Seamless integration with `crypto/tls.Config`
- Enables TLS 1.3+ enforcement and custom security policies

#### 📊 Standardized Timing Metrics (Phase 1 - High Priority)
- **New industry-standard field names**: `DNSLookup`, `TCPConnect`, `TLSHandshake`, `TotalTime`
- Improved String() formatting with clear metric names
- Full backward compatibility with deprecated field names (`DNS`, `TCP`, `TLS`, `Total`)
- Enhanced metric calculation methods

#### ⚙️ HTTP/2 Settings Exposure (Phase 2 - High Priority)
- **RFC 7540 compliant** HTTP/2 SETTINGS configuration
- Direct control over `SETTINGS_MAX_CONCURRENT_STREAMS`, `SETTINGS_INITIAL_WINDOW_SIZE`, etc.
- New `DisableServerPush` flag for security (enabled by default)
- Comprehensive documentation for each SETTINGS parameter

#### 🔍 Enhanced Connection Metadata (Phase 2 - Medium Priority)
- **Socket-level information**: `LocalAddr`, `RemoteAddr`, `ConnectionID`
- **TLS session tracking**: `TLSSessionID`, `TLSResumed`
- Unique connection identifiers for request correlation
- Session resumption detection for performance monitoring

#### 🚨 Error Type Classification (Phase 3 - Medium Priority)
- **Operation tracking** in all errors: `Op` field (lookup, dial, handshake, read, write)
- Enhanced error formatting: `[type] op addr: message: cause`
- `TransportError` type alias for naming compatibility
- Smart retry logic support with error phase detection

#### 📈 Connection Pool Observability (Phase 3 - Low Priority)
- **PoolStats API** for monitoring pool health
- Track `ActiveConns`, `IdleConns`, and `TotalReused`
- Connection leak detection capabilities
- Performance monitoring for connection reuse efficiency

### Changed

- Timing metrics now use standardized field names (backward compatible)
- HTTP/2 server push disabled by default for security
- Error formatting improved with operation context
- String representations updated for better readability

### Fixed

- Fixed timing test compatibility with new metric field names
- Updated all tests to use standardized naming conventions

### Developer Experience

- **301 lines** of comprehensive unit tests
- **7 test suites** covering all enhancement features
- 100% backward compatibility verified
- All tests passing (40+ test cases)

### Migration Guide

All enhancements are **opt-in** and **100% backward compatible**:

```go
// Old code continues to work unchanged
resp, _ := sender.Do(ctx, req, rawhttp.Options{
    Scheme: "https",
    Host:   "example.com",
    Port:   443,
})
fmt.Printf("DNS: %v\n", resp.Metrics.DNS) // ✅ Still works

// New features available when needed
resp, _ := sender.Do(ctx, req, rawhttp.Options{
    Scheme: "https",
    Host:   "example.com",
    Port:   443,
    TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13}, // ✅ New feature
})
fmt.Printf("DNS: %v\n", resp.Metrics.DNSLookup) // ✅ New naming
stats := sender.PoolStats() // ✅ New API
```

### Documentation

- Comprehensive README updates with code examples
- Detailed inline documentation for all new fields
- RFC 7540 references for HTTP/2 settings
- Use case examples for each enhancement

## [1.0.0] - 2024-11-XX

### Added

- Initial release with HTTP/1.1 and HTTP/2 support
- Raw socket-based HTTP communication
- Connection pooling and Keep-Alive
- Proxy support (HTTP, HTTPS, SOCKS5)
- Custom CA certificates
- Performance monitoring (basic timing metrics)
- Memory-efficient buffering with disk spilling
- Comprehensive error handling
- Production-ready features

[1.1.4]: https://github.com/WhileEndless/go-rawhttp/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/WhileEndless/go-rawhttp/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/WhileEndless/go-rawhttp/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/WhileEndless/go-rawhttp/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/WhileEndless/go-rawhttp/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/WhileEndless/go-rawhttp/releases/tag/v1.0.0
