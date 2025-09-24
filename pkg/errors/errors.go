// Package errors provides structured error types for the rawhttp library.
package errors

import (
	"context"
	"errors"
	"fmt"
	"net"
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
type Error struct {
	Type      ErrorType `json:"type"`
	Message   string    `json:"message"`
	Cause     error     `json:"cause,omitempty"`
	Host      string    `json:"host,omitempty"`
	Port      int       `json:"port,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
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
		Message:   fmt.Sprintf("DNS lookup failed for host %s", host),
		Cause:     cause,
		Host:      host,
		Timestamp: time.Now(),
	}
}

// NewConnectionError creates a connection error.
func NewConnectionError(host string, port int, cause error) *Error {
	return &Error{
		Type:      ErrorTypeConnection,
		Message:   fmt.Sprintf("failed to connect to %s:%d", host, port),
		Cause:     cause,
		Host:      host,
		Port:      port,
		Timestamp: time.Now(),
	}
}

// NewTLSError creates a TLS handshake error.
func NewTLSError(host string, port int, cause error) *Error {
	return &Error{
		Type:      ErrorTypeTLS,
		Message:   fmt.Sprintf("TLS handshake failed for %s:%d", host, port),
		Cause:     cause,
		Host:      host,
		Port:      port,
		Timestamp: time.Now(),
	}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(operation string, timeout time.Duration) *Error {
	return &Error{
		Type:      ErrorTypeTimeout,
		Message:   fmt.Sprintf("%s timed out after %v", operation, timeout),
		Timestamp: time.Now(),
	}
}

// NewProtocolError creates a protocol error.
func NewProtocolError(message string, cause error) *Error {
	return &Error{
		Type:      ErrorTypeProtocol,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewIOError creates an I/O error.
func NewIOError(operation string, cause error) *Error {
	return &Error{
		Type:      ErrorTypeIO,
		Message:   fmt.Sprintf("I/O error during %s", operation),
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewValidationError creates a validation error.
func NewValidationError(message string) *Error {
	return &Error{
		Type:      ErrorTypeValidation,
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
