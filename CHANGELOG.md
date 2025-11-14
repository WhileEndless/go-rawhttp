# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.1.0]: https://github.com/WhileEndless/go-rawhttp/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/WhileEndless/go-rawhttp/releases/tag/v1.0.0
