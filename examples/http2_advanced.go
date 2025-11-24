//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	// Example 1: HTTP/2 Multiplexing (multiple requests on same connection)
	fmt.Println("=== HTTP/2 Multiplexing Example ===")
	demonstrateMultiplexing()

	// Example 2: H2C (HTTP/2 over cleartext)
	fmt.Println("\n=== H2C (Cleartext HTTP/2) Example ===")
	demonstrateH2C()

	// Example 3: Custom headers and pseudo-headers
	fmt.Println("\n=== Custom Headers Example ===")
	demonstrateCustomHeaders()

	// Example 4: Large file download with flow control
	fmt.Println("\n=== Large Download with Flow Control ===")
	demonstrateLargeDownload()
}

func demonstrateMultiplexing() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Enable connection reuse for multiplexing
	baseOpts := rawhttp.Options{
		Scheme:   "https",
		Host:     "http2.golang.org",
		Port:     443,
		Protocol: "http/2",
		HTTP2Settings: &rawhttp.HTTP2Settings{
			MaxConcurrentStreams: 10,
			EnableServerPush:     false,
		},
	}

	// Prepare multiple requests
	requests := []struct {
		path string
		data []byte
	}{
		{"/page1", []byte("GET /page1 HTTP/2\r\nHost: http2.golang.org\r\n\r\n")},
		{"/page2", []byte("GET /page2 HTTP/2\r\nHost: http2.golang.org\r\n\r\n")},
		{"/page3", []byte("GET /page3 HTTP/2\r\nHost: http2.golang.org\r\n\r\n")},
		{"/api/status", []byte("GET /api/status HTTP/2\r\nHost: http2.golang.org\r\n\r\n")},
		{"/api/info", []byte("GET /api/info HTTP/2\r\nHost: http2.golang.org\r\n\r\n")},
	}

	// Send requests concurrently
	var wg sync.WaitGroup
	start := time.Now()

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, request []byte, path string) {
			defer wg.Done()

			resp, err := sender.Do(ctx, request, baseOpts)
			if err != nil {
				log.Printf("Request %d (%s) failed: %v", idx, path, err)
				return
			}

			fmt.Printf("Request %d (%s): Status %d, Protocol %s\n",
				idx, path, resp.StatusCode, resp.HTTPVersion)
		}(i, req.data, req.path)
	}

	wg.Wait()
	fmt.Printf("All requests completed in %v\n", time.Since(start))
}

func demonstrateH2C() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// H2C request (HTTP/2 over cleartext)
	request := []byte(`GET / HTTP/2
Host: localhost:8080
Accept: text/html

`)

	opts := rawhttp.Options{
		Scheme:   "http", // Note: http, not https
		Host:     "localhost",
		Port:     8080,
		Protocol: "http/2",
		HTTP2Settings: &rawhttp.HTTP2Settings{
			// H2C specific settings
			MaxConcurrentStreams: 100,
			InitialWindowSize:    65535,
		},
	}

	// Note: This will only work if you have an H2C-enabled server running
	resp, err := sender.Do(ctx, request, opts)
	if err != nil {
		fmt.Printf("H2C request failed (expected if no H2C server): %v\n", err)
		return
	}

	fmt.Printf("H2C Response: Status %d, Protocol %s\n",
		resp.StatusCode, resp.HTTPVersion)
}

func demonstrateCustomHeaders() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Request with custom headers and values
	request := []byte(`GET /api/test HTTP/2
Host: http2.golang.org
X-Custom-Header: custom-value
X-Request-ID: 12345-67890-abcdef
Authorization: Bearer example-token
Accept-Encoding: gzip, deflate, br
Cache-Control: no-cache
User-Agent: go-rawhttp/2.0 (custom-client)

`)

	opts := rawhttp.Options{
		Scheme:      "https",
		Host:        "http2.golang.org",
		Port:        443,
		Protocol:    "http/2",
		ConnTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second,
	}

	resp, err := sender.Do(ctx, request, opts)
	if err != nil {
		log.Printf("Custom headers request failed: %v", err)
		return
	}

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Response Protocol: %s\n", resp.HTTPVersion)

	// Print response headers
	fmt.Println("Response Headers:")
	for name, values := range resp.Headers {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", name, value)
		}
	}
}

func demonstrateLargeDownload() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Request for large resource
	request := []byte(`GET /large-file HTTP/2
Host: http2.golang.org
Accept: application/octet-stream

`)

	opts := rawhttp.Options{
		Scheme:       "https",
		Host:         "http2.golang.org",
		Port:         443,
		Protocol:     "http/2",
		ReadTimeout:  60 * time.Second,
		BodyMemLimit: 1024 * 1024, // 1MB before spilling to disk
		HTTP2Settings: &rawhttp.HTTP2Settings{
			InitialWindowSize: 1024 * 1024 * 10, // 10MB window
			MaxFrameSize:      1024 * 64,        // 64KB frames
		},
	}

	start := time.Now()
	resp, err := sender.Do(ctx, request, opts)
	if err != nil {
		log.Printf("Large download failed: %v", err)
		return
	}

	downloadTime := time.Since(start)

	fmt.Printf("Download completed:\n")
	fmt.Printf("  Status: %d\n", resp.StatusCode)
	fmt.Printf("  Protocol: %s\n", resp.HTTPVersion)
	fmt.Printf("  Size: %d bytes\n", resp.BodyBytes)
	fmt.Printf("  Time: %v\n", downloadTime)

	if resp.BodyBytes > 0 {
		throughput := float64(resp.BodyBytes) / downloadTime.Seconds() / 1024 / 1024
		fmt.Printf("  Throughput: %.2f MB/s\n", throughput)
	}

	// Clean up
	resp.Body.Close()
	resp.Raw.Close()
}
