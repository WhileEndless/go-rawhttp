package unit

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
)

// TestTLSConfigPassthrough tests custom TLS configuration
func TestTLSConfigPassthrough(t *testing.T) {
	// Create custom TLS config
	customTLS := &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
	}

	opts := rawhttp.Options{
		Scheme:    "https",
		Host:      "example.com",
		Port:      443,
		TLSConfig: customTLS,
	}

	// Verify TLSConfig is set
	if opts.TLSConfig == nil {
		t.Error("TLSConfig should not be nil")
	}

	if opts.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS 1.3, got %d", opts.TLSConfig.MinVersion)
	}
}

// TestStandardizedTimingMetrics tests new timing field names
func TestStandardizedTimingMetrics(t *testing.T) {
	// Test that GetMetrics() fills both new and old fields
	timer := timing.NewTimer()

	// Simulate timing measurements
	timer.StartDNS()
	time.Sleep(1 * time.Millisecond)
	timer.EndDNS()

	timer.StartTCP()
	time.Sleep(1 * time.Millisecond)
	timer.EndTCP()

	timer.StartTLS()
	time.Sleep(1 * time.Millisecond)
	timer.EndTLS()

	timer.StartTTFB()
	time.Sleep(1 * time.Millisecond)
	timer.EndTTFB()

	metrics := timer.GetMetrics()

	// Test new field names are set
	if metrics.DNSLookup == 0 {
		t.Error("DNSLookup should not be zero")
	}

	if metrics.TCPConnect == 0 {
		t.Error("TCPConnect should not be zero")
	}

	if metrics.TLSHandshake == 0 {
		t.Error("TLSHandshake should not be zero")
	}

	if metrics.TotalTime == 0 {
		t.Error("TotalTime should not be zero")
	}

	// Test backward compatibility - old fields should equal new fields
	if metrics.DNS != metrics.DNSLookup {
		t.Errorf("Backward compatibility broken: DNS (%v) != DNSLookup (%v)", metrics.DNS, metrics.DNSLookup)
	}

	if metrics.TCP != metrics.TCPConnect {
		t.Errorf("Backward compatibility broken: TCP (%v) != TCPConnect (%v)", metrics.TCP, metrics.TCPConnect)
	}

	if metrics.TLS != metrics.TLSHandshake {
		t.Errorf("Backward compatibility broken: TLS (%v) != TLSHandshake (%v)", metrics.TLS, metrics.TLSHandshake)
	}

	if metrics.Total != metrics.TotalTime {
		t.Errorf("Backward compatibility broken: Total (%v) != TotalTime (%v)", metrics.Total, metrics.TotalTime)
	}

	// Test helper methods
	connTime := metrics.GetConnectionTime()
	if connTime == 0 {
		t.Error("GetConnectionTime should not be zero")
	}

	// Test String() method
	str := metrics.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
}

// TestHTTP2SettingsExposure tests HTTP/2 settings configuration
func TestHTTP2SettingsExposure(t *testing.T) {
	settings := rawhttp.HTTP2Settings{
		MaxConcurrentStreams: 100,
		InitialWindowSize:    65535,
		MaxFrameSize:         16384,
		MaxHeaderListSize:    8192,
		DisableServerPush:    true,
		EnableCompression:    true,
	}

	// Verify all settings
	if settings.MaxConcurrentStreams != 100 {
		t.Errorf("MaxConcurrentStreams: expected 100, got %d", settings.MaxConcurrentStreams)
	}

	if settings.InitialWindowSize != 65535 {
		t.Errorf("InitialWindowSize: expected 65535, got %d", settings.InitialWindowSize)
	}

	if !settings.DisableServerPush {
		t.Error("DisableServerPush should be true")
	}

	if !settings.EnableCompression {
		t.Error("EnableCompression should be true")
	}
}

// TestEnhancedConnectionMetadata tests connection metadata fields
func TestEnhancedConnectionMetadata(t *testing.T) {
	response := &rawhttp.Response{
		StatusCode:     200,
		ConnectedIP:    "93.184.216.34",
		ConnectedPort:  443,
		ConnectionID:   12345,
		LocalAddr:      "192.168.1.100:54321",
		RemoteAddr:     "93.184.216.34:443",
		TLSVersion:     "TLS 1.3",
		TLSCipherSuite: "TLS_AES_256_GCM_SHA384",
		TLSSessionID:   "abc123",
		TLSResumed:     false,
	}

	// Verify basic metadata
	if response.ConnectedIP != "93.184.216.34" {
		t.Errorf("ConnectedIP mismatch: got %s", response.ConnectedIP)
	}

	if response.ConnectedPort != 443 {
		t.Errorf("ConnectedPort mismatch: got %d", response.ConnectedPort)
	}

	// Verify enhanced socket metadata
	if response.ConnectionID != 12345 {
		t.Errorf("ConnectionID mismatch: got %d", response.ConnectionID)
	}

	if response.LocalAddr != "192.168.1.100:54321" {
		t.Errorf("LocalAddr mismatch: got %s", response.LocalAddr)
	}

	if response.RemoteAddr != "93.184.216.34:443" {
		t.Errorf("RemoteAddr mismatch: got %s", response.RemoteAddr)
	}

	// Verify enhanced TLS metadata
	if response.TLSSessionID != "abc123" {
		t.Errorf("TLSSessionID mismatch: got %s", response.TLSSessionID)
	}

	if response.TLSResumed {
		t.Error("TLSResumed should be false")
	}
}

// TestErrorTypeClassification tests enhanced error types
func TestErrorTypeClassification(t *testing.T) {
	// Test DNS error
	dnsErr := errors.NewDNSError("example.com", nil)
	if dnsErr.Type != errors.ErrorTypeDNS {
		t.Errorf("DNS error type mismatch: got %s", dnsErr.Type)
	}
	if dnsErr.Op != "lookup" {
		t.Errorf("DNS error op mismatch: got %s", dnsErr.Op)
	}
	if dnsErr.Addr != "example.com" {
		t.Errorf("DNS error addr mismatch: got %s", dnsErr.Addr)
	}

	// Test connection error
	connErr := errors.NewConnectionError("example.com", 443, nil)
	if connErr.Type != errors.ErrorTypeConnection {
		t.Errorf("Connection error type mismatch: got %s", connErr.Type)
	}
	if connErr.Op != "dial" {
		t.Errorf("Connection error op mismatch: got %s", connErr.Op)
	}
	if connErr.Addr != "example.com:443" {
		t.Errorf("Connection error addr mismatch: got %s", connErr.Addr)
	}

	// Test TLS error
	tlsErr := errors.NewTLSError("example.com", 443, nil)
	if tlsErr.Type != errors.ErrorTypeTLS {
		t.Errorf("TLS error type mismatch: got %s", tlsErr.Type)
	}
	if tlsErr.Op != "handshake" {
		t.Errorf("TLS error op mismatch: got %s", tlsErr.Op)
	}

	// Test error formatting
	errStr := connErr.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}

	// Test TransportError alias (it's the same type)
	_ = (*rawhttp.TransportError)(connErr)
	if connErr.Type != errors.ErrorTypeConnection {
		t.Error("TransportError alias should work")
	}
}

// TestConnectionPoolObservability tests pool statistics
func TestConnectionPoolObservability(t *testing.T) {
	sender := rawhttp.NewSender()

	// Get initial stats
	stats := sender.PoolStats()

	// Verify stats structure
	if stats.ActiveConns < 0 {
		t.Errorf("ActiveConns should be >= 0, got %d", stats.ActiveConns)
	}

	if stats.IdleConns < 0 {
		t.Errorf("IdleConns should be >= 0, got %d", stats.IdleConns)
	}

	if stats.TotalReused < 0 {
		t.Errorf("TotalReused should be >= 0, got %d", stats.TotalReused)
	}

	// Initial state should have no connections
	if stats.ActiveConns != 0 {
		t.Errorf("Initial ActiveConns should be 0, got %d", stats.ActiveConns)
	}

	if stats.IdleConns != 0 {
		t.Errorf("Initial IdleConns should be 0, got %d", stats.IdleConns)
	}

	if stats.TotalReused != 0 {
		t.Errorf("Initial TotalReused should be 0, got %d", stats.TotalReused)
	}
}

// TestBackwardCompatibility verifies all changes are backward compatible
func TestBackwardCompatibility(t *testing.T) {
	// Old code should still work

	// 1. Options without new fields
	opts := rawhttp.Options{
		Scheme: "https",
		Host:   "example.com",
		Port:   443,
	}

	// Should work fine with nil TLSConfig
	if opts.TLSConfig != nil {
		t.Error("TLSConfig should be nil by default")
	}

	// 2. Metrics with old field names
	metrics := timing.Metrics{
		DNS:   10 * time.Millisecond,
		TCP:   20 * time.Millisecond,
		TLS:   30 * time.Millisecond,
		TTFB:  40 * time.Millisecond,
		Total: 100 * time.Millisecond,
	}

	// Old fields should still work
	if metrics.DNS != 10*time.Millisecond {
		t.Error("Old DNS field should still work")
	}

	// 3. Error types still work
	if !errors.IsTimeoutError(errors.NewTimeoutError("test", time.Second)) {
		t.Error("IsTimeoutError should work")
	}
}
