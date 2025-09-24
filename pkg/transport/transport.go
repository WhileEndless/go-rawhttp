// Package transport provides the low-level HTTP transport implementation.
package transport

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/errors"
	"github.com/WhileEndless/go-rawhttp/pkg/timing"
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
}

// Transport handles the network connection and protocol negotiation.
type Transport struct {
	resolver *net.Resolver
}

// New creates a new Transport instance.
func New() *Transport {
	return &Transport{
		resolver: net.DefaultResolver,
	}
}

// NewWithResolver creates a new Transport with a custom resolver.
func NewWithResolver(resolver *net.Resolver) *Transport {
	return &Transport{
		resolver: resolver,
	}
}

// Connect establishes a connection based on the configuration.
func (t *Transport) Connect(ctx context.Context, config Config, timer *timing.Timer) (net.Conn, error) {
	if err := t.validateConfig(config); err != nil {
		return nil, err
	}

	// Setup timeouts
	connTimeout := config.ConnTimeout
	if connTimeout <= 0 {
		connTimeout = 10 * time.Second
	}

	// Resolve DNS if needed
	dialAddr, err := t.resolveAddress(ctx, config, timer)
	if err != nil {
		return nil, err
	}

	// Establish TCP connection
	conn, err := t.connectTCP(ctx, dialAddr, connTimeout, timer)
	if err != nil {
		return nil, errors.NewConnectionError(config.Host, config.Port, err)
	}

	// Upgrade to TLS if needed
	if strings.EqualFold(config.Scheme, "https") {
		conn, err = t.upgradeTLS(ctx, conn, config, timer)
		if err != nil {
			conn.Close()
			return nil, errors.NewTLSError(config.Host, config.Port, err)
		}
	}

	return conn, nil
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

func (t *Transport) resolveAddress(ctx context.Context, config Config, timer *timing.Timer) (string, error) {
	// If ConnectIP is specified, use it directly
	if config.ConnectIP != "" {
		return net.JoinHostPort(config.ConnectIP, strconv.Itoa(config.Port)), nil
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
		return "", errors.NewDNSError(config.Host, err)
	}

	if len(addrs) == 0 {
		return "", errors.NewDNSError(config.Host, errors.NewValidationError("no IP addresses found"))
	}

	// Use the first address
	ip := addrs[0].IP.String()
	return net.JoinHostPort(ip, strconv.Itoa(config.Port)), nil
}

func (t *Transport) connectTCP(ctx context.Context, dialAddr string, timeout time.Duration, timer *timing.Timer) (net.Conn, error) {
	timer.StartTCP()
	defer timer.EndTCP()

	dialer := &net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, "tcp", dialAddr)
}

func (t *Transport) upgradeTLS(ctx context.Context, conn net.Conn, config Config, timer *timing.Timer) (net.Conn, error) {
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

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.InsecureTLS,
		NextProtos:         []string{"http/1.1"},
	}

	// Configure SNI
	if !config.DisableSNI {
		serverName := config.SNI
		if serverName == "" {
			serverName = config.Host
		}
		tlsConfig.ServerName = serverName
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(tlsCtx); err != nil {
		return nil, err
	}

	return tlsConn, nil
}
