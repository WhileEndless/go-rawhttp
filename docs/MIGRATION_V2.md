# Migration Guide: go-rawhttp v1.x → v2.0.0

**Version:** v2.0.0
**Release Date:** 2025-11-23
**Type:** MAJOR (Breaking Changes)

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Breaking Changes](#breaking-changes)
3. [Migration Steps](#migration-steps)
4. [New Features](#new-features)
5. [Code Examples](#code-examples)
6. [FAQ](#faq)

---

## Overview

go-rawhttp v2.0.0 introduces a comprehensive redesign of the proxy configuration API for better clarity, extensibility, and developer experience. The main change is replacing the simple `ProxyURL string` field with a dedicated `ProxyConfig` struct and `ParseProxyURL()` helper function.

### Why the Change?

The old `ProxyURL` string field had limitations:
- ❌ No support for advanced features (custom headers, timeouts, TLS config)
- ❌ Confusing for users needing fine-grained control
- ❌ Two different ways to configure proxy (ProxyURL vs future ProxyConfig)

The new API provides:
- ✅ Single, clear way to configure proxy
- ✅ Simple usage remains simple (with `ParseProxyURL`)
- ✅ Advanced features available when needed
- ✅ Better error messages and type safety

---

## Breaking Changes

### ⚠️ BREAKING CHANGE #1: `ProxyURL` Field Removed

**Removed:**
```go
type Options struct {
    ProxyURL string // ❌ REMOVED in v2.0.0
}
```

**Replaced with:**
```go
type Options struct {
    Proxy *ProxyConfig // ✅ NEW in v2.0.0
}
```

### Impact

- ⚠️ **Code using `ProxyURL` will NOT compile** in v2.0.0
- ✅ **Migration is simple** - use `ParseProxyURL()` helper
- ✅ **No runtime behavior changes** - same proxy protocols supported

---

## Migration Steps

### Step 1: Identify Usage

Search your codebase for `ProxyURL`:

```bash
grep -r "ProxyURL" . --include="*.go"
```

### Step 2: Update Code

For each occurrence, replace with `ParseProxyURL()`:

**Before (v1.x):**
```go
opts := rawhttp.Options{
    Scheme:   "https",
    Host:     "api.example.com",
    Port:     443,
    ProxyURL: "socks5://user:pass@proxy.com:1080",
}
```

**After (v2.0.0):**
```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy:  rawhttp.ParseProxyURL("socks5://user:pass@proxy.com:1080"),
}
```

### Step 3: Test

Run your tests to ensure proxy functionality works:

```bash
go test ./...
```

---

## New Features

### 1. Advanced Proxy Configuration

You can now use `ProxyConfig` struct for advanced scenarios:

```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.example.com",
    Port:   443,
    Proxy: &rawhttp.ProxyConfig{
        Type:        "http",
        Host:        "proxy.com",
        Port:        8080,
        Username:    "user",
        Password:    "pass",
        ConnTimeout: 10 * time.Second,           // NEW
        ProxyHeaders: map[string]string{         // NEW
            "X-Custom-Header": "value",
        },
    },
}
```

### 2. SOCKS4 Support

SOCKS4 protocol support added:

```go
opts.Proxy = rawhttp.ParseProxyURL("socks4://user@proxy.com:1080")
```

### 3. Custom Proxy Headers

For HTTP/HTTPS proxies, you can now send custom headers:

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type: "http",
    Host: "proxy.com",
    Port: 8080,
    ProxyHeaders: map[string]string{
        "X-Request-ID": "12345",
    },
}
```

### 4. Proxy-Specific Timeout

Configure timeout specifically for proxy connection:

```go
opts.Proxy = &rawhttp.ProxyConfig{
    Type:        "socks5",
    Host:        "proxy.com",
    Port:        1080,
    ConnTimeout: 5 * time.Second, // Proxy timeout
}
opts.ConnTimeout = 10 * time.Second // Target timeout
```

### 5. Proxy Metadata in Response

Response now includes proxy information:

```go
resp, _ := sender.Do(ctx, req, opts)
fmt.Printf("Proxy Used: %v\n", resp.ProxyUsed)
fmt.Printf("Proxy Type: %s\n", resp.ProxyType)
fmt.Printf("Proxy Addr: %s\n", resp.ProxyAddr)
```

### 6. ProxyError Type

Better error handling for proxy failures:

```go
resp, err := sender.Do(ctx, req, opts)
if err != nil {
    if proxyErr, ok := err.(*rawhttp.ProxyError); ok {
        fmt.Printf("Proxy %s at %s failed: %v\n",
            proxyErr.ProxyType, proxyErr.ProxyAddr, proxyErr.Err)
    }
}
```

---

## Code Examples

### Example 1: Simple Migration

**v1.x:**
```go
package main

import (
    "context"
    "github.com/WhileEndless/go-rawhttp"
)

func main() {
    sender := rawhttp.NewSender()

    opts := rawhttp.Options{
        Scheme:   "https",
        Host:     "api.github.com",
        Port:     443,
        ProxyURL: "http://127.0.0.1:8080",
    }

    req := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")
    resp, _ := sender.Do(context.Background(), req, opts)
    defer resp.Body.Close()
}
```

**v2.0.0:**
```go
package main

import (
    "context"
    "github.com/WhileEndless/go-rawhttp"
)

func main() {
    sender := rawhttp.NewSender()

    opts := rawhttp.Options{
        Scheme: "https",
        Host:   "api.github.com",
        Port:   443,
        Proxy:  rawhttp.ParseProxyURL("http://127.0.0.1:8080"), // ← ONLY CHANGE
    }

    req := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")
    resp, _ := sender.Do(context.Background(), req, opts)
    defer resp.Body.Close()
}
```

### Example 2: Advanced Usage (NEW)

This wasn't possible in v1.x:

```go
opts := rawhttp.Options{
    Scheme: "https",
    Host:   "api.github.com",
    Port:   443,
    Proxy: &rawhttp.ProxyConfig{
        Type:     "http",
        Host:     "corporate-proxy.company.com",
        Port:     8080,
        Username: "employee",
        Password: "secret",

        // NEW: Advanced features
        ConnTimeout: 5 * time.Second,
        ProxyHeaders: map[string]string{
            "X-Employee-ID": "12345",
            "X-Department":  "Engineering",
        },
    },
}
```

---

## FAQ

### Q: Why break backward compatibility?

**A:** The old `ProxyURL` API had fundamental limitations. We made this breaking change early (v1.2.0 → v2.0.0) to avoid more pain later. Since `ProxyURL` was introduced in v1.2.0, very few users are affected.

### Q: Can I use both `ProxyURL` and `Proxy`?

**A:** No, `ProxyURL` has been completely removed in v2.0.0. Use `Proxy` with `ParseProxyURL()`.

### Q: Do I lose any functionality?

**A:** No, all v1.x proxy functionality is preserved and enhanced in v2.0.0.

### Q: What if I don't need advanced features?

**A:** Use `ParseProxyURL()` - it's just as simple as `ProxyURL` was:

```go
Proxy: rawhttp.ParseProxyURL("http://proxy:8080")
```

### Q: Does ParseProxyURL handle all URL formats?

**A:** Yes, it supports:
- `http://proxy:8080`
- `http://user:pass@proxy:8080`
- `https://proxy:443`
- `socks4://proxy:1080`
- `socks5://user:pass@proxy:1080`

Default ports are applied automatically (http=8080, https=443, socks=1080).

### Q: Can HTTP proxy handle HTTPS targets?

**A:** **YES!** This is a common question.
- Proxy type (http/https) = How you connect TO the proxy
- Target scheme (http/https) = Traffic THROUGH the proxy

Example: `http://proxy:8080` can proxy HTTPS requests - the proxy connection is cleartext but the target traffic is TLS-encrypted.

### Q: What about SOCKS4a?

**A:** SOCKS4 support includes SOCKS4a behavior where applicable. For full features, use SOCKS5.

### Q: Can I still use connection pooling with proxies?

**A:** Yes, `ReuseConnection: true` works with all proxy types.

---

## Support

**Questions?**
- GitHub Issues: https://github.com/WhileEndless/go-rawhttp/issues
- Examples: `examples/proxy_comprehensive.go`
- API Docs: `README.md`

**Found a bug?**
- Report: https://github.com/WhileEndless/go-rawhttp/issues

---

## Summary Checklist

✅ **Search for `ProxyURL` in codebase**
✅ **Replace with `Proxy: rawhttp.ParseProxyURL(...)`**
✅ **Test proxy functionality**
✅ **Consider using advanced features if needed**
✅ **Update to v2.0.0**

---

**Last Updated:** 2025-11-23
**Document Version:** 1.0
