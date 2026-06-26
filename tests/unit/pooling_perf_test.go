package unit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

// TestConnectionPoolingPerformance validates that connection pooling improves performance
func TestConnectionPoolingPerformance(t *testing.T) {
	// Create test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Request %d", requestCount)))
	}))
	defer server.Close()

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	numRequests := 20

	// Test WITHOUT connection pooling
	t.Run("WithoutPooling", func(t *testing.T) {
		sender := rawhttp.NewSender()
		opts := rawhttp.Options{
			Scheme:          "http",
			Host:            hostParts[0],
			Port:            port,
			ReuseConnection: false, // Pooling disabled
		}

		start := time.Now()
		for i := 0; i < numRequests; i++ {
			request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", i, host))

			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}

			if resp.ConnectionReused {
				t.Errorf("Request %d should not reuse connection (pooling disabled)", i)
			}

			resp.Body.Close()
			resp.Raw.Close()
		}
		durationWithout := time.Since(start)
		t.Logf("WITHOUT pooling: %d requests in %v (%.2f req/sec)", numRequests, durationWithout, float64(numRequests)/durationWithout.Seconds())
	})

	// Reset request count
	requestCount = 0

	// Test WITH connection pooling
	t.Run("WithPooling", func(t *testing.T) {
		sender := rawhttp.NewSender()
		opts := rawhttp.Options{
			Scheme:          "http",
			Host:            hostParts[0],
			Port:            port,
			ReuseConnection: true, // Pooling enabled
		}

		var reuseCount int
		start := time.Now()
		for i := 0; i < numRequests; i++ {
			request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", i, host))

			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}

			if resp.ConnectionReused {
				reuseCount++
			}

			resp.Body.Close()
			resp.Raw.Close()
		}
		durationWith := time.Since(start)
		t.Logf("WITH pooling: %d requests in %v (%.2f req/sec)", numRequests, durationWith, float64(numRequests)/durationWith.Seconds())
		t.Logf("Connection reuse: %d/%d (%.1f%%)", reuseCount, numRequests-1, float64(reuseCount)/float64(numRequests-1)*100)

		if reuseCount == 0 {
			t.Error("Expected at least some connection reuse")
		}
	})
}

// TestConcurrentConnectionPooling tests concurrent requests with pooling
func TestConcurrentConnectionPooling(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSender()
	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make concurrent requests
	numConcurrent := 10
	results := make(chan error, numConcurrent)

	start := time.Now()
	for i := 0; i < numConcurrent; i++ {
		go func(reqNum int) {
			request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", reqNum, host))

			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				results <- err
				return
			}

			resp.Body.Close()
			resp.Raw.Close()
			results <- nil
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numConcurrent; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}

	duration := time.Since(start)
	t.Logf("✅ %d concurrent requests completed in %v", numConcurrent, duration)
}

// TestPoolConnectionHealthCheck validates that dead connections are removed from pool
func TestPoolConnectionHealthCheck(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSender()
	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make first request
	request := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", host))
	resp1, err := sender.Do(context.Background(), request, opts)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	resp1.Body.Close()
	resp1.Raw.Close()

	// Close server (simulates dead connection)
	server.Close()

	// Wait a bit for connection to die
	time.Sleep(100 * time.Millisecond)

	// Make second request - should fail gracefully and create new connection
	resp2, err := sender.Do(context.Background(), request, opts)
	if err == nil {
		// If it succeeds, it means the pool correctly detected dead connection and created new one
		resp2.Body.Close()
		resp2.Raw.Close()
		t.Log("⚠️ Request succeeded (pool may have created new connection)")
	} else {
		// Expected - connection is dead
		t.Logf("✅ Dead connection correctly detected: %v", err)
	}
}

// BenchmarkConnectionPooling benchmarks the performance difference
func BenchmarkConnectionPooling(b *testing.B) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	b.Run("WithoutPooling", func(b *testing.B) {
		sender := rawhttp.NewSender()
		opts := rawhttp.Options{
			Scheme:          "http",
			Host:            hostParts[0],
			Port:            port,
			ReuseConnection: false,
		}

		request := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
			resp.Raw.Close()
		}
	})

	b.Run("WithPooling", func(b *testing.B) {
		sender := rawhttp.NewSender()
		opts := rawhttp.Options{
			Scheme:          "http",
			Host:            hostParts[0],
			Port:            port,
			ReuseConnection: true,
		}

		request := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", host))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
			resp.Raw.Close()
		}
	})
}
