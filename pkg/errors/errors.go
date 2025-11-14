// Package errors provides structured error types for the rawhttp library.
package errors

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// ErrorType represents the category of error that occurred.
type ErrorType string

const (
	// ErrorTypeDNS represents DNS resolution errors
	ErrorTypeDNS ErrorType = "dns"
	// ErrorTypeConnection represents TCP connection errors
	ErrorTypeConnection ErrorType = "connection"
	// ErrorTypeTLS represents TLS handshake errors
	ErrorTypeTLS ErrorType = "tls"
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeProtocol represents HTTP protocol errors
	ErrorTypeProtocol ErrorType = "protocol"
	// ErrorTypeIO represents I/O errors
	ErrorTypeIO ErrorType = "io"
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation ErrorType = "validation"
)

// Error represents a structured error with context information.
// This provides detailed transport-layer error classification for debugging and error handling.
type Error struct {
	Type      ErrorType `json:"type"`      // Error category (dns, tcp, tls, etc.)
	Op        string    `json:"op"`        // Operation that failed (dial, handshake, read, write, etc.)
	Message   string    `json:"message"`   // Human-readable error message
	Cause     error     `json:"cause,omitempty"` // Underlying error
	Host      string    `json:"host,omitempty"`  // Target host
	Port      int       `json:"port,omitempty"`  // Target port
	Addr      string    `json:"addr,omitempty"`  // Full address (host:port)
	Timestamp time.Time `json:"timestamp"` // When the error occurred
}

// TransportError is an alias for Error, provided for API compatibility
// with transport error naming conventions.
type TransportError = Error

// Error implements the error interface.
// Format: [type] op addr: message: cause
func (e *Error) Error() string {
	var parts []string

	// Add type
	parts = append(parts, fmt.Sprintf("[%s]", e.Type))

	// Add operation if present
	if e.Op != "" {
		parts = append(parts, e.Op)
	}

	// Add address if present
	if e.Addr != "" {
		parts = append(parts, e.Addr)
	} else if e.Host != "" {
		if e.Port > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", e.Host, e.Port))
		} else {
			parts = append(parts, e.Host)
		}
	}

	// Build error string
	errStr := strings.Join(parts, " ")
	if e.Message != "" {
		errStr += ": " + e.Message
	}
	if e.Cause != nil {
		errStr += ": " + e.Cause.Error()
	}

	return errStr
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target type.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Type == t.Type
	}
	return false
}

// NewDNSError creates a DNS resolution error.
func NewDNSError(host string, cause error) *Error {
	return &Error{
		Type:      ErrorTypeDNS,
		Op:        "lookup",
		Message:   fmt.Sprintf("DNS lookup failed for host %s", host),
		Cause:     cause,
		Host:      host,
		Addr:      host,
		Timestamp: time.Now(),
	}
}

// NewConnectionError creates a connection error.
func NewConnectionError(host string, port int, cause error) *Error {
	addr := fmt.Sprintf("%s:%d", host, port)
	return &Error{
		Type:      ErrorTypeConnection,
		Op:        "dial",
		Message:   fmt.Sprintf("failed to connect to %s", addr),
		Cause:     cause,
		Host:      host,
		Port:      port,
		Addr:      addr,
		Timestamp: time.Now(),
	}
}

// NewTLSError creates a TLS handshake error.
func NewTLSError(host string, port int, cause error) *Error {
	addr := fmt.Sprintf("%s:%d", host, port)
	return &Error{
		Type:      ErrorTypeTLS,
		Op:        "handshake",
		Message:   fmt.Sprintf("TLS handshake failed for %s", addr),
		Cause:     cause,
		Host:      host,
		Port:      port,
		Addr:      addr,
		Timestamp: time.Now(),
	}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(operation string, timeout time.Duration) *Error {
	return &Error{
		Type:      ErrorTypeTimeout,
		Op:        operation,
		Message:   fmt.Sprintf("operation timed out after %v", timeout),
		Timestamp: time.Now(),
	}
}

// NewProtocolError creates a protocol error.
func NewProtocolError(message string, cause error) *Error {
	return &Error{
		Type:      ErrorTypeProtocol,
		Op:        "parse",
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewIOError creates an I/O error.
func NewIOError(operation string, cause error) *Error {
	// Extract operation type (read/write) from message
	op := operation
	if strings.Contains(strings.ToLower(operation), "read") {
		op = "read"
	} else if strings.Contains(strings.ToLower(operation), "writ") {
		op = "write"
	}

	return &Error{
		Type:      ErrorTypeIO,
		Op:        op,
		Message:   fmt.Sprintf("I/O error during %s", operation),
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewValidationError creates a validation error.
func NewValidationError(message string) *Error {
	return &Error{
		Type:      ErrorTypeValidation,
		Op:        "validate",
		Message:   message,
		Timestamp: time.Now(),
	}
}

// IsTimeoutError checks if an error is a timeout error.
func IsTimeoutError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Type == ErrorTypeTimeout
	}
	// Also check for net timeout errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

// IsTemporaryError checks if an error is temporary.
func IsTemporaryError(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary()
	}
	return false
}

// GetErrorType returns the error type if it's a structured error.
func GetErrorType(err error) ErrorType {
	if e, ok := err.(*Error); ok {
		return e.Type
	}
	return ""
}

// IsContextCanceled checks if an error is due to context cancellation.
func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

// IsContextTimeout checks if an error is due to context deadline exceeded.
func IsContextTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
