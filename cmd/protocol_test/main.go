package main

import (
	"context"
	"fmt"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

func main() {
	fmt.Println("=== Protocol Selection Test with Proxy ===\n")

	sender := rawhttp.NewSender()
	ctx := context.Background()

	// Test 1: Explicit HTTP/1.1
	fmt.Println("Test 1: Explicit Protocol = http/1.1")
	opts1 := rawhttp.Options{
		Host:        "httpbin.org",
		Port:        443,
		Scheme:      "https",
		Protocol:    "http/1.1", // EXPLICIT
		InsecureTLS: true,
		Proxy: &rawhttp.ProxyConfig{
			Type: "http",
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	req1 := []byte("GET /get HTTP/1.1\r\nHost: httpbin.org\r\n\r\n")
	resp1, err := sender.Do(ctx, req1, opts1)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("  HTTPVersion: %s\n", resp1.HTTPVersion)
		fmt.Printf("  NegotiatedProtocol: %s\n", resp1.NegotiatedProtocol)
		fmt.Printf("  TLSVersion: %s\n", resp1.TLSVersion)
		resp1.Body.Close()
		resp1.Raw.Close()
	}

	fmt.Println("\n============================================================\n")

	// Test 2: Explicit HTTP/2
	fmt.Println("Test 2: Explicit Protocol = http/2")
	opts2 := rawhttp.Options{
		Host:        "httpbin.org",
		Port:        443,
		Scheme:      "https",
		Protocol:    "http/2", // EXPLICIT
		InsecureTLS: true,
		Proxy: &rawhttp.ProxyConfig{
			Type: "http",
			Host: "127.0.0.1",
			Port: 8080,
		},
	}

	req2 := []byte("GET /get HTTP/2\r\nHost: httpbin.org\r\n\r\n")
	resp2, err := sender.Do(ctx, req2, opts2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("  HTTPVersion: %s\n", resp2.HTTPVersion)
		fmt.Printf("  NegotiatedProtocol: %s\n", resp2.NegotiatedProtocol)
		fmt.Printf("  TLSVersion: %s\n", resp2.TLSVersion)
		resp2.Body.Close()
		resp2.Raw.Close()
	}

	fmt.Println("\n============================================================\n")
	fmt.Println("Burpsuite'te şimdi 2 istek görmelisin:")
	fmt.Println("  - 1. İstek: HTTP/1.1 (eğer kod doğruysa)")
	fmt.Println("  - 2. İstek: HTTP/2")
	fmt.Println("\nEğer ikisi de HTTP/2 görünüyorsa, Burpsuite MITM yapıyor demektir.")
}
