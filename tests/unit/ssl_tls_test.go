package unit

import (
	"crypto/tls"
	"testing"

	"github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/pkg/tlsconfig"
)

// TestTLSVersionControl tests SSL/TLS version configuration
func TestTLSVersionControl(t *testing.T) {
	t.Run("MinTLSVersion_TLS12", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "example.com",
			Port:          443,
			MinTLSVersion: tls.VersionTLS12,
		}

		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Errorf("MinTLSVersion not set correctly, got: %d", opts.MinTLSVersion)
		}
	})

	t.Run("MaxTLSVersion_TLS13", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "example.com",
			Port:          443,
			MaxTLSVersion: tls.VersionTLS13,
		}

		if opts.MaxTLSVersion != tls.VersionTLS13 {
			t.Errorf("MaxTLSVersion not set correctly, got: %d", opts.MaxTLSVersion)
		}
	})

	t.Run("TLSVersionRange_TLS12_to_TLS13", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "example.com",
			Port:          443,
			MinTLSVersion: tls.VersionTLS12,
			MaxTLSVersion: tls.VersionTLS13,
		}

		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Error("MinTLSVersion should be TLS 1.2")
		}
		if opts.MaxTLSVersion != tls.VersionTLS13 {
			t.Error("MaxTLSVersion should be TLS 1.3")
		}
	})

	t.Run("LegacySSL30Support", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "legacy-server.example.com",
			Port:          443,
			MinTLSVersion: tls.VersionSSL30,
			InsecureTLS:   true, // Required for SSL 3.0
		}

		if opts.MinTLSVersion != tls.VersionSSL30 {
			t.Error("MinTLSVersion should be SSL 3.0")
		}
		if !opts.InsecureTLS {
			t.Error("InsecureTLS should be true for SSL 3.0")
		}
	})
}

// TestCipherSuiteConfiguration tests cipher suite control
func TestCipherSuiteConfiguration(t *testing.T) {
	t.Run("TLS12SecureCipherSuites", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:       "https",
			Host:         "example.com",
			Port:         443,
			CipherSuites: tlsconfig.CipherSuitesTLS12Secure,
		}

		if len(opts.CipherSuites) == 0 {
			t.Error("CipherSuites should be set")
		}
		if len(opts.CipherSuites) != len(tlsconfig.CipherSuitesTLS12Secure) {
			t.Errorf("Expected %d cipher suites, got %d", len(tlsconfig.CipherSuitesTLS12Secure), len(opts.CipherSuites))
		}
	})

	t.Run("CustomCipherSuites", func(t *testing.T) {
		customSuites := []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		}

		opts := rawhttp.Options{
			Scheme:       "https",
			Host:         "example.com",
			Port:         443,
			CipherSuites: customSuites,
		}

		if len(opts.CipherSuites) != 2 {
			t.Errorf("Expected 2 cipher suites, got %d", len(opts.CipherSuites))
		}
	})

	t.Run("LegacyCipherSuites", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:       "https",
			Host:         "legacy-server.example.com",
			Port:         443,
			CipherSuites: tlsconfig.CipherSuitesLegacy,
		}

		if len(opts.CipherSuites) == 0 {
			t.Error("Legacy cipher suites should be set")
		}
	})
}

// TestTLSRenegotiationSupport tests TLS renegotiation configuration
func TestTLSRenegotiationSupport(t *testing.T) {
	t.Run("RenegotiateNever_Default", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme: "https",
			Host:   "example.com",
			Port:   443,
		}

		// Default should be 0 (which maps to RenegotiateNever)
		if opts.TLSRenegotiation != 0 {
			t.Errorf("Default TLSRenegotiation should be 0, got: %d", opts.TLSRenegotiation)
		}
	})

	t.Run("RenegotiateOnceAsClient", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:           "https",
			Host:             "example.com",
			Port:             443,
			TLSRenegotiation: tls.RenegotiateOnceAsClient,
		}

		if opts.TLSRenegotiation != tls.RenegotiateOnceAsClient {
			t.Error("TLSRenegotiation should be RenegotiateOnceAsClient")
		}
	})

	t.Run("RenegotiateFreelyAsClient", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:           "https",
			Host:             "example.com",
			Port:             443,
			TLSRenegotiation: tls.RenegotiateFreelyAsClient,
		}

		if opts.TLSRenegotiation != tls.RenegotiateFreelyAsClient {
			t.Error("TLSRenegotiation should be RenegotiateFreelyAsClient")
		}
	})
}

// TestTLSConfigHelpers tests tlsconfig package helper functions
func TestTLSConfigHelpers(t *testing.T) {
	t.Run("GetVersionName", func(t *testing.T) {
		tests := []struct {
			version  uint16
			expected string
		}{
			{tlsconfig.VersionSSL30, "SSL 3.0"},
			{tlsconfig.VersionTLS10, "TLS 1.0"},
			{tlsconfig.VersionTLS11, "TLS 1.1"},
			{tlsconfig.VersionTLS12, "TLS 1.2"},
			{tlsconfig.VersionTLS13, "TLS 1.3"},
			{0x9999, "Unknown"},
		}

		for _, tt := range tests {
			name := tlsconfig.GetVersionName(tt.version)
			if name != tt.expected {
				t.Errorf("GetVersionName(%d) = %s, want %s", tt.version, name, tt.expected)
			}
		}
	})

	t.Run("IsVersionDeprecated", func(t *testing.T) {
		// SSL 3.0, TLS 1.0, TLS 1.1 are deprecated
		if !tlsconfig.IsVersionDeprecated(tlsconfig.VersionSSL30) {
			t.Error("SSL 3.0 should be deprecated")
		}
		if !tlsconfig.IsVersionDeprecated(tlsconfig.VersionTLS10) {
			t.Error("TLS 1.0 should be deprecated")
		}
		if !tlsconfig.IsVersionDeprecated(tlsconfig.VersionTLS11) {
			t.Error("TLS 1.1 should be deprecated")
		}

		// TLS 1.2 and 1.3 are not deprecated
		if tlsconfig.IsVersionDeprecated(tlsconfig.VersionTLS12) {
			t.Error("TLS 1.2 should not be deprecated")
		}
		if tlsconfig.IsVersionDeprecated(tlsconfig.VersionTLS13) {
			t.Error("TLS 1.3 should not be deprecated")
		}
	})

	t.Run("GetCipherSuiteName", func(t *testing.T) {
		name := tlsconfig.GetCipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
		if name != "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" {
			t.Errorf("GetCipherSuiteName returned: %s", name)
		}

		unknownName := tlsconfig.GetCipherSuiteName(0x9999)
		if unknownName != "Unknown" {
			t.Errorf("GetCipherSuiteName for unknown suite returned: %s", unknownName)
		}
	})
}

// TestVersionProfiles tests pre-configured version profiles
func TestVersionProfiles(t *testing.T) {
	t.Run("ProfileModern", func(t *testing.T) {
		profile := tlsconfig.ProfileModern
		if profile.Min != tlsconfig.VersionTLS13 {
			t.Errorf("ProfileModern min should be TLS 1.3, got: %s", tlsconfig.GetVersionName(profile.Min))
		}
		if profile.Max != tlsconfig.VersionTLS13 {
			t.Errorf("ProfileModern max should be TLS 1.3, got: %s", tlsconfig.GetVersionName(profile.Max))
		}
	})

	t.Run("ProfileSecure", func(t *testing.T) {
		profile := tlsconfig.ProfileSecure
		if profile.Min != tlsconfig.VersionTLS12 {
			t.Errorf("ProfileSecure min should be TLS 1.2, got: %s", tlsconfig.GetVersionName(profile.Min))
		}
		if profile.Max != tlsconfig.VersionTLS13 {
			t.Errorf("ProfileSecure max should be TLS 1.3, got: %s", tlsconfig.GetVersionName(profile.Max))
		}
	})

	t.Run("ProfileCompatible", func(t *testing.T) {
		profile := tlsconfig.ProfileCompatible
		if profile.Min != tlsconfig.VersionTLS10 {
			t.Errorf("ProfileCompatible min should be TLS 1.0, got: %s", tlsconfig.GetVersionName(profile.Min))
		}
		if profile.Max != tlsconfig.VersionTLS13 {
			t.Errorf("ProfileCompatible max should be TLS 1.3, got: %s", tlsconfig.GetVersionName(profile.Max))
		}
	})

	t.Run("ProfileLegacy", func(t *testing.T) {
		profile := tlsconfig.ProfileLegacy
		if profile.Min != tlsconfig.VersionSSL30 {
			t.Errorf("ProfileLegacy min should be SSL 3.0, got: %s", tlsconfig.GetVersionName(profile.Min))
		}
		if profile.Max != tlsconfig.VersionTLS13 {
			t.Errorf("ProfileLegacy max should be TLS 1.3, got: %s", tlsconfig.GetVersionName(profile.Max))
		}
	})

	t.Run("ApplyVersionProfile", func(t *testing.T) {
		tlsConf := &tls.Config{}
		tlsconfig.ApplyVersionProfile(tlsConf, tlsconfig.ProfileSecure)

		if tlsConf.MinVersion != tlsconfig.VersionTLS12 {
			t.Error("MinVersion should be TLS 1.2 after applying ProfileSecure")
		}
		if tlsConf.MaxVersion != tlsconfig.VersionTLS13 {
			t.Error("MaxVersion should be TLS 1.3 after applying ProfileSecure")
		}
	})
}

// TestCombinedTLSConfiguration tests combination of TLS options
func TestCombinedTLSConfiguration(t *testing.T) {
	t.Run("TLSVersion_WithCipherSuites", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "example.com",
			Port:          443,
			MinTLSVersion: tls.VersionTLS12,
			MaxTLSVersion: tls.VersionTLS13,
			CipherSuites:  tlsconfig.CipherSuitesTLS12Secure,
		}

		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Error("MinTLSVersion should be TLS 1.2")
		}
		if len(opts.CipherSuites) == 0 {
			t.Error("CipherSuites should be set")
		}
	})

	t.Run("TLSVersion_WithRenegotiation", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:           "https",
			Host:             "example.com",
			Port:             443,
			MinTLSVersion:    tls.VersionTLS12,
			TLSRenegotiation: tls.RenegotiateOnceAsClient,
		}

		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Error("MinTLSVersion should be TLS 1.2")
		}
		if opts.TLSRenegotiation != tls.RenegotiateOnceAsClient {
			t.Error("TLSRenegotiation should be RenegotiateOnceAsClient")
		}
	})

	t.Run("CompleteTLSConfiguration", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:           "https",
			Host:             "secure-server.example.com",
			Port:             443,
			MinTLSVersion:    tls.VersionTLS12,
			MaxTLSVersion:    tls.VersionTLS13,
			CipherSuites:     tlsconfig.CipherSuitesTLS12Secure,
			TLSRenegotiation: tls.RenegotiateNever,
			InsecureTLS:      false,
		}

		// Verify all fields are set correctly
		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Error("MinTLSVersion should be TLS 1.2")
		}
		if opts.MaxTLSVersion != tls.VersionTLS13 {
			t.Error("MaxTLSVersion should be TLS 1.3")
		}
		if len(opts.CipherSuites) == 0 {
			t.Error("CipherSuites should be set")
		}
		if opts.TLSRenegotiation != tls.RenegotiateNever {
			t.Error("TLSRenegotiation should be RenegotiateNever")
		}
		if opts.InsecureTLS {
			t.Error("InsecureTLS should be false")
		}
	})
}

// TestTLSWithMTLS tests SSL/TLS version control combined with mTLS
func TestTLSWithMTLS(t *testing.T) {
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	t.Run("TLS12_WithClientCert", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "mtls-server.example.com",
			Port:          443,
			MinTLSVersion: tls.VersionTLS12,
			MaxTLSVersion: tls.VersionTLS12, // Force TLS 1.2
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		if opts.MinTLSVersion != tls.VersionTLS12 {
			t.Error("MinTLSVersion should be TLS 1.2")
		}
		if opts.MaxTLSVersion != tls.VersionTLS12 {
			t.Error("MaxTLSVersion should be TLS 1.2")
		}
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
	})

	t.Run("TLS13_WithClientCert", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:        "https",
			Host:          "modern-mtls.example.com",
			Port:          443,
			MinTLSVersion: tls.VersionTLS13,
			MaxTLSVersion: tls.VersionTLS13, // Force TLS 1.3
			ClientCertPEM: certPEM,
			ClientKeyPEM:  keyPEM,
		}

		if opts.MinTLSVersion != tls.VersionTLS13 {
			t.Error("MinTLSVersion should be TLS 1.3")
		}
		if len(opts.ClientCertPEM) == 0 {
			t.Error("ClientCertPEM should be set")
		}
	})
}

// TestApplyCipherSuites tests automatic cipher suite application
func TestApplyCipherSuites(t *testing.T) {
	t.Run("TLS13_NoCipherSuitesNeeded", func(t *testing.T) {
		tlsConf := &tls.Config{}
		tlsconfig.ApplyCipherSuites(tlsConf, tlsconfig.VersionTLS13)

		// TLS 1.3 doesn't use CipherSuites field
		if tlsConf.CipherSuites != nil {
			t.Error("TLS 1.3 should not have CipherSuites set")
		}
	})

	t.Run("TLS12_SecureCipherSuites", func(t *testing.T) {
		tlsConf := &tls.Config{}
		tlsconfig.ApplyCipherSuites(tlsConf, tlsconfig.VersionTLS12)

		if len(tlsConf.CipherSuites) == 0 {
			t.Error("TLS 1.2 should have cipher suites set")
		}
		if len(tlsConf.CipherSuites) != len(tlsconfig.CipherSuitesTLS12Secure) {
			t.Error("Should use secure TLS 1.2 cipher suites")
		}
	})

	t.Run("TLS10_CompatibleCipherSuites", func(t *testing.T) {
		tlsConf := &tls.Config{}
		tlsconfig.ApplyCipherSuites(tlsConf, tlsconfig.VersionTLS10)

		if len(tlsConf.CipherSuites) == 0 {
			t.Error("TLS 1.0 should have cipher suites set")
		}
		if len(tlsConf.CipherSuites) != len(tlsconfig.CipherSuitesTLS12Compatible) {
			t.Error("Should use compatible TLS 1.2 cipher suites for TLS 1.0")
		}
	})

	t.Run("SSL30_LegacyCipherSuites", func(t *testing.T) {
		tlsConf := &tls.Config{}
		tlsconfig.ApplyCipherSuites(tlsConf, tlsconfig.VersionSSL30)

		if len(tlsConf.CipherSuites) == 0 {
			t.Error("SSL 3.0 should have cipher suites set")
		}
		if len(tlsConf.CipherSuites) != len(tlsconfig.CipherSuitesLegacy) {
			t.Error("Should use legacy cipher suites for SSL 3.0")
		}
	})
}
