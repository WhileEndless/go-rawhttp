package http2

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const (
	// HTTP/2 connection preface
	ClientPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
)

// Transport manages HTTP/2 connections
type Transport struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	options     *Options
}

// NewTransport creates a new HTTP/2 transport
func NewTransport(opts *Options) *Transport {
	if opts == nil {
		opts = DefaultOptions()
	}

	t := &Transport{
		connections: make(map[string]*Connection),
		options:     opts,
	}

	// Start connection health checker
	go t.healthChecker()

	return t
}

// healthChecker periodically checks connection health
func (t *Transport) healthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		t.checkConnectionHealth()
	}
}

// checkConnectionHealth sends PING frames and removes unhealthy connections
func (t *Transport) checkConnectionHealth() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for addr, conn := range t.connections {
		// Check if connection is idle
		conn.mu.RLock()
		idleTime := now.Sub(conn.LastActivity)
		closed := conn.Closed
		conn.mu.RUnlock()

		if closed {
			// Remove closed connections
			delete(t.connections, addr)
			continue
		}

		// Send PING for keep-alive if idle for more than 15 seconds
		if idleTime > 15*time.Second {
			pingData := [8]byte{0, 0, 0, 0, 0, 0, 0, byte(now.Unix())}
			if err := conn.Framer.WritePing(false, pingData); err != nil {
				// Connection is broken, close and remove it
				conn.Close()
				delete(t.connections, addr)
			} else {
				// Update last activity
				conn.mu.Lock()
				conn.LastActivity = now
				conn.mu.Unlock()
			}
		}

		// Remove connections idle for too long (5 minutes)
		if idleTime > 5*time.Minute {
			conn.Close()
			delete(t.connections, addr)
		}
	}
}

// Connect establishes an HTTP/2 connection
func (t *Transport) Connect(ctx context.Context, host string, port int, scheme string) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	// Check for existing connection if reuse is enabled
	// Use write lock to prevent race conditions when multiple goroutines
	// try to create a connection simultaneously
	var needUnlock bool
	if t.options.ReuseConnection {
		t.mu.Lock()
		needUnlock = true
		if conn, exists := t.connections[addr]; exists && !conn.Closed {
			// Wait for connection to be ready if it's still initializing
			if !conn.Ready {
				t.mu.Unlock()
				// Wait a bit for connection to become ready
				for i := 0; i < 100 && !conn.Ready && !conn.Closed; i++ {
					time.Sleep(10 * time.Millisecond)
				}
				if conn.Ready && !conn.Closed {
					return conn, nil
				}
				// Connection failed or timed out, create a new one
				t.mu.Lock()
				needUnlock = true
				delete(t.connections, addr)
			} else {
				t.mu.Unlock()
				return conn, nil
			}
		}
		// Keep the lock held to prevent race - will be released after storing connection
	}

	// Establish new connection
	var rawConn net.Conn
	var err error

	if scheme == "https" {
		// TLS connection with ALPN
		rawConn, err = t.connectTLS(ctx, addr, host)
	} else {
		// Plain TCP connection (H2C)
		rawConn, err = t.connectH2C(ctx, addr)
	}

	if err != nil {
		if needUnlock {
			t.mu.Unlock() // Release lock on connection error
		}
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Create HTTP/2 connection
	conn := &Connection{
		Conn:          rawConn, // Store the underlying connection
		Framer:        http2.NewFramer(rawConn, rawConn),
		Streams:       make(map[uint32]*Stream),
		NextStreamID:  1, // Client streams use odd IDs
		MaxConcurrent: t.options.MaxConcurrentStreams,
		WindowSize:    int32(t.options.InitialWindowSize),
		Settings:      make(map[http2.SettingID]uint32),
		PeerSettings:  make(map[http2.SettingID]uint32),
		LastActivity:  time.Now(),
	}

	// Initialize HPACK encoder/decoder for this connection
	// Each connection needs its own HPACK context
	conn.EncoderBuf = &bytes.Buffer{}
	conn.Encoder = hpack.NewEncoder(conn.EncoderBuf)
	conn.Encoder.SetMaxDynamicTableSize(t.options.HeaderTableSize)
	conn.Decoder = hpack.NewDecoder(t.options.HeaderTableSize, nil)

	// Important: Release lock before blocking I/O but don't store connection yet
	if needUnlock {
		t.mu.Unlock()
		needUnlock = false // We'll re-acquire if needed
	}

	// Send initial settings (this can block waiting for ACK)
	if err := t.sendInitialSettings(conn); err != nil {
		// Defensive nil check before closing (rawConn should not be nil here, but being extra safe)
		if rawConn != nil {
			rawConn.Close()
		}
		return nil, fmt.Errorf("failed to send settings: %w", err)
	}

	// Mark connection as ready
	conn.Ready = true

	// Now store the fully initialized connection
	if t.options.ReuseConnection {
		t.mu.Lock()
		// Check if someone else created a connection while we were initializing
		if existing, exists := t.connections[addr]; exists && existing.Ready && !existing.Closed {
			t.mu.Unlock()
			// Use the existing connection and close ours
			conn.Close()
			return existing, nil
		}
		t.connections[addr] = conn
		t.mu.Unlock()
	}

	return conn, nil
}

// connectTLS establishes a TLS connection with ALPN negotiation
func (t *Transport) connectTLS(ctx context.Context, addr, serverName string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	// Create TLS config with ALPN
	var tlsConfig *tls.Config

	// Use custom TLS config if provided
	if t.options.TLSConfig != nil {
		// Clone to avoid modifying the original
		tlsConfig = t.options.TLSConfig.Clone()

		// Ensure HTTP/2 ALPN is included
		if len(tlsConfig.NextProtos) == 0 {
			tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		} else {
			// Check if h2 is already present
			hasH2 := false
			for _, proto := range tlsConfig.NextProtos {
				if proto == "h2" {
					hasH2 = true
					break
				}
			}
			if !hasH2 {
				// Prepend h2 to the list
				tlsConfig.NextProtos = append([]string{"h2"}, tlsConfig.NextProtos...)
			}
		}

		// Apply InsecureTLS flag (overrides TLSConfig setting)
		if t.options.InsecureTLS {
			tlsConfig.InsecureSkipVerify = true
		}
	} else {
		// Use default TLS config
		tlsConfig = &tls.Config{
			ServerName:         serverName,
			NextProtos:         []string{"h2", "http/1.1"},
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: t.options.InsecureTLS,
		}
	}

	// Dial with context
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Perform TLS handshake
	tlsConn := tls.Client(conn, tlsConfig)

	// Set handshake timeout
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	tlsConn.SetDeadline(deadline)

	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Clear deadline
	tlsConn.SetDeadline(time.Time{})

	// Verify ALPN negotiation
	state := tlsConn.ConnectionState()
	if state.NegotiatedProtocol != "h2" {
		tlsConn.Close() // Close TLS connection (which also closes underlying TCP connection)
		return nil, fmt.Errorf("server does not support HTTP/2 (negotiated: %s)", state.NegotiatedProtocol)
	}

	// Send HTTP/2 preface
	if _, err := tlsConn.Write([]byte(ClientPreface)); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("failed to send HTTP/2 preface: %w", err)
	}

	return tlsConn, nil
}

// connectH2C establishes a cleartext HTTP/2 connection
func (t *Transport) connectH2C(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	// Option 1: Direct HTTP/2 (prior knowledge)
	if t.options.EnableMultiplexing {
		// Send HTTP/2 preface directly
		if _, err := conn.Write([]byte(ClientPreface)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to send H2C preface: %w", err)
		}
		return conn, nil
	}

	// Option 2: HTTP/1.1 Upgrade
	upgradeReq := t.buildH2CUpgradeRequest(addr)
	if _, err := conn.Write(upgradeReq); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send H2C upgrade request: %w", err)
	}

	// Read upgrade response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read H2C upgrade response: %w", err)
	}

	// Check for successful upgrade
	response := string(buf[:n])
	if !containsUpgradeSuccess(response) {
		conn.Close()
		return nil, fmt.Errorf("H2C upgrade failed: %s", response)
	}

	// Send HTTP/2 preface after successful upgrade
	if _, err := conn.Write([]byte(ClientPreface)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send H2C preface after upgrade: %w", err)
	}

	return conn, nil
}

// buildH2CUpgradeRequest builds an HTTP/1.1 upgrade request for H2C
func (t *Transport) buildH2CUpgradeRequest(host string) []byte {
	// Encode settings for HTTP2-Settings header
	settings := t.encodeHTTP2Settings()

	req := fmt.Sprintf(
		"GET / HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Connection: Upgrade, HTTP2-Settings\r\n"+
			"Upgrade: h2c\r\n"+
			"HTTP2-Settings: %s\r\n"+
			"\r\n",
		host, settings)

	return []byte(req)
}

// encodeHTTP2Settings encodes settings for H2C upgrade
func (t *Transport) encodeHTTP2Settings() string {
	builder := NewRawFrameBuilder()

	settings := map[http2.SettingID]uint32{
		http2.SettingHeaderTableSize:      t.options.HeaderTableSize,
		http2.SettingEnablePush:           boolToUint32(t.options.EnableServerPush),
		http2.SettingMaxConcurrentStreams: t.options.MaxConcurrentStreams,
		http2.SettingInitialWindowSize:    t.options.InitialWindowSize,
		http2.SettingMaxFrameSize:         t.options.MaxFrameSize,
		http2.SettingMaxHeaderListSize:    t.options.MaxHeaderListSize,
	}

	frame := builder.BuildSettingsFrame(settings, false)
	// Skip frame header (9 bytes) and encode only payload
	payload := frame[9:]

	return base64.RawURLEncoding.EncodeToString(payload)
}

// sendInitialSettings sends initial SETTINGS frame (aligned with Go's approach)
func (t *Transport) sendInitialSettings(conn *Connection) error {
	// Send only the settings that Go's HTTP/2 sends (minimal set)
	settings := map[http2.SettingID]uint32{
		http2.SettingEnablePush:        boolToUint32(t.options.EnableServerPush), // Always 0
		http2.SettingInitialWindowSize: t.options.InitialWindowSize,              // 4MB
		http2.SettingMaxFrameSize:      t.options.MaxFrameSize,                   // 16KB
		http2.SettingMaxHeaderListSize: t.options.MaxHeaderListSize,              // 10MB
	}

	// Store our settings
	for id, value := range settings {
		conn.Settings[id] = value
	}

	// Send SETTINGS frame
	if err := conn.Framer.WriteSettings(convertSettings(settings)...); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	// Wait for SETTINGS ACK from server (required by HTTP/2 spec)
	if err := t.waitForSettingsAck(conn); err != nil {
		return fmt.Errorf("failed to receive settings ACK: %w", err)
	}

	// Send connection-level window update (like Go's HTTP/2 does)
	// Go sends a WINDOW_UPDATE to increase the connection window size
	if t.options.InitialWindowSize > 65535 {
		increment := t.options.InitialWindowSize - 65535
		if err := conn.Framer.WriteWindowUpdate(0, increment); err != nil {
			return fmt.Errorf("failed to write connection window update: %w", err)
		}
	}

	return nil
}

// waitForSettingsAck waits for SETTINGS ACK from server
func (t *Transport) waitForSettingsAck(conn *Connection) error {
	// Set a reasonable timeout for the handshake
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	// Read frames until we get SETTINGS ACK
	for {
		// Check for timeout
		select {
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for SETTINGS ACK")
		default:
		}

		frame, err := conn.Framer.ReadFrame()
		if err != nil {
			return fmt.Errorf("failed to read frame while waiting for SETTINGS ACK: %w", err)
		}

		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if f.IsAck() {
				// Got SETTINGS ACK, we can proceed
				return nil
			} else {
				// Server sent its own SETTINGS, we should ACK it
				if err := conn.Framer.WriteSettingsAck(); err != nil {
					return fmt.Errorf("failed to ACK server settings: %w", err)
				}
				// Continue waiting for our SETTINGS ACK
			}

		case *http2.WindowUpdateFrame:
			// Server might send window updates, that's fine, ignore for now
			continue

		case *http2.PingFrame:
			// Server might send PING, we should respond
			if err := conn.Framer.WritePing(true, f.Data); err != nil {
				return fmt.Errorf("failed to respond to PING: %w", err)
			}

		case *http2.GoAwayFrame:
			return fmt.Errorf("server sent GOAWAY during handshake: last stream %d, error %v",
				f.LastStreamID, f.ErrCode)

		default:
			// Unexpected frame during handshake
			return fmt.Errorf("unexpected frame during SETTINGS handshake: %T", frame)
		}
	}
}

// Close closes all connections
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lastErr error
	for addr, conn := range t.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(t.connections, addr)
	}

	return lastErr
}

// CloseConnection closes a specific connection
func (t *Transport) CloseConnection(addr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if conn, exists := t.connections[addr]; exists {
		err := conn.Close()
		delete(t.connections, addr)
		return err
	}

	return nil
}

// Helper functions

func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func convertSettings(settings map[http2.SettingID]uint32) []http2.Setting {
	var result []http2.Setting
	for id, val := range settings {
		result = append(result, http2.Setting{
			ID:  id,
			Val: val,
		})
	}
	return result
}

func containsUpgradeSuccess(response string) bool {
	// Check for successful upgrade response
	return len(response) > 12 && response[:12] == "HTTP/1.1 101"
}
