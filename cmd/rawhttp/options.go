package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

// BuildOptions maps the parsed Config onto rawhttp.Options for the given target.
func BuildOptions(cfg *Config, t *target) (rawhttp.Options, error) {
	opts := rawhttp.DefaultOptions(t.scheme, t.host, t.port)

	if cfg.Insecure {
		opts.InsecureTLS = true
	}
	if cfg.ConnectTimeout > 0 {
		opts.ConnTimeout = secondsToDuration(cfg.ConnectTimeout)
	}

	// Response-read timeout. curl has no default read timeout — it waits for the
	// response as long as needed (e.g. a server that delays its reply while waiting
	// for more request body). DefaultOptions imposes a 30s read cap, which made
	// rawhttp time out where curl succeeds. Match curl: when -m/--max-time is set,
	// bound the read/write by it; otherwise wait without an arbitrary cap.
	if cfg.MaxTime > 0 {
		d := secondsToDuration(cfg.MaxTime)
		opts.ReadTimeout = d
		opts.WriteTimeout = d
	} else {
		opts.ReadTimeout = 0
		opts.DisableReadDeadlineFallback = true
	}

	if cfg.Reuse {
		opts.ReuseConnection = true
	}

	// Protocol selection.
	//   --http2-prior-knowledge : force HTTP/2, no fallback (error if unsupported).
	//   --http2                  : try HTTP/2, fall back to HTTP/1.1 if the server
	//                              doesn't support it (like curl --http2).
	//   --http1.1                : force HTTP/1.1.
	//   (none)                   : HTTP/1.1 (curl's default; no HTTP/2 attempt).
	switch {
	case cfg.HTTP2Prior:
		opts.Protocol = "http/2"
	case cfg.HTTP2:
		opts.Protocol = "http/2"
		opts.EnableProtocolFallback = true
	case cfg.HTTP11:
		opts.Protocol = "http/1.1"
	}

	// SNI control (mutually exclusive).
	if cfg.DisableSNI && cfg.SNI != "" {
		return opts, fmt.Errorf("--sni and --disable-sni cannot be used together")
	}
	if cfg.DisableSNI {
		opts.DisableSNI = true
	}
	if cfg.SNI != "" {
		opts.SNI = cfg.SNI
	}

	// Direct-connect IP resolution: --connect-ip > --resolve > --connect-to.
	if ip := resolveConnectIP(cfg, t); ip != "" {
		opts.ConnectIP = ip
	}

	// Proxy.
	if cfg.Proxy != "" {
		proxy := rawhttp.ParseProxyURL(cfg.Proxy)
		if proxy == nil {
			return opts, fmt.Errorf("could not parse proxy URL %q", cfg.Proxy)
		}
		if cfg.ProxyUser != "" {
			user, pass, _ := strings.Cut(cfg.ProxyUser, ":")
			proxy.Username = user
			proxy.Password = pass
		}
		opts.Proxy = proxy
	}

	// mTLS client certificate.
	if cfg.CertFile != "" {
		opts.ClientCertFile = cfg.CertFile
	}
	if cfg.KeyFile != "" {
		opts.ClientKeyFile = cfg.KeyFile
	}

	// Custom CA bundle.
	if cfg.CACert != "" {
		pem, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return opts, fmt.Errorf("could not read CA certificate %q: %w", cfg.CACert, err)
		}
		opts.CustomCACerts = [][]byte{pem}
	}

	// TLS version bounds.
	if cfg.TLSMin != "" {
		v, err := parseTLSVersion(cfg.TLSMin)
		if err != nil {
			return opts, err
		}
		opts.MinTLSVersion = v
	}
	if cfg.TLSMax != "" {
		v, err := parseTLSVersion(cfg.TLSMax)
		if err != nil {
			return opts, err
		}
		opts.MaxTLSVersion = v
	}

	return opts, nil
}

// resolveConnectIP picks a direct-connect IP for the target from the various
// curl-compatible flags, returning "" when none apply.
func resolveConnectIP(cfg *Config, t *target) string {
	if cfg.ConnectIP != "" {
		return cfg.ConnectIP
	}
	// --resolve host:port:addr (host may be "*" to match any host)
	for _, r := range cfg.Resolve {
		host, port, addr, ok := parseResolve(r)
		if !ok {
			continue
		}
		hostMatch := host == "*" || strings.EqualFold(host, t.host)
		portMatch := host == "*" || port == t.port
		if hostMatch && portMatch {
			return addr
		}
	}
	// --connect-to HOST1:PORT1:HOST2:PORT2 — use HOST2 only when it is an IP.
	for _, c := range cfg.ConnectTo {
		parts := strings.Split(c, ":")
		if len(parts) != 4 {
			continue
		}
		h1, p1, h2 := parts[0], parts[1], parts[2]
		if (h1 == "" || strings.EqualFold(h1, t.host)) &&
			(p1 == "" || p1 == fmt.Sprintf("%d", t.port)) && isIPLiteral(h2) {
			return h2
		}
	}
	return ""
}

// parseResolve parses a curl --resolve entry "host:port:addr".
func parseResolve(s string) (host string, port int, addr string, ok bool) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return "", 0, "", false
	}
	host = parts[0]
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		return "", 0, "", false
	}
	addr = parts[2]
	return host, port, addr, addr != ""
}

func isIPLiteral(s string) bool {
	return strings.Count(s, ".") == 3 || strings.Contains(s, ":")
}

func parseTLSVersion(v string) (uint16, error) {
	switch strings.TrimSpace(v) {
	case "1.0", "1":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q (use 1.0/1.1/1.2/1.3)", v)
	}
}

func secondsToDuration(sec float64) time.Duration {
	return time.Duration(sec * float64(time.Second))
}

// libVersion returns the backend library version for --version output.
func libVersion() string {
	return rawhttp.GetVersion()
}
