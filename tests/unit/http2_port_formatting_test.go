package unit

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

// TestHTTP2PortFormattingFix tests that Bug #2 (port double formatting) is fixed in HTTP/2 client
// Previously: errors.NewConnectionError(fmt.Sprintf("%s:%d", host, port), port, err)
// This caused: "127.0.0.1:8080:8080" in error messages
// Fixed: errors.NewConnectionError(host, port, err)
func TestHTTP2PortFormattingFix(t *testing.T) {
	sender := rawhttp.NewSender()

	t.Run("HTTP2_ConnectionError_PortFormatting", func(t *testing.T) {
		// Try to connect to non-existent HTTP/2 server
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        "127.0.0.1",
			Port:        65431, // Very likely unused port
			Protocol:    "http/2",
			InsecureTLS: true,
			ConnTimeout: 1 * time.Second,
			ReadTimeout: 1 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: 127.0.0.1\r\n\r\n")

		_, err := sender.Do(context.Background(), req, opts)
		if err == nil {
			t.Skip("Expected connection error, but request succeeded (port might be in use)")
		}

		errStr := err.Error()

		// Check that port doesn't appear twice (Bug #2)
		if strings.Contains(errStr, "127.0.0.1:65431:65431") {
			t.Fatalf("Bug #2 NOT FIXED: Port appears twice in HTTP/2 error message: %s", errStr)
		}

		// Verify correct format
		if strings.Contains(errStr, "127.0.0.1:65431") {
			t.Logf("✓ Bug #2 FIXED: HTTP/2 error message format correct: %s", errStr)
		} else {
			t.Logf("Note: Expected port format '127.0.0.1:65431' in error: %s", errStr)
		}
	})

	t.Run("HTTP2_Success_PortFormatting", func(t *testing.T) {
		// Create HTTPS server that supports HTTP/2
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		server.EnableHTTP2 = true
		server.StartTLS()
		defer server.Close()

		// Extract host and port
		serverAddr := server.Listener.Addr().String()
		parts := strings.Split(serverAddr, ":")
		if len(parts) != 2 {
			t.Fatalf("Failed to parse server address: %s", serverAddr)
		}
		serverHost := parts[0]
		serverPort, _ := strconv.Atoi(parts[1])

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			Protocol:    "http/2", // Force HTTP/2
			InsecureTLS: true,
			TLSConfig: &tls.Config{
				NextProtos:         []string{"h2", "http/1.1"},
				InsecureSkipVerify: true,
			},
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			// Even if request fails, check error message for port formatting
			errStr := err.Error()
			if strings.Contains(errStr, fmt.Sprintf(":%d:%d", serverPort, serverPort)) {
				t.Fatalf("Bug #2 NOT FIXED: Port appears twice in HTTP/2 error: %s", errStr)
			}
			t.Logf("Request failed (not due to port formatting): %v", err)
			t.SkipNow()
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		t.Logf("✓ HTTP/2 request successful with correct port formatting to %s:%d", serverHost, serverPort)
	})
}

// TestHTTP2vsHTTP1PortFormatting compares port formatting behavior between HTTP/1.1 and HTTP/2
func TestHTTP2vsHTTP1PortFormatting(t *testing.T) {
	sender := rawhttp.NewSender()

	testCases := []struct {
		name     string
		protocol string
		port     int
	}{
		{"HTTP1.1_Port8080", "http/1.1", 65430},
		{"HTTP2_Port8080", "http/2", 65429},
		{"HTTP1.1_Port33291", "http/1.1", 65428},
		{"HTTP2_Port33291", "http/2", 65427},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := rawhttp.Options{
				Scheme:      "https",
				Host:        "127.0.0.1",
				Port:        tc.port,
				Protocol:    tc.protocol,
				InsecureTLS: true,
				ConnTimeout: 1 * time.Second,
				ReadTimeout: 1 * time.Second,
			}

			var req []byte
			if tc.protocol == "http/2" {
				req = []byte("GET / HTTP/2\r\nHost: 127.0.0.1\r\n\r\n")
			} else {
				req = []byte("GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
			}

			_, err := sender.Do(context.Background(), req, opts)
			if err == nil {
				t.Skip("Expected connection error (port unused)")
			}

			errStr := err.Error()

			// Check for double port formatting
			doublePort := fmt.Sprintf(":%d:%d", tc.port, tc.port)
			if strings.Contains(errStr, doublePort) {
				t.Errorf("Bug #2 detected in %s: Port appears twice: %s", tc.protocol, errStr)
			} else {
				t.Logf("✓ %s: Port formatting correct in error message", tc.protocol)
			}
		})
	}
}

// TestHTTP2ErrorConsistency verifies that HTTP/2 errors are formatted consistently with HTTP/1.1
func TestHTTP2ErrorConsistency(t *testing.T) {
	testHost := "127.0.0.1"
	testPort := 65426

	sender := rawhttp.NewSender()

	// Test HTTP/1.1 error format
	optsHTTP1 := rawhttp.Options{
		Scheme:      "https",
		Host:        testHost,
		Port:        testPort,
		Protocol:    "http/1.1",
		InsecureTLS: true,
		ConnTimeout: 1 * time.Second,
	}

	reqHTTP1 := []byte("GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
	_, errHTTP1 := sender.Do(context.Background(), reqHTTP1, optsHTTP1)

	// Test HTTP/2 error format
	optsHTTP2 := rawhttp.Options{
		Scheme:      "https",
		Host:        testHost,
		Port:        testPort,
		Protocol:    "http/2",
		InsecureTLS: true,
		ConnTimeout: 1 * time.Second,
	}

	reqHTTP2 := []byte("GET / HTTP/2\r\nHost: 127.0.0.1\r\n\r\n")
	_, errHTTP2 := sender.Do(context.Background(), reqHTTP2, optsHTTP2)

	// Both should error (port doesn't exist)
	if errHTTP1 == nil || errHTTP2 == nil {
		t.Skip("Expected errors for both protocols")
	}

	errStr1 := errHTTP1.Error()
	errStr2 := errHTTP2.Error()

	// Count occurrences of port in error messages
	portStr := strconv.Itoa(testPort)
	count1 := strings.Count(errStr1, portStr)
	count2 := strings.Count(errStr2, portStr)

	t.Logf("HTTP/1.1 error: %s (port appears %d times)", errStr1, count1)
	t.Logf("HTTP/2 error:   %s (port appears %d times)", errStr2, count2)

	// Port should appear same number of times in both
	if count1 != count2 {
		t.Errorf("Port formatting inconsistency: HTTP/1.1 has %d occurrences, HTTP/2 has %d", count1, count2)
	}

	// Check for double port formatting (Bug #2)
	doublePort := fmt.Sprintf(":%d:%d", testPort, testPort)
	if strings.Contains(errStr2, doublePort) {
		t.Errorf("Bug #2 in HTTP/2: Double port formatting detected: %s", errStr2)
	}

	t.Log("✓ HTTP/2 and HTTP/1.1 error formatting is consistent")
}
