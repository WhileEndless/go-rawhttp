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

	// Example 1: InsecureTLS flag for self-signed certificates
	fmt.Println("=== Example 1: Simple InsecureTLS (Self-Signed Certs) ===")
	request1 := []byte(`GET / HTTP/1.1
Host: self-signed.badssl.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts1 := rawhttp.Options{
		Scheme:      "https",
		Host:        "self-signed.badssl.com",
		Port:        443,
		InsecureTLS: true, // Skip certificate verification
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp1, err := sender.Do(ctx, request1, opts1)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d (accepted self-signed cert)\n", resp1.StatusCode)
		fmt.Printf("Protocol: %s\n\n", resp1.HTTPVersion)
	}

	// Example 2: Custom TLS config with specific TLS version
	fmt.Println("=== Example 2: Custom TLS Config (TLS 1.3 Required) ===")
	request2 := []byte(`GET / HTTP/1.1
Host: tls-v1-2.badssl.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts2 := rawhttp.Options{
		Scheme: "https",
		Host:   "tls-v1-2.badssl.com",
		Port:   1012,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // Require TLS 1.2 or higher
			MaxVersion: tls.VersionTLS13,
		},
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp2, err := sender.Do(ctx, request2, opts2)
	if err != nil {
		log.Printf("Expected: Server only supports TLS 1.2: %v\n\n", err)
	} else {
		fmt.Printf("Status: %d\n", resp2.StatusCode)
		fmt.Printf("TLS Version negotiated\n\n")
	}

	// Example 3: InsecureTLS WITH custom TLS config (v1.1.5 Bug #1 fix)
	// Previously this combination didn't work - InsecureTLS was ignored
	fmt.Println("=== Example 3: InsecureTLS + Custom TLS Config (v1.1.5 Fix) ===")
	request3 := []byte(`GET / HTTP/1.1
Host: self-signed.badssl.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts3 := rawhttp.Options{
		Scheme:      "https",
		Host:        "self-signed.badssl.com",
		Port:        443,
		InsecureTLS: true, // ✅ v1.1.5: Now works WITH custom TLSConfig
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Custom cipher suites, ALPN protocols, etc.
			NextProtos: []string{"http/1.1", "h2"},
		},
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp3, err := sender.Do(ctx, request3, opts3)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp3.StatusCode)
		fmt.Printf("✓ Bug #1 FIXED: InsecureTLS works with custom TLSConfig!\n")
		fmt.Printf("Protocol: %s\n\n", resp3.HTTPVersion)
	}

	// Example 4: HTTP/2 with InsecureTLS (v1.1.5 Bug #3 fix)
	fmt.Println("=== Example 4: HTTP/2 with InsecureTLS (v1.1.5 Fix) ===")
	request4 := []byte(`GET / HTTP/2
Host: self-signed.badssl.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts4 := rawhttp.Options{
		Scheme:      "https",
		Host:        "self-signed.badssl.com",
		Port:        443,
		Protocol:    "http/2",
		InsecureTLS: true, // ✅ v1.1.5: Now works with HTTP/2!
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp4, err := sender.Do(ctx, request4, opts4)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp4.StatusCode)
		fmt.Printf("✓ Bug #3 FIXED: HTTP/2 now supports InsecureTLS!\n")
		fmt.Printf("Protocol: %s\n\n", resp4.HTTPVersion)
	}

	// Example 5: HTTP/2 with custom TLS config
	fmt.Println("=== Example 5: HTTP/2 + Custom TLS Config ===")
	request5 := []byte(`GET / HTTP/2
Host: http2.golang.org
User-Agent: go-rawhttp
Accept: */*

`)

	opts5 := rawhttp.Options{
		Scheme:   "https",
		Host:     "http2.golang.org",
		Port:     443,
		Protocol: "http/2",
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2"}, // HTTP/2 ALPN
			// Custom settings for security or compatibility
		},
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp5, err := sender.Do(ctx, request5, opts5)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp5.StatusCode)
		fmt.Printf("Protocol: %s\n", resp5.HTTPVersion)
		fmt.Printf("✓ HTTP/2 with full TLS control\n\n")
	}

	// Example 6: Priority of TLS settings
	fmt.Println("=== Example 6: TLS Configuration Priority ===")
	fmt.Println("Priority order:")
	fmt.Println("1. If TLSConfig is provided → use it (clone)")
	fmt.Println("2. If InsecureTLS is true → set InsecureSkipVerify = true")
	fmt.Println("3. Otherwise → use default TLS config")
	fmt.Println()
	fmt.Println("✓ InsecureTLS flag now works as override even with custom TLSConfig")
	fmt.Println("✓ Both HTTP/1.1 and HTTP/2 support full TLS configuration")
}
