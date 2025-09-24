// Package timing provides performance measurement utilities for HTTP requests.
package timing

import (
	"fmt"
	"time"
)

// Metrics captures detailed timing information for a request.
type Metrics struct {
	// DNS resolution time
	DNS time.Duration `json:"dns"`
	// TCP connection establishment time
	TCP time.Duration `json:"tcp"`
	// TLS handshake time (0 for HTTP)
	TLS time.Duration `json:"tls"`
	// Time to first byte (server processing time)
	TTFB time.Duration `json:"ttfb"`
	// Total request time
	Total time.Duration `json:"total"`
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
	metrics := Metrics{
		Total: time.Since(t.start),
	}

	if !t.dnsStart.IsZero() && !t.dnsEnd.IsZero() {
		metrics.DNS = t.dnsEnd.Sub(t.dnsStart)
	}

	if !t.tcpStart.IsZero() && !t.tcpEnd.IsZero() {
		metrics.TCP = t.tcpEnd.Sub(t.tcpStart)
	}

	if !t.tlsStart.IsZero() && !t.tlsEnd.IsZero() {
		metrics.TLS = t.tlsEnd.Sub(t.tlsStart)
	}

	if !t.ttfbStart.IsZero() && !t.ttfbEnd.IsZero() {
		metrics.TTFB = t.ttfbEnd.Sub(t.ttfbStart)
	}

	return metrics
}

// GetConnectionTime returns the total connection establishment time (DNS + TCP + TLS).
func (m Metrics) GetConnectionTime() time.Duration {
	return m.DNS + m.TCP + m.TLS
}

// GetServerTime returns the server processing time.
func (m Metrics) GetServerTime() time.Duration {
	return m.TTFB
}

// GetNetworkTime returns the total network time (excluding server processing).
func (m Metrics) GetNetworkTime() time.Duration {
	return m.Total - m.TTFB
}

// String provides a human-readable representation of the metrics.
func (m Metrics) String() string {
	return fmt.Sprintf("DNS: %v, TCP: %v, TLS: %v, TTFB: %v, Total: %v",
		m.DNS, m.TCP, m.TLS, m.TTFB, m.Total)
}
