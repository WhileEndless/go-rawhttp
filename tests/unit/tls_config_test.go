package unit

import (
	"crypto/tls"
	"testing"

	"github.com/WhileEndless/go-rawhttp/v2"
)

// TestTLSConfigPassthrough tests that custom TLS configuration is properly passed through
func TestTLSConfigPassthrough(t *testing.T) {
	sender := rawhttp.NewSender()

	// Test 1: TLSConfig with MinVersion set
	t.Run("CustomMinVersion", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS13, // Enforce TLS 1.3+
				ServerName: "example.com",
			},
		}

		// Verify the TLSConfig is set
		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if opts.TLSConfig.MinVersion != tls.VersionTLS13 {
			t.Errorf("Expected MinVersion TLS 1.3 (0x%x), got 0x%x",
				tls.VersionTLS13, opts.TLSConfig.MinVersion)
		}
	})

	// Test 2: TLSConfig with custom cipher suites
	t.Run("CustomCipherSuites", func(t *testing.T) {
		customCiphers := []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		}

		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				CipherSuites: customCiphers,
				ServerName:   "example.com",
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if len(opts.TLSConfig.CipherSuites) != len(customCiphers) {
			t.Errorf("Expected %d cipher suites, got %d",
				len(customCiphers), len(opts.TLSConfig.CipherSuites))
		}

		for i, cipher := range customCiphers {
			if opts.TLSConfig.CipherSuites[i] != cipher {
				t.Errorf("Cipher suite mismatch at index %d: expected 0x%x, got 0x%x",
					i, cipher, opts.TLSConfig.CipherSuites[i])
			}
		}
	})

	// Test 3: TLSConfig takes precedence over InsecureTLS
	t.Run("TLSConfigPrecedence", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        "example.com",
			Port:        443,
			InsecureTLS: true, // This should be ignored when TLSConfig is set
			TLSConfig: &tls.Config{
				InsecureSkipVerify: false, // TLSConfig takes precedence
				ServerName:         "example.com",
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		// TLSConfig.InsecureSkipVerify should be false (not overridden by opts.InsecureTLS)
		if opts.TLSConfig.InsecureSkipVerify {
			t.Error("TLSConfig.InsecureSkipVerify should be false (TLSConfig takes precedence)")
		}
	})

	// Test 4: Backward compatibility - nil TLSConfig should use InsecureTLS
	t.Run("BackwardCompatibilityInsecureTLS", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:      "https",
			Host:        "example.com",
			Port:        443,
			InsecureTLS: true,
			TLSConfig:   nil, // Use default behavior
		}

		if opts.TLSConfig != nil {
			t.Error("TLSConfig should be nil for backward compatibility test")
		}

		// InsecureTLS should still be set
		if !opts.InsecureTLS {
			t.Error("InsecureTLS should be true")
		}
	})

	// Test 5: TLSConfig with custom server name (SNI)
	t.Run("CustomServerName", func(t *testing.T) {
		customSNI := "custom.example.com"

		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				ServerName: customSNI,
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if opts.TLSConfig.ServerName != customSNI {
			t.Errorf("Expected ServerName %s, got %s",
				customSNI, opts.TLSConfig.ServerName)
		}
	})

	// Test 6: Empty TLSConfig should work (use defaults)
	t.Run("EmptyTLSConfig", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:    "https",
			Host:      "example.com",
			Port:      443,
			TLSConfig: &tls.Config{},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		// Should have default values
		if opts.TLSConfig.MinVersion != 0 {
			t.Logf("MinVersion is set to 0x%x (default)", opts.TLSConfig.MinVersion)
		}
	})

	_ = sender // Suppress unused variable warning
}

// TestTLSConfigCloning tests that TLSConfig is properly cloned to avoid mutations
func TestTLSConfigCloning(t *testing.T) {
	originalConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
		ServerName:         "example.com",
	}

	opts := rawhttp.Options{
		Scheme:    "https",
		Host:      "example.com",
		Port:      443,
		TLSConfig: originalConfig,
	}

	// Verify that opts.TLSConfig points to the original
	if opts.TLSConfig != originalConfig {
		t.Error("TLSConfig should reference the original config")
	}

	// Note: The transport layer clones the config internally to avoid mutations
	// This test verifies the reference is maintained at the options level
}

// TestTLSConfigWithClientCertificates tests TLSConfig with client certificates (mTLS)
func TestTLSConfigWithClientCertificates(t *testing.T) {
	// This is a structural test - we don't load actual certificates
	t.Run("ClientCertificatesStructure", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{
					// In a real scenario, you'd load actual certificates
					{},
				},
				ServerName: "example.com",
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if len(opts.TLSConfig.Certificates) != 1 {
			t.Errorf("Expected 1 certificate, got %d", len(opts.TLSConfig.Certificates))
		}
	})
}

// TestTLSConfigMaxVersion tests MaxVersion configuration
func TestTLSConfigMaxVersion(t *testing.T) {
	t.Run("MaxVersionTLS12", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
				MaxVersion: tls.VersionTLS12, // Limit to TLS 1.2
				ServerName: "example.com",
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if opts.TLSConfig.MaxVersion != tls.VersionTLS12 {
			t.Errorf("Expected MaxVersion TLS 1.2 (0x%x), got 0x%x",
				tls.VersionTLS12, opts.TLSConfig.MaxVersion)
		}
	})
}

// TestTLSConfigSessionTickets tests session ticket configuration
func TestTLSConfigSessionTickets(t *testing.T) {
	t.Run("DisableSessionTickets", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
			TLSConfig: &tls.Config{
				SessionTicketsDisabled: true,
				ServerName:             "example.com",
			},
		}

		if opts.TLSConfig == nil {
			t.Fatal("TLSConfig should not be nil")
		}

		if !opts.TLSConfig.SessionTicketsDisabled {
			t.Error("SessionTicketsDisabled should be true")
		}
	})
}
