// Package transport provides the low-level HTTP transport implementation.
package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WhileEndless/go-rawhttp/v2/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/v2/pkg/timing"
	netproxy "golang.org/x/net/proxy"
)

// ProxyConfig provides detailed configuration for upstream proxy connections.
// This is a transport-layer copy of client.ProxyConfig to avoid circular dependencies.
type ProxyConfig struct {
	Type               string
	Host               string
	Port               int
	Username           string
	Password           string
	ConnTimeout        time.Duration
	ProxyHeaders       map[string]string
	TLSConfig          *tls.Config
	ResolveDNSViaProxy bool
}

// Config holds transport configuration.
type Config struct {
	Scheme     string
	Host       string
	Port       int
	ConnectIP  string // Optional: specific IP to connect to (bypasses DNS)

	// TLS/SNI configuration
	// SNI specifies custom Server Name Indication for TLS handshake.
	// Priority: TLSConfig.ServerName > SNI > Host (if DisableSNI is false)
	SNI string

	// DisableSNI completely disables SNI extension in TLS handshake.
	// Cannot be used together with SNI option (validation error).
	DisableSNI bool

	// InsecureTLS skips TLS certificate verification (for testing/development).
	// IMPORTANT (DEF-13): This flag ALWAYS overrides TLSConfig.InsecureSkipVerify,
	// even when custom TLSConfig is provided. This is intentional to support proxy
	// MITM scenarios where you need custom TLS settings AND disabled verification.
	// Example: InsecureTLS=true + custom TLSConfig → verification is DISABLED.
	InsecureTLS bool

	// Timeouts
	ConnTimeout  time.Duration
	DNSTimeout   time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Connection pooling
	ReuseConnection bool

	// Proxy configuration (v2.0.0+)
	// Holds the proxy configuration for upstream proxy support.
	// This is passed from client.Options.Proxy
	Proxy *ProxyConfig

	// Custom CA certificates (PEM format)
	CustomCACerts [][]byte

	// Client certificate for mutual TLS (mTLS authentication)
	ClientCertPEM  []byte // Client certificate in PEM format
	ClientKeyPEM   []byte // Client private key in PEM format
	ClientCertFile string // Path to client certificate file
	ClientKeyFile  string // Path to client private key file

	// TLSConfig allows direct passthrough of crypto/tls.Config for full TLS control.
	// If nil, default configuration will be used based on other options.
	// Note: InsecureTLS flag will override InsecureSkipVerify if set to true.
	TLSConfig *tls.Config

	// SSL/TLS Protocol Version Control (v1.2.0+)
	MinTLSVersion    uint16                   // Minimum SSL/TLS version
	MaxTLSVersion    uint16                   // Maximum SSL/TLS version
	TLSRenegotiation tls.RenegotiationSupport // TLS renegotiation support
	CipherSuites     []uint16                 // Allowed cipher suites
}

// ConnectionMetadata holds metadata about the established connection
type ConnectionMetadata struct {
	// Basic connection info
	ConnectedIP        string
	ConnectedPort      int
	NegotiatedProtocol string
	ConnectionReused   bool

	// Socket-level information
	LocalAddr    string // Local socket address
	RemoteAddr   string // Remote socket address
	ConnectionID uint64 // Unique identifier for this connection

	// TLS information
	TLSVersion     string
	TLSCipherSuite string
	TLSServerName  string

	// Enhanced TLS metadata
	TLSSessionID string // TLS session ID (hex-encoded)
	TLSResumed   bool   // Whether TLS session was resumed

	// Proxy metadata (v2.0.0+)
	ProxyUsed bool   // Whether request went through proxy
	ProxyType string // Proxy type: "http", "https", "socks4", "socks5"
	ProxyAddr string // Proxy address: "proxy.com:8080"

	// Connection pooling (v2.0.3+)
	PoolKey string // Pool key used for this connection (includes proxy info)
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
	resolver          *net.Resolver
	connPool          sync.Map      // map[string]*pooledConnection (key: "host:port")
	poolMutex         sync.Mutex
	maxIdleTime       time.Duration // Maximum idle time for pooled connections
	connectionIDCounter uint64      // Atomic counter for unique connection IDs

	// Pool statistics (atomic counters)
	statsConnectionsReused uint64 // Lifetime count of reused connections

	// Lifecycle management
	stopChan chan struct{} // Channel to signal cleanup goroutine to stop
	wg       sync.WaitGroup // WaitGroup to track running goroutines
}

// PoolStats provides read-only statistics about the connection pool.
type PoolStats struct {
	ActiveConns int // Currently in use (checked out)
	IdleConns   int // Idle in pool (available)
	TotalReused int // Lifetime reuse count
}

// New creates a new Transport instance.
func New() *Transport {
	t := &Transport{
		resolver:    net.DefaultResolver,
		maxIdleTime: 90 * time.Second, // Default 90 seconds idle timeout
		stopChan:    make(chan struct{}),
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
		stopChan:    make(chan struct{}),
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

	// Create a connection pool key that includes proxy information if present
	// This ensures different proxies use different pooled connections
	var poolKey string
	if config.Proxy != nil {
		proxyPort := config.Proxy.Port
		if proxyPort == 0 {
			// Apply default port
			switch config.Proxy.Type {
			case "http":
				proxyPort = 8080
			case "https":
				proxyPort = 443
			case "socks4", "socks5":
				proxyPort = 1080
			}
		}
		// Format: "proxy_type:proxy_host:proxy_port->target_host:target_port"
		poolKey = fmt.Sprintf("%s:%s:%d->%s:%d", config.Proxy.Type, config.Proxy.Host, proxyPort, config.Host, config.Port)
	} else {
		// Direct connection: just use target address
		poolKey = fmt.Sprintf("%s:%d", config.Host, config.Port)
	}

	// Try to get connection from pool if ReuseConnection is enabled
	if config.ReuseConnection {
		if conn, meta, ok := t.getFromPool(poolKey); ok {
			metadata = meta
			metadata.ConnectionReused = true
			metadata.PoolKey = poolKey // Store pool key for release/close
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
	if config.Proxy != nil {
		conn, metadata, err = t.connectViaProxy(ctx, config, dialAddr, connTimeout, timer, metadata)
		if err != nil {
			return nil, nil, err // Error already wrapped by connectViaProxy
		}
	} else {
		// Direct TCP connection
		conn, err = t.connectTCP(ctx, dialAddr, connTimeout, timer)
		if err != nil {
			return nil, nil, errors.NewConnectionError(config.Host, config.Port, err)
		}
	}

	// Populate socket-level metadata
	if conn.LocalAddr() != nil {
		metadata.LocalAddr = conn.LocalAddr().String()
	}
	if conn.RemoteAddr() != nil {
		metadata.RemoteAddr = conn.RemoteAddr().String()
	}
	// Generate unique connection ID
	metadata.ConnectionID = atomic.AddUint64(&t.connectionIDCounter, 1)

	// Upgrade to TLS if needed
	if strings.EqualFold(config.Scheme, "https") {
		conn, err = t.upgradeTLS(ctx, conn, config, timer, metadata)
		if err != nil {
			// conn may be nil if upgradeTLS failed, add defensive check
			if conn != nil {
				conn.Close()
			}
			return nil, nil, errors.NewTLSError(config.Host, config.Port, err)
		}
	} else {
		metadata.NegotiatedProtocol = "HTTP/1.1"
	}

	// Store pool key in metadata for release/close operations
	metadata.PoolKey = poolKey

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

	// Validate conflicting options (DEF-1)
	if config.DisableSNI && config.SNI != "" {
		return errors.NewValidationError("cannot set both DisableSNI=true and SNI (conflicting options)")
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

		// IMPORTANT: Also respect InsecureTLS flag as override
		// This allows users to set InsecureTLS=true even when providing custom TLSConfig
		// This is critical for proxy scenarios where certificate validation must be disabled
		if config.InsecureTLS {
			tlsConfig.InsecureSkipVerify = true
		}

		// BUG FIX (v2.0.3): Force HTTP/1.1 ALPN for HTTP/1.1 transport
		// HTTP/1.1 transport should NEVER negotiate HTTP/2, regardless of user's TLSConfig
		// If user wants HTTP/2, they should use Protocol="http/2" which routes to http2.Transport
		// This prevents servers from upgrading to HTTP/2 when user explicitly wants HTTP/1.1
		tlsConfig.NextProtos = []string{"http/1.1"}
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
			for i, caCert := range config.CustomCACerts {
				if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
					return nil, errors.NewTLSError(config.Host, config.Port,
						errors.NewValidationError(fmt.Sprintf("failed to parse CA certificate at index %d", i)))
				}
			}
			tlsConfig.RootCAs = rootCAs
		}

		// Configure SNI (DEF-4: using helper function)
		ConfigureSNI(tlsConfig, config.SNI, config.DisableSNI, config.Host)
	}

	// Apply SSL/TLS version control (v1.2.0+)
	// Priority: TLSConfig values > MinTLSVersion/MaxTLSVersion > defaults
	if config.MinTLSVersion > 0 && tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = config.MinTLSVersion
	}
	if config.MaxTLSVersion > 0 && tlsConfig.MaxVersion == 0 {
		tlsConfig.MaxVersion = config.MaxTLSVersion
	}

	// Apply cipher suites if specified (v1.2.0+)
	if len(config.CipherSuites) > 0 && len(tlsConfig.CipherSuites) == 0 {
		tlsConfig.CipherSuites = config.CipherSuites
	}

	// Apply renegotiation support (v1.2.0+)
	// Default is RenegotiateNever for security
	if config.TLSRenegotiation != 0 {
		tlsConfig.Renegotiation = config.TLSRenegotiation
	}

	// Load client certificate for mutual TLS (mTLS) if provided
	clientCert, err := t.loadClientCertificate(config)
	if err != nil {
		return nil, errors.NewTLSError(config.Host, config.Port, err)
	}
	if clientCert != nil {
		tlsConfig.Certificates = append(tlsConfig.Certificates, *clientCert)
	}

	// Store SNI in metadata
	if tlsConfig.ServerName != "" {
		metadata.TLSServerName = tlsConfig.ServerName
	} else if !config.DisableSNI {
		metadata.TLSServerName = config.Host
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(tlsCtx); err != nil {
		conn.Close() // Close original TCP connection to prevent resource leak
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

	// Enhanced TLS metadata
	metadata.TLSResumed = state.DidResume

	// BUG-10: TLS Session ID handling
	// IMPORTANT: TLSSessionID is unreliable and should not be used for session tracking.
	// Reasons:
	// 1. TLS 1.3 uses session tickets instead of session IDs (not exposed in Go's crypto/tls)
	// 2. Go's crypto/tls does not expose the actual session ID from TLS 1.2 handshake
	// 3. state.TLSUnique is a channel binding value (RFC 5929), NOT a session ID
	//
	// RECOMMENDATION: Use TLSResumed field instead to detect session resumption.
	// This field is reliable across all TLS versions.
	//
	// We set TLSSessionID to TLSUnique for backward compatibility, but it should
	// only be used for debugging, not for security-critical session tracking.
	if len(state.TLSUnique) > 0 {
		metadata.TLSSessionID = hex.EncodeToString(state.TLSUnique)
	} else {
		// TLS 1.3 or session resumption - no TLSUnique available
		metadata.TLSSessionID = ""
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

	// Skip liveness check for recently used connections (v2.0.3+)
	// Recent connections are likely still alive, and liveness check has false positives
	// with HTTP/2 (control frames) and TLS (buffered data)
	recentlyUsed := time.Since(pooled.lastUsed) < 5*time.Second

	if !recentlyUsed {
		// Check if connection is still alive (only for stale connections)
		if !t.isConnectionAlive(pooled.conn) {
			t.connPool.Delete(key)
			pooled.conn.Close()
			return nil, nil, false
		}
	}

	pooled.inUse = true
	pooled.lastUsed = time.Now()

	// Increment reuse counter
	atomic.AddUint64(&t.statsConnectionsReused, 1)

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
	t.ReleaseConnectionWithMetadata(host, port, conn, nil)
}

// ReleaseConnectionWithMetadata marks a connection as available for reuse using metadata pool key
func (t *Transport) ReleaseConnectionWithMetadata(host string, port int, conn net.Conn, metadata *ConnectionMetadata) {
	// Use pool key from metadata if available (v2.0.3+), otherwise fallback to old format
	var key string
	if metadata != nil && metadata.PoolKey != "" {
		key = metadata.PoolKey
	} else {
		key = fmt.Sprintf("%s:%d", host, port)
	}

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
	t.CloseConnectionWithMetadata(host, port, conn, nil)
}

// CloseConnectionWithMetadata closes and removes a connection from the pool using metadata pool key
func (t *Transport) CloseConnectionWithMetadata(host string, port int, conn net.Conn, metadata *ConnectionMetadata) {
	// Use pool key from metadata if available (v2.0.3+), otherwise fallback to old format
	var key string
	if metadata != nil && metadata.PoolKey != "" {
		key = metadata.PoolKey
	} else {
		key = fmt.Sprintf("%s:%d", host, port)
	}

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
// Note: This is a best-effort check. It may return false positives (marking
// good connections as dead) if server sends unexpected data like late frames.
// This is acceptable as it only causes unnecessary connection recreation.
func (t *Transport) isConnectionAlive(conn net.Conn) bool {
	// Set a very short read deadline to check if connection is alive
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	defer conn.SetReadDeadline(time.Time{})

	one := make([]byte, 1)
	_, err := conn.Read(one)

	// Check for timeout first (expected for idle connection)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// If we read data without error, connection is alive
	// Note: This shouldn't happen in HTTP/1.1 keep-alive, but it's not necessarily
	// an error. For HTTP/2, servers might send frames. We conservatively mark as dead
	// to avoid dealing with buffering the data, but this is safe (just inefficient).
	if err == nil {
		return false // Conservative: mark as dead to recreate connection
	}

	// Any other error (EOF, etc.) means connection is dead
	return false
}

// PoolStats returns current connection pool statistics.
// This is a read-only snapshot of the pool state.
func (t *Transport) PoolStats() PoolStats {
	var active, idle int

	t.poolMutex.Lock()
	t.connPool.Range(func(key, value interface{}) bool {
		pooled := value.(*pooledConnection)
		if pooled.inUse {
			active++
		} else {
			idle++
		}
		return true
	})
	t.poolMutex.Unlock()

	return PoolStats{
		ActiveConns: active,
		IdleConns:   idle,
		TotalReused: int(atomic.LoadUint64(&t.statsConnectionsReused)),
	}
}

// cleanupIdleConnections periodically removes idle connections from pool
func (t *Transport) cleanupIdleConnections() {
	t.wg.Add(1)
	defer t.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
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
		case <-t.stopChan:
			// Cleanup and exit
			return
		}
	}
}

// connectViaProxy connects to the target through an upstream proxy.
// Returns connection and updates metadata with proxy information.
func (t *Transport) connectViaProxy(ctx context.Context, config Config, targetAddr string, timeout time.Duration, timer *timing.Timer, metadata *ConnectionMetadata) (net.Conn, *ConnectionMetadata, error) {
	proxy := config.Proxy
	if proxy == nil {
		return nil, nil, errors.NewValidationError("proxy configuration is nil")
	}

	// Validate proxy config
	if proxy.Type == "" {
		return nil, nil, errors.NewValidationError("proxy type cannot be empty")
	}
	if proxy.Host == "" {
		return nil, nil, errors.NewValidationError("proxy host cannot be empty")
	}

	// Apply default ports if not specified
	proxyPort := proxy.Port
	if proxyPort == 0 {
		switch proxy.Type {
		case "http":
			proxyPort = 8080
		case "https":
			proxyPort = 443
		case "socks4", "socks5":
			proxyPort = 1080
		default:
			return nil, nil, errors.NewValidationError(fmt.Sprintf("unsupported proxy type: %s", proxy.Type))
		}
	}

	// Use proxy-specific timeout if configured
	proxyTimeout := proxy.ConnTimeout
	if proxyTimeout <= 0 {
		proxyTimeout = timeout
	}

	// Update metadata
	proxyAddr := fmt.Sprintf("%s:%d", proxy.Host, proxyPort)
	metadata.ProxyUsed = true
	metadata.ProxyType = proxy.Type
	metadata.ProxyAddr = proxyAddr

	timer.StartTCP()
	defer timer.EndTCP()

	var conn net.Conn
	var err error

	// Route to appropriate proxy handler
	switch proxy.Type {
	case "http", "https":
		conn, err = t.connectViaHTTPProxy(ctx, proxy, proxyAddr, config, targetAddr, proxyTimeout)
	case "socks4":
		conn, err = t.connectViaSOCKS4Proxy(ctx, proxy, proxyAddr, targetAddr, proxyTimeout)
	case "socks5":
		conn, err = t.connectViaSOCKS5Proxy(ctx, proxy, proxyAddr, targetAddr, proxyTimeout)
	default:
		return nil, nil, errors.NewValidationError(fmt.Sprintf("unsupported proxy type: %s", proxy.Type))
	}

	if err != nil {
		// Wrap error as ProxyError
		return nil, nil, errors.NewProxyError(proxy.Type, proxyAddr, "connect", err)
	}

	// Update metadata with actual connected address (proxy, not target)
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		if tcpAddr, ok := remoteAddr.(*net.TCPAddr); ok {
			metadata.ConnectedIP = tcpAddr.IP.String()
			metadata.ConnectedPort = tcpAddr.Port
		}
	}

	return conn, metadata, nil
}

// connectViaHTTPProxy connects through an HTTP/HTTPS CONNECT proxy with custom headers support.
//
// HTTP CONNECT Protocol Flow:
//  1. Connect to proxy server (TCP or TLS if HTTPS proxy)
//  2. Send CONNECT request: "CONNECT target.host:port HTTP/1.1"
//  3. Receive response: "HTTP/1.1 200 Connection Established"
//  4. Connection tunneled - can now send target traffic (HTTP or HTTPS)
//
// Note: The proxy type (http vs https) determines how we connect TO the proxy.
// The target scheme (http vs https) determines traffic THROUGH the tunnel.
// Example: http://proxy:8080 can proxy HTTPS requests - the tunnel is cleartext
// but the target traffic inside is TLS-encrypted.
func (t *Transport) connectViaHTTPProxy(ctx context.Context, proxy *ProxyConfig, proxyAddr string, config Config, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Connect to proxy server
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	// If proxy type is HTTPS, upgrade connection to TLS
	if proxy.Type == "https" {
		tlsConfig := proxy.TLSConfig
		if tlsConfig == nil {
			// Default TLS config for HTTPS proxy
			tlsConfig = &tls.Config{
				ServerName:         proxy.Host,
				InsecureSkipVerify: config.InsecureTLS,
			}
		} else {
			// Use custom TLS config but respect InsecureTLS override
			tlsConfig = tlsConfig.Clone()
			if config.InsecureTLS {
				tlsConfig.InsecureSkipVerify = true
			}
			if tlsConfig.ServerName == "" {
				tlsConfig.ServerName = proxy.Host
			}
		}

		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS handshake to proxy failed: %w", err)
		}
		conn = tlsConn
	}

	// Build CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nConnection: keep-alive\r\n", targetAddr, config.Host)

	// Add custom headers if provided
	for key, value := range proxy.ProxyHeaders {
		connectReq += fmt.Sprintf("%s: %s\r\n", key, value)
	}

	// Add proxy authentication if credentials provided
	if proxy.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(proxy.Username + ":" + proxy.Password))
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}

	connectReq += "\r\n"

	// Send CONNECT request
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %w", err)
	}

	// Read CONNECT response
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}

	// Check if CONNECT succeeded (HTTP/1.x 200)
	if !strings.Contains(statusLine, " 200") {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", strings.TrimSpace(statusLine))
	}

	// Read and discard remaining headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to read CONNECT response headers: %w", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return conn, nil
}

// connectViaSOCKS4Proxy connects through a SOCKS4 proxy.
//
// SOCKS4 Protocol (RFC 1928):
//   - IPv4 only (no IPv6 support)
//   - Simple authentication via user ID
//   - DNS resolution must be done locally
//
// Request format: [VER(1)][CMD(1)][PORT(2)][IP(4)][USERID][NULL]
// Response format: [VER(1)][STATUS(1)][PORT(2)][IP(4)]
//
// Status codes:
//   - 0x5A: Request granted
//   - 0x5B: Request rejected or failed
//   - 0x5C: Request failed (identd not running)
//   - 0x5D: Request failed (identd auth failed)
func (t *Transport) connectViaSOCKS4Proxy(ctx context.Context, proxy *ProxyConfig, proxyAddr string, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Parse target address
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid target address: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// SOCKS4 requires IPv4 address - resolve hostname
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}

	var targetIP net.IP
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			targetIP = ip4
			break
		}
	}
	if targetIP == nil {
		return nil, fmt.Errorf("no IPv4 address found for %s (SOCKS4 requires IPv4)", host)
	}

	// Connect to SOCKS4 proxy
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SOCKS4 proxy: %w", err)
	}

	// Build SOCKS4 request
	// Format: [VER(0x04)][CMD(0x01=CONNECT)][PORT(2 bytes)][IP(4 bytes)][USERID][NULL]
	req := []byte{
		0x04, // VER: SOCKS version 4
		0x01, // CMD: CONNECT command
		byte(port >> 8),   // PORT high byte
		byte(port & 0xFF), // PORT low byte
	}
	req = append(req, targetIP...) // IP address (4 bytes)

	// Add user ID if provided
	if proxy.Username != "" {
		req = append(req, []byte(proxy.Username)...)
	}
	req = append(req, 0x00) // NULL terminator

	// Send SOCKS4 request
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send SOCKS4 request: %w", err)
	}

	// Read SOCKS4 response (8 bytes)
	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read SOCKS4 response: %w", err)
	}

	// Check response status
	status := resp[1]
	switch status {
	case 0x5A:
		// Request granted - success!
		return conn, nil
	case 0x5B:
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 request rejected or failed")
	case 0x5C:
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 request failed: identd not running on client")
	case 0x5D:
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 request failed: identd could not confirm user ID")
	default:
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 unknown status code: 0x%02X", status)
	}
}

// connectViaSOCKS5Proxy connects through a SOCKS5 proxy using golang.org/x/net/proxy.
//
// SOCKS5 Protocol (RFC 1928):
//   - Supports IPv4 and IPv6
//   - Optional authentication (username/password)
//   - Can resolve DNS via proxy or locally
//
// We use the proven golang.org/x/net/proxy library for SOCKS5 instead of
// manual implementation for reliability and RFC compliance.
func (t *Transport) connectViaSOCKS5Proxy(ctx context.Context, proxy *ProxyConfig, proxyAddr string, targetAddr string, timeout time.Duration) (net.Conn, error) {
	// Create SOCKS5 authentication if credentials provided
	var auth *netproxy.Auth
	if proxy.Username != "" {
		auth = &netproxy.Auth{
			User:     proxy.Username,
			Password: proxy.Password,
		}
	}

	// Create SOCKS5 dialer
	dialer, err := netproxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Dial target through SOCKS5 proxy
	// Note: golang.org/x/net/proxy automatically resolves DNS via proxy by default
	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 connection failed: %w", err)
	}

	return conn, nil
}

// Close gracefully shuts down the Transport by stopping background goroutines
// and closing all pooled connections. This method should be called when the
// Transport is no longer needed to prevent goroutine leaks.
func (t *Transport) Close() error {
	// Signal cleanup goroutine to stop
	close(t.stopChan)

	// Wait for all goroutines to finish
	t.wg.Wait()

	// Close all pooled connections
	t.poolMutex.Lock()
	defer t.poolMutex.Unlock()

	t.connPool.Range(func(key, value interface{}) bool {
		pooled := value.(*pooledConnection)
		pooled.conn.Close()
		t.connPool.Delete(key)
		return true
	})

	return nil
}

// loadClientCertificate loads client certificate for mTLS from config.
// Supports both file paths and PEM byte arrays. Returns nil if no client cert is configured.
func (t *Transport) loadClientCertificate(config Config) (*tls.Certificate, error) {
	// Check if we have client certificate data
	hasPEM := len(config.ClientCertPEM) > 0 && len(config.ClientKeyPEM) > 0
	hasFile := config.ClientCertFile != "" && config.ClientKeyFile != ""

	if !hasPEM && !hasFile {
		// No client certificate configured
		return nil, nil
	}

	var certPEM, keyPEM []byte
	var err error

	if hasPEM {
		// Use provided PEM data directly
		certPEM = config.ClientCertPEM
		keyPEM = config.ClientKeyPEM
	} else if hasFile {
		// Load from files
		certPEM, err = os.ReadFile(config.ClientCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client certificate file %s: %w", config.ClientCertFile, err)
		}

		keyPEM, err = os.ReadFile(config.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client key file %s: %w", config.ClientKeyFile, err)
		}
	}

	// Parse certificate and key
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client certificate/key: %w", err)
	}

	return &cert, nil
}

// ConfigureSNI applies SNI (Server Name Indication) configuration to a TLS config.
// It follows this priority order:
// 1. If tlsConfig.ServerName is already set, it's preserved (highest priority)
// 2. If disableSNI is true, ServerName is left empty
// 3. If customSNI is set, it's used
// 4. Otherwise, fallbackHost is used as ServerName
//
// This function eliminates code duplication between HTTP/1.1 and HTTP/2 transports (DEF-4).
func ConfigureSNI(tlsConfig *tls.Config, customSNI string, disableSNI bool, fallbackHost string) {
	if tlsConfig == nil {
		return
	}

	// If ServerName is already set (user provided it in TLSConfig), keep it
	if tlsConfig.ServerName != "" {
		return
	}

	// If SNI is disabled, leave ServerName empty
	if disableSNI {
		return
	}

	// Use custom SNI if provided, otherwise use fallback host
	if customSNI != "" {
		tlsConfig.ServerName = customSNI
	} else {
		tlsConfig.ServerName = fallbackHost
	}
}
