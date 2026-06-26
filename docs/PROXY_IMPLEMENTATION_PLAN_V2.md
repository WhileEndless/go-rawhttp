# go-rawhttp v2.0.0 - Comprehensive Proxy Implementation Plan

**Version:** 2.0.0
**Type:** MAJOR (Breaking Changes)
**Date:** 2025-11-23
**Status:** In Progress

---

## 🎯 Executive Summary

This document outlines the implementation plan for comprehensive upstream proxy support in go-rawhttp v2.0.0. This is a **MAJOR** version release containing breaking changes for a cleaner, more maintainable API.

### Key Changes

1. **BREAKING:** Remove `ProxyURL string` field from Options
2. **NEW:** Add `Proxy *ProxyConfig` struct for advanced proxy configuration
3. **NEW:** Add `ParseProxyURL(url string)` helper for simple use cases
4. **NEW:** Add SOCKS4 proxy protocol support
5. **NEW:** Add custom headers support for HTTP proxies
6. **NEW:** Add proxy-specific error handling
7. **NEW:** Add proxy metadata to Response

---

## 📋 Implementation Phases

### Phase 1: Core Data Structures ✅

**Files to modify:**
- `pkg/client/client.go` - Add ProxyConfig struct, remove ProxyURL
- `pkg/transport/transport.go` - Update Config struct
- `pkg/errors/errors.go` - Add ProxyError type

**Estimated time:** 2-3 hours

### Phase 2: ParseProxyURL Helper ✅

**Files to create/modify:**
- `pkg/client/proxy_parser.go` - New file for ParseProxyURL
- `pkg/client/proxy_parser_test.go` - Unit tests

**Estimated time:** 2-3 hours

### Phase 3: SOCKS4 Implementation ✅

**Files to modify:**
- `pkg/transport/transport.go` - Add connectViaSOCKS4Proxy
- `tests/unit/proxy_socks4_test.go` - New test file

**Estimated time:** 4-5 hours

### Phase 4: HTTP Proxy Enhancements ✅

**Files to modify:**
- `pkg/transport/transport.go` - Add custom headers support
- `tests/unit/proxy_http_test.go` - New test file

**Estimated time:** 2-3 hours

### Phase 5: Transport Layer Integration ✅

**Files to modify:**
- `pkg/transport/transport.go` - Update connectViaProxy
- `pkg/client/client.go` - Update Do method

**Estimated time:** 3-4 hours

### Phase 6: Response Metadata ✅

**Files to modify:**
- `pkg/client/client.go` - Add proxy fields to Response
- `rawhttp.go` - Update Response type

**Estimated time:** 1-2 hours

### Phase 7: Testing ✅

**Files to create:**
- `tests/unit/proxy_config_test.go` - ProxyConfig tests
- `tests/unit/proxy_parser_test.go` - ParseProxyURL tests
- `tests/unit/proxy_socks4_test.go` - SOCKS4 tests
- `tests/unit/proxy_http_enhanced_test.go` - HTTP custom headers tests
- `tests/integration/proxy_integration_test.go` - Integration tests

**Test coverage target:** >85%

**Estimated time:** 8-10 hours

### Phase 8: Examples ✅

**Files to create:**
- `examples/proxy_basic.go` - Simple proxy examples
- `examples/proxy_advanced.go` - Advanced ProxyConfig examples
- `examples/proxy_all_types.go` - All proxy types showcase

**Estimated time:** 2-3 hours

### Phase 9: Documentation ✅

**Files to modify:**
- `README.md` - Update proxy section
- `CHANGELOG.md` - Add v2.0.0 breaking changes
- `docs/MIGRATION_V2.md` - Create migration guide

**Estimated time:** 3-4 hours

---

## 🔧 Detailed Technical Specifications

### 1. ProxyConfig Struct

```go
// ProxyConfig provides detailed configuration for upstream proxy connections.
type ProxyConfig struct {
    // Required fields
    Type string `json:"type"` // "http", "https", "socks4", "socks5"
    Host string `json:"host"`
    Port int    `json:"port"`

    // Authentication (optional)
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`

    // Advanced options (optional)

    // ConnTimeout: Proxy-specific connection timeout.
    // If zero, uses Options.ConnTimeout.
    ConnTimeout time.Duration `json:"conn_timeout,omitempty"`

    // ProxyHeaders: Custom headers for HTTP CONNECT request.
    // Only applies to "http" and "https" proxy types.
    // Example: map[string]string{"X-Custom-Header": "value"}
    ProxyHeaders map[string]string `json:"proxy_headers,omitempty"`

    // TLSConfig: Custom TLS configuration for HTTPS proxy connection.
    // Only applies when connecting TO the proxy (Type="https").
    // This is separate from Options.TLSConfig which is for target connection.
    TLSConfig *tls.Config `json:"-"`

    // ResolveDNSViaProxy: For SOCKS5, whether to resolve DNS through proxy.
    // Default: true (SOCKS5 handles DNS by default).
    // Set to false to resolve locally before SOCKS5 connection.
    // Ignored for other proxy types.
    ResolveDNSViaProxy bool `json:"resolve_dns_via_proxy,omitempty"`
}
```

### 2. ParseProxyURL Function

```go
// ParseProxyURL parses a proxy URL string into a ProxyConfig.
//
// Supported formats:
//   - http://proxy:8080
//   - http://user:pass@proxy:8080
//   - https://proxy:443
//   - socks4://proxy:1080
//   - socks4://user@proxy:1080
//   - socks5://proxy:1080
//   - socks5://user:pass@proxy:1080
//
// Default ports:
//   - http: 8080
//   - https: 443
//   - socks4/socks5: 1080
//
// Returns error if URL format is invalid or scheme is unsupported.
func ParseProxyURL(proxyURL string) (*ProxyConfig, error)
```

### 3. ProxyError Type

```go
// ProxyError represents a proxy-specific error with detailed context.
type ProxyError struct {
    ProxyType string // "http", "https", "socks4", "socks5"
    ProxyAddr string // "proxy.com:8080"
    Operation string // "connect", "auth", "handshake", "tunnel"
    Err       error  // Underlying error
}

func (e *ProxyError) Error() string {
    return fmt.Sprintf("proxy error (%s %s) during %s: %v",
        e.ProxyType, e.ProxyAddr, e.Operation, e.Err)
}

func (e *ProxyError) Unwrap() error {
    return e.Err
}

// Constructor
func NewProxyError(proxyType, proxyAddr, operation string, err error) *ProxyError
```

### 4. Response Metadata Fields

```go
type Response struct {
    // ... existing fields ...

    // Proxy information (v2.0.0+)
    ProxyUsed bool   `json:"proxy_used"` // Whether request went through proxy
    ProxyType string `json:"proxy_type"` // "http", "https", "socks4", "socks5" (if ProxyUsed=true)
    ProxyAddr string `json:"proxy_addr"` // "proxy.com:8080" (if ProxyUsed=true)
}
```

### 5. SOCKS4 Implementation

**Protocol Flow:**
1. Connect to SOCKS4 proxy (TCP)
2. Resolve target hostname to IPv4 (SOCKS4 is IPv4-only)
3. Build SOCKS4 request: [VER][CMD][PORT][IP][USERID][NULL]
4. Send request
5. Read response: [VER][STATUS][PORT][IP]
6. Verify status == 0x5A (request granted)
7. Return connection

**Error Handling:**
- IPv6 address → Error (SOCKS4 doesn't support IPv6)
- DNS failure → Error with clear message
- SOCKS4 status codes:
  - 0x5A: Request granted (success)
  - 0x5B: Request rejected or failed
  - 0x5C: Request failed (identd not running)
  - 0x5D: Request failed (identd auth failed)

---

## 🔄 Breaking Changes

### What's Removed

1. **`Options.ProxyURL` field** - REMOVED
   - **Old:** `ProxyURL: "socks5://proxy:1080"`
   - **New:** `Proxy: ParseProxyURL("socks5://proxy:1080")`

### Migration Path

**Before (v1.2.0):**
```go
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "api.example.com",
    Port:     443,
    ProxyURL: "socks5://user:pass@proxy.com:1080",
}
```

**After (v2.0.0) - Simple:**
```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("socks5://user:pass@proxy.com:1080"),
}
```

**After (v2.0.0) - Advanced:**
```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy: &rawhttp.ProxyConfig{
        Type:        "socks5",
        Host:        "proxy.com",
        Port:        1080,
        Username:    "user",
        Password:    "pass",
        ConnTimeout: 10 * time.Second,
    },
}
```

---

## ✅ Test Plan

### Unit Tests (Minimum 20 tests)

#### ProxyConfig Tests (5 tests)
- [x] Test ProxyConfig struct creation
- [x] Test ProxyConfig with all fields
- [x] Test ProxyConfig validation
- [x] Test ProxyConfig default values
- [x] Test ProxyConfig with custom headers

#### ParseProxyURL Tests (8 tests)
- [x] Parse HTTP proxy URL
- [x] Parse HTTPS proxy URL
- [x] Parse SOCKS4 proxy URL
- [x] Parse SOCKS5 proxy URL
- [x] Parse URL with authentication
- [x] Parse URL with default ports
- [x] Parse invalid URL (error)
- [x] Parse unsupported scheme (error)

#### SOCKS4 Tests (6 tests)
- [x] SOCKS4 successful connection
- [x] SOCKS4 with username
- [x] SOCKS4 IPv6 failure (not supported)
- [x] SOCKS4 DNS resolution failure
- [x] SOCKS4 error status codes
- [x] SOCKS4 connection timeout

#### HTTP Proxy Tests (4 tests)
- [x] HTTP proxy with custom headers
- [x] HTTPS proxy with custom TLS config
- [x] HTTP proxy CONNECT failure
- [x] HTTP proxy authentication

#### Error Handling Tests (3 tests)
- [x] ProxyError creation and formatting
- [x] Error unwrapping
- [x] Proxy error types

### Integration Tests (Minimum 5 tests)

- [x] Real HTTP proxy connection
- [x] Real SOCKS5 proxy connection
- [x] Real SOCKS4 proxy connection (if available)
- [x] Proxy with HTTP/2 target
- [x] Proxy error scenarios

### Manual Testing Checklist

- [ ] Test with real HTTP proxy (Squid, mitmproxy)
- [ ] Test with real SOCKS5 proxy (Dante, ss5)
- [ ] Test with authentication enabled proxies
- [ ] Test error messages are clear
- [ ] Test all examples run successfully
- [ ] Verify README examples work

---

## 📚 Documentation Updates

### README.md

**Sections to update:**
1. Features list - Add SOCKS4, ProxyConfig
2. Quick Start - Update proxy example
3. Proxy Support section - Complete rewrite
4. API Reference - Update Options struct
5. Examples - Link to new proxy examples
6. Migration section - Add v2.0.0 migration notes

### CHANGELOG.md

**v2.0.0 Entry:**
```markdown
## [2.0.0] - 2025-11-23

### 🔴 BREAKING CHANGES

**Proxy Configuration API Redesign**

The proxy configuration has been completely redesigned for clarity and extensibility. The `ProxyURL` field has been removed in favor of a dedicated `ProxyConfig` struct.

**Migration Required:**

Before (v1.x):
```go
opts := rawhttp.Options{
    ProxyURL: "socks5://user:pass@proxy:1080",
}
```

After (v2.0.0):
```go
// Simple usage
opts := rawhttp.Options{
    Proxy: rawhttp.ParseProxyURL("socks5://user:pass@proxy:1080"),
}

// Advanced usage
opts := rawhttp.Options{
    Proxy: &rawhttp.ProxyConfig{
        Type: "socks5",
        Host: "proxy",
        Port: 1080,
        Username: "user",
        Password: "pass",
        ConnTimeout: 10 * time.Second,
    },
}
```

See [MIGRATION_V2.md](docs/MIGRATION_V2.md) for complete migration guide.

### ✨ New Features

- **SOCKS4 Proxy Support** - Legacy SOCKS version 4 protocol support
- **ProxyConfig Struct** - Advanced proxy configuration with fine-grained control
- **ParseProxyURL Helper** - Convenient helper for parsing proxy URLs
- **Custom Proxy Headers** - Add custom headers to HTTP CONNECT requests
- **Proxy-Specific Timeouts** - Configure timeout specifically for proxy connections
- **Proxy TLS Config** - Separate TLS configuration for HTTPS proxies
- **Proxy Metadata** - Response includes proxy type and address used
- **ProxyError Type** - Detailed error type for proxy-specific failures

### 🐛 Bug Fixes

- None (this is a feature release)

### 📝 Documentation

- Added comprehensive proxy documentation to README
- Created MIGRATION_V2.md guide
- Added 3 new proxy examples
- Updated all proxy-related documentation

### 🧪 Testing

- Added 20+ unit tests for proxy functionality
- Added 5+ integration tests
- Added mock proxy servers for testing
- Test coverage: >85%
```

### New Documentation Files

**docs/MIGRATION_V2.md** - Migration guide from v1.x to v2.0.0
**examples/proxy_basic.go** - Simple proxy examples
**examples/proxy_advanced.go** - Advanced ProxyConfig examples
**examples/proxy_all_types.go** - Showcase all proxy types

---

## 🚀 Release Checklist

- [ ] All code implemented and reviewed
- [ ] All unit tests passing (>85% coverage)
- [ ] All integration tests passing
- [ ] All examples running successfully
- [ ] README.md updated
- [ ] CHANGELOG.md updated
- [ ] MIGRATION_V2.md created
- [ ] All proxy types tested manually
- [ ] Version bumped to 2.0.0
- [ ] Git tag created: v2.0.0
- [ ] Release notes prepared

---

## 📊 Success Metrics

- **Test Coverage:** >85% for new code
- **API Clarity:** Single, clear way to configure proxy (no confusion)
- **Feature Completeness:** All 4 proxy types supported (HTTP, HTTPS, SOCKS4, SOCKS5)
- **Documentation:** Comprehensive examples and migration guide
- **Breaking Changes:** Clearly documented with migration path

---

## 📅 Timeline

**Total Estimated Time:** 35-40 hours (~5-6 working days)

| Day | Tasks | Hours |
|-----|-------|-------|
| 1 | Phase 1-3: Core structures, parser, SOCKS4 | 8h |
| 2 | Phase 4-5: HTTP enhancements, transport integration | 8h |
| 3 | Phase 6-7: Metadata, testing (unit tests) | 8h |
| 4 | Phase 7: Testing (integration), manual testing | 8h |
| 5 | Phase 8-9: Examples, documentation | 6h |
| 6 | Final review, release preparation | 2h |

---

**Document Version:** 1.0
**Last Updated:** 2025-11-23
**Status:** Ready for Implementation
