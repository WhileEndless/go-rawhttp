// Package main demonstrates comprehensive proxy support in go-rawhttp v2.0.0
//
// This example showcases:
//   - All 4 proxy types: HTTP, HTTPS, SOCKS4, SOCKS5
//   - Simple usage with ParseProxyURL
//   - Advanced usage with ProxyConfig
//   - Custom headers for HTTP proxies
//   - Proxy authentication
//   - Error handling
//   - Proxy metadata in Response
//
// IMPORTANT: This example requires running proxy servers.
// You can use mitmproxy, Squid, or Dante for testing.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== go-rawhttp v2.0.0 - Comprehensive Proxy Examples ===\n")

	// Example 1: Simple HTTP Proxy with ParseProxyURL
	example1_SimpleHTTPProxy()

	// Example 2: HTTPS Proxy with Authentication
	example2_HTTPSProxyWithAuth()

	// Example 3: SOCKS5 Proxy (most common)
	example3_SOCKS5Proxy()

	// Example 4: SOCKS4 Proxy (legacy)
	example4_SOCKS4Proxy()

	// Example 5: Advanced - Custom Headers for HTTP Proxy
	example5_CustomHeaders()

	// Example 6: Advanced - Proxy-Specific Timeout
	example6_ProxyTimeout()

	// Example 7: HTTP Proxy with HTTPS Target
	example7_HTTPProxyHTTPSTarget()

	// Example 8: Error Handling
	example8_ErrorHandling()
}

// Example 1: Simple HTTP Proxy using ParseProxyURL
func example1_SimpleHTTPProxy() {
	fmt.Println("📡 Example 1: Simple HTTP Proxy\n")

	sender := rawhttp.NewSender()

	// Simple proxy configuration using URL string
	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,

		// Parse proxy URL - simple and convenient!
		Proxy: rawhttp.ParseProxyURL("http://127.0.0.1:8080"),
	}

	request := []byte("GET /rate_limit HTTP/1.1\r\nHost: api.github.com\r\nUser-Agent: go-rawhttp/2.0\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ Status: %d\n", resp.StatusCode)
	fmt.Printf("✓ Proxy Used: %v\n", resp.ProxyUsed)
	fmt.Printf("✓ Proxy Type: %s\n", resp.ProxyType)
	fmt.Printf("✓ Proxy Address: %s\n\n", resp.ProxyAddr)
}

// Example 2: HTTPS Proxy with Authentication
func example2_HTTPSProxyWithAuth() {
	fmt.Println("🔒 Example 2: HTTPS Proxy with Authentication\n")

	sender := rawhttp.NewSender()

	// HTTPS proxy with username and password
	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,

		// Parse URL with authentication
		Proxy: rawhttp.ParseProxyURL("https://user:password@secure-proxy.example.com:8443"),
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ Connected via HTTPS proxy\n")
	fmt.Printf("✓ Proxy: %s\n\n", resp.ProxyAddr)
}

// Example 3: SOCKS5 Proxy (recommended for privacy)
func example3_SOCKS5Proxy() {
	fmt.Println("🧦 Example 3: SOCKS5 Proxy\n")

	sender := rawhttp.NewSender()

	// SOCKS5 with authentication
	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,

		// SOCKS5 proxy - DNS resolution via proxy by default
		Proxy: rawhttp.ParseProxyURL("socks5://user:pass@127.0.0.1:1080"),
	}

	request := []byte("GET /zen HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ SOCKS5 connection established\n")
	fmt.Printf("✓ DNS resolved via proxy: true\n\n")
}

// Example 4: SOCKS4 Proxy (legacy support)
func example4_SOCKS4Proxy() {
	fmt.Println("🧦 Example 4: SOCKS4 Proxy (Legacy)\n")

	sender := rawhttp.NewSender()

	// SOCKS4 with user ID
	opts := rawhttp.Options{
		Scheme: "http",
		Host:   "httpbin.org",
		Port:   80,

		// SOCKS4 - IPv4 only, user ID instead of password
		Proxy: rawhttp.ParseProxyURL("socks4://myuser@127.0.0.1:1080"),
	}

	request := []byte("GET /ip HTTP/1.1\r\nHost: httpbin.org\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ SOCKS4 connection (IPv4 only)\n\n")
}

// Example 5: Advanced - Custom Headers for HTTP Proxy
func example5_CustomHeaders() {
	fmt.Println("⚙️  Example 5: Custom Headers for HTTP Proxy\n")

	sender := rawhttp.NewSender()

	// Create advanced ProxyConfig with custom headers
	proxyConfig := &rawhttp.ProxyConfig{
		Type:     "http",
		Host:     "127.0.0.1",
		Port:     8080,
		Username: "admin",
		Password: "secret",

		// Custom headers sent in CONNECT request
		ProxyHeaders: map[string]string{
			"X-Custom-Header":  "MyValue",
			"Proxy-Connection": "keep-alive",
			"X-Request-ID":     "12345",
		},
	}

	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,
		Proxy:  proxyConfig,
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ Custom proxy headers sent\n")
	fmt.Printf("✓ Headers: %v\n\n", proxyConfig.ProxyHeaders)
}

// Example 6: Advanced - Proxy-Specific Timeout
func example6_ProxyTimeout() {
	fmt.Println("⏱️  Example 6: Proxy-Specific Timeout\n")

	sender := rawhttp.NewSender()

	proxyConfig := &rawhttp.ProxyConfig{
		Type:     "http",
		Host:     "127.0.0.1",
		Port:     8080,
		Username: "user",
		Password: "pass",

		// Separate timeout for proxy connection
		ConnTimeout: 5 * time.Second,
	}

	opts := rawhttp.Options{
		Scheme:      "https",
		Host:        "api.github.com",
		Port:        443,
		Proxy:       proxyConfig,
		ConnTimeout: 10 * time.Second, // Different timeout for target
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ Proxy timeout: 5s, Target timeout: 10s\n\n")
}

// Example 7: HTTP Proxy can handle HTTPS Targets
// This is a common question - YES, http:// proxy can proxy HTTPS!
func example7_HTTPProxyHTTPSTarget() {
	fmt.Println("🔐 Example 7: HTTP Proxy with HTTPS Target\n")
	fmt.Println("NOTE: http:// proxy can proxy HTTPS requests!")
	fmt.Println("      The proxy type (http/https) = how you connect TO proxy")
	fmt.Println("      The target scheme (http/https) = traffic THROUGH proxy\n")

	sender := rawhttp.NewSender()

	opts := rawhttp.Options{
		// Target is HTTPS
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,

		// Proxy is HTTP (cleartext to proxy, but HTTPS to target)
		Proxy: rawhttp.ParseProxyURL("http://127.0.0.1:8080"),
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ HTTP proxy successfully proxied HTTPS request\n")
	fmt.Printf("✓ Flow: Client -[cleartext]→ Proxy -[CONNECT]→ Client -[TLS]→ Target\n\n")
}

// Example 8: Error Handling
func example8_ErrorHandling() {
	fmt.Println("⚠️  Example 8: Proxy Error Handling\n")

	sender := rawhttp.NewSender()

	// Intentionally wrong proxy (should fail)
	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,
		Proxy:  rawhttp.ParseProxyURL("http://127.0.0.1:9999"), // Non-existent proxy
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		// Check if it's a proxy error
		if proxyErr, ok := err.(*rawhttp.ProxyError); ok {
			fmt.Printf("✓ Detected ProxyError:\n")
			fmt.Printf("  - Proxy Type: %s\n", proxyErr.ProxyType)
			fmt.Printf("  - Proxy Addr: %s\n", proxyErr.ProxyAddr)
			fmt.Printf("  - Operation: %s\n", proxyErr.Operation)
			fmt.Printf("  - Error: %v\n\n", proxyErr.Err)
		} else {
			fmt.Printf("✓ Other error: %v\n\n", err)
		}
		return
	}

	if resp != nil {
		defer resp.Body.Close()
		defer resp.Raw.Close()
	}
}

// Advanced Example: HTTPS Proxy with Custom TLS Config
func exampleAdvanced_HTTPSProxyCustomTLS() {
	fmt.Println("🔧 Advanced: HTTPS Proxy with Custom TLS Config\n")

	sender := rawhttp.NewSender()

	// Two separate TLS connections:
	// 1. Client → Proxy (uses ProxyConfig.TLSConfig)
	// 2. Proxy → Target (uses Options.TLSConfig)
	proxyConfig := &rawhttp.ProxyConfig{
		Type:     "https",
		Host:     "secure-proxy.example.com",
		Port:     8443,
		Username: "user",
		Password: "pass",

		// TLS config for connecting TO the proxy
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, // Accept proxy's self-signed cert
			MinVersion:         tls.VersionTLS12,
		},
	}

	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "api.github.com",
		Port:   443,
		Proxy:  proxyConfig,

		// TLS config for connecting to TARGET through proxy
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13, // Require TLS 1.3 for target
		},
	}

	request := []byte("GET / HTTP/1.1\r\nHost: api.github.com\r\n\r\n")

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	fmt.Printf("✓ Two separate TLS connections configured\n")
	fmt.Printf("✓ Proxy TLS: TLS 1.2+ (self-signed OK)\n")
	fmt.Printf("✓ Target TLS: TLS 1.3 required\n\n")
}
