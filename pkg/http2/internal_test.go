package http2

import (
	stderrors "errors"
	"fmt"
	"io"
	"syscall"
	"testing"
)

// B4: stream-ID exhaustion on a connection must produce a stale-classified error
// (so the request-level retry opens a fresh connection) and tear the connection down.
func TestOpenStream_Exhaustion(t *testing.T) {
	c := NewClient(nil)
	conn := &Connection{
		Streams:      make(map[uint32]*Stream),
		NextStreamID: maxClientStreamID + 2, // already past the limit
		closedCh:     make(chan struct{}),
	}

	_, err := c.openStream(conn, []byte("GET / HTTP/2\r\nHost: x\r\n\r\n"), &Request{})
	if err == nil {
		t.Fatal("expected stream-ID exhaustion error, got nil")
	}
	if !stderrors.Is(err, errStreamIDExhausted) {
		t.Fatalf("expected errStreamIDExhausted, got: %v", err)
	}
	if !IsStaleConnError(err) {
		t.Fatalf("exhaustion error must be stale-classified for retry, got: %v", err)
	}
	if !conn.isClosed() {
		t.Fatal("connection must be torn down after stream-ID exhaustion")
	}
}

func TestIsStaleConnError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"eof", io.EOF, true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"epipe", syscall.EPIPE, true},
		{"econnreset", syscall.ECONNRESET, true},
		{"wrapped conn closed", wrapStaleHTTP2Error("reading frame", errConnClosed), true},
		{"wrapped goaway", wrapStaleHTTP2Error("goaway", errGoAway), true},
		{"goaway string", fmt.Errorf("http2: server sent GOAWAY: x"), true},
		{"reset string", fmt.Errorf("read tcp: connection reset by peer"), true},
		{"unrelated", fmt.Errorf("some validation error"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStaleConnError(tc.err); got != tc.want {
				t.Fatalf("IsStaleConnError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
