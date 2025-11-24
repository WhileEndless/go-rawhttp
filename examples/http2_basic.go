//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	// Create a new sender with HTTP/2 support
	sender := rawhttp.NewSender()

	// Example 1: Simple HTTP/2 GET request (auto-detection from request line)
	fmt.Println("=== Example 1: Simple HTTP/2 GET ===")
	request1 := []byte(`GET / HTTP/2
Host: http2.golang.org
User-Agent: go-rawhttp/2.0
Accept: text/html

`)

	ctx := context.Background()
	opts1 := rawhttp.Options{
		Scheme:      "https",
		Host:        "http2.golang.org",
		Port:        443,
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
		// Protocol is auto-detected from "HTTP/2" in request line
	}

	resp1, err := sender.Do(ctx, request1, opts1)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Status: %d\n", resp1.StatusCode)
		fmt.Printf("Protocol: %s\n", resp1.HTTPVersion)
		fmt.Printf("Headers: %v\n", resp1.Headers)
		fmt.Println()
	}

	// Example 2: HTTP/2 POST with explicit protocol setting
	fmt.Println("=== Example 2: HTTP/2 POST with JSON ===")
	request2 := []byte(`POST /api/echo HTTP/1.1
Host: http2.golang.org
Content-Type: application/json
Accept: application/json

{"message": "Hello HTTP/2"}`)

	opts2 := rawhttp.Options{
		Scheme:      "https",
		Host:        "http2.golang.org",
		Port:        443,
		Protocol:    "http/2", // Explicitly use HTTP/2
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
	}

	resp2, err := sender.Do(ctx, request2, opts2)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Status: %d\n", resp2.StatusCode)
		fmt.Printf("Protocol: %s\n", resp2.HTTPVersion)

		// Read response body
		bodyBytes := resp2.Body.Bytes()
		if len(bodyBytes) > 0 {
			fmt.Printf("Response: %s\n", string(bodyBytes))
		}
		fmt.Println()
	}

	// Example 3: HTTP/2 with custom settings
	fmt.Println("=== Example 3: HTTP/2 with Custom Settings ===")
	request3 := []byte(`GET /large-resource HTTP/2
Host: http2.golang.org
Accept: */*

`)

	opts3 := rawhttp.Options{
		Scheme:      "https",
		Host:        "http2.golang.org",
		Port:        443,
		Protocol:    "http/2",
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
		HTTP2Settings: &rawhttp.HTTP2Settings{
			EnableServerPush:     false,    // Recommended for security
			EnableCompression:    true,     // HPACK compression
			MaxConcurrentStreams: 100,      // Production default
			InitialWindowSize:    4194304,  // 4MB window (production optimized)
			MaxFrameSize:         16384,    // 16KB frames (RFC compliant)
			MaxHeaderListSize:    10485760, // 10MB header limit
			HeaderTableSize:      4096,     // 4KB HPACK table
		},
	}

	resp3, err := sender.Do(ctx, request3, opts3)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Status: %d\n", resp3.StatusCode)
		fmt.Printf("Protocol: %s\n", resp3.HTTPVersion)
		fmt.Printf("Response size: %d bytes\n", resp3.BodyBytes)
		fmt.Println()
	}

	// Example 4: Comparing HTTP/1.1 vs HTTP/2
	fmt.Println("=== Example 4: Protocol Comparison ===")
	request4 := []byte(`GET /benchmark HTTP/1.1
Host: http2.golang.org
User-Agent: go-rawhttp/benchmark

`)

	// Test with HTTP/1.1
	opts4_http1 := rawhttp.Options{
		Scheme:   "https",
		Host:     "http2.golang.org",
		Port:     443,
		Protocol: "http/1.1", // Force HTTP/1.1
	}

	start := time.Now()
	resp4_1, err := sender.Do(ctx, request4, opts4_http1)
	http1Time := time.Since(start)

	if err == nil {
		fmt.Printf("HTTP/1.1 - Status: %d, Time: %v\n", resp4_1.StatusCode, http1Time)
	}

	// Test with HTTP/2
	opts4_http2 := rawhttp.Options{
		Scheme:   "https",
		Host:     "http2.golang.org",
		Port:     443,
		Protocol: "http/2", // Force HTTP/2
	}

	start = time.Now()
	resp4_2, err := sender.Do(ctx, request4, opts4_http2)
	http2Time := time.Since(start)

	if err == nil {
		fmt.Printf("HTTP/2   - Status: %d, Time: %v\n", resp4_2.StatusCode, http2Time)
	}

	fmt.Printf("Speed improvement: %.2fx\n", float64(http1Time)/float64(http2Time))
}
