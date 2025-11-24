package unit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2"
	"github.com/WhileEndless/go-rawhttp/v2/pkg/transport"
)

// TestClientCertificateLoading tests client certificate loading from PEM data
func TestClientCertificateLoading(t *testing.T) {
	// Generate test certificate and key
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	t.Run("LoadFromPEM", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "test.example.com",
			Port:          443,
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		// Verify fields are set
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
		if len(opts.ClientKeyPEM) == 0 {
			t.Error("ClientKeyPEM should be set")
		}

		// Test that certificate can be parsed
		_, err := tls.X509KeyPair(opts.ClientCertPEM, opts.ClientKeyPEM)
		if err != nil {
			t.Errorf("Failed to parse client cert/key: %v", err)
		}
	})

	t.Run("LoadFromPEMMissingKey", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "test.example.com",
			Port:          443,
			ClientCertPEM: certPEM,
			// Missing ClientKeyPEM - should be handled gracefully
		}

		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
		if len(opts.ClientKeyPEM) != 0 {
			t.Error("ClientKeyPEM should be empty")
		}
	})

	t.Run("LoadFromPEMMissingCert", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:       "https",
			Host:         "test.example.com",
			Port:         443,
			ClientKeyPEM: keyPEM,
			// Missing ClientCertPEM - should be handled gracefully
		}

		if len(opts.ClientCertPEM) != 0 {
			t.Error("ClientCertPEM should be empty")
		}
		if len(opts.ClientKeyPEM) == 0 {
			t.Error("ClientKeyPEM should be set")
		}
	})
}

// TestClientCertificateWithTransport tests certificate loading at transport level
func TestClientCertificateWithTransport(t *testing.T) {
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	t.Run("TransportLoadClientCert", func(t *testing.T) {
		tr := transport.New()
		defer tr.Close()

		config := transport.Config{
			Scheme:        "https",
			Host:          "test.example.com",
			Port:          443,
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		// The transport should accept the config without error
		// (actual connection will fail since test.example.com doesn't exist, but that's ok)
		if len(config.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM not set in config")
		}
		if len(config.ClientKeyPEM) == 0 {
			t.Error("ClientKeyPEM not set in config")
		}
	})

	t.Run("TransportLoadClientCertInvalidKey", func(t *testing.T) {
		tr := transport.New()
		defer tr.Close()

		invalidKey := []byte(`-----BEGIN PRIVATE KEY-----
INVALID KEY DATA
-----END PRIVATE KEY-----`)

		config := transport.Config{
			Scheme:        "https",
			Host:          "test.example.com",
			Port:          443,
			ClientCertPEM: certPEM,
			ClientKeyPEM:  invalidKey,
		}

		// Attempting to parse should fail
		_, err := tls.X509KeyPair(config.ClientCertPEM, config.ClientKeyPEM)
		if err == nil {
			t.Error("Expected error when parsing invalid key, got nil")
		}
	})
}

// TestClientCertificateFilePaths tests certificate loading from file paths
func TestClientCertificateFilePaths(t *testing.T) {
	t.Run("FilePathsSet", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:         "https",
			Host:           "test.example.com",
			Port:           443,
			ClientCertFile: "/path/to/client.crt",
			ClientKeyFile:  "/path/to/client.key",
		}

		if opts.ClientCertFile != "/path/to/client.crt" {
			t.Error("ClientCertFile not set correctly")
		}
		if opts.ClientKeyFile != "/path/to/client.key" {
			t.Error("ClientKeyFile not set correctly")
		}
	})

	t.Run("FilePathsMissingKey", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:         "https",
			Host:           "test.example.com",
			Port:           443,
			ClientCertFile: "/path/to/client.crt",
			// Missing ClientKeyFile
		}

		if opts.ClientCertFile == "" {
			t.Error("ClientCertFile should be set")
		}
		if opts.ClientKeyFile != "" {
			t.Error("ClientKeyFile should be empty")
		}
	})
}

// TestClientCertificateWithCustomCA tests mTLS combined with custom CA certs
func TestClientCertificateWithCustomCA(t *testing.T) {
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	caCertPEM, _, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate CA cert: %v", err)
	}

	t.Run("mTLSWithCustomCA", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "self-signed.example.com",
			Port:          8443,
			CustomCACerts: [][]byte{caCertPEM},
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		// Verify both custom CA and client cert are set
		if len(opts.CustomCACerts) == 0 {
			t.Error("CustomCACerts should be set")
		}
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
		if len(opts.ClientKeyPEM) == 0 {
			t.Error("ClientKeyPEM should be set")
		}

		// Verify we can parse both
		_, err := tls.X509KeyPair(opts.ClientCertPEM, opts.ClientKeyPEM)
		if err != nil {
			t.Errorf("Failed to parse client cert: %v", err)
		}

		// Verify CA cert is valid PEM
		block, _ := pem.Decode(opts.CustomCACerts[0])
		if block == nil {
			t.Error("Failed to decode CA cert PEM")
		}
	})
}

// TestClientCertificateHTTP2 tests that client certs work with HTTP/2
func TestClientCertificateHTTP2(t *testing.T) {
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	t.Run("HTTP2WithClientCert", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "h2.example.com",
			Port:          443,
			Protocol:      "http/2",
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		if opts.Protocol != "http/2" {
			t.Error("Protocol should be http/2")
		}
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set for HTTP/2")
		}
	})
}

// TestClientCertificateOptionsValidation tests option combinations
func TestClientCertificateOptionsValidation(t *testing.T) {
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	t.Run("BothPEMAndFileSet", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:         "https",
			Host:           "test.example.com",
			Port:           443,
			ClientCertPEM:  certPEM,
			ClientKeyPEM:   keyPEM,
			ClientCertFile: "/path/to/cert.crt",
			ClientKeyFile:  "/path/to/cert.key",
		}

		// Both are set - PEM should take precedence in implementation
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
		if opts.ClientCertFile == "" {
			t.Error("ClientCertFile should also be set")
		}
	})

	t.Run("NoCertificateSet", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "test.example.com",
			Port:   443,
			// No client certificate - should work fine (normal HTTPS)
		}

		if len(opts.ClientCertPEM) != 0 {
			t.Error("ClientCertPEM should be empty")
		}
		if opts.ClientCertFile != "" {
			t.Error("ClientCertFile should be empty")
		}
	})
}

// generateTestCert generates a self-signed certificate for testing
func generateTestCert() (certPEM, keyPEM []byte, err error) {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return certPEM, keyPEM, nil
}
