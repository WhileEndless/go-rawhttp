// Package tlsconfig provides helpers and constants for SSL/TLS configuration.
package tlsconfig

import "crypto/tls"

// SSL/TLS Protocol Versions
// These constants provide easy access to SSL/TLS version identifiers
const (
	// SSL 3.0 (DEPRECATED - insecure, use only for legacy compatibility)
	// WARNING: SSL 3.0 has known security vulnerabilities (POODLE attack)
	// Only use when absolutely necessary for legacy system compatibility
	VersionSSL30 uint16 = tls.VersionSSL30 // 0x0300

	// TLS 1.0 (DEPRECATED - insecure, use only for legacy compatibility)
	// Most modern systems have disabled TLS 1.0
	VersionTLS10 uint16 = tls.VersionTLS10 // 0x0301

	// TLS 1.1 (DEPRECATED - weak, use only for legacy compatibility)
	// Most modern systems have disabled TLS 1.1
	VersionTLS11 uint16 = tls.VersionTLS11 // 0x0302

	// TLS 1.2 (RECOMMENDED - widely supported and secure)
	// This is the minimum recommended version for production use
	VersionTLS12 uint16 = tls.VersionTLS12 // 0x0303

	// TLS 1.3 (PREFERRED - most secure, modern standard)
	// Use this when both client and server support it
	VersionTLS13 uint16 = tls.VersionTLS13 // 0x0304
)

// Recommended SSL/TLS Version Profiles
// These provide pre-configured version ranges for common use cases
type VersionProfile struct {
	Min         uint16
	Max         uint16
	Description string
}

var (
	// Modern - TLS 1.3 only (most secure, may not work with all servers)
	ProfileModern = VersionProfile{
		Min:         VersionTLS13,
		Max:         VersionTLS13,
		Description: "TLS 1.3 only - maximum security, modern servers only",
	}

	// Secure - TLS 1.2 and 1.3 (recommended for production)
	ProfileSecure = VersionProfile{
		Min:         VersionTLS12,
		Max:         VersionTLS13,
		Description: "TLS 1.2+ - secure and widely compatible",
	}

	// Compatible - TLS 1.0 through 1.3 (maximum compatibility, less secure)
	ProfileCompatible = VersionProfile{
		Min:         VersionTLS10,
		Max:         VersionTLS13,
		Description: "TLS 1.0+ - maximum compatibility, includes deprecated versions",
	}

	// Legacy - SSL 3.0 through TLS 1.3 (includes deprecated SSL, use with caution)
	ProfileLegacy = VersionProfile{
		Min:         VersionSSL30,
		Max:         VersionTLS13,
		Description: "SSL 3.0+ - legacy compatibility, includes insecure versions",
	}
)

// GetVersionName returns human-readable name for SSL/TLS version
func GetVersionName(version uint16) string {
	switch version {
	case VersionSSL30:
		return "SSL 3.0"
	case VersionTLS10:
		return "TLS 1.0"
	case VersionTLS11:
		return "TLS 1.1"
	case VersionTLS12:
		return "TLS 1.2"
	case VersionTLS13:
		return "TLS 1.3"
	default:
		return "Unknown"
	}
}

// IsVersionDeprecated returns true if the version is deprecated/insecure
func IsVersionDeprecated(version uint16) bool {
	return version < VersionTLS12
}

// Recommended Cipher Suites
// These are ordered by security strength (strongest first)
var (
	// TLS 1.3 Cipher Suites (most secure)
	CipherSuitesTLS13 = []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}

	// TLS 1.2 Secure Cipher Suites (ECDHE with AEAD)
	CipherSuitesTLS12Secure = []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	}

	// TLS 1.2 Compatible Cipher Suites (includes CBC mode)
	CipherSuitesTLS12Compatible = []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	}

	// Legacy Cipher Suites (for SSL 3.0 / TLS 1.0 compatibility)
	// WARNING: Some of these are insecure, use only for legacy compatibility
	CipherSuitesLegacy = []uint16{
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
)

// GetCipherSuiteName returns human-readable name for cipher suite
func GetCipherSuiteName(suite uint16) string {
	switch suite {
	// TLS 1.3
	case tls.TLS_AES_128_GCM_SHA256:
		return "TLS_AES_128_GCM_SHA256"
	case tls.TLS_AES_256_GCM_SHA384:
		return "TLS_AES_256_GCM_SHA384"
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		return "TLS_CHACHA20_POLY1305_SHA256"

	// TLS 1.2 ECDHE
	case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256:
		return "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256:
		return "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256"

	// TLS 1.2 CBC
	case tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:
		return "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:
		return "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:
		return "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA"

	// Legacy RSA
	case tls.TLS_RSA_WITH_AES_128_GCM_SHA256:
		return "TLS_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_RSA_WITH_AES_256_GCM_SHA384:
		return "TLS_RSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_RSA_WITH_AES_128_CBC_SHA256:
		return "TLS_RSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_RSA_WITH_AES_128_CBC_SHA:
		return "TLS_RSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_RSA_WITH_AES_256_CBC_SHA:
		return "TLS_RSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:
		return "TLS_RSA_WITH_3DES_EDE_CBC_SHA"

	default:
		return "Unknown"
	}
}

// ApplyVersionProfile applies a pre-configured version profile to tls.Config
func ApplyVersionProfile(config *tls.Config, profile VersionProfile) {
	config.MinVersion = profile.Min
	config.MaxVersion = profile.Max
}

// ApplyCipherSuites applies recommended cipher suites based on minimum TLS version
func ApplyCipherSuites(config *tls.Config, minVersion uint16) {
	switch {
	case minVersion >= VersionTLS13:
		// TLS 1.3 uses its own cipher suites automatically
		config.CipherSuites = nil
	case minVersion >= VersionTLS12:
		config.CipherSuites = CipherSuitesTLS12Secure
	case minVersion >= VersionTLS10:
		config.CipherSuites = CipherSuitesTLS12Compatible
	default:
		// SSL 3.0 or unknown - use legacy suites
		config.CipherSuites = CipherSuitesLegacy
	}
}
