package http2

import (
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
)

// Sentinel errors describing connection-level conditions that make an HTTP/2
// connection unusable. They are wrapped into the errors returned from a request
// so the request-level retry logic (rawhttp.Do) can recover transparently on a
// fresh connection.
var (
	// errConnClosed indicates the connection was closed (EOF / reset) before the
	// in-flight stream completed.
	errConnClosed = stderrors.New("http2: connection closed before response complete")

	// errGoAway indicates the server sent a GOAWAY frame, retiring the connection.
	errGoAway = stderrors.New("http2: server sent GOAWAY")

	// errStreamIDExhausted indicates the connection ran out of client stream IDs
	// (2^31-1) and must be replaced with a new connection.
	errStreamIDExhausted = stderrors.New("http2: stream ID exhausted")
)

// wrapStaleHTTP2Error wraps a cause with a stable, stale-classifiable message so
// IsStaleConnError recognizes it even across the rawhttp package boundary (which
// inspects err.Error()).
func wrapStaleHTTP2Error(op string, cause error) error {
	if cause == nil {
		return fmt.Errorf("http2 stale connection: %s", op)
	}
	return fmt.Errorf("http2 stale connection: %s: %w", op, cause)
}

// IsStaleConnError reports whether err indicates a stale/closed HTTP/2 connection
// that should be retried on a fresh connection. It mirrors the HTTP/1.1 classifier
// in pkg/client and is intentionally tolerant: it matches both wrapped sentinels
// and common network error strings so classification survives error wrapping and
// the rawhttp package boundary.
func IsStaleConnError(err error) bool {
	if err == nil {
		return false
	}

	// Wrapped sentinels (same-package callers).
	if stderrors.Is(err, errConnClosed) ||
		stderrors.Is(err, errGoAway) ||
		stderrors.Is(err, errStreamIDExhausted) {
		return true
	}

	// Low-level network conditions.
	if stderrors.Is(err, io.EOF) ||
		stderrors.Is(err, io.ErrUnexpectedEOF) ||
		stderrors.Is(err, net.ErrClosed) ||
		stderrors.Is(err, syscall.EPIPE) ||
		stderrors.Is(err, syscall.ECONNRESET) {
		return true
	}

	// String markers (survive crossing the package boundary as plain text).
	errStr := strings.ToLower(err.Error())
	markers := []string{
		"connection closed before response complete",
		"server sent goaway",
		"stream id exhausted",
		"broken pipe",
		"connection reset by peer",
		"use of closed network connection",
		"forcibly closed",
		"connection was aborted",
	}
	for _, m := range markers {
		if strings.Contains(errStr, m) {
			return true
		}
	}

	return false
}
