// Package transport provides the low-level HTTP transport implementation.
package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
	"golang.org/x/net/proxy"
)

// Config holds transport configuration.
type Config struct {
	Scheme       string
	Host         string
	Port         int
	ConnectIP    string
	SNI          string
	DisableSNI   bool
	InsecureTLS  bool
	ConnTimeout  time.Duration
	DNSTimeout   time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Connection pooling
	ReuseConnection bool

	// Proxy configuration
	ProxyURL string

	// Custom CA certificates
	CustomCACerts [][]byte

	// TLSConfig allows direct passthrough of crypto/tls.Config for full TLS control.
	// If nil, default configuration will be used based on other options.
	TLSConfig *tls.Config
}

// ConnectionMetadata holds metadata about the established connection
type ConnectionMetadata struct {
	ConnectedIP        string
	ConnectedPort      int
	NegotiatedProtocol string
	TLSVersion         string
	TLSCipherSuite     string
	TLSServerName      string
	ConnectionReused   bool
}

// pooledConnection wraps a connection with metadata
type pooledConnection struct {
	conn      net.Conn
	metadata  ConnectionMetadata
	lastUsed  time.Time
	inUse     bool
	keepAlive bool
}

// Transport handles the network connection and protocol negotiation.
type Transport struct {
	resolver   *net.Resolver
	connPool   sync.Map // map[string]*pooledConnection (key: "host:port")
	poolMutex  sync.Mutex
	maxIdleTime time.Duration // Maximum idle time for pooled connections
}

// New creates a new Transport instance.
func New() *Transport {
	t := &Transport{
		resolver:    net.DefaultResolver,
		maxIdleTime: 90 * time.Second, // Default 90 seconds idle timeout
	}
	// Start connection pool cleanup goroutine
	go t.cleanupIdleConnections()
	return t
}

// NewWithResolver creates a new Transport with a custom resolver.
func NewWithResolver(resolver *net.Resolver) *Transport {
	t := &Transport{
		resolver:    resolver,
		maxIdleTime: 90 * time.Second,
	}
	go t.cleanupIdleConnections()
	return t
}

// Connect establishes a connection based on the configuration.
// Returns the connection and metadata about the connection.
func (t *Transport) Connect(ctx context.Context, config Config, timer *timing.Timer) (net.Conn, *ConnectionMetadata, error) {
	if err := t.validateConfig(config); err != nil {
		return nil, nil, err
	}

	metadata := &ConnectionMetadata{}
	poolKey := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Try to get connection from pool if ReuseConnection is enabled
	if config.ReuseConnection {
		if conn, meta, ok := t.getFromPool(poolKey); ok {
			metadata = meta
			metadata.ConnectionReused = true
			return conn, metadata, nil
		}
	}

	// Setup timeouts
	connTimeout := config.ConnTimeout
	if connTimeout <= 0 {
		connTimeout = 10 * time.Second
	}

	// Resolve DNS if needed
	dialAddr, _, err := t.resolveAddress(ctx, config, timer)
	if err != nil {
		return nil, nil, err
	}

	// Store resolved IP in metadata
	host, portStr, _ := net.SplitHostPort(dialAddr)
	metadata.ConnectedIP = host
	if port, err := strconv.Atoi(portStr); err == nil {
		metadata.ConnectedPort = port
	}

	var conn net.Conn

	// Connect through proxy if configured
	if config.ProxyURL != "" {
		conn, err = t.connectViaProxy(ctx, config, dialAddr, connTimeout, timer)
		if err != nil {
			return nil, nil, errors.NewConnectionError(config.Host, config.Port, err)
		}
	} else {
		// Direct TCP connection
		conn, err = t.connectTCP(ctx, dialAddr, connTimeout, timer)
		if err != nil {
			return nil, nil, errors.NewConnectionError(config.Host, config.Port, err)
		}
	}

	// Upgrade to TLS if needed
	if strings.EqualFold(config.Scheme, "https") {
		conn, err = t.upgradeTLS(ctx, conn, config, timer, metadata)
		if err != nil {
			conn.Close()
			return nil, nil, errors.NewTLSError(config.Host, config.Port, err)
		}
	} else {
		metadata.NegotiatedProtocol = "HTTP/1.1"
	}

	// Add to pool if ReuseConnection is enabled
	if config.ReuseConnection {
		t.addToPool(poolKey, conn, metadata)
	}

	return conn, metadata, nil
}

func (t *Transport) validateConfig(config Config) error {
	if config.Host == "" {
		return errors.NewValidationError("host cannot be empty")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return errors.NewValidationError("port must be between 1 and 65535")
	}
	if config.Scheme != "http" && config.Scheme != "https" {
		return errors.NewValidationError("scheme must be http or https")
	}
	return nil
}

func (t *Transport) resolveAddress(ctx context.Context, config Config, timer *timing.Timer) (dialAddr string, resolvedIP string, err error) {
	// If ConnectIP is specified, use it directly
	if config.ConnectIP != "" {
		dialAddr = net.JoinHostPort(config.ConnectIP, strconv.Itoa(config.Port))
		return dialAddr, config.ConnectIP, nil
	}

	// Perform DNS resolution with separate timeout
	timer.StartDNS()
	defer timer.EndDNS()

	dnsTimeout := config.DNSTimeout
	if dnsTimeout <= 0 {
		dnsTimeout = config.ConnTimeout // Fallback to connection timeout
	}
	if dnsTimeout <= 0 {
		dnsTimeout = 5 * time.Second // Default DNS timeout
	}

	ctxLookup, cancel := context.WithTimeout(ctx, dnsTimeout)
	defer cancel()

	addrs, err := t.resolver.LookupIPAddr(ctxLookup, config.Host)
	if err != nil {
		return "", "", errors.NewDNSError(config.Host, err)
	}

	if len(addrs) == 0 {
		return "", "", errors.NewDNSError(config.Host, errors.NewValidationError("no IP addresses found"))
	}

	// Use the first address
	ip := addrs[0].IP.String()
	dialAddr = net.JoinHostPort(ip, strconv.Itoa(config.Port))
	return dialAddr, ip, nil
}

func (t *Transport) connectTCP(ctx context.Context, dialAddr string, timeout time.Duration, timer *timing.Timer) (net.Conn, error) {
	timer.StartTCP()
	defer timer.EndTCP()

	dialer := &net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, "tcp", dialAddr)
}

func (t *Transport) upgradeTLS(ctx context.Context, conn net.Conn, config Config, timer *timing.Timer, metadata *ConnectionMetadata) (net.Conn, error) {
	timer.StartTLS()
	defer timer.EndTLS()

	// Set TLS handshake timeout (default to connection timeout or 10s)
	handshakeTimeout := config.ConnTimeout
	if handshakeTimeout <= 0 {
		handshakeTimeout = 10 * time.Second
	}

	// Create a context with TLS-specific timeout
	tlsCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	var tlsConfig *tls.Config

	// Use provided TLSConfig if available (direct passthrough)
	if config.TLSConfig != nil {
		// Clone the provided config to avoid modifying the original
		tlsConfig = config.TLSConfig.Clone()
	} else {
		// Create default TLS configuration
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: config.InsecureTLS,
			NextProtos:         []string{"http/1.1"},
		}

		// Add custom CA certificates if provided
		if len(config.CustomCACerts) > 0 {
			rootCAs := x509.NewCertPool()
			for _, caCert := range config.CustomCACerts {
				if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
					return nil, errors.NewTLSError(config.Host, config.Port,
						errors.NewValidationError("failed to parse CA certificate"))
				}
			}
			tlsConfig.RootCAs = rootCAs
		}

		// Configure SNI
		if !config.DisableSNI {
			serverName := config.SNI
			if serverName == "" {
				serverName = config.Host
			}
			tlsConfig.ServerName = serverName
		}
	}

	// Store SNI in metadata
	if tlsConfig.ServerName != "" {
		metadata.TLSServerName = tlsConfig.ServerName
	} else if !config.DisableSNI {
		metadata.TLSServerName = config.Host
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(tlsCtx); err != nil {
		return nil, err
	}

	// Fill TLS metadata
	state := tlsConn.ConnectionState()
	metadata.TLSVersion = t.tlsVersionString(state.Version)
	metadata.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)
	metadata.NegotiatedProtocol = state.NegotiatedProtocol
	if metadata.NegotiatedProtocol == "" {
		metadata.NegotiatedProtocol = "HTTP/1.1"
	}

	return tlsConn, nil
}

// tlsVersionString converts TLS version constant to string
func (t *Transport) tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown TLS version: 0x%04X", version)
	}
}

// getFromPool retrieves a connection from the pool
func (t *Transport) getFromPool(key string) (net.Conn, *ConnectionMetadata, bool) {
	t.poolMutex.Lock()
	defer t.poolMutex.Unlock()

	val, ok := t.connPool.Load(key)
	if !ok {
		return nil, nil, false
	}

	pooled := val.(*pooledConnection)
	if pooled.inUse {
		return nil, nil, false
	}

	// Check if connection is still alive
	if !t.isConnectionAlive(pooled.conn) {
		t.connPool.Delete(key)
		pooled.conn.Close()
		return nil, nil, false
	}

	pooled.inUse = true
	pooled.lastUsed = time.Now()

	// Copy metadata
	metaCopy := pooled.metadata
	return pooled.conn, &metaCopy, true
}

// addToPool adds a connection to the pool
func (t *Transport) addToPool(key string, conn net.Conn, metadata *ConnectionMetadata) {
	t.poolMutex.Lock()
	defer t.poolMutex.Unlock()

	pooled := &pooledConnection{
		conn:      conn,
		metadata:  *metadata,
		lastUsed:  time.Now(),
		inUse:     true,
		keepAlive: true,
	}

	t.connPool.Store(key, pooled)
}

// ReleaseConnection marks a connection as available for reuse
func (t *Transport) ReleaseConnection(host string, port int, conn net.Conn) {
	key := fmt.Sprintf("%s:%d", host, port)

	t.poolMutex.Lock()
	defer t.poolMutex.Unlock()

	val, ok := t.connPool.Load(key)
	if !ok {
		return
	}

	pooled := val.(*pooledConnection)
	if pooled.conn == conn {
		pooled.inUse = false
		pooled.lastUsed = time.Now()
	}
}

// CloseConnection closes and removes a connection from the pool
func (t *Transport) CloseConnection(host string, port int, conn net.Conn) {
	key := fmt.Sprintf("%s:%d", host, port)

	t.poolMutex.Lock()
	defer t.poolMutex.Unlock()

	val, ok := t.connPool.Load(key)
	if ok {
		pooled := val.(*pooledConnection)
		if pooled.conn == conn {
			t.connPool.Delete(key)
			pooled.conn.Close()
		}
	} else {
		// Connection not in pool, just close it
		conn.Close()
	}
}

// isConnectionAlive checks if a connection is still alive
func (t *Transport) isConnectionAlive(conn net.Conn) bool {
	// Set a very short read deadline to check if connection is alive
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.SetReadDeadline(time.Time{})

	one := make([]byte, 1)
	_, err := conn.Read(one)

	// If we get EOF or timeout, connection might be closed or idle (good)
	// If we get data, we need to handle it (shouldn't happen in HTTP keep-alive)
	if err == nil {
		// We read data, connection is alive but has data - this is unexpected
		// We can't put the byte back, so connection is compromised
		return false
	}

	// Check for timeout (expected for idle connection)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Any other error means connection is dead
	return false
}

// cleanupIdleConnections periodically removes idle connections from pool
func (t *Transport) cleanupIdleConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		t.poolMutex.Lock()
		t.connPool.Range(func(key, value interface{}) bool {
			pooled := value.(*pooledConnection)

			// Remove connections that have been idle too long
			if !pooled.inUse && time.Since(pooled.lastUsed) > t.maxIdleTime {
				t.connPool.Delete(key)
				pooled.conn.Close()
			}

			return true
		})
		t.poolMutex.Unlock()
	}
}

// connectViaProxy connects to the target through an upstream proxy
func (t *Transport) connectViaProxy(ctx context.Context, config Config, targetAddr string, timeout time.Duration, timer *timing.Timer) (net.Conn, error) {
	proxyURL, err := url.Parse(config.ProxyURL)
	if err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("invalid proxy URL: %v", err))
	}

	timer.StartTCP()
	defer timer.EndTCP()

	switch proxyURL.Scheme {
	case "http", "https":
		return t.connectViaHTTPProxy(ctx, proxyURL, config, targetAddr, timeout)
	case "socks5":
		return t.connectViaSOCKS5Proxy(ctx, proxyURL, targetAddr, timeout)
	default:
		return nil, errors.NewValidationError(fmt.Sprintf("unsupported proxy scheme: %s", proxyURL.Scheme))
	}
}

// connectViaHTTPProxy connects through an HTTP CONNECT proxy
func (t *Transport) connectViaHTTPProxy(ctx context.Context, proxyURL *url.URL, config Config, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Connect to proxy
	dialer := &net.Dialer{Timeout: timeout}
	proxyAddr := proxyURL.Host
	if !strings.Contains(proxyAddr, ":") {
		if proxyURL.Scheme == "https" {
			proxyAddr = net.JoinHostPort(proxyAddr, "443")
		} else {
			proxyAddr = net.JoinHostPort(proxyAddr, "8080")
		}
	}

	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// If proxy is HTTPS, upgrade connection to TLS
	if proxyURL.Scheme == "https" {
		tlsConfig := &tls.Config{
			ServerName:         proxyURL.Hostname(),
			InsecureSkipVerify: config.InsecureTLS,
		}
		conn = tls.Client(conn, tlsConfig)
	}

	// Send CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, config.Host)

	// Add proxy authentication if credentials provided
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}

	connectReq += "\r\n"

	if _, err := conn.Write([]byte(connectReq)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read CONNECT response
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Check if CONNECT succeeded (HTTP/1.x 200)
	if !strings.Contains(statusLine, " 200") {
		conn.Close()
		return nil, errors.NewConnectionError(config.Host, config.Port,
			fmt.Errorf("proxy CONNECT failed: %s", strings.TrimSpace(statusLine)))
	}

	// Read and discard remaining headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return conn, nil
}

// connectViaSOCKS5Proxy connects through a SOCKS5 proxy
func (t *Transport) connectViaSOCKS5Proxy(ctx context.Context, proxyURL *url.URL, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Parse proxy address
	proxyAddr := proxyURL.Host
	if !strings.Contains(proxyAddr, ":") {
		proxyAddr = net.JoinHostPort(proxyAddr, "1080")
	}

	// Create SOCKS5 dialer
	var auth *proxy.Auth
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{Timeout: timeout})
	if err != nil {
		return nil, err
	}

	return dialer.Dial("tcp", targetAddr)
}
