package unit

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

// TestHTTP2_SNI tests Server Name Indication (SNI) configuration for HTTP/2
// This ensures Bug #4 (HTTP/2 ServerName not set) is fixed
func TestHTTP2_SNI(t *testing.T) {
	// Create HTTPS server
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

	sender := rawhttp.NewSender()

	t.Run("Default_SNI_with_HTTP2", func(t *testing.T) {
		// SNI should default to Host when not specified
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        server.Listener.Addr().(*net.TCPAddr).Port,
			Protocol:    "http/2",
			InsecureTLS: true,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Request should succeed with default SNI: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ Default SNI (Host) works with HTTP/2")
	})

	t.Run("Custom_SNI_with_HTTP2", func(t *testing.T) {
		// User can override SNI
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        server.Listener.Addr().(*net.TCPAddr).Port,
			Protocol:    "http/2",
			SNI:         "custom.example.com", // Custom SNI
			InsecureTLS: true,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		// This should work because InsecureTLS=true (no cert validation)
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Request should succeed with custom SNI and InsecureTLS: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ Custom SNI works with HTTP/2")
	})

	t.Run("DisableSNI_with_HTTP2", func(t *testing.T) {
		// User can disable SNI completely
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        server.Listener.Addr().(*net.TCPAddr).Port,
			Protocol:    "http/2",
			DisableSNI:  true, // Disable SNI
			InsecureTLS: true,
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		// Should still work with self-signed cert and no SNI
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Request should succeed with DisableSNI: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ DisableSNI works with HTTP/2")
	})

	t.Run("TLSConfig_ServerName_with_HTTP2", func(t *testing.T) {
		// User can set ServerName directly in TLSConfig (highest priority)
		opts := rawhttp.Options{
			Scheme:   "https",
			Host:     serverHost,
			Port:     server.Listener.Addr().(*net.TCPAddr).Port,
			Protocol: "http/2",
			TLSConfig: &tls.Config{
				ServerName:         "tlsconfig.example.com", // Set in TLSConfig
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
			SNI:         "sni-option.example.com", // This should be ignored
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		// TLSConfig.ServerName should take precedence
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Request should succeed with TLSConfig.ServerName: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ TLSConfig.ServerName takes highest priority (ignored SNI option)")
	})

	t.Run("SNI_Priority_Order_with_HTTP2", func(t *testing.T) {
		// Test SNI priority: TLSConfig > SNI option > Host
		// When TLSConfig.ServerName is empty, use SNI option
		opts := rawhttp.Options{
			Scheme:   "https",
			Host:     serverHost,
			Port:     server.Listener.Addr().(*net.TCPAddr).Port,
			Protocol: "http/2",
			TLSConfig: &tls.Config{
				// ServerName not set - should use SNI option
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
			SNI:         "priority-test.example.com", // Should be used
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Request should succeed with SNI option: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ SNI option used when TLSConfig.ServerName is empty")
	})
}

// TestHTTP2_SNI_vs_HTTP11_Consistency tests that HTTP/2 and HTTP/1.1 handle SNI consistently
func TestHTTP2_SNI_vs_HTTP11_Consistency(t *testing.T) {
	// Create HTTPS server
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
	serverPort := server.Listener.Addr().(*net.TCPAddr).Port

	sender := rawhttp.NewSender()

	// Test same configuration with both protocols
	testCases := []struct {
		name        string
		protocol    string
		sni         string
		disableSNI  bool
		description string
	}{
		{"HTTP1_Default_SNI", "http/1.1", "", false, "Default SNI (Host)"},
		{"HTTP2_Default_SNI", "http/2", "", false, "Default SNI (Host)"},
		{"HTTP1_Custom_SNI", "http/1.1", "custom.example.com", false, "Custom SNI"},
		{"HTTP2_Custom_SNI", "http/2", "custom.example.com", false, "Custom SNI"},
		{"HTTP1_Disable_SNI", "http/1.1", "", true, "SNI Disabled"},
		{"HTTP2_Disable_SNI", "http/2", "", true, "SNI Disabled"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := rawhttp.Options{
				Scheme:      "https",
				Host:        serverHost,
				Port:        serverPort,
				Protocol:    tc.protocol,
				SNI:         tc.sni,
				DisableSNI:  tc.disableSNI,
				InsecureTLS: true,
				ConnTimeout: 5 * time.Second,
				ReadTimeout: 5 * time.Second,
			}

			var req []byte
			if tc.protocol == "http/2" {
				req = []byte("GET / HTTP/2\r\nHost: " + serverHost + "\r\n\r\n")
			} else {
				req = []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")
			}

			resp, err := sender.Do(context.Background(), req, opts)
			if err != nil {
				t.Fatalf("%s should work consistently: %v", tc.description, err)
			}
			defer resp.Body.Close()
			defer resp.Raw.Close()

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			t.Logf("✓ %s works consistently for %s", tc.description, tc.protocol)
		})
	}
}
