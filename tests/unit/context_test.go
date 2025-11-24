package unit

import (
	"context"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2/pkg/errors"
)

func TestContextCancellationDetection(t *testing.T) {
	// Test that we can distinguish context cancellation from timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	err := ctx.Err()
	if !errors.IsContextCanceled(err) {
		t.Errorf("expected IsContextCanceled to return true for canceled context")
	}

	if errors.IsContextTimeout(err) {
		t.Errorf("expected IsContextTimeout to return false for canceled context")
	}
}

func TestContextTimeoutDetection(t *testing.T) {
	// Test that we can detect context deadline exceeded
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(10 * time.Millisecond)

	err := ctx.Err()
	if !errors.IsContextTimeout(err) {
		t.Errorf("expected IsContextTimeout to return true for deadline exceeded")
	}

	// This is also a timeout in the general sense
	if !errors.IsTimeoutError(err) {
		t.Errorf("expected IsTimeoutError to return true for deadline exceeded")
	}

	if errors.IsContextCanceled(err) {
		t.Errorf("expected IsContextCanceled to return false for deadline exceeded")
	}
}

func TestTimeoutErrorWithNetError(t *testing.T) {
	// Test that IsTimeoutError still works with regular timeout errors
	err := errors.NewTimeoutError("test operation", 5*time.Second)

	if !errors.IsTimeoutError(err) {
		t.Errorf("expected IsTimeoutError to return true for timeout error")
	}

	if errors.IsContextCanceled(err) {
		t.Errorf("expected IsContextCanceled to return false for regular timeout")
	}

	if errors.IsContextTimeout(err) {
		t.Errorf("expected IsContextTimeout to return false for regular timeout")
	}
}

func TestErrorTypeHelpers(t *testing.T) {
	// Test all error type helper functions
	testCases := []struct {
		name     string
		err      error
		canceled bool
		timeout  bool
		deadline bool
	}{
		{
			name:     "nil error",
			err:      nil,
			canceled: false,
			timeout:  false,
			deadline: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			canceled: true,
			timeout:  false,
			deadline: false,
		},
		{
			name:     "context deadline",
			err:      context.DeadlineExceeded,
			canceled: false,
			timeout:  true,
			deadline: true,
		},
		{
			name:     "regular error",
			err:      errors.NewProtocolError("test", nil),
			canceled: false,
			timeout:  false,
			deadline: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if errors.IsContextCanceled(tc.err) != tc.canceled {
				t.Errorf("IsContextCanceled mismatch for %s", tc.name)
			}
			if errors.IsTimeoutError(tc.err) != tc.timeout {
				t.Errorf("IsTimeoutError mismatch for %s", tc.name)
			}
			if errors.IsContextTimeout(tc.err) != tc.deadline {
				t.Errorf("IsContextTimeout mismatch for %s", tc.name)
			}
		})
	}
}
