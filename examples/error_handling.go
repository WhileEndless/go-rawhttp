//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== Error Handling Example ===")

	sender := rawhttp.NewSender()

	// Test different error scenarios
	testCases := []struct {
		name    string
		request []byte
		opts    rawhttp.Options
	}{
		{
			name:    "DNS Resolution Error",
			request: []byte("GET / HTTP/1.1\r\nHost: nonexistent.example\r\nConnection: close\r\n\r\n"),
			opts: rawhttp.Options{
				Scheme:      "http",
				Host:        "nonexistent.example",
				Port:        80,
				ConnTimeout: 5 * time.Second,
			},
		},
		{
			name:    "Connection Timeout",
			request: []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"),
			opts: rawhttp.Options{
				Scheme:      "http",
				Host:        "example.com",
				Port:        81, // Typically closed port
				ConnTimeout: 2 * time.Second,
			},
		},
		{
			name:    "Validation Error",
			request: []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"),
			opts: rawhttp.Options{
				Scheme: "ftp", // Invalid scheme
				Host:   "example.com",
				Port:   80,
			},
		},
	}

	for _, tc := range testCases {
		fmt.Printf("\n--- %s ---\n", tc.name)

		resp, err := sender.Do(context.Background(), tc.request, tc.opts)
		if err != nil {
			// Handle structured errors
			if rawErr, ok := err.(*rawhttp.Error); ok {
				fmt.Printf("Error Type: %s\n", rawErr.Type)
				fmt.Printf("Error Message: %s\n", rawErr.Message)
				fmt.Printf("Host: %s\n", rawErr.Host)
				if rawErr.Port > 0 {
					fmt.Printf("Port: %d\n", rawErr.Port)
				}
				fmt.Printf("Timestamp: %s\n", rawErr.Timestamp.Format(time.RFC3339))

				// Check specific error types
				if rawhttp.IsTimeoutError(err) {
					fmt.Printf("This is a timeout error\n")
				}
			} else {
				fmt.Printf("Unexpected error: %v\n", err)
			}
		} else {
			fmt.Printf("Unexpected success: %d\n", resp.StatusCode)
			resp.Body.Close()
			resp.Raw.Close()
		}
	}

	// Example of successful request for comparison
	fmt.Printf("\n--- Successful Request ---\n")
	request := []byte("GET /ip HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n")
	opts := rawhttp.DefaultOptions("https", "httpbin.org", 443)

	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Printf("Unexpected error: %v", err)
	} else {
		fmt.Printf("✅ Success: %d\n", resp.StatusCode)
		fmt.Printf("Timings: %s\n", resp.Timings.String())
		resp.Body.Close()
		resp.Raw.Close()
	}
}
