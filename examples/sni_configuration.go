//go:build ignore
// +build ignore

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Example 1: Default SNI (uses Host automatically)
	fmt.Println("=== Example 1: Default SNI Behavior ===")
	request1 := []byte(`GET / HTTP/1.1
Host: example.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts1 := rawhttp.Options{
		Scheme:      "https",
		Host:        "example.com",
		Port:        443,
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
		// SNI automatically set to "example.com" from Host field
	}

	resp1, err := sender.Do(ctx, request1, opts1)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp1.StatusCode)
		fmt.Printf("✓ SNI automatically set to Host: example.com\n\n")
	}

	// Example 2: Custom SNI (v1.1.5 new feature)
	// Useful for CDN endpoints, virtual hosting, testing
	fmt.Println("=== Example 2: Custom SNI Configuration ===")
	request2 := []byte(`GET / HTTP/1.1
Host: example.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts2 := rawhttp.Options{
		Scheme:      "https",
		Host:        "203.0.113.10", // IP address or different host
		Port:        443,
		SNI:         "example.com", // ✅ v1.1.5: Custom SNI override
		InsecureTLS: true,          // For testing
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp2, err := sender.Do(ctx, request2, opts2)
	if err != nil {
		log.Printf("Note: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp2.StatusCode)
		fmt.Printf("✓ Connected to IP but sent SNI: example.com\n\n")
	}

	// Example 3: Disable SNI completely
	// Useful for non-SNI servers, legacy systems, or special testing
	fmt.Println("=== Example 3: Disable SNI ===")
	request3 := []byte(`GET / HTTP/1.1
Host: example.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts3 := rawhttp.Options{
		Scheme:      "https",
		Host:        "example.com",
		Port:        443,
		DisableSNI:  true, // ✅ v1.1.5: Completely disable SNI
		InsecureTLS: true,
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp3, err := sender.Do(ctx, request3, opts3)
	if err != nil {
		log.Printf("Note: Some servers require SNI: %v\n\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp3.StatusCode)
		fmt.Printf("✓ Connection made without SNI\n\n")
	}

	// Example 4: HTTP/2 with custom SNI
	fmt.Println("=== Example 4: HTTP/2 with Custom SNI ===")
	request4 := []byte(`GET / HTTP/2
Host: http2.golang.org
User-Agent: go-rawhttp
Accept: */*

`)

	opts4 := rawhttp.Options{
		Scheme:      "https",
		Host:        "http2.golang.org",
		Port:        443,
		Protocol:    "http/2",
		SNI:         "http2.golang.org", // Explicit SNI for HTTP/2
		InsecureTLS: true,
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp4, err := sender.Do(ctx, request4, opts4)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp4.StatusCode)
		fmt.Printf("✓ Bug #4 FIXED: HTTP/2 now supports SNI configuration!\n")
		fmt.Printf("Protocol: %s\n\n", resp4.HTTPVersion)
	}

	// Example 5: SNI priority order with TLSConfig
	fmt.Println("=== Example 5: SNI Priority Order ===")
	request5 := []byte(`GET / HTTP/1.1
Host: example.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts5 := rawhttp.Options{
		Scheme: "https",
		Host:   "example.com", // Priority 3 (lowest)
		Port:   443,
		TLSConfig: &tls.Config{
			ServerName:         "priority1.example.com", // Priority 1 (highest)
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
		SNI:         "priority2.example.com", // Priority 2 (ignored when TLSConfig.ServerName is set)
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp5, err := sender.Do(ctx, request5, opts5)
	if err != nil {
		log.Printf("Note: %v\n\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp5.StatusCode)
		fmt.Printf("✓ TLSConfig.ServerName takes highest priority\n")
		fmt.Printf("Used SNI: priority1.example.com (from TLSConfig.ServerName)\n\n")
	}

	// Example 6: SNI priority when TLSConfig.ServerName is empty
	fmt.Println("=== Example 6: SNI Option Used When TLSConfig.ServerName Empty ===")
	request6 := []byte(`GET / HTTP/1.1
Host: example.com
User-Agent: go-rawhttp
Accept: */*

`)

	opts6 := rawhttp.Options{
		Scheme: "https",
		Host:   "host-field.example.com", // Priority 3
		Port:   443,
		TLSConfig: &tls.Config{
			// ServerName not set - will use SNI option
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		},
		SNI:         "sni-option.example.com", // Priority 2 (used!)
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp6, err := sender.Do(ctx, request6, opts6)
	if err != nil {
		log.Printf("Note: %v\n\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp6.StatusCode)
		fmt.Printf("✓ SNI option used when TLSConfig.ServerName is empty\n")
		fmt.Printf("Used SNI: sni-option.example.com\n\n")
	}

	// Example 7: CDN and Virtual Hosting use case
	fmt.Println("=== Example 7: Real-World CDN Use Case ===")
	fmt.Println("Scenario: Connecting to CDN edge server with specific SNI")
	request7 := []byte(`GET /api/data HTTP/1.1
Host: myapi.example.com
User-Agent: go-rawhttp
Accept: application/json

`)

	opts7 := rawhttp.Options{
		Scheme:      "https",
		Host:        "151.101.1.69", // Fastly CDN edge IP (example)
		Port:        443,
		SNI:         "myapi.example.com", // ✅ SNI matches the virtual host
		InsecureTLS: true,                // For demo purposes
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp7, err := sender.Do(ctx, request7, opts7)
	if err != nil {
		log.Printf("Note: CDN example: %v\n", err)
	} else {
		fmt.Printf("✓ Status: %d\n", resp7.StatusCode)
		fmt.Printf("✓ Connected to CDN edge IP with correct SNI\n")
		fmt.Printf("✓ Virtual hosting works correctly\n\n")
	}

	// Summary
	fmt.Println("=== SNI Configuration Summary (v1.1.5) ===")
	fmt.Println("Priority order (highest to lowest):")
	fmt.Println("  1. TLSConfig.ServerName (if set)")
	fmt.Println("  2. SNI option (if TLSConfig.ServerName is empty)")
	fmt.Println("  3. Host field (default fallback)")
	fmt.Println("  4. Empty (if DisableSNI is true)")
	fmt.Println()
	fmt.Println("✓ Works with both HTTP/1.1 and HTTP/2")
	fmt.Println("✓ Useful for CDNs, virtual hosting, and testing scenarios")
	fmt.Println("✓ Can be completely disabled with DisableSNI flag")
}
