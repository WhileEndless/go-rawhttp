//go:build ignore
// +build ignore

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
	sender := rawhttp.NewSender()
	ctx := context.Background()

	fmt.Println("=== Proxy MITM Examples (v1.1.5 Bug Fixes) ===")
	fmt.Println()

	// Example 1: Basic MITM with self-signed certificates
	// This is the scenario that revealed Bug #1 and Bug #3
	fmt.Println("=== Example 1: Basic MITM Proxy Setup ===")
	fmt.Println("Scenario: Intercepting proxy with self-signed certificates")
	request1 := []byte(`GET /api/users HTTP/1.1
Host: api.example.com
User-Agent: go-rawhttp-proxy
Accept: application/json
Authorization: Bearer token123

`)

	opts1 := rawhttp.Options{
		Scheme:      "https",
		Host:        "api.example.com",
		Port:        443,
		InsecureTLS: true, // ✅ Accept proxy's self-signed cert
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp1, err := sender.Do(ctx, request1, opts1)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp1.StatusCode)
		fmt.Printf("✓ Successfully intercepted through MITM proxy\n\n")
	}

	// Example 2: MITM with custom TLS requirements
	// Bug #1 Fix: InsecureTLS now works WITH custom TLSConfig
	fmt.Println("=== Example 2: MITM with Custom Cipher Suites ===")
	fmt.Println("Scenario: Proxy requiring specific TLS configuration")
	request2 := []byte(`POST /api/login HTTP/1.1
Host: secure.example.com
User-Agent: go-rawhttp-proxy
Content-Type: application/json
Content-Length: 45

{"username":"admin","password":"secret123"}`)

	opts2 := rawhttp.Options{
		Scheme:      "https",
		Host:        "secure.example.com",
		Port:        443,
		InsecureTLS: true, // ✅ v1.1.5: Works with custom TLSConfig!
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			// Custom cipher suites for compatibility
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
			// InsecureSkipVerify will be set by InsecureTLS flag
		},
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp2, err := sender.Do(ctx, request2, opts2)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp2.StatusCode)
		fmt.Printf("✓ Bug #1 FIXED: InsecureTLS + custom TLSConfig working!\n")
		fmt.Printf("✓ Proxy accepts custom cipher suite configuration\n\n")
	}

	// Example 3: HTTP/2 MITM proxy
	// Bug #3 Fix: HTTP/2 now supports TLS configuration
	fmt.Println("=== Example 3: HTTP/2 MITM Proxy ===")
	fmt.Println("Scenario: Modern HTTP/2 API through intercepting proxy")
	request3 := []byte(`GET /v2/api/data HTTP/2
Host: api.example.com
User-Agent: go-rawhttp-proxy
Accept: application/json

`)

	opts3 := rawhttp.Options{
		Scheme:      "https",
		Host:        "api.example.com",
		Port:        443,
		Protocol:    "http/2",
		InsecureTLS: true, // ✅ v1.1.5: HTTP/2 now supports InsecureTLS!
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp3, err := sender.Do(ctx, request3, opts3)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp3.StatusCode)
		fmt.Printf("✓ Bug #3 FIXED: HTTP/2 proxy with self-signed certs!\n")
		fmt.Printf("Protocol: %s\n\n", resp3.HTTPVersion)
	}

	// Example 4: HTTP/2 MITM with custom TLS and SNI
	// Combines Bug #3 and Bug #4 fixes
	fmt.Println("=== Example 4: HTTP/2 MITM with SNI Configuration ===")
	fmt.Println("Scenario: CDN-backed API through MITM proxy")
	request4 := []byte(`GET /cdn/assets/data.json HTTP/2
Host: cdn.example.com
User-Agent: go-rawhttp-proxy
Accept: application/json

`)

	opts4 := rawhttp.Options{
		Scheme:      "https",
		Host:        "151.101.1.69", // CDN edge IP
		Port:        443,
		Protocol:    "http/2",
		SNI:         "cdn.example.com", // ✅ v1.1.5: HTTP/2 SNI support!
		InsecureTLS: true,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2"}, // HTTP/2 ALPN
		},
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp4, err := sender.Do(ctx, request4, opts4)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp4.StatusCode)
		fmt.Printf("✓ Bug #3 FIXED: HTTP/2 TLS configuration working!\n")
		fmt.Printf("✓ Bug #4 FIXED: HTTP/2 SNI configuration working!\n")
		fmt.Printf("✓ Connected to CDN IP with correct SNI over HTTP/2\n\n")
	}

	// Example 5: Scheme override (HTTP→HTTPS with MITM)
	// Common in proxy scenarios where request is HTTP but proxy upgrades to HTTPS
	fmt.Println("=== Example 5: HTTP→HTTPS Scheme Override ===")
	fmt.Println("Scenario: Proxy receives HTTP but forwards as HTTPS")
	request5 := []byte(`GET /api/status HTTP/1.1
Host: internal.example.com
User-Agent: go-rawhttp-proxy
Accept: */*

`)

	opts5 := rawhttp.Options{
		Scheme:      "https", // Override: upgrade to HTTPS
		Host:        "internal.example.com",
		Port:        443,
		InsecureTLS: true, // ✅ Accept internal CA certificates
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp5, err := sender.Do(ctx, request5, opts5)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp5.StatusCode)
		fmt.Printf("✓ Scheme override working with InsecureTLS\n")
		fmt.Printf("✓ HTTP request upgraded to HTTPS successfully\n\n")
	}

	// Example 6: Multiple backends with different TLS requirements
	fmt.Println("=== Example 6: Multiple Backends (Different TLS Configs) ===")
	fmt.Println("Scenario: Proxy routing to multiple backends with different certs")

	backends := []struct {
		name string
		host string
		port int
		sni  string
	}{
		{"Production API", "prod-api.example.com", 443, "prod-api.example.com"},
		{"Staging API", "staging-api.example.com", 8443, "staging-api.example.com"},
		{"Internal Service", "10.0.1.50", 443, "internal.example.com"},
	}

	for _, backend := range backends {
		request := []byte(fmt.Sprintf(`GET /health HTTP/1.1
Host: %s
User-Agent: go-rawhttp-proxy
Accept: */*

`, backend.host))

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        backend.host,
			Port:        backend.port,
			SNI:         backend.sni, // ✅ Custom SNI per backend
			InsecureTLS: true,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		resp, err := sender.Do(ctx, request, opts)
		if err != nil {
			fmt.Printf("  [%s] Error: %v\n", backend.name, err)
		} else {
			fmt.Printf("  ✓ [%s] Status: %d (SNI: %s)\n", backend.name, resp.StatusCode, backend.sni)
		}
	}
	fmt.Println()

	// Summary
	fmt.Println("=== v1.1.5 MITM Proxy Improvements ===")
	fmt.Println("✓ Bug #1 FIXED: InsecureTLS works with custom TLSConfig")
	fmt.Println("✓ Bug #3 FIXED: HTTP/2 fully supports TLS configuration")
	fmt.Println("✓ Bug #4 FIXED: HTTP/2 supports SNI configuration")
	fmt.Println()
	fmt.Println("Common MITM Proxy Use Cases:")
	fmt.Println("  • Security testing and penetration testing")
	fmt.Println("  • API debugging and request inspection")
	fmt.Println("  • Development with self-signed certificates")
	fmt.Println("  • HTTP→HTTPS scheme upgrade in proxy chain")
	fmt.Println("  • CDN routing with IP-to-hostname SNI mapping")
	fmt.Println("  • Multi-backend routing with different TLS configs")
}
