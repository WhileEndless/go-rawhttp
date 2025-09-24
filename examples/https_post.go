//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== HTTPS POST Request Example ===")

	// Create a new sender
	sender := rawhttp.NewSender()

	// JSON payload
	jsonData := `{"name": "go-rawhttp", "version": "2.0", "type": "HTTP client library"}`

	// Prepare raw HTTPS POST request
	request := fmt.Sprintf(
		"POST /post HTTP/1.1\r\n"+
			"Host: httpbin.org\r\n"+
			"Content-Type: application/json\r\n"+
			"Content-Length: %d\r\n"+
			"User-Agent: go-rawhttp/2.0\r\n"+
			"Accept: application/json\r\n"+
			"Connection: close\r\n\r\n"+
			"%s",
		len(jsonData), jsonData)

	// Configure HTTPS options
	opts := rawhttp.Options{
		Scheme:       "https",
		Host:         "httpbin.org",
		Port:         443,
		ConnTimeout:  15 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
		BodyMemLimit: 1024 * 1024, // 1MB memory limit
	}

	fmt.Printf("Sending POST request to https://%s:%d\n", opts.Host, opts.Port)

	// Send request
	resp, err := sender.Do(context.Background(), []byte(request), opts)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Print response information
	fmt.Printf("✅ Request completed successfully\n")
	fmt.Printf("Status: %s\n", resp.StatusLine)
	fmt.Printf("Body Size: %d bytes\n", resp.BodyBytes)
	fmt.Printf("Connection Time: %v\n", resp.Timings.GetConnectionTime())
	fmt.Printf("Server Time: %v\n", resp.Timings.GetServerTime())

	// Read response body
	reader, err := resp.Body.Reader()
	if err != nil {
		log.Fatalf("Failed to get body reader: %v", err)
	}
	defer reader.Close()

	bodyData, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to read body: %v", err)
	}

	fmt.Printf("Response Body:\n%s\n", bodyData)
}
