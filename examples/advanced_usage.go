//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

func main() {
	fmt.Println("=== Advanced Usage Examples ===")

	sender := rawhttp.NewSender()

	// Example 1: Custom headers and authentication
	fmt.Println("\n1. Custom Headers and Authentication:")
	authRequest := strings.Join([]string{
		"GET /bearer HTTP/1.1",
		"Host: httpbin.org",
		"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...",
		"User-Agent: MyApp/1.0",
		"Accept: application/json",
		"X-Custom-Header: custom-value",
		"Connection: close",
		"",
		"",
	}, "\r\n")

	resp, err := sender.Do(context.Background(), []byte(authRequest), rawhttp.DefaultOptions("https", "httpbin.org", 443))
	if err != nil {
		fmt.Printf("Auth request failed: %v\n", err)
	} else {
		fmt.Printf("✅ Auth request successful: %d\n", resp.StatusCode)
		resp.Body.Close()
		resp.Raw.Close()
	}

	// Example 2: File upload simulation
	fmt.Println("\n2. File Upload Simulation:")
	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
	uploadData := fmt.Sprintf(strings.Join([]string{
		"--%s",
		"Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\"",
		"Content-Type: text/plain",
		"",
		"This is test file content",
		"--%s",
		"Content-Disposition: form-data; name=\"description\"",
		"",
		"Test file upload",
		"--%s--",
		"",
	}, "\r\n"), boundary, boundary, boundary)

	uploadRequest := fmt.Sprintf(strings.Join([]string{
		"POST /post HTTP/1.1",
		"Host: httpbin.org",
		"Content-Type: multipart/form-data; boundary=%s",
		"Content-Length: %d",
		"Connection: close",
		"",
		"%s",
	}, "\r\n"), boundary, len(uploadData), uploadData)

	resp, err = sender.Do(context.Background(), []byte(uploadRequest), rawhttp.DefaultOptions("https", "httpbin.org", 443))
	if err != nil {
		fmt.Printf("Upload failed: %v\n", err)
	} else {
		fmt.Printf("✅ Upload successful: %d\n", resp.StatusCode)
		resp.Body.Close()
		resp.Raw.Close()
	}

	// Example 3: Connection reuse pattern (manual)
	fmt.Println("\n3. Multiple Requests with Timing Analysis:")
	urls := []string{"/ip", "/user-agent", "/headers"}
	for i, path := range urls {
		request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n", path)

		start := time.Now()
		resp, err := sender.Do(context.Background(), []byte(request), rawhttp.DefaultOptions("https", "httpbin.org", 443))
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i+1, err)
			continue
		}

		fmt.Printf("Request %d: %d - Connection: %v, Server: %v, Total: %v\n",
			i+1, resp.StatusCode,
			resp.Timings.GetConnectionTime(),
			resp.Timings.GetServerTime(),
			time.Since(start))

		resp.Body.Close()
		resp.Raw.Close()
	}

	// Example 4: Large response handling
	fmt.Println("\n4. Large Response Handling:")
	largeRequest := "GET /bytes/1048576 HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n" // 1MB

	opts := rawhttp.Options{
		Scheme:       "https",
		Host:         "httpbin.org",
		Port:         443,
		BodyMemLimit: 512 * 1024, // 512KB limit to force disk spilling
		ConnTimeout:  30 * time.Second,
		ReadTimeout:  60 * time.Second,
	}

	resp, err = sender.Do(context.Background(), []byte(largeRequest), opts)
	if err != nil {
		fmt.Printf("Large request failed: %v\n", err)
	} else {
		fmt.Printf("✅ Large response: %d bytes", resp.BodyBytes)
		if resp.Body.IsSpilled() {
			fmt.Printf(" (spilled to disk: %s)", resp.Body.Path())
		} else {
			fmt.Printf(" (kept in memory)")
		}
		fmt.Println()
		resp.Body.Close()
		resp.Raw.Close()
	}

	// Example 5: Custom timeout and retry logic
	fmt.Println("\n5. Timeout and Retry Logic:")
	retryRequest := "GET /delay/2 HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n"

	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(attempt)*time.Second)

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        "httpbin.org",
			Port:        443,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: time.Duration(attempt) * time.Second,
		}

		resp, err := sender.Do(ctx, []byte(retryRequest), opts)
		cancel()

		if err != nil {
			if rawhttp.IsTimeoutError(err) {
				fmt.Printf("Attempt %d: Timeout (%v)\n", attempt, err)
				continue
			}
			fmt.Printf("Attempt %d: Error - %v\n", attempt, err)
			break
		}

		fmt.Printf("✅ Attempt %d: Success - %d\n", attempt, resp.StatusCode)
		resp.Body.Close()
		resp.Raw.Close()
		break
	}

	// Example 6: Header inspection
	fmt.Println("\n6. Response Header Analysis:")
	headerRequest := "GET /response-headers?Server=CustomServer&X-Test=TestValue HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n"

	resp, err = sender.Do(context.Background(), []byte(headerRequest), rawhttp.DefaultOptions("https", "httpbin.org", 443))
	if err != nil {
		fmt.Printf("Header request failed: %v\n", err)
	} else {
		fmt.Printf("Response Headers (%d):\n", resp.StatusCode)
		for name, values := range resp.Headers {
			for _, value := range values {
				fmt.Printf("  %s: %s\n", name, value)
			}
		}

		// Read partial body
		reader, _ := resp.Body.Reader()
		if reader != nil {
			data, _ := io.ReadAll(reader)
			if len(data) < 500 { // Only show if small
				fmt.Printf("Body: %s\n", string(data))
			}
			reader.Close()
		}

		resp.Body.Close()
		resp.Raw.Close()
	}

	fmt.Println("\n=== Advanced Usage Examples Complete ===")
}
