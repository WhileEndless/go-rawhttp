package unit

import (
	"fmt"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2/pkg/errors"
)

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name         string
		err          *errors.Error
		expectedType errors.ErrorType
	}{
		{
			name:         "DNS Error",
			err:          errors.NewDNSError("example.com", fmt.Errorf("lookup failed")),
			expectedType: errors.ErrorTypeDNS,
		},
		{
			name:         "Connection Error",
			err:          errors.NewConnectionError("example.com", 443, fmt.Errorf("connection refused")),
			expectedType: errors.ErrorTypeConnection,
		},
		{
			name:         "TLS Error",
			err:          errors.NewTLSError("example.com", 443, fmt.Errorf("handshake failed")),
			expectedType: errors.ErrorTypeTLS,
		},
		{
			name:         "Timeout Error",
			err:          errors.NewTimeoutError("connection", 5*time.Second),
			expectedType: errors.ErrorTypeTimeout,
		},
		{
			name:         "Protocol Error",
			err:          errors.NewProtocolError("invalid status line", fmt.Errorf("parse error")),
			expectedType: errors.ErrorTypeProtocol,
		},
		{
			name:         "IO Error",
			err:          errors.NewIOError("reading", fmt.Errorf("broken pipe")),
			expectedType: errors.ErrorTypeIO,
		},
		{
			name:         "Validation Error",
			err:          errors.NewValidationError("host cannot be empty"),
			expectedType: errors.ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Type != tt.expectedType {
				t.Errorf("expected type %v, got %v", tt.expectedType, tt.err.Type)
			}

			if tt.err.Error() == "" {
				t.Error("error message should not be empty")
			}

			if tt.err.Timestamp.IsZero() {
				t.Error("timestamp should be set")
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := errors.NewDNSError("example.com", cause)

	if err.Unwrap() != cause {
		t.Errorf("expected unwrapped error to be %v, got %v", cause, err.Unwrap())
	}
}

func TestErrorIs(t *testing.T) {
	err1 := errors.NewDNSError("example.com", fmt.Errorf("lookup failed"))
	err2 := &errors.Error{Type: errors.ErrorTypeDNS}

	if !err1.Is(err2) {
		t.Error("errors with same type should match")
	}

	err3 := &errors.Error{Type: errors.ErrorTypeConnection}
	if err1.Is(err3) {
		t.Error("errors with different types should not match")
	}
}

func TestIsTimeoutError(t *testing.T) {
	timeoutErr := errors.NewTimeoutError("connection", 5*time.Second)
	if !errors.IsTimeoutError(timeoutErr) {
		t.Error("should identify timeout error")
	}

	dnsErr := errors.NewDNSError("example.com", fmt.Errorf("lookup failed"))
	if errors.IsTimeoutError(dnsErr) {
		t.Error("should not identify DNS error as timeout")
	}
}

func TestGetErrorType(t *testing.T) {
	err := errors.NewValidationError("test")
	errType := errors.GetErrorType(err)

	if errType != errors.ErrorTypeValidation {
		t.Errorf("expected %v, got %v", errors.ErrorTypeValidation, errType)
	}

	// Test with non-structured error
	regularErr := fmt.Errorf("regular error")
	errType = errors.GetErrorType(regularErr)

	if errType != "" {
		t.Errorf("expected empty type for regular error, got %v", errType)
	}
}
