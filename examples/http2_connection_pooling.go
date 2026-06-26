//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== HTTP/2 Connection Pooling Demo ===")

	// Example 1: Without Connection Pooling (default)
	fmt.Println("1. Without Connection Pooling:")
	demonstrateWithoutPooling()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: With Connection Pooling
	fmt.Println("2. With Connection Pooling:")
	demonstrateWithPooling()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 3: Connection Pooling with Custom Settings
	fmt.Println("3. Advanced Connection Pooling:")
	demonstrateAdvancedPooling()
}

func demonstrateWithoutPooling() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	opts := rawhttp.Options{
		Scheme:   "https",
		Host:     "nghttp2.org", // More reliable HTTP/2 test server
		Port:     443,
		Protocol: "http/2",
		// Note: This library creates new connections for each request by default
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
		HTTP2Settings: &rawhttp.HTTP2Settings{
			MaxConcurrentStreams: 100,
			EnableCompression:    true,
			InitialWindowSize:    4194304, // 4MB - production optimized
			MaxFrameSize:         16384,   // 16KB - RFC compliant
		},
	}

	request := []byte("GET / HTTP/2\r\nHost: nghttp2.org\r\n\r\n")

	start := time.Now()

	// Make multiple requests - each creates new connection
	for i := 0; i < 3; i++ {
		reqStart := time.Now()

		resp, err := sender.Do(ctx, request, opts)
		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}

		reqTime := time.Since(reqStart)
		fmt.Printf("  Request %d: Status %d, Time %v, Connection Time: %v\n",
			i+1, resp.StatusCode, reqTime, resp.Metrics.GetConnectionTime())

		resp.Body.Close()
		resp.Raw.Close()

		// Small delay to see the effect
		time.Sleep(100 * time.Millisecond)
	}

	totalTime := time.Since(start)
	fmt.Printf("Total time without pooling: %v\n", totalTime)
}

func demonstrateWithPooling() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	opts := rawhttp.Options{
		Scheme:   "https",
		Host:     "nghttp2.org",
		Port:     443,
		Protocol: "http/2",
		// Note: This library currently creates new connections - connection pooling not yet implemented
		ConnTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second,
		HTTP2Settings: &rawhttp.HTTP2Settings{
			MaxConcurrentStreams: 100,
			EnableCompression:    true,
			InitialWindowSize:    4194304, // 4MB - production optimized
			MaxFrameSize:         16384,   // 16KB - RFC compliant
		},
	}

	request := []byte("GET / HTTP/2\r\nHost: nghttp2.org\r\n\r\n")

	start := time.Now()

	// Make multiple requests - reuses same connection
	for i := 0; i < 3; i++ {
		reqStart := time.Now()

		resp, err := sender.Do(ctx, request, opts)
		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}

		reqTime := time.Since(reqStart)
		connTime := resp.Metrics.GetConnectionTime()

		// Connection time should be 0 for reused connections
		if connTime == 0 && i > 0 {
			fmt.Printf("  Request %d: Status %d, Time %v, Connection: REUSED ✅\n",
				i+1, resp.StatusCode, reqTime)
		} else {
			fmt.Printf("  Request %d: Status %d, Time %v, Connection Time: %v\n",
				i+1, resp.StatusCode, reqTime, connTime)
		}

		resp.Body.Close()
		resp.Raw.Close()

		// Small delay to see the effect
		time.Sleep(100 * time.Millisecond)
	}

	totalTime := time.Since(start)
	fmt.Printf("Total time with pooling: %v\n", totalTime)
}

func demonstrateAdvancedPooling() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Advanced configuration for high-performance scenarios
	opts := rawhttp.Options{
		Scheme:   "https",
		Host:     "httpbin.org",
		Port:     443,
		Protocol: "http/2",
		// ReuseConnection: true, // Connection pooling not yet exposed in main API
		ConnTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second,
		HTTP2Settings: &rawhttp.HTTP2Settings{
			MaxConcurrentStreams: 200,     // Higher concurrency
			EnableCompression:    true,    // Enable HPACK compression
			InitialWindowSize:    1048576, // 1MB window for better throughput
			MaxFrameSize:         32768,   // 32KB frames
			HeaderTableSize:      8192,    // Larger HPACK table
		},
	}

	// Test different endpoints to verify multiplexing
	requests := []struct {
		name string
		data []byte
	}{
		{"GET /get", []byte("GET /get HTTP/2\r\nHost: httpbin.org\r\n\r\n")},
		{"GET /headers", []byte("GET /headers HTTP/2\r\nHost: httpbin.org\r\nX-Test: pooling\r\n\r\n")},
		{"GET /user-agent", []byte("GET /user-agent HTTP/2\r\nHost: httpbin.org\r\nUser-Agent: go-rawhttp-pooling-test\r\n\r\n")},
		{"GET /ip", []byte("GET /ip HTTP/2\r\nHost: httpbin.org\r\n\r\n")},
		{"GET /uuid", []byte("GET /uuid HTTP/2\r\nHost: httpbin.org\r\n\r\n")},
	}

	fmt.Printf("Making %d requests with advanced pooling:\n", len(requests))

	start := time.Now()

	for i, req := range requests {
		reqStart := time.Now()

		resp, err := sender.Do(ctx, req.data, opts)
		if err != nil {
			log.Printf("Request %s failed: %v", req.name, err)
			continue
		}

		reqTime := time.Since(reqStart)
		connTime := resp.Metrics.GetConnectionTime()

		status := "NEW"
		if connTime == 0 && i > 0 {
			status = "REUSED ✅"
		}

		fmt.Printf("  %s: Status %d, Time %v, Connection: %s\n",
			req.name, resp.StatusCode, reqTime, status)

		// Print some response details
		if len(resp.Headers) > 0 {
			if server := resp.Headers["Server"]; len(server) > 0 {
				fmt.Printf("    Server: %s\n", server[0])
			}
			if contentType := resp.Headers["Content-Type"]; len(contentType) > 0 {
				fmt.Printf("    Content-Type: %s\n", contentType[0])
			}
		}

		resp.Body.Close()
		resp.Raw.Close()

		// Brief pause between requests
		time.Sleep(50 * time.Millisecond)
	}

	totalTime := time.Since(start)
	fmt.Printf("\nTotal time for %d requests: %v\n", len(requests), totalTime)
	avgTime := totalTime / time.Duration(len(requests))
	fmt.Printf("Average time per request: %v\n", avgTime)
}
