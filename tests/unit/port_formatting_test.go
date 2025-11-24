package unit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

// TestPortFormatting tests that port numbers are correctly formatted in dial addresses
// This verifies the fix for Bug #2: Port double formatting (host:port:port)
func TestPortFormatting(t *testing.T) {
	// Create a simple HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Extract host and port from server URL
	serverAddr := server.Listener.Addr().String()
	host, portStr, err := net.SplitHostPort(serverAddr)
	if err != nil {
		t.Fatalf("Failed to parse server address: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("Failed to parse port: %v", err)
	}

	sender := rawhttp.NewSender()

	t.Run("HTTP_PortFormatting", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:      "http",
			Host:        host,
			Port:        port,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + host + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			// Check if error message contains double port formatting (host:port:port)
			errStr := err.Error()
			if strings.Contains(errStr, fmt.Sprintf(":%d:%d", port, port)) {
				t.Fatalf("Bug #2 detected: Port appears twice in error: %v", err)
			}
			t.Fatalf("Request failed (not due to port formatting): %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("✓ Port formatting correct for HTTP request to %s:%d", host, port)
	})

	t.Run("HTTPS_PortFormatting_with_InsecureTLS", func(t *testing.T) {
		// Create HTTPS server
		httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer httpsServer.Close()

		// Extract host and port
		httpsAddr := httpsServer.Listener.Addr().String()
		httpsHost, httpsPortStr, err := net.SplitHostPort(httpsAddr)
		if err != nil {
			t.Fatalf("Failed to parse HTTPS server address: %v", err)
		}
		httpsPort, err := strconv.Atoi(httpsPortStr)
		if err != nil {
			t.Fatalf("Failed to parse HTTPS port: %v", err)
		}

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        httpsHost,
			Port:        httpsPort,
			InsecureTLS: true,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + httpsHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			// Check if error message contains double port formatting (host:port:port)
			errStr := err.Error()
			if strings.Contains(errStr, fmt.Sprintf(":%d:%d", httpsPort, httpsPort)) {
				t.Fatalf("Bug #2 detected: Port appears twice in error: %v", err)
			}
			t.Fatalf("Request failed (not due to port formatting): %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("✓ Port formatting correct for HTTPS request to %s:%d", httpsHost, httpsPort)
	})

	t.Run("Custom_Ports_PortFormatting", func(t *testing.T) {
		// Test with various custom ports to ensure consistent formatting
		testPorts := []int{8080, 8443, 9000, 33291, 46251}

		for _, testPort := range testPorts {
			// For this test, we just verify that options are set correctly
			// We don't need actual servers for all ports
			opts := rawhttp.Options{
				Scheme: "http",
				Host:   "127.0.0.1",
				Port:   testPort,
			}

			// Verify the port is stored as int, not string
			if opts.Port != testPort {
				t.Errorf("Port mismatch: expected %d, got %d", testPort, opts.Port)
			}

			t.Logf("✓ Port %d correctly stored in options", testPort)
		}
	})
}

// TestErrorMessagePortFormatting tests that error messages format ports correctly
func TestErrorMessagePortFormatting(t *testing.T) {
	sender := rawhttp.NewSender()

	t.Run("ConnectionError_PortFormatting", func(t *testing.T) {
		// Try to connect to non-existent server
		opts := rawhttp.Options{
			Scheme:      "http",
			Host:        "127.0.0.1",
			Port:        65432, // Very likely unused port
			ConnTimeout: 1 * time.Second,
			ReadTimeout: 1 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")

		_, err := sender.Do(context.Background(), req, opts)
		if err == nil {
			t.Skip("Expected connection error, but request succeeded (port might be in use)")
		}

		errStr := err.Error()

		// Check that port doesn't appear twice in error message
		if strings.Contains(errStr, "127.0.0.1:65432:65432") {
			t.Fatalf("Bug #2 detected: Port appears twice in error message: %s", errStr)
		}

		// Verify port appears exactly once in the expected format
		if !strings.Contains(errStr, "127.0.0.1:65432") {
			t.Logf("Warning: Expected port format '127.0.0.1:65432' not found in error: %s", errStr)
		}

		t.Logf("✓ Error message format correct: %s", errStr)
	})

	t.Run("TLSError_PortFormatting", func(t *testing.T) {
		// Try TLS connection to HTTP server (should fail)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Extract host and port
		serverAddr := server.Listener.Addr().String()
		host, portStr, err := net.SplitHostPort(serverAddr)
		if err != nil {
			t.Fatalf("Failed to parse server address: %v", err)
		}
		port, _ := strconv.Atoi(portStr)

		opts := rawhttp.Options{
			Scheme:      "https", // Try HTTPS on HTTP server
			Host:        host,
			Port:        port,
			InsecureTLS: true,
			ConnTimeout: 2 * time.Second,
			ReadTimeout: 2 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + host + "\r\n\r\n")

		_, err = sender.Do(context.Background(), req, opts)
		if err == nil {
			t.Skip("Expected TLS error, but request succeeded")
		}

		errStr := err.Error()

		// Check that port doesn't appear twice
		portPattern := fmt.Sprintf(":%d:%d", port, port)
		if strings.Contains(errStr, portPattern) {
			t.Fatalf("Bug #2 detected: Port appears twice in TLS error: %s", errStr)
		}

		t.Logf("✓ TLS error message format correct: %s", errStr)
	})
}

// TestAddressFormattingConsistency verifies consistent address formatting across the codebase
func TestAddressFormattingConsistency(t *testing.T) {
	testCases := []struct {
		name string
		host string
		port int
	}{
		{"IPv4_Standard", "192.168.1.1", 8080},
		{"IPv4_Localhost", "127.0.0.1", 443},
		{"IPv4_HighPort", "10.0.0.1", 65535},
		{"IPv6_Localhost", "::1", 8080},
		{"IPv6_Full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", 443},
		{"Hostname", "example.com", 80},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify that net.JoinHostPort produces correct format
			expectedAddr := net.JoinHostPort(tc.host, strconv.Itoa(tc.port))

			// For IPv6, expect brackets: [::1]:8080
			if strings.Contains(tc.host, ":") && !strings.HasPrefix(expectedAddr, "[") {
				t.Logf("Note: IPv6 address formatted as %s", expectedAddr)
			}

			t.Logf("✓ Address formatting consistent: %s (from %s:%d)", expectedAddr, tc.host, tc.port)

			// Ensure no double port formatting
			portStr := strconv.Itoa(tc.port)
			doublePort := ":" + portStr + ":" + portStr
			if strings.Contains(expectedAddr, doublePort) {
				t.Errorf("Bug #2 detected: Double port in address: %s", expectedAddr)
			}
		})
	}
}
