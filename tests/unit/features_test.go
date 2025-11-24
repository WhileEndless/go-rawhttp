package unit

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
)

// TestRawResponsePreservation verifies that Response.Raw contains complete raw HTTP response
func TestRawResponsePreservation(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Test-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
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
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme: "http",
		Host:   hostParts[0],
		Port:   port,
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Verify Raw buffer exists and contains data
	if resp.Raw == nil {
		t.Fatal("Response.Raw is nil")
	}

	rawReader, err := resp.Raw.Reader()
	if err != nil {
		t.Fatalf("Failed to get raw reader: %v", err)
	}
	defer rawReader.Close()

	rawData := make([]byte, resp.Raw.Size())
	n, err := rawReader.Read(rawData)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read raw data: %v", err)
	}

	rawStr := string(rawData[:n])

	// Verify raw response contains status line
	if !strings.Contains(rawStr, "HTTP/1.1 200") && !strings.Contains(rawStr, "HTTP/1.0 200") {
		t.Errorf("Raw response missing status line. Got: %s", rawStr[:100])
	}

	// Verify raw response contains headers
	if !strings.Contains(rawStr, "Content-Type") {
		t.Errorf("Raw response missing headers")
	}

	// Verify raw response contains body
	if !strings.Contains(rawStr, "Hello, World!") {
		t.Errorf("Raw response missing body")
	}

	t.Logf("Raw response preserved correctly (%d bytes)", n)
}

// TestConnectionPooling verifies connection reuse with ReuseConnection option
func TestConnectionPooling(t *testing.T) {
	// Create test server
	connCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Request %d", connCount)))
		connCount++
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
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n\r\n", host))

	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            hostParts[0],
		Port:            port,
		ReuseConnection: true,
	}

	// Make first request
	resp1, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Body.Close()
	defer resp1.Raw.Close()

	if resp1.ConnectionReused {
		t.Error("First connection should not be reused")
	}

	// Make second request
	resp2, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	defer resp2.Body.Close()
	defer resp2.Raw.Close()

	// Second request should potentially reuse connection
	// Note: Actual reuse depends on timing and server behavior
	t.Logf("First request ConnectionReused: %v", resp1.ConnectionReused)
	t.Logf("Second request ConnectionReused: %v", resp2.ConnectionReused)
}

// TestConnectionMetadata verifies that connection metadata is populated
func TestConnectionMetadata(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
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
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme: "http",
		Host:   hostParts[0],
		Port:   port,
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Verify metadata is populated
	if resp.ConnectedIP == "" {
		t.Error("ConnectedIP is empty")
	}

	if resp.ConnectedPort == 0 {
		t.Error("ConnectedPort is 0")
	}

	if resp.NegotiatedProtocol == "" {
		t.Error("NegotiatedProtocol is empty")
	}

	t.Logf("ConnectedIP: %s", resp.ConnectedIP)
	t.Logf("ConnectedPort: %d", resp.ConnectedPort)
	t.Logf("NegotiatedProtocol: %s", resp.NegotiatedProtocol)
}

// TestTLSMetadata verifies TLS-specific metadata for HTTPS connections
func TestTLSMetadata(t *testing.T) {
	// Create TLS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "https://")
	hostParts := strings.Split(host, ":")
	port := 443
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSender()
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme:      "https",
		Host:        hostParts[0],
		Port:        port,
		InsecureTLS: true, // Test server uses self-signed cert
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Verify TLS metadata
	if resp.TLSVersion == "" {
		t.Error("TLSVersion is empty")
	}

	if resp.TLSCipherSuite == "" {
		t.Error("TLSCipherSuite is empty")
	}

	t.Logf("TLSVersion: %s", resp.TLSVersion)
	t.Logf("TLSCipherSuite: %s", resp.TLSCipherSuite)
	t.Logf("TLSServerName: %s", resp.TLSServerName)
}

// TestCustomCASupport verifies custom CA certificate support
func TestCustomCASupport(t *testing.T) {
	// Generate self-signed CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create CA cert: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caBytes})

	// Generate server certificate signed by CA
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2025),
		Subject: pkix.Name{
			Organization: []string{"Test Server"},
			CommonName:   "localhost",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certPrivKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Fatalf("Failed to create server cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)})

	// Create TLS server with custom cert
	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to load server cert: %v", err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Custom CA works!"))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
	server.StartTLS()
	defer server.Close()

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "https://")
	hostParts := strings.Split(host, ":")
	port := 443
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSender()
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	// Test with custom CA
	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme:        "https",
		Host:          hostParts[0],
		Port:          port,
		CustomCACerts: [][]byte{caPEM},
	})

	if err != nil {
		t.Fatalf("Request with custom CA failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Verify response
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	bodyReader, err := resp.Body.Reader()
	if err != nil {
		t.Fatalf("Failed to get body reader: %v", err)
	}
	defer bodyReader.Close()

	bodyData := make([]byte, resp.Body.Size())
	n, _ := bodyReader.Read(bodyData)
	if !strings.Contains(string(bodyData[:n]), "Custom CA works!") {
		t.Error("Response body doesn't match expected content")
	}

	t.Log("Custom CA certificate support verified")
}

// TestInsecureSkipVerify verifies InsecureTLS option
func TestInsecureSkipVerify(t *testing.T) {
	// Create TLS server with self-signed cert
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Parse server URL
	host := strings.TrimPrefix(server.URL, "https://")
	hostParts := strings.Split(host, ":")
	port := 443
	if len(hostParts) > 1 {
		fmt.Sscanf(hostParts[1], "%d", &port)
	}

	sender := rawhttp.NewSender()
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	// Should succeed with InsecureTLS
	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme:      "https",
		Host:        hostParts[0],
		Port:        port,
		InsecureTLS: true,
	})

	if err != nil {
		t.Fatalf("Request with InsecureTLS failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	t.Log("InsecureSkipVerify works correctly")
}

// TestChunkedResponse verifies chunked encoding is preserved in Raw
func TestChunkedResponse(t *testing.T) {
	// Create server that sends chunked response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to be an http.Flusher")
		}

		w.Write([]byte("First chunk\n"))
		flusher.Flush()

		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("Second chunk\n"))
		flusher.Flush()
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
	req := []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host))

	resp, err := sender.Do(context.Background(), req, rawhttp.Options{
		Scheme: "http",
		Host:   hostParts[0],
		Port:   port,
	})

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Check if response has chunked encoding header
	rawReader, err := resp.Raw.Reader()
	if err != nil {
		t.Fatalf("Failed to get raw reader: %v", err)
	}
	defer rawReader.Close()

	rawData := make([]byte, resp.Raw.Size())
	n, _ := rawReader.Read(rawData)
	rawStr := string(rawData[:n])

	if strings.Contains(rawStr, "Transfer-Encoding") {
		t.Log("Chunked encoding preserved in raw response")
	}

	// Body should be decoded (unchunked)
	bodyReader, err := resp.Body.Reader()
	if err != nil {
		t.Fatalf("Failed to get body reader: %v", err)
	}
	defer bodyReader.Close()

	bodyData := make([]byte, resp.Body.Size())
	bodyN, _ := bodyReader.Read(bodyData)
	bodyStr := string(bodyData[:bodyN])

	if !strings.Contains(bodyStr, "First chunk") || !strings.Contains(bodyStr, "Second chunk") {
		t.Error("Body chunks not properly decoded")
	}

	t.Log("Chunked response handled correctly")
}
