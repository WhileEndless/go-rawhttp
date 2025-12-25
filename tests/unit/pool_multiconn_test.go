package unit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	rawhttp "github.com/WhileEndless/go-rawhttp/v2"
	"github.com/WhileEndless/go-rawhttp/v2/pkg/transport"
)

// TestMultiConnectionPool_Basic tests that multiple connections are created and reused
func TestMultiConnectionPool_Basic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	// Use custom pool config with higher idle limit
	sender := rawhttp.NewSenderWithPoolConfig(transport.PoolConfig{
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     0,
		MaxIdleTime:         90 * time.Second,
	})

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make concurrent requests
	var wg sync.WaitGroup
	var reuseCount int32
	var successCount int32
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", reqNum, host))

			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				t.Errorf("Request %d failed: %v", reqNum, err)
				return
			}

			atomic.AddInt32(&successCount, 1)
			if resp.ConnectionReused {
				atomic.AddInt32(&reuseCount, 1)
			}

			resp.Body.Close()
			resp.Raw.Close()
		}(i)
	}

	wg.Wait()

	stats := sender.PoolStats()
	reuseRate := float64(reuseCount) / float64(successCount-1) * 100

	t.Logf("Results: %d/%d succeeded, %d reused (%.1f%% reuse rate)",
		successCount, numRequests, reuseCount, reuseRate)
	t.Logf("Pool Stats: Active=%d, Idle=%d, TotalCreated=%d, TotalReused=%d",
		stats.ActiveConns, stats.IdleConns, stats.TotalCreated, stats.TotalReused)

	if successCount < int32(numRequests) {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
	}
}

// TestMultiConnectionPool_MaxIdleLimit verifies excess idle connections are closed
func TestMultiConnectionPool_MaxIdleLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	// Create sender with low idle limit
	maxIdle := 2
	sender := rawhttp.NewSenderWithPoolConfig(transport.PoolConfig{
		MaxIdleConnsPerHost: maxIdle,
		MaxConnsPerHost:     0,
		MaxIdleTime:         90 * time.Second,
	})

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make 5 sequential requests
	for i := 0; i < 5; i++ {
		request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", i, host))
		resp, err := sender.Do(context.Background(), request, opts)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()
		resp.Raw.Close()
	}

	// Check idle count doesn't exceed limit
	stats := sender.PoolStats()
	if stats.IdleConns > maxIdle {
		t.Errorf("Idle connections (%d) exceeded MaxIdleConnsPerHost (%d)", stats.IdleConns, maxIdle)
	}

	t.Logf("Pool Stats: Idle=%d (max=%d), TotalReused=%d", stats.IdleConns, maxIdle, stats.TotalReused)
}

// TestMultiConnectionPool_BackwardCompatible ensures default behavior is unchanged
func TestMultiConnectionPool_BackwardCompatible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	// Use default sender (without custom config)
	sender := rawhttp.NewSender()

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make sequential requests - should all succeed with default pool config
	for i := 0; i < 5; i++ {
		request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", i, host))
		resp, err := sender.Do(context.Background(), request, opts)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}

		if i > 0 && !resp.ConnectionReused {
			t.Errorf("Request %d: expected connection to be reused", i)
		}

		resp.Body.Close()
		resp.Raw.Close()
	}

	stats := sender.PoolStats()
	if stats.TotalReused < 4 {
		t.Errorf("Expected at least 4 reuses, got %d", stats.TotalReused)
	}

	t.Logf("Backward compatibility test: %d reuses", stats.TotalReused)
}

// TestMultiConnectionPool_ConcurrentAccess verifies race-free concurrent access
func TestMultiConnectionPool_ConcurrentAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSenderWithPoolConfig(transport.PoolConfig{
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     0,
		MaxIdleTime:         90 * time.Second,
	})

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Stress test with many concurrent requests
	var wg sync.WaitGroup
	var errors int32
	numRequests := 50

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", reqNum, host))

			resp, err := sender.Do(context.Background(), request, opts)
			if err != nil {
				atomic.AddInt32(&errors, 1)
				return
			}
			resp.Body.Close()
			resp.Raw.Close()
		}(i)
	}

	wg.Wait()

	if errors > 0 {
		t.Errorf("%d requests failed", errors)
	}

	stats := sender.PoolStats()
	t.Logf("Concurrent access test: %d requests, %d errors, TotalReused=%d",
		numRequests, errors, stats.TotalReused)
}

// TestMultiConnectionPool_PoolStats verifies pool statistics accuracy
func TestMultiConnectionPool_PoolStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	hostParts := strings.Split(host, ":")
	port := 80
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSenderWithPoolConfig(transport.PoolConfig{
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     0,
		MaxIdleTime:         90 * time.Second,
	})

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make 10 sequential requests
	for i := 0; i < 10; i++ {
		request := []byte(fmt.Sprintf("GET /?req=%d HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", i, host))
		resp, err := sender.Do(context.Background(), request, opts)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()
		resp.Raw.Close()
	}

	stats := sender.PoolStats()

	// Verify statistics
	if stats.TotalCreated != 1 {
		t.Errorf("Expected 1 connection created, got %d", stats.TotalCreated)
	}
	if stats.TotalReused != 9 {
		t.Errorf("Expected 9 reuses, got %d", stats.TotalReused)
	}
	if stats.IdleConns != 1 {
		t.Errorf("Expected 1 idle connection, got %d", stats.IdleConns)
	}
	if stats.ActiveConns != 0 {
		t.Errorf("Expected 0 active connections, got %d", stats.ActiveConns)
	}

	// Verify host stats exist
	if len(stats.HostStats) == 0 {
		t.Error("Expected host stats to be populated")
	}

	t.Logf("Stats: Created=%d, Reused=%d, Idle=%d, Active=%d, Hosts=%d",
		stats.TotalCreated, stats.TotalReused, stats.IdleConns, stats.ActiveConns, len(stats.HostStats))
}
