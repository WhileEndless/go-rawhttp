package integration

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp"
)

func TestClientHTTPChunked(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		line, _ := reader.ReadString('\n')
		if !strings.Contains(line, "/chunk") {
			t.Errorf("unexpected request line: %s", line)
		}

		// Read headers
		for {
			l, err := reader.ReadString('\n')
			if err != nil || l == "\r\n" {
				break
			}
		}

		response := "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n4\r\nTest\r\n0\r\n\r\n"
		conn.Write([]byte(response))
	}()

	addr := ln.Addr().(*net.TCPAddr)
	req := []byte("GET /chunk HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")

	client := rawhttp.NewSender()
	resp, err := client.Do(context.Background(), req, rawhttp.Options{
		Scheme:       "http",
		Host:         "example.com",
		Port:         addr.Port,
		ConnectIP:    addr.IP.String(),
		ConnTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})

	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	if resp.BodyBytes != 4 {
		t.Fatalf("expected body size 4, got %d", resp.BodyBytes)
	}
}

func TestClientHTTPS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "hello")
	})
	addr, shutdown := startTLSServer(t, handler)
	defer shutdown()

	host := "localhost"
	req := []byte(fmt.Sprintf("GET /hello HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host))

	client := rawhttp.NewSender()
	resp, err := client.Do(context.Background(), req, rawhttp.Options{
		Scheme:       "https",
		Host:         host,
		Port:         addr.Port,
		ConnectIP:    addr.IP.String(),
		InsecureTLS:  true,
		ConnTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	if resp.BodyBytes == 0 {
		t.Fatalf("expected non-empty body")
	}
}

func TestClientPartialBodyError(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for {
			l, err := reader.ReadString('\n')
			if err != nil || l == "\r\n" {
				break
			}
		}
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 10\r\nConnection: close\r\n\r\nshort"))
	}()

	addr := ln.Addr().(*net.TCPAddr)
	req := []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")

	client := rawhttp.NewSender()
	resp, err := client.Do(context.Background(), req, rawhttp.Options{
		Scheme:       "http",
		Host:         "example.com",
		Port:         addr.Port,
		ConnectIP:    addr.IP.String(),
		ConnTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	})

	// As of v1.1.4, the library gracefully handles Content-Length mismatches
	// (RFC violations) by accepting partial reads without error
	if err != nil {
		t.Fatalf("expected no error for partial body (v1.1.4+), got: %v", err)
	}

	if resp == nil {
		t.Fatalf("expected response")
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Verify we got the partial body data (5 bytes instead of 10)
	if resp.BodyBytes != int64(len("short")) {
		t.Fatalf("unexpected body size %d, expected %d", resp.BodyBytes, len("short"))
	}
}

func TestClientTimings(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "timing test")
	})
	addr, shutdown := startTLSServer(t, handler)
	defer shutdown()

	host := "localhost"
	req := []byte(fmt.Sprintf("GET /timing HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host))

	client := rawhttp.NewSender()
	resp, err := client.Do(context.Background(), req, rawhttp.Options{
		Scheme:       "https",
		Host:         host,
		Port:         addr.Port,
		ConnectIP:    addr.IP.String(),
		InsecureTLS:  true,
		ConnTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer resp.Body.Close()
	defer resp.Raw.Close()

	// Check that timings are recorded
	if resp.Timings.TCP <= 0 {
		t.Errorf("expected positive TCP timing, got %v", resp.Timings.TCP)
	}
	if resp.Timings.TLS <= 0 {
		t.Errorf("expected positive TLS timing, got %v", resp.Timings.TLS)
	}
	if resp.Timings.TTFB <= 0 {
		t.Errorf("expected positive TTFB timing, got %v", resp.Timings.TTFB)
	}
	// DNS should be ~0 since we're using ConnectIP
	if resp.Timings.DNS > time.Millisecond {
		t.Errorf("expected minimal DNS timing with ConnectIP, got %v", resp.Timings.DNS)
	}
}

func TestClientContext(t *testing.T) {
	ln := listenTCP(t)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Never respond to simulate hanging connection
		time.Sleep(10 * time.Second)
	}()

	addr := ln.Addr().(*net.TCPAddr)
	req := []byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := rawhttp.NewSender()
	_, err := client.Do(ctx, req, rawhttp.Options{
		Scheme:    "http",
		Host:      "example.com",
		Port:      addr.Port,
		ConnectIP: addr.IP.String(),
	})

	if err == nil {
		t.Fatalf("expected context timeout error")
	}
}

// Helper functions

func listenTCP(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if isPerm(err) {
			t.Skip("network sockets not permitted in sandbox")
		}
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func isPerm(err error) bool {
	if err == nil {
		return false
	}
	if op, ok := err.(*net.OpError); ok {
		if se, ok := op.Err.(*os.SyscallError); ok {
			if se.Err == syscall.EPERM {
				return true
			}
		}
		if strings.Contains(op.Err.Error(), "operation not permitted") {
			return true
		}
	}
	return strings.Contains(err.Error(), "operation not permitted")
}

func startTLSServer(t *testing.T, handler http.Handler) (*net.TCPAddr, func()) {
	ln := listenTCP(t)
	cert, err := generateSelfSigned()
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	tlsListener := tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}})
	srv := &http.Server{Handler: handler}
	go srv.Serve(tlsListener)

	addr := ln.Addr().(*net.TCPAddr)
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}
	return addr, shutdown
}

func generateSelfSigned() (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return tls.X509KeyPair(certPEM, keyPEM)
}
