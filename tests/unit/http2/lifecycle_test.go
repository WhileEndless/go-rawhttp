package http2_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	h2 "github.com/WhileEndless/go-rawhttp/pkg/http2"
)

func startH2Server(handler http.HandlerFunc) *httptest.Server {
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	return srv
}

func h2TestOptions() *h2.Options {
	opts := h2.DefaultOptions()
	opts.InsecureTLS = true
	opts.ReuseConnection = true
	opts.ReadTimeout = 3 * time.Second
	return opts
}

// B1: a dead reused connection must surface a real, stale-classified error from
// DoWithOptions — never a phantom empty response (Status=0, err=nil). This exercises
// the http2 client directly (no rawhttp-level retry) to observe the raw error.
func TestH2_EOF_NoPhantom(t *testing.T) {
	srv := startH2Server(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	defer srv.Close()

	port := srv.Listener.Addr().(*net.TCPAddr).Port
	client := h2.NewClient(h2TestOptions())
	defer client.Close()

	req := []byte("GET / HTTP/2\r\nHost: localhost\r\n\r\n")

	resp, err := client.DoWithOptions(context.Background(), req, "localhost", port, "https", h2TestOptions())
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("request 1: expected 200, got %d", resp.Status)
	}

	// Server closes the pooled connection.
	srv.CloseClientConnections()
	time.Sleep(100 * time.Millisecond)

	// The read loop may have proactively evicted the dead connection, in which case a
	// fresh connection is dialed and the request succeeds; otherwise the dead reuse
	// surfaces a stale-classified error. The one thing that must NEVER happen is a
	// phantom response (err == nil with Status == 0).
	resp2, err := client.DoWithOptions(context.Background(), req, "localhost", port, "https", h2TestOptions())
	if err != nil {
		if !h2.IsStaleConnError(err) {
			t.Fatalf("error must be stale-classified for retry, got: %v", err)
		}
		return
	}
	if resp2 == nil || resp2.Status == 0 {
		t.Fatalf("phantom response on dead reuse (no error, no status): %+v", resp2)
	}
	if resp2.Status != 200 {
		t.Fatalf("expected 200 on fresh-connection recovery, got %d", resp2.Status)
	}
}
