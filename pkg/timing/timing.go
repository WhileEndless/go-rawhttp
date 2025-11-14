// Package timing provides performance measurement utilities for HTTP requests.
package timing

import (
	"fmt"
	"time"
)

// Metrics captures detailed timing information for a request.
// All fields are properly named to match industry-standard conventions.
type Metrics struct {
	// DNSLookup is the time spent performing DNS resolution
	DNSLookup time.Duration `json:"dns_lookup"`

	// TCPConnect is the time spent establishing TCP connection (handshake)
	TCPConnect time.Duration `json:"tcp_connect"`

	// TLSHandshake is the time spent performing TLS handshake (0 for HTTP)
	TLSHandshake time.Duration `json:"tls_handshake"`

	// TTFB (Time To First Byte) is the time spent waiting for the first response byte
	// This represents server processing time
	TTFB time.Duration `json:"ttfb"`

	// TotalTime is the total end-to-end request time
	TotalTime time.Duration `json:"total_time"`

	// Deprecated: Use DNSLookup instead
	DNS time.Duration `json:"dns,omitempty"`

	// Deprecated: Use TCPConnect instead
	TCP time.Duration `json:"tcp,omitempty"`

	// Deprecated: Use TLSHandshake instead
	TLS time.Duration `json:"tls,omitempty"`

	// Deprecated: Use TotalTime instead
	Total time.Duration `json:"total,omitempty"`
}

// Timer helps measure request timings.
type Timer struct {
	start     time.Time
	dnsStart  time.Time
	dnsEnd    time.Time
	tcpStart  time.Time
	tcpEnd    time.Time
	tlsStart  time.Time
	tlsEnd    time.Time
	ttfbStart time.Time
	ttfbEnd   time.Time
}

// NewTimer creates a new timing measurement session.
func NewTimer() *Timer {
	return &Timer{
		start: time.Now(),
	}
}

// StartDNS marks the beginning of DNS resolution.
func (t *Timer) StartDNS() {
	t.dnsStart = time.Now()
}

// EndDNS marks the end of DNS resolution.
func (t *Timer) EndDNS() {
	t.dnsEnd = time.Now()
}

// StartTCP marks the beginning of TCP connection.
func (t *Timer) StartTCP() {
	t.tcpStart = time.Now()
}

// EndTCP marks the end of TCP connection.
func (t *Timer) EndTCP() {
	t.tcpEnd = time.Now()
}

// StartTLS marks the beginning of TLS handshake.
func (t *Timer) StartTLS() {
	t.tlsStart = time.Now()
}

// EndTLS marks the end of TLS handshake.
func (t *Timer) EndTLS() {
	t.tlsEnd = time.Now()
}

// StartTTFB marks when we start waiting for the first response byte.
func (t *Timer) StartTTFB() {
	t.ttfbStart = time.Now()
}

// EndTTFB marks when we receive the first response byte.
func (t *Timer) EndTTFB() {
	t.ttfbEnd = time.Now()
}

// GetMetrics returns the calculated timing metrics.
func (t *Timer) GetMetrics() Metrics {
	totalTime := time.Since(t.start)

	metrics := Metrics{
		TotalTime: totalTime,
		Total:     totalTime, // Deprecated: for backward compatibility
	}

	if !t.dnsStart.IsZero() && !t.dnsEnd.IsZero() {
		dnsTime := t.dnsEnd.Sub(t.dnsStart)
		metrics.DNSLookup = dnsTime
		metrics.DNS = dnsTime // Deprecated: for backward compatibility
	}

	if !t.tcpStart.IsZero() && !t.tcpEnd.IsZero() {
		tcpTime := t.tcpEnd.Sub(t.tcpStart)
		metrics.TCPConnect = tcpTime
		metrics.TCP = tcpTime // Deprecated: for backward compatibility
	}

	if !t.tlsStart.IsZero() && !t.tlsEnd.IsZero() {
		tlsTime := t.tlsEnd.Sub(t.tlsStart)
		metrics.TLSHandshake = tlsTime
		metrics.TLS = tlsTime // Deprecated: for backward compatibility
	}

	if !t.ttfbStart.IsZero() && !t.ttfbEnd.IsZero() {
		metrics.TTFB = t.ttfbEnd.Sub(t.ttfbStart)
	}

	return metrics
}

// GetConnectionTime returns the total connection establishment time (DNS + TCP + TLS).
func (m Metrics) GetConnectionTime() time.Duration {
	return m.DNSLookup + m.TCPConnect + m.TLSHandshake
}

// GetServerTime returns the server processing time.
func (m Metrics) GetServerTime() time.Duration {
	return m.TTFB
}

// GetNetworkTime returns the total network time (excluding server processing).
func (m Metrics) GetNetworkTime() time.Duration {
	return m.TotalTime - m.TTFB
}

// String provides a human-readable representation of the metrics.
func (m Metrics) String() string {
	return fmt.Sprintf("DNSLookup: %v, TCPConnect: %v, TLSHandshake: %v, TTFB: %v, TotalTime: %v",
		m.DNSLookup, m.TCPConnect, m.TLSHandshake, m.TTFB, m.TotalTime)
}
