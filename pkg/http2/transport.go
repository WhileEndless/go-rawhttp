package http2

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/transport"
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

	// Lifecycle management
	stopChan chan struct{} // Channel to signal background goroutines to stop
	wg       sync.WaitGroup // WaitGroup to track running goroutines
}

// NewTransport creates a new HTTP/2 transport
func NewTransport(opts *Options) *Transport {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Validate options (DEF-9)
	if err := ValidateOptions(opts); err != nil {
		// Return transport with default options if validation fails
		// Log the error but don't panic (graceful degradation)
		opts = DefaultOptions()
	}

	t := &Transport{
		connections: make(map[string]*Connection),
		options:     opts,
		stopChan:    make(chan struct{}),
	}

	// Start connection health checker
	go t.healthChecker()

	return t
}

// healthChecker periodically checks connection health
func (t *Transport) healthChecker() {
	t.wg.Add(1)
	defer t.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.checkConnectionHealth()
		case <-t.stopChan:
			// Cleanup and exit
			return
		}
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

			// Lock before writing to prevent concurrent write panic
			conn.mu.Lock()
			err := conn.Framer.WritePing(false, pingData)
			if err == nil {
				// Update last activity on success
				conn.LastActivity = now
			}
			conn.mu.Unlock()

			if err != nil {
				// Connection is broken, close and remove it
				conn.Close()
				delete(t.connections, addr)
			}
		}

		// Remove connections idle for too long (5 minutes)
		if idleTime > 5*time.Minute {
			conn.Close()
			delete(t.connections, addr)
		}
	}
}

// Connect establishes an HTTP/2 connection with the given options.
// The opts parameter takes precedence over the transport's default options.
func (t *Transport) Connect(ctx context.Context, host string, port int, scheme string, opts *Options) (*Connection, error) {
	// Use provided options or fall back to transport defaults
	if opts == nil {
		opts = t.options
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Check for existing connection if reuse is enabled
	// Use write lock to prevent race conditions when multiple goroutines
	// try to create a connection simultaneously
	var needUnlock bool
	if opts.ReuseConnection {
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
		rawConn, err = t.connectTLS(ctx, addr, host, opts)
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
		MaxConcurrent: opts.MaxConcurrentStreams,
		WindowSize:    int32(opts.InitialWindowSize),
		Settings:      make(map[http2.SettingID]uint32),
		PeerSettings:  make(map[http2.SettingID]uint32),
		LastActivity:  time.Now(),
	}

	// Initialize HPACK encoder/decoder for this connection
	// Each connection needs its own HPACK context
	conn.EncoderBuf = &bytes.Buffer{}
	conn.Encoder = hpack.NewEncoder(conn.EncoderBuf)
	conn.Encoder.SetMaxDynamicTableSize(opts.HeaderTableSize)
	conn.Decoder = hpack.NewDecoder(opts.HeaderTableSize, nil)

	// Important: Release lock before blocking I/O but don't store connection yet
	if needUnlock {
		t.mu.Unlock()
		needUnlock = false // We'll re-acquire if needed
	}

	// Send initial settings (this can block waiting for ACK)
	if err := t.sendInitialSettings(conn, opts); err != nil {
		// Defensive nil check before closing (rawConn should not be nil here, but being extra safe)
		if rawConn != nil {
			rawConn.Close()
		}
		return nil, fmt.Errorf("failed to send settings: %w", err)
	}

	// Mark connection as ready
	conn.Ready = true

	// Now store the fully initialized connection
	if opts.ReuseConnection {
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
func (t *Transport) connectTLS(ctx context.Context, addr, serverName string, opts *Options) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	// Create TLS config with ALPN
	var tlsConfig *tls.Config

	// Use custom TLS config if provided
	if opts.TLSConfig != nil {
		// Clone to avoid modifying the original
		tlsConfig = opts.TLSConfig.Clone()

		// Handle NextProtos with respect for user's explicit configuration
		if len(tlsConfig.NextProtos) == 0 {
			// User didn't specify NextProtos, use defaults for HTTP/2
			tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		} else {
			// User explicitly set NextProtos - check if h2 is present
			hasH2 := false
			for _, proto := range tlsConfig.NextProtos {
				if proto == "h2" {
					hasH2 = true
					break
				}
			}

			// IMPORTANT: Only add h2 if user seems to want HTTP/2 support
			// If user explicitly set NextProtos without h2, they likely want to avoid HTTP/2
			// However, since they're using HTTP/2 transport, we need h2 for ALPN to succeed
			//
			// Best practice: Users should set opts.Protocol = "http/1.1" instead of
			// trying to control protocol via NextProtos when using go-rawhttp
			if !hasH2 {
				// Prepend h2 to the list for backward compatibility
				// NOTE: This maintains existing behavior but logs a warning would be better
				tlsConfig.NextProtos = append([]string{"h2"}, tlsConfig.NextProtos...)
			}
		}

		// Apply InsecureTLS flag (overrides TLSConfig setting)
		if opts.InsecureTLS {
			tlsConfig.InsecureSkipVerify = true
		}

		// Configure SNI (DEF-4: using shared helper function)
		transport.ConfigureSNI(tlsConfig, opts.SNI, opts.DisableSNI, serverName)
	} else {
		// Use default TLS config
		tlsConfig = &tls.Config{
			NextProtos:         []string{"h2", "http/1.1"},
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opts.InsecureTLS,
		}
		// Configure SNI (DEF-4: using shared helper function)
		transport.ConfigureSNI(tlsConfig, opts.SNI, opts.DisableSNI, serverName)
	}

	// Load client certificate for mutual TLS (mTLS) if provided
	clientCert, err := t.loadClientCertificate(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}
	if clientCert != nil {
		tlsConfig.Certificates = append(tlsConfig.Certificates, *clientCert)
	}

	// Apply SSL/TLS version control (v1.2.0+)
	// Priority: TLSConfig values > MinTLSVersion/MaxTLSVersion > defaults
	if opts.MinTLSVersion > 0 && tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = opts.MinTLSVersion
	}
	if opts.MaxTLSVersion > 0 && tlsConfig.MaxVersion == 0 {
		tlsConfig.MaxVersion = opts.MaxTLSVersion
	}

	// Apply cipher suites if specified (v1.2.0+)
	if len(opts.CipherSuites) > 0 && len(tlsConfig.CipherSuites) == 0 {
		tlsConfig.CipherSuites = opts.CipherSuites
	}

	// Apply renegotiation support (v1.2.0+)
	if opts.TLSRenegotiation != 0 {
		tlsConfig.Renegotiation = opts.TLSRenegotiation
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
func (t *Transport) sendInitialSettings(conn *Connection, opts *Options) error {
	// Send only the settings that Go's HTTP/2 sends (minimal set)
	settings := map[http2.SettingID]uint32{
		http2.SettingEnablePush:        boolToUint32(opts.EnableServerPush), // Always 0
		http2.SettingInitialWindowSize: opts.InitialWindowSize,              // 4MB
		http2.SettingMaxFrameSize:      opts.MaxFrameSize,                   // 16KB
		http2.SettingMaxHeaderListSize: opts.MaxHeaderListSize,              // 10MB
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
	if opts.InitialWindowSize > 65535 {
		increment := opts.InitialWindowSize - 65535
		if err := conn.Framer.WriteWindowUpdate(0, increment); err != nil {
			return fmt.Errorf("failed to write connection window update: %w", err)
		}
	}

	return nil
}

// waitForSettingsAck waits for SETTINGS ACK from server
func (t *Transport) waitForSettingsAck(conn *Connection) error {
	// Set deadline on the connection to prevent indefinite blocking (DEF-7)
	deadline := time.Now().Add(10 * time.Second)
	if err := conn.Conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer conn.Conn.SetReadDeadline(time.Time{}) // Clear deadline

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

// Close gracefully shuts down the HTTP/2 Transport by stopping background goroutines
// and closing all active connections. This method should be called when the
// Transport is no longer needed to prevent goroutine leaks.
func (t *Transport) Close() error {
	// Signal health checker goroutine to stop
	close(t.stopChan)

	// Wait for all goroutines to finish
	t.wg.Wait()

	// Close all active connections
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

// GetPoolStats returns current HTTP/2 connection pool statistics (DEF-5).
// This provides visibility into connection reuse, active streams, and pool health.
func (t *Transport) GetPoolStats() *ConnectionPoolStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &ConnectionPoolStats{
		ActiveConnections: len(t.connections),
		Connections:       make(map[string]ConnectionStats),
	}

	totalStreams := 0
	for addr, conn := range t.connections {
		conn.mu.RLock()
		activeStreams := 0
		for _, stream := range conn.Streams {
			if !stream.Closed {
				activeStreams++
			}
		}
		totalStreams += len(conn.Streams)

		stats.Connections[addr] = ConnectionStats{
			Address:       addr,
			StreamsActive: activeStreams,
			StreamsTotal:  len(conn.Streams),
			LastActivity:  conn.LastActivity,
			Ready:         conn.Ready,
		}
		conn.mu.RUnlock()
	}

	stats.TotalStreams = totalStreams
	return stats
}

// Helper functions

// loadClientCertificate loads client certificate for mTLS from HTTP/2 options.
// Reuses the same logic as HTTP/1.1 transport.
func (t *Transport) loadClientCertificate(opts *Options) (*tls.Certificate, error) {
	// Check if we have client certificate data
	hasPEM := len(opts.ClientCertPEM) > 0 && len(opts.ClientKeyPEM) > 0
	hasFile := opts.ClientCertFile != "" && opts.ClientKeyFile != ""

	if !hasPEM && !hasFile {
		// No client certificate configured
		return nil, nil
	}

	var certPEM, keyPEM []byte
	var err error

	if hasPEM {
		// Use provided PEM data directly
		certPEM = opts.ClientCertPEM
		keyPEM = opts.ClientKeyPEM
	} else if hasFile {
		// Load from files
		certPEM, err = os.ReadFile(opts.ClientCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client certificate file %s: %w", opts.ClientCertFile, err)
		}

		keyPEM, err = os.ReadFile(opts.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key file %s: %w", opts.ClientKeyFile, err)
		}
	}

	// Parse certificate and key
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client certificate/key: %w", err)
	}

	return &cert, nil
}

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
