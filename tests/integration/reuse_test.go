package integration

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

// readRequest consumes a single HTTP request head (request line + headers) from r.
func readRequest(r *bufio.Reader) error {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "\r\n" {
			return nil
		}
	}
}

// writeKeepAliveResp writes a minimal keep-alive 200 response with the given body.
func writeKeepAliveResp(conn net.Conn, body string) {
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
}

// startOneShotServer accepts connections and, for each, reads one request, sends a
// keep-alive 200 response, then closes the connection. This simulates a server that
// closes idle keep-alive connections between requests, forcing stale reuse on the client.
func startOneShotServer(ln net.Listener, body string) {
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				if readRequest(r) != nil {
					return
				}
				writeKeepAliveResp(c, body)
			}(conn)
		}
	}()
}

func newSenderOpts(addr *net.TCPAddr) rawhttp.Options {
	return rawhttp.Options{
		Scheme:          "http",
		Host:            "example.com",
		Port:            addr.Port,
		ConnectIP:       addr.IP.String(),
		ConnTimeout:     time.Second,
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		ReuseConnection: true,
	}
}

// A1 + A2: a server that closes the keep-alive connection between requests must not
// cause "out of nowhere" failures; the second request transparently retries on a
// fresh connection.
func TestRetry_StaleKeepAlive_ServerClosesIdleConn(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	startOneShotServer(ln, "hello")

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := newSenderOpts(addr)
	req := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")

	resp1, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	resp1.Body.Close()
	resp1.Raw.Close()
	if resp1.StatusCode != 200 {
		t.Fatalf("request 1: expected 200, got %d", resp1.StatusCode)
	}

	// Give the server a moment to close the pooled connection.
	time.Sleep(50 * time.Millisecond)

	resp2, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("request 2 failed (stale reuse not recovered): %v", err)
	}
	defer resp2.Body.Close()
	defer resp2.Raw.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("request 2: expected 200, got %d", resp2.StatusCode)
	}
}

// A2: many sequential requests against a server that closes every connection after a
// single response must all succeed (retry must reach a fresh connection, never get
// stuck reusing stale ones).
func TestRetry_ForcesFreshConn(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	startOneShotServer(ln, "ok")

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := newSenderOpts(addr)
	req := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")

	for i := 0; i < 5; i++ {
		resp, err := sender.Do(context.Background(), req, opts)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
		resp.Body.Close()
		resp.Raw.Close()
		time.Sleep(30 * time.Millisecond)
	}
}

// A3: a short body (server sends fewer bytes than Content-Length, then closes) must
// return the partial body but the connection must NOT be returned to the pool.
func TestFixedBody_ShortRead_ConnectionNotPooled(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		if readRequest(r) != nil {
			return
		}
		// Content-Length says 10, but only 5 bytes are sent, then close.
		fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\nshort")
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := newSenderOpts(addr)
	resp, err := sender.Do(context.Background(), []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), opts)
	if err != nil {
		t.Fatalf("expected partial body without error, got: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()
	if resp.BodyBytes != 5 {
		t.Fatalf("expected 5 partial body bytes, got %d", resp.BodyBytes)
	}
	if stats := sender.PoolStats(); stats.IdleConns != 0 {
		t.Fatalf("short-read connection must not be pooled, IdleConns=%d", stats.IdleConns)
	}
}

// A3: Content-Length too small with trailing bytes is ambiguous; the connection must
// not be reused.
func TestFixedBody_ExtraData_ConnectionNotPooled(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		if readRequest(r) != nil {
			return
		}
		// Content-Length 5 but body has extra trailing bytes.
		fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhelloEXTRA")
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := newSenderOpts(addr)
	resp, err := sender.Do(context.Background(), []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()
	if resp.BodyBytes != 5 {
		t.Fatalf("expected 5 body bytes, got %d", resp.BodyBytes)
	}
	if stats := sender.PoolStats(); stats.IdleConns != 0 {
		t.Fatalf("ambiguous-framing connection must not be pooled, IdleConns=%d", stats.IdleConns)
	}
}

// A3: a connection-close framed body consumes the connection; it must never be pooled.
func TestReadUntilClose_NotPooled(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		r := bufio.NewReader(conn)
		if readRequest(r) != nil {
			conn.Close()
			return
		}
		// No Content-Length, body framed by connection close.
		fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nConnection: close\r\n\r\nbody-data")
		conn.Close()
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := newSenderOpts(addr)
	resp, err := sender.Do(context.Background(), []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()
	if resp.BodyBytes != int64(len("body-data")) {
		t.Fatalf("expected %d body bytes, got %d", len("body-data"), resp.BodyBytes)
	}
	if stats := sender.PoolStats(); stats.IdleConns != 0 {
		t.Fatalf("connection-close framed connection must not be pooled, IdleConns=%d", stats.IdleConns)
	}
}

// A4: with ReadTimeout=0, a dead/half-open reused connection must fail fast (fallback
// deadline) and recover via retry on a fresh connection instead of blocking forever.
func TestReadDeadlineFallback_DeadReusedConn_FailsFast(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()

	var mu sync.Mutex
	var kept []net.Conn
	firstHandled := false
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				r := bufio.NewReader(c)
				if readRequest(r) != nil {
					c.Close()
					return
				}
				mu.Lock()
				first := !firstHandled
				firstHandled = true
				mu.Unlock()
				writeKeepAliveResp(c, "hello")
				if first {
					// Leave the first connection open and silent (half-open) so a
					// reused request stalls on the read and must time out + retry.
					mu.Lock()
					kept = append(kept, c)
					mu.Unlock()
					return
				}
				c.Close()
			}(conn)
		}
	}()
	defer func() {
		mu.Lock()
		for _, c := range kept {
			c.Close()
		}
		mu.Unlock()
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            "example.com",
		Port:            addr.Port,
		ConnectIP:       addr.IP.String(),
		ConnTimeout:     500 * time.Millisecond,
		ReadTimeout:     0, // unset → fallback deadline applies to the response head
		WriteTimeout:    time.Second,
		ReuseConnection: true,
	}
	req := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")

	resp1, err := sender.Do(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	resp1.Body.Close()
	resp1.Raw.Close()

	done := make(chan struct{})
	var resp2 *rawhttp.Response
	var err2 error
	go func() {
		resp2, err2 = sender.Do(context.Background(), req, opts)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request 2 hung on dead reused connection (no fallback deadline)")
	}
	if err2 != nil {
		t.Fatalf("request 2 failed instead of recovering: %v", err2)
	}
	defer resp2.Body.Close()
	defer resp2.Raw.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("request 2: expected 200, got %d", resp2.StatusCode)
	}
}

// A4: the fallback deadline must apply only to the response head; a slowly trickled
// body with ReadTimeout=0 must still be received in full.
func TestReadDeadlineFallback_DoesNotBreakStreaming(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	body := "0123456789"
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		if readRequest(r) != nil {
			return
		}
		// Head sent promptly; body trickled slower than the fallback (300ms) deadline.
		fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", len(body))
		for i := 0; i < len(body); i++ {
			conn.Write([]byte{body[i]})
			time.Sleep(60 * time.Millisecond)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := rawhttp.Options{
		Scheme:       "http",
		Host:         "example.com",
		Port:         addr.Port,
		ConnectIP:    addr.IP.String(),
		ConnTimeout:  300 * time.Millisecond,
		ReadTimeout:  0,
		WriteTimeout: time.Second,
	}
	resp, err := sender.Do(context.Background(), []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), opts)
	if err != nil {
		t.Fatalf("streaming body read failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()
	if resp.BodyBytes != int64(len(body)) {
		t.Fatalf("expected full body %d bytes, got %d", len(body), resp.BodyBytes)
	}
}

// A1: an EOF on a FRESH (non-reused) connection is a real error and must NOT be
// retried/treated as stale.
func TestEOF_FreshConn_NotRetried(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		r := bufio.NewReader(conn)
		_ = readRequest(r)
		conn.Close() // close without any response
	}()

	addr := ln.Addr().(*net.TCPAddr)
	sender := rawhttp.NewSender()
	opts := rawhttp.Options{
		Scheme:          "http",
		Host:            "example.com",
		Port:            addr.Port,
		ConnectIP:       addr.IP.String(),
		ConnTimeout:     time.Second,
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		ReuseConnection: false, // fresh connection, never reused
	}
	_, err := sender.Do(context.Background(), []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), opts)
	if err == nil {
		t.Fatal("expected an error for EOF on a fresh connection, got nil")
	}
}
