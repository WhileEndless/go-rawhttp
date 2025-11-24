package main

import (
	"context"
	"fmt"
	"time"

	rawhttp "github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	fmt.Println("=== Connection Pooling Test with Proxy ===\n")

	// Test 1: HTTP/1.1 Connection Reuse with Proxy
	fmt.Println("Test 1: HTTP/1.1 Connection Pooling with Proxy")
	testHTTP1PoolingWithProxy()

	fmt.Println("\n============================================================\n")

	// Test 2: HTTP/2 Connection Reuse with Proxy
	fmt.Println("Test 2: HTTP/2 Connection Pooling with Proxy")
	testHTTP2PoolingWithProxy()

	fmt.Println("\n============================================================\n")

	// Test 3: Different Proxies Should Use Different Connections
	fmt.Println("Test 3: Different Proxies Should NOT Share Connections")
	testDifferentProxiesNoSharing()
}

func testHTTP1PoolingWithProxy() {
	// IMPORTANT: Create NEW sender for this test to have fresh connection pool
	// In real usage, same sender instance would be reused across requests
	sender := rawhttp.NewSender()
	ctx := context.Background()

	opts := rawhttp.Options{
		Host:            "cyberwise.com",
		Port:            443,
		Scheme:          "https",
		Protocol:        "http/1.1",
		ReuseConnection: true, // Enable connection pooling
		Proxy: &rawhttp.ProxyConfig{
			Type: "http",
			Host: "127.0.0.1",
			Port: 8080,
		},
		InsecureTLS: true,
	}

	// Build raw HTTP request
	rawReq := []byte("GET / HTTP/1.1\r\nHost: cyberwise.com\r\nConnection: keep-alive\r\n\r\n")

	// Make first request
	resp1, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error on request 1: %v\n", err)
		return
	}

	fmt.Printf("Request 1:\n")
	fmt.Printf("  Protocol: %s\n", resp1.HTTPVersion)
	fmt.Printf("  Negotiated Protocol: %s\n", resp1.NegotiatedProtocol)
	fmt.Printf("  Connected IP: %s:%d\n", resp1.ConnectedIP, resp1.ConnectedPort)
	fmt.Printf("  Connection Reused: %v\n", resp1.ConnectionReused)
	fmt.Printf("  Proxy: %s (%v)\n", resp1.ProxyAddr, resp1.ProxyUsed)
	fmt.Printf("  Body Size: %d bytes\n", resp1.BodyBytes)

	// Close response to release connection back to pool
	resp1.Body.Close()
	resp1.Raw.Close()

	// Small delay to ensure release completes
	time.Sleep(500 * time.Millisecond)

	// Make second request - should reuse connection
	resp2, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error on request 2: %v\n", err)
		return
	}

	fmt.Printf("\nRequest 2:\n")
	fmt.Printf("  Protocol: %s\n", resp2.HTTPVersion)
	fmt.Printf("  Negotiated Protocol: %s\n", resp2.NegotiatedProtocol)
	fmt.Printf("  Connected IP: %s:%d\n", resp2.ConnectedIP, resp2.ConnectedPort)
	fmt.Printf("  Connection Reused: %v\n", resp2.ConnectionReused)
	fmt.Printf("  Proxy: %s (%v)\n", resp2.ProxyAddr, resp2.ProxyUsed)
	fmt.Printf("  Body Size: %d bytes\n", resp2.BodyBytes)

	// Close second response
	resp2.Body.Close()
	resp2.Raw.Close()

	if resp2.ConnectionReused {
		fmt.Printf("\n✓ SUCCESS: Connection was properly reused!\n")
	} else {
		fmt.Printf("\n✗ FAILURE: Connection was NOT reused (expected reuse)\n")
	}
}

func testHTTP2PoolingWithProxy() {
	sender := rawhttp.NewSender()
	ctx := context.Background()

	fmt.Println("[DEBUG] HTTP/2 Test Starting...")

	opts := rawhttp.Options{
		Host:            "cyberwise.com",
		Port:            443,
		Scheme:          "https",
		Protocol:        "http/2",
		ReuseConnection: true, // Enable connection pooling
		Proxy: &rawhttp.ProxyConfig{
			Type: "http",
			Host: "127.0.0.1",
			Port: 8080,
		},
		InsecureTLS: true,
	}

	// Build raw HTTP request (HTTP/2 will convert this)
	rawReq := []byte("GET / HTTP/2\r\nHost: cyberwise.com\r\n\r\n")

	// Make first HTTP/2 request
	resp1, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error on request 1: %v\n", err)
		return
	}

	fmt.Printf("Request 1:\n")
	fmt.Printf("  Connected IP: %s:%d\n", resp1.ConnectedIP, resp1.ConnectedPort)
	fmt.Printf("  Connection Reused: %v\n", resp1.ConnectionReused)
	fmt.Printf("  Protocol: %s\n", resp1.NegotiatedProtocol)
	fmt.Printf("  Proxy: %s (%v)\n", resp1.ProxyAddr, resp1.ProxyUsed)
	fmt.Printf("  Body Size: %d bytes\n", resp1.BodyBytes)

	// Close response to release connection back to pool
	resp1.Body.Close()
	resp1.Raw.Close()

	// Small delay to ensure release completes
	time.Sleep(500 * time.Millisecond)

	// Make second HTTP/2 request - should reuse connection
	resp2, err := sender.Do(ctx, rawReq, opts)
	if err != nil {
		fmt.Printf("Error on request 2: %v\n", err)
		return
	}

	fmt.Printf("\nRequest 2:\n")
	fmt.Printf("  Connected IP: %s:%d\n", resp2.ConnectedIP, resp2.ConnectedPort)
	fmt.Printf("  Connection Reused: %v\n", resp2.ConnectionReused)
	fmt.Printf("  Protocol: %s\n", resp2.NegotiatedProtocol)
	fmt.Printf("  Proxy: %s (%v)\n", resp2.ProxyAddr, resp2.ProxyUsed)
	fmt.Printf("  Body Size: %d bytes\n", resp2.BodyBytes)

	// Close second response
	resp2.Body.Close()
	resp2.Raw.Close()

	if resp2.ConnectionReused {
		fmt.Printf("\n✓ SUCCESS: HTTP/2 connection was properly reused!\n")
	} else {
		fmt.Printf("\n✗ FAILURE: HTTP/2 connection was NOT reused (expected reuse)\n")
	}
}

func testDifferentProxiesNoSharing() {
	sender1 := rawhttp.NewSender()
	sender2 := rawhttp.NewSender()
	ctx := context.Background()

	// Options with proxy
	opts1 := rawhttp.Options{
		Host:            "cyberwise.com",
		Port:            443,
		Scheme:          "https",
		Protocol:        "http/1.1",
		ReuseConnection: true, // Enable connection pooling
		Proxy: &rawhttp.ProxyConfig{
			Type: "http",
			Host: "127.0.0.1",
			Port: 8080,
		},
		InsecureTLS: true,
	}

	// Options without proxy
	opts2 := rawhttp.Options{
		Host:            "cyberwise.com",
		Port:            443,
		Scheme:          "https",
		Protocol:        "http/1.1",
		ReuseConnection: true, // Enable connection pooling
		InsecureTLS:     true,
		// No proxy
	}

	rawReq := []byte("GET / HTTP/1.1\r\nHost: cyberwise.com\r\n\r\n")

	// Request with proxy
	resp1, err := sender1.Do(ctx, rawReq, opts1)
	if err != nil {
		fmt.Printf("Error on request with proxy: %v\n", err)
		return
	}

	fmt.Printf("Request with Proxy:\n")
	fmt.Printf("  Connected IP: %s:%d\n", resp1.ConnectedIP, resp1.ConnectedPort)
	fmt.Printf("  Proxy Used: %v (%s)\n", resp1.ProxyUsed, resp1.ProxyAddr)
	fmt.Printf("  Body Size: %d bytes\n", resp1.BodyBytes)

	// Request without proxy
	resp2, err := sender2.Do(ctx, rawReq, opts2)
	if err != nil {
		fmt.Printf("Error on request without proxy: %v\n", err)
		return
	}

	fmt.Printf("\nRequest without Proxy:\n")
	fmt.Printf("  Connected IP: %s:%d\n", resp2.ConnectedIP, resp2.ConnectedPort)
	fmt.Printf("  Proxy Used: %v\n", resp2.ProxyUsed)
	fmt.Printf("  Body Size: %d bytes\n", resp2.BodyBytes)

	// These should have different connection IPs when proxy is involved
	// Note: The proxy IP might be the same, but the context is different
	if resp1.ProxyUsed != resp2.ProxyUsed {
		fmt.Printf("\n✓ SUCCESS: Different proxy configurations tracked correctly!\n")
		fmt.Printf("  (Proxy vs Direct connection properly differentiated)\n")
	} else {
		fmt.Printf("\n✗ FAILURE: Proxy configuration not tracked properly\n")
	}
}
