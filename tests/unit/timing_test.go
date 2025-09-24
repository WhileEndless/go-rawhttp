package unit

import (
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/timing"
)

func TestTimer(t *testing.T) {
	timer := timing.NewTimer()

	// Simulate DNS resolution
	timer.StartDNS()
	time.Sleep(10 * time.Millisecond)
	timer.EndDNS()

	// Simulate TCP connection
	timer.StartTCP()
	time.Sleep(20 * time.Millisecond)
	timer.EndTCP()

	// Simulate TLS handshake
	timer.StartTLS()
	time.Sleep(30 * time.Millisecond)
	timer.EndTLS()

	// Simulate TTFB
	timer.StartTTFB()
	time.Sleep(40 * time.Millisecond)
	timer.EndTTFB()

	metrics := timer.GetMetrics()

	// Check that timings are reasonable (allowing for some variance)
	if metrics.DNS < 5*time.Millisecond || metrics.DNS > 20*time.Millisecond {
		t.Errorf("unexpected DNS timing: %v", metrics.DNS)
	}

	if metrics.TCP < 15*time.Millisecond || metrics.TCP > 30*time.Millisecond {
		t.Errorf("unexpected TCP timing: %v", metrics.TCP)
	}

	if metrics.TLS < 25*time.Millisecond || metrics.TLS > 40*time.Millisecond {
		t.Errorf("unexpected TLS timing: %v", metrics.TLS)
	}

	if metrics.TTFB < 35*time.Millisecond || metrics.TTFB > 50*time.Millisecond {
		t.Errorf("unexpected TTFB timing: %v", metrics.TTFB)
	}

	if metrics.Total <= 0 {
		t.Error("total timing should be positive")
	}
}

func TestMetricsCalculations(t *testing.T) {
	metrics := timing.Metrics{
		DNS:   10 * time.Millisecond,
		TCP:   20 * time.Millisecond,
		TLS:   30 * time.Millisecond,
		TTFB:  40 * time.Millisecond,
		Total: 150 * time.Millisecond,
	}

	// Test connection time calculation
	expectedConnectionTime := 10 + 20 + 30 // DNS + TCP + TLS
	if metrics.GetConnectionTime() != time.Duration(expectedConnectionTime)*time.Millisecond {
		t.Errorf("expected connection time %v, got %v",
			time.Duration(expectedConnectionTime)*time.Millisecond,
			metrics.GetConnectionTime())
	}

	// Test server time
	if metrics.GetServerTime() != 40*time.Millisecond {
		t.Errorf("expected server time %v, got %v", 40*time.Millisecond, metrics.GetServerTime())
	}

	// Test network time
	expectedNetworkTime := 150 - 40 // Total - TTFB
	if metrics.GetNetworkTime() != time.Duration(expectedNetworkTime)*time.Millisecond {
		t.Errorf("expected network time %v, got %v",
			time.Duration(expectedNetworkTime)*time.Millisecond,
			metrics.GetNetworkTime())
	}
}

func TestMetricsString(t *testing.T) {
	metrics := timing.Metrics{
		DNS:   10 * time.Millisecond,
		TCP:   20 * time.Millisecond,
		TLS:   30 * time.Millisecond,
		TTFB:  40 * time.Millisecond,
		Total: 100 * time.Millisecond,
	}

	str := metrics.String()
	if str == "" {
		t.Error("string representation should not be empty")
	}

	// Check that it contains the expected components
	expectedSubstrings := []string{"DNS:", "TCP:", "TLS:", "TTFB:", "Total:"}
	for _, substr := range expectedSubstrings {
		if !contains(str, substr) {
			t.Errorf("string representation should contain %q", substr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
