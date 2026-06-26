package unit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func newHTTP2Server(handler http.HandlerFunc) *httptest.Server {
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	return srv
}

func h2Opts(srv *httptest.Server) rawhttp.Options {
	return rawhttp.Options{
		Scheme:          "https",
		Host:            "localhost",
		Port:            srv.Listener.Addr().(*net.TCPAddr).Port,
		Protocol:        "http/2",
		InsecureTLS:     true,
		ReuseConnection: true,
		ConnTimeout:     5 * time.Second,
		ReadTimeout:     5 * time.Second,
	}
}

// B3: many sequential requests on a reused HTTP/2 connection must all succeed and
// must not leak streams (the per-connection stream table must not grow unbounded).
func TestH2_Reuse_Sequential_NoLeak(t *testing.T) {
	srv := newHTTP2Server(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	defer srv.Close()

	sender := rawhttp.NewSender()
	opts := h2Opts(srv)

	reused := 0
	for i := 0; i < 25; i++ {
		req := []byte(fmt.Sprintf("GET /seq/%d HTTP/2\r\nHost: localhost\r\n\r\n", i))
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
		if resp.ConnectionReused {
			reused++
		}
		resp.Body.Close()
		resp.Raw.Close()
	}

	// Most requests must reuse the single pooled connection (stream-table leak is
	// asserted white-box in pkg/http2). A reuse of 0 would indicate pooling broke.
	if reused == 0 {
		t.Fatal("expected the HTTP/2 connection to be reused across sequential requests")
	}
}

// B0 + B5: concurrent requests multiplexed on a single reused HTTP/2 connection must
// be race-free and each must receive its OWN response (no cross-stream frame theft).
// Run with -race.
func TestH2_Multiplexing_Concurrent(t *testing.T) {
	srv := newHTTP2Server(func(w http.ResponseWriter, r *http.Request) {
		// Echo the path so each stream can verify it got its own response.
		fmt.Fprint(w, r.URL.Path)
	})
	defer srv.Close()

	sender := rawhttp.NewSender()
	opts := h2Opts(srv)

	// Warm up so a single pooled connection exists; concurrent requests then
	// multiplex on it.
	warm := []byte("GET /warmup HTTP/2\r\nHost: localhost\r\n\r\n")
	if resp, err := sender.Do(context.Background(), warm, opts); err != nil {
		t.Fatalf("warmup failed: %v", err)
	} else {
		resp.Body.Close()
		resp.Raw.Close()
	}

	const n = 24
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := fmt.Sprintf("/mux/%d", i)
			req := []byte(fmt.Sprintf("GET %s HTTP/2\r\nHost: localhost\r\n\r\n", path))
			resp, err := sender.Do(context.Background(), req, opts)
			if err != nil {
				errs <- fmt.Errorf("request %d: %v", i, err)
				return
			}
			defer resp.Body.Close()
			defer resp.Raw.Close()
			if resp.StatusCode != 200 {
				errs <- fmt.Errorf("request %d: status %d", i, resp.StatusCode)
				return
			}
			if got := string(resp.Body.Bytes()); got != path {
				errs <- fmt.Errorf("request %d: body %q != %q (cross-stream frame mixup)", i, got, path)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// B1 + B2: when the server closes the pooled connection between requests, the next
// request must transparently retry on a fresh connection instead of returning a
// phantom empty response or a hard error.
func TestH2_StaleRetry_ServerClosesConn(t *testing.T) {
	srv := newHTTP2Server(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	defer srv.Close()

	sender := rawhttp.NewSender()
	opts := h2Opts(srv)
	req := []byte("GET / HTTP/2\r\nHost: localhost\r\n\r\n")

	resp1, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	resp1.Body.Close()
	resp1.Raw.Close()

	// Simulate the server closing the idle keep-alive connection.
	srv.CloseClientConnections()
	time.Sleep(100 * time.Millisecond)

	resp2, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("request 2 failed (stale HTTP/2 reuse not recovered): %v", err)
	}
	defer resp2.Body.Close()
	defer resp2.Raw.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("request 2: expected 200, got %d", resp2.StatusCode)
	}
}

// B6: a slow server must hit the per-request ReadTimeout and return a timeout error
// without blocking forever.
func TestH2_ReadDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := newHTTP2Server(func(w http.ResponseWriter, r *http.Request) {
		<-release // hold the response open
	})
	defer srv.Close()
	defer close(release)

	sender := rawhttp.NewSender()
	opts := h2Opts(srv)
	opts.ReadTimeout = 400 * time.Millisecond
	req := []byte("GET / HTTP/2\r\nHost: localhost\r\n\r\n")

	done := make(chan error, 1)
	go func() {
		_, err := sender.Do(context.Background(), req, opts)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a timeout error, got nil")
		}
		if !rawhttp.IsTimeoutError(err) {
			t.Fatalf("expected timeout error, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request hung past ReadTimeout")
	}
}
