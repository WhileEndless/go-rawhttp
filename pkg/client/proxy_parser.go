// Package client provides the main HTTP client API.
package client

import (
	"fmt"
	"net/url"
	"strconv"
)

// ParseProxyURL parses a proxy URL string into a ProxyConfig.
// This is a convenience function for simple proxy configuration.
//
// Supported URL formats:
//   - http://proxy:8080                    - HTTP proxy without auth
//   - http://user:pass@proxy:8080          - HTTP proxy with Basic auth
//   - https://proxy:443                    - HTTPS proxy (TLS to proxy)
//   - https://user:pass@proxy:443          - HTTPS proxy with auth
//   - socks4://proxy:1080                  - SOCKS4 proxy
//   - socks4://user@proxy:1080             - SOCKS4 with user ID
//   - socks5://proxy:1080                  - SOCKS5 proxy
//   - socks5://user:pass@proxy:1080        - SOCKS5 with auth
//
// Default ports (when not specified in URL):
//   - http: 8080
//   - https: 443
//   - socks4: 1080
//   - socks5: 1080
//
// Example usage:
//
//	// Simple SOCKS5 proxy with authentication
//	proxy, err := ParseProxyURL("socks5://user:secret@proxy.example.com:1080")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	opts := rawhttp.Options{
//	    Scheme: "https",
//	    Host:   "api.example.com",
//	    Port:   443,
//	    Proxy:  proxy,
//	}
//
// Returns error if:
//   - URL format is invalid
//   - Scheme is not supported (must be http, https, socks4, or socks5)
//   - Host is empty
//   - Port is invalid (must be 1-65535)
func ParseProxyURL(proxyURL string) (*ProxyConfig, error) {
	if proxyURL == "" {
		return nil, fmt.Errorf("proxy URL cannot be empty")
	}

	// Parse the URL
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	// Validate scheme
	scheme := u.Scheme
	switch scheme {
	case "http", "https", "socks4", "socks5":
		// Valid schemes
	case "":
		return nil, fmt.Errorf("proxy URL must include scheme (http://, https://, socks4://, or socks5://)")
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s (must be http, https, socks4, or socks5)", scheme)
	}

	// Extract host
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("proxy URL must include host")
	}

	// Extract port with defaults
	var port int
	portStr := u.Port()
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy port: %s", portStr)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("proxy port must be between 1 and 65535, got: %d", port)
		}
	} else {
		// Apply default ports
		switch scheme {
		case "http":
			port = 8080
		case "https":
			port = 443
		case "socks4", "socks5":
			port = 1080
		}
	}

	// Extract authentication credentials
	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password() // Password is optional
	}

	// Create ProxyConfig
	config := &ProxyConfig{
		Type:     scheme,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,

		// SOCKS5 defaults to DNS via proxy
		ResolveDNSViaProxy: (scheme == "socks5"),
	}

	return config, nil
}
