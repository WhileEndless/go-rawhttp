//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/WhileEndless/go-rawhttp/pkg/http2"
)

// This example demonstrates low-level HTTP/2 frame manipulation
// Similar to how Burp Suite allows manual frame editing

func main() {
	fmt.Println("=== HTTP/2 Raw Frame Manipulation ===\n")

	// Example 1: Basic frame creation and sending
	fmt.Println("1. Basic Frame Manipulation:")
	demonstrateBasicFrames()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: Custom HEADERS frame with specific flags
	fmt.Println("2. Custom HEADERS Frame:")
	demonstrateCustomHeaders()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 3: Multiple frames for large payload
	fmt.Println("3. Multi-Frame Request:")
	demonstrateMultiFrame()
}

func demonstrateBasicFrames() {
	// Create HTTP/2 client directly
	opts := &http2.Options{
		EnableCompression:    true,
		MaxConcurrentStreams: 100,
		InitialWindowSize:    65535,
		MaxFrameSize:         16384,
		HeaderTableSize:      4096,
	}

	client := http2.NewClient(opts)
	defer client.Close()

	ctx := context.Background()

	// Method 1: Use high-level interface (converts HTTP/1.1 to frames)
	request := []byte("GET /get HTTP/2\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n")

	fmt.Println("Using high-level interface:")
	resp, err := client.Do(ctx, request, "httpbin.org", 443, "https")
	if err != nil {
		log.Printf("High-level request failed: %v", err)
		return
	}

	fmt.Printf("  Status: %d\n", resp.Status)
	fmt.Printf("  Protocol: %s\n", resp.HTTPVersion)
	fmt.Printf("  Frames received: %d\n", len(resp.Frames))

	// Show frame details
	for i, frame := range resp.Frames {
		fmt.Printf("  Frame %d: %T\n", i, frame)
	}
}

func demonstrateCustomHeaders() {
	opts := &http2.Options{
		EnableCompression:    true,
		MaxConcurrentStreams: 100,
		InitialWindowSize:    65535,
	}

	client := http2.NewClient(opts)
	defer client.Close()

	ctx := context.Background()

	// Method 2: Create frames manually for fine-grained control
	fmt.Println("Creating custom HEADERS frame:")

	// Custom headers map
	headers := map[string]string{
		":method":         "POST",
		":path":           "/post",
		":scheme":         "https",
		":authority":      "httpbin.org",
		"content-type":    "application/json",
		"content-length":  "27",
		"x-custom-header": "custom-value",
		"user-agent":      "go-rawhttp-frame-test/1.0",
		"accept":          "application/json",
		"authorization":   "Bearer example-token",
	}

	// Create HEADERS frame
	headersFrame := &http2.HeadersFrame{
		StreamId:   1,
		Headers:    headers,
		EndStream:  false, // We'll send DATA frame after
		EndHeaders: true,
		Priority:   nil,
	}

	// Create DATA frame with JSON payload
	jsonPayload := `{"message": "Hello HTTP/2!"}`
	dataFrame := &http2.DataFrame{
		StreamId:  1,
		Data:      []byte(jsonPayload),
		EndStream: true, // End of request
	}

	// Send frames
	frames := []http2.Frame{headersFrame, dataFrame}

	resp, err := client.DoFrames(ctx, frames, "httpbin.org", 443, "https")
	if err != nil {
		log.Printf("Frame-based request failed: %v", err)
		return
	}

	fmt.Printf("  Status: %d\n", resp.Status)
	fmt.Printf("  Body length: %d bytes\n", len(resp.Body))
	fmt.Printf("  Response frames: %d\n", len(resp.Frames))

	// Show response body (httpbin.org echoes back the JSON)
	if len(resp.Body) > 0 && len(resp.Body) < 1000 {
		fmt.Printf("  Response body: %s\n", string(resp.Body[:min(len(resp.Body), 200)]))
	}
}

func demonstrateMultiFrame() {
	opts := &http2.Options{
		EnableCompression:    true,
		MaxConcurrentStreams: 100,
		InitialWindowSize:    65535,
		MaxFrameSize:         16384, // Force smaller frames
	}

	client := http2.NewClient(opts)
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Sending large payload across multiple DATA frames:")

	// Headers for large POST request
	headers := map[string]string{
		":method":        "POST",
		":path":          "/post",
		":scheme":        "https",
		":authority":     "httpbin.org",
		"content-type":   "text/plain",
		"content-length": "65536", // 64KB
	}

	// Create HEADERS frame
	headersFrame := &http2.HeadersFrame{
		StreamId:   1,
		Headers:    headers,
		EndStream:  false,
		EndHeaders: true,
	}

	// Create multiple DATA frames for large payload
	frames := []http2.Frame{headersFrame}

	// Generate large payload (64KB)
	chunkSize := 8192 // 8KB per frame
	totalSize := 65536

	for offset := 0; offset < totalSize; offset += chunkSize {
		end := offset + chunkSize
		if end > totalSize {
			end = totalSize
		}

		// Create chunk data
		chunk := make([]byte, end-offset)
		for i := range chunk {
			chunk[i] = byte('A' + (offset/chunkSize)%26) // Different letter per chunk
		}

		// Create DATA frame
		dataFrame := &http2.DataFrame{
			StreamId:  1,
			Data:      chunk,
			EndStream: (end == totalSize), // Last frame ends the stream
		}

		frames = append(frames, dataFrame)
	}

	fmt.Printf("  Created %d frames (%d HEADERS + %d DATA)\n",
		len(frames), 1, len(frames)-1)

	resp, err := client.DoFrames(ctx, frames, "httpbin.org", 443, "https")
	if err != nil {
		log.Printf("Multi-frame request failed: %v", err)
		return
	}

	fmt.Printf("  Status: %d\n", resp.Status)
	fmt.Printf("  Response body length: %d bytes\n", len(resp.Body))
	fmt.Printf("  Response frames received: %d\n", len(resp.Frames))

	// Analyze response frames
	for i, frame := range resp.Frames {
		switch f := frame.(type) {
		case *http2.HeadersFrame:
			fmt.Printf("  Frame %d: HEADERS (stream %d, headers: %d)\n",
				i, f.StreamId, len(f.Headers))
		case *http2.DataFrame:
			fmt.Printf("  Frame %d: DATA (stream %d, %d bytes, end_stream: %v)\n",
				i, f.StreamId, len(f.Data), f.EndStream)
		}
	}
}

// Helper function for Go versions that don't have min builtin
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
