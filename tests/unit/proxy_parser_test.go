package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/WhileEndless/go-rawhttp/pkg/client"
)

func TestParseProxyURL_HTTP(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *client.ProxyConfig
		wantErr  bool
	}{
		{
			name: "HTTP proxy without port",
			url:  "http://proxy.example.com",
			expected: &client.ProxyConfig{
				Type: "http",
				Host: "proxy.example.com",
				Port: 8080, // default
			},
		},
		{
			name: "HTTP proxy with custom port",
			url:  "http://proxy.example.com:3128",
			expected: &client.ProxyConfig{
				Type: "http",
				Host: "proxy.example.com",
				Port: 3128,
			},
		},
		{
			name: "HTTP proxy with authentication",
			url:  "http://user:pass@proxy.example.com:8080",
			expected: &client.ProxyConfig{
				Type:     "http",
				Host:     "proxy.example.com",
				Port:     8080,
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "HTTP proxy with username only",
			url:  "http://user@proxy.example.com:8080",
			expected: &client.ProxyConfig{
				Type:     "http",
				Host:     "proxy.example.com",
				Port:     8080,
				Username: "user",
				Password: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ParseProxyURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProxyURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.expected.Type)
			}
			if got.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
			if got.Username != tt.expected.Username {
				t.Errorf("Username = %v, want %v", got.Username, tt.expected.Username)
			}
			if got.Password != tt.expected.Password {
				t.Errorf("Password = %v, want %v", got.Password, tt.expected.Password)
			}
		})
	}
}

func TestParseProxyURL_HTTPS(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *client.ProxyConfig
	}{
		{
			name: "HTTPS proxy without port",
			url:  "https://secure-proxy.example.com",
			expected: &client.ProxyConfig{
				Type: "https",
				Host: "secure-proxy.example.com",
				Port: 443, // default
			},
		},
		{
			name: "HTTPS proxy with custom port",
			url:  "https://secure-proxy.example.com:8443",
			expected: &client.ProxyConfig{
				Type: "https",
				Host: "secure-proxy.example.com",
				Port: 8443,
			},
		},
		{
			name: "HTTPS proxy with authentication",
			url:  "https://admin:secret@secure-proxy.example.com:8443",
			expected: &client.ProxyConfig{
				Type:     "https",
				Host:     "secure-proxy.example.com",
				Port:     8443,
				Username: "admin",
				Password: "secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ParseProxyURL(tt.url)
			if err != nil {
				t.Fatalf("ParseProxyURL() error = %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.expected.Type)
			}
			if got.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
			if got.Username != tt.expected.Username {
				t.Errorf("Username = %v, want %v", got.Username, tt.expected.Username)
			}
			if got.Password != tt.expected.Password {
				t.Errorf("Password = %v, want %v", got.Password, tt.expected.Password)
			}
		})
	}
}

func TestParseProxyURL_SOCKS4(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *client.ProxyConfig
	}{
		{
			name: "SOCKS4 proxy without port",
			url:  "socks4://socks-proxy.example.com",
			expected: &client.ProxyConfig{
				Type: "socks4",
				Host: "socks-proxy.example.com",
				Port: 1080, // default
			},
		},
		{
			name: "SOCKS4 proxy with custom port",
			url:  "socks4://socks-proxy.example.com:9050",
			expected: &client.ProxyConfig{
				Type: "socks4",
				Host: "socks-proxy.example.com",
				Port: 9050,
			},
		},
		{
			name: "SOCKS4 proxy with user ID",
			url:  "socks4://myuser@socks-proxy.example.com:1080",
			expected: &client.ProxyConfig{
				Type:     "socks4",
				Host:     "socks-proxy.example.com",
				Port:     1080,
				Username: "myuser",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ParseProxyURL(tt.url)
			if err != nil {
				t.Fatalf("ParseProxyURL() error = %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.expected.Type)
			}
			if got.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
			if got.Username != tt.expected.Username {
				t.Errorf("Username = %v, want %v", got.Username, tt.expected.Username)
			}
		})
	}
}

func TestParseProxyURL_SOCKS5(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *client.ProxyConfig
	}{
		{
			name: "SOCKS5 proxy without port",
			url:  "socks5://socks5-proxy.example.com",
			expected: &client.ProxyConfig{
				Type:               "socks5",
				Host:               "socks5-proxy.example.com",
				Port:               1080, // default
				ResolveDNSViaProxy: true, // SOCKS5 default
			},
		},
		{
			name: "SOCKS5 proxy with authentication",
			url:  "socks5://user:password@socks5-proxy.example.com:1080",
			expected: &client.ProxyConfig{
				Type:               "socks5",
				Host:               "socks5-proxy.example.com",
				Port:               1080,
				Username:           "user",
				Password:           "password",
				ResolveDNSViaProxy: true,
			},
		},
		{
			name: "SOCKS5 with special characters in password",
			url:  "socks5://user:p@ss:word@socks5-proxy.example.com:1080",
			expected: &client.ProxyConfig{
				Type:               "socks5",
				Host:               "socks5-proxy.example.com",
				Port:               1080,
				Username:           "user",
				Password:           "p@ss:word",
				ResolveDNSViaProxy: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ParseProxyURL(tt.url)
			if err != nil {
				t.Fatalf("ParseProxyURL() error = %v", err)
			}

			if got.Type != tt.expected.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.expected.Type)
			}
			if got.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.expected.Host)
			}
			if got.Port != tt.expected.Port {
				t.Errorf("Port = %v, want %v", got.Port, tt.expected.Port)
			}
			if got.Username != tt.expected.Username {
				t.Errorf("Username = %v, want %v", got.Username, tt.expected.Username)
			}
			if got.Password != tt.expected.Password {
				t.Errorf("Password = %v, want %v", got.Password, tt.expected.Password)
			}
			if got.ResolveDNSViaProxy != tt.expected.ResolveDNSViaProxy {
				t.Errorf("ResolveDNSViaProxy = %v, want %v", got.ResolveDNSViaProxy, tt.expected.ResolveDNSViaProxy)
			}
		})
	}
}

func TestParseProxyURL_Errors(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name:    "Empty URL",
			url:     "",
			wantErr: "proxy URL cannot be empty",
		},
		{
			name:    "Invalid URL",
			url:     "://invalid",
			wantErr: "invalid proxy URL",
		},
		{
			name:    "No scheme",
			url:     "proxy.example.com:8080",
			wantErr: "unsupported proxy scheme", // Go's url.Parse treats it as scheme
		},
		{
			name:    "Unsupported scheme",
			url:     "ftp://proxy.example.com:8080",
			wantErr: "unsupported proxy scheme: ftp",
		},
		{
			name:    "No host",
			url:     "http://:8080",
			wantErr: "proxy URL must include host",
		},
		{
			name:    "Invalid port",
			url:     "http://proxy.example.com:abc",
			wantErr: "invalid proxy URL", // Go's url.Parse fails early
		},
		{
			name:    "Port out of range",
			url:     "http://proxy.example.com:99999",
			wantErr: "proxy port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ParseProxyURL(tt.url)
			if err == nil {
				t.Fatalf("ParseProxyURL() expected error, got nil")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseProxyURL() error = %v, want error containing %v", err, tt.wantErr)
			}
		})
	}
}

func TestProxyConfig_WithAdvancedOptions(t *testing.T) {
	// Test that ParseProxyURL creates a config that can be extended
	proxy, err := client.ParseProxyURL("http://user:pass@proxy.example.com:8080")
	if err != nil {
		t.Fatalf("ParseProxyURL() error = %v", err)
	}

	// Add advanced options
	proxy.ConnTimeout = 10 * time.Second
	proxy.ProxyHeaders = map[string]string{
		"X-Custom-Header": "value",
	}

	// Verify all fields
	if proxy.Type != "http" {
		t.Errorf("Type = %v, want http", proxy.Type)
	}
	if proxy.ConnTimeout != 10*time.Second {
		t.Errorf("ConnTimeout = %v, want 10s", proxy.ConnTimeout)
	}
	if proxy.ProxyHeaders["X-Custom-Header"] != "value" {
		t.Errorf("ProxyHeaders not set correctly")
	}
}
