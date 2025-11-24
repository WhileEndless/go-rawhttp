package main

import (
	"context"
	"fmt"
	"time"

	rawhttp "github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	fmt.Println("=== Simple Connection Pooling Test (No Proxy) ===\n")

	sender := rawhttp.NewSender()
	ctx := context.Background()

	opts := rawhttp.Options{
		Host:            "cyberwise.com",
		Port:            443,
		Scheme:          "https",
		Protocol:        "http/1.1",
		ReuseConnection: true,
		InsecureTLS:     true,
	}

	rawReq := []byte("GET / HTTP/1.1\r\nHost: cyberwise.com\r\n\r\n")

	// Request 1
	fmt.Println("Making Request 1...")
	resp1, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request 1:\n")
	fmt.Printf("  Connection Reused: %v\n", resp1.ConnectionReused)
	fmt.Printf("  Body Size: %d bytes\n", resp1.BodyBytes)

	resp1.Body.Close()
	resp1.Raw.Close()

	time.Sleep(100 * time.Millisecond)

	// Request 2
	fmt.Println("\nMaking Request 2...")
	resp2, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request 2:\n")
	fmt.Printf("  Connection Reused: %v\n", resp2.ConnectionReused)
	fmt.Printf("  Body Size: %d bytes\n", resp2.BodyBytes)

	resp2.Body.Close()
	resp2.Raw.Close()

	if resp2.ConnectionReused {
		fmt.Println("\n✅ SUCCESS: Connection pooling works without proxy!")
	} else {
		fmt.Println("\n❌ FAILURE: Connection pooling not working even without proxy!")
	}
}
