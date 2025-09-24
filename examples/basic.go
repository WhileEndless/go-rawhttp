//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== Basic HTTP Request Example ===")

	// Create a new sender
	sender := rawhttp.NewSender()

	// Prepare raw HTTP request
	request := []byte("GET / HTTP/1.1\r\nHost: httpbin.org\r\nUser-Agent: go-rawhttp/1.0\r\nConnection: close\r\n\r\n")

	// Use default options
	opts := rawhttp.DefaultOptions("http", "httpbin.org", 80)

	// Send request
	resp, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Print response information
	fmt.Printf("Status: %s\n", resp.StatusLine)
	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("Body Size: %d bytes\n", resp.BodyBytes)
	fmt.Printf("Timings: %s\n", resp.Timings.String())

	// Print some headers
	fmt.Printf("Content-Type: %v\n", resp.Headers["Content-Type"])
	fmt.Printf("Server: %v\n", resp.Headers["Server"])
}
