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

// TestInsecureTLSWithCustomTLSConfig tests that InsecureTLS flag works with custom TLSConfig
// This is the fix for Bug #1: TLS InsecureSkipVerify being ignored when custom TLSConfig is provided
func TestInsecureTLSWithCustomTLSConfig(t *testing.T) {
	// Create a test HTTPS server with self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Extract host and port from server URL
	host := server.Listener.Addr().String()
	// Parse host:port
	var serverHost string
	var serverPort int
	_, err := fmt.Sscanf(host, "%s:%d", &serverHost, &serverPort)
	if err != nil {
		// Try without scanning
		parts := strings.Split(host, ":")
		if len(parts) != 2 {
			t.Fatalf("Failed to parse server address: %s", host)
		}
		serverHost = parts[0]
		serverPort, _ = strconv.Atoi(parts[1])
	}

	sender := rawhttp.NewSender()

	t.Run("InsecureTLS_with_custom_TLSConfig", func(t *testing.T) {
		// Scenario: User provides custom TLSConfig (e.g., for specific TLS version)
		// AND also sets InsecureTLS=true (e.g., for proxy scenarios with self-signed certs)
		// Expected: InsecureTLS should override InsecureSkipVerify to true

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			InsecureTLS: true, // Should accept any certificate
			TLSConfig: &tls.Config{
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS13,
				InsecureSkipVerify: false, // This should be overridden by InsecureTLS
			},
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")

		// This should succeed because InsecureTLS=true overrides InsecureSkipVerify
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Expected request to succeed with InsecureTLS=true, got error: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Custom_TLSConfig_without_InsecureTLS_should_fail", func(t *testing.T) {
		// Scenario: User provides custom TLSConfig without InsecureTLS
		// Expected: Should fail with certificate verification error (self-signed cert)

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			InsecureTLS: false, // Certificate validation enabled
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
				MaxVersion: tls.VersionTLS13,
			},
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")

		// This should fail with certificate verification error
		_, err := sender.Do(context.Background(), req, opts)
		if err == nil {
			t.Fatal("Expected certificate verification error, but request succeeded")
		}

		// Check that error is TLS-related
		errStr := err.Error()
		if !strings.Contains(errStr, "tls") && !strings.Contains(errStr, "certificate") {
			t.Errorf("Expected TLS/certificate error, got: %v", err)
		}
	})

	t.Run("InsecureTLS_without_custom_TLSConfig", func(t *testing.T) {
		// Scenario: User only sets InsecureTLS=true (backward compatibility test)
		// Expected: Should accept self-signed certificate

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			InsecureTLS: true, // Should accept any certificate
			TLSConfig:   nil,  // Use default TLS config
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Expected request to succeed with InsecureTLS=true, got error: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Custom_TLSConfig_with_InsecureSkipVerify_true", func(t *testing.T) {
		// Scenario: User provides custom TLSConfig with InsecureSkipVerify=true
		// Expected: Should accept self-signed certificate

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			InsecureTLS: false, // Not using InsecureTLS flag
			TLSConfig: &tls.Config{
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS13,
				InsecureSkipVerify: true, // Accept any certificate
			},
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")

		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("Expected request to succeed with InsecureSkipVerify=true, got error: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}

// TestInsecureTLSPriorityOverCustomConfig tests that InsecureTLS has priority
func TestInsecureTLSPriorityOverCustomConfig(t *testing.T) {
	// Create a test HTTPS server with self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Extract host and port
	host := server.Listener.Addr().String()
	parts := strings.Split(host, ":")
	if len(parts) != 2 {
		t.Fatalf("Failed to parse server address: %s", host)
	}
	serverHost := parts[0]
	serverPort, _ := strconv.Atoi(parts[1])

	sender := rawhttp.NewSender()

	t.Run("InsecureTLS_overrides_custom_TLSConfig_InsecureSkipVerify", func(t *testing.T) {
		// This is the CORE test for Bug #1 fix
		// When both InsecureTLS=true and custom TLSConfig with InsecureSkipVerify=false are set,
		// InsecureTLS should take priority and override InsecureSkipVerify to true

		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        serverHost,
			Port:        serverPort,
			InsecureTLS: true, // This should override TLSConfig.InsecureSkipVerify
			TLSConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: false, // This should be overridden
			},
			ConnTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}

		req := []byte("GET / HTTP/1.1\r\nHost: " + serverHost + "\r\n\r\n")

		// Should succeed because InsecureTLS=true overrides InsecureSkipVerify
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("InsecureTLS should override custom TLSConfig.InsecureSkipVerify. Got error: %v", err)
		}
		defer resp.Body.Close()
		defer resp.Raw.Close()

		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ InsecureTLS successfully overrode custom TLSConfig.InsecureSkipVerify")
	})
}
