package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

// appVersion is the rawhttp CLI version. It tracks the go-rawhttp library
// version so `--version` and the default User-Agent always reflect the release.
func appVersion() string { return libVersion() }

// Config holds every parsed command-line option. It is the single source of
// truth that request.go / options.go / output.go consume.
type Config struct {
	// Request shaping
	Method     string
	Headers    []string
	Data       []string
	DataBinary []string
	DataRaw    []string
	DataHex    []string
	DataBase64 []string
	Forms      []string
	UserAgent  string
	Referer    string
	Cookie     string
	User       string
	Get        bool
	Head       bool

	// Redirects (the library does not auto-follow; redirect.go implements -L)
	FollowRedirects bool
	MaxRedirs       int

	// Output / presentation
	OutputFile string
	RemoteName bool
	Silent     bool
	Verbose    bool
	Include    bool
	WriteOut   string
	ShowTiming bool

	// Output rendering (beautify + syntax-highlight + color layer)
	Color         bool
	NoColor       bool
	Beautify      bool
	NoBeautify    bool
	Theme         string
	Style         string
	PrintBinary   bool
	MaxRenderSize int64

	// Connection / TLS
	Insecure       bool
	ConnectTimeout float64
	MaxTime        float64
	Proxy          string
	ProxyUser      string
	HTTP11         bool
	HTTP2          bool
	HTTP2Prior     bool
	Resolve        []string
	CertFile       string
	KeyFile        string
	CACert         string

	// Download manager (IDM-style segmented, multi-connection downloads)
	Download   bool
	Parallel   int
	ChunkSize  string
	NoProgress bool

	// Library superpowers
	SNI        string
	DisableSNI bool
	ConnectTo  []string
	ConnectIP  string
	RawRequest string
	Reuse      bool
	TLSMin     string
	TLSMax     string

	// Additional curl-compatible flags (so pasted curl commands don't break)
	PathAsIs        bool
	Compressed      bool
	NoCompressed    bool
	DataURLEnc      []string
	Fail            bool
	Range           string
	GlobOff         bool
	NoBuffer        bool
	NoKeepalive     bool
	NoContentLength bool

	// Structured output
	JSONOut bool
	XMLOut  bool
	HTMLOut string // filename for the styled HTML report

	// Positional
	URL string

	showVersion bool
}

// parseFlags parses os.Args using a curl-compatible flag set built on pflag.
// On -h/--help or --version it prints and exits directly.
func parseFlags(args []string) (*Config, error) {
	cfg := &Config{}
	fs := flag.NewFlagSet("rawhttp", flag.ContinueOnError)
	fs.SortFlags = false

	// --- Request shaping ---------------------------------------------------
	fs.StringVarP(&cfg.Method, "request", "X", "", "Specify request method to use")
	fs.StringArrayVarP(&cfg.Headers, "header", "H", nil, "Pass custom header(s) to server (repeatable)")
	fs.StringArrayVarP(&cfg.Data, "data", "d", nil, "HTTP POST data (urlencoded; @file to read a file)")
	fs.StringArrayVar(&cfg.DataBinary, "data-binary", nil, "HTTP POST data exactly as specified")
	fs.StringArrayVar(&cfg.DataRaw, "data-raw", nil, "HTTP POST data, '@' allowed literally")
	fs.StringArrayVar(&cfg.DataHex, "data-hex", nil, "HTTP POST data given as hex; decoded to raw bytes (@file to read a file). Safe for NUL/binary bodies")
	fs.StringArrayVar(&cfg.DataBase64, "data-base64", nil, "HTTP POST data given as base64; decoded to raw bytes (@file to read a file). Safe for NUL/binary bodies")
	fs.StringArrayVar(&cfg.DataURLEnc, "data-urlencode", nil, "HTTP POST data, URL-encoded")
	fs.StringArrayVarP(&cfg.Forms, "form", "F", nil, "Specify multipart MIME data (repeatable)")
	fs.StringVarP(&cfg.UserAgent, "user-agent", "A", "", "Send User-Agent <name> to server")
	fs.StringVarP(&cfg.Referer, "referer", "e", "", "Referrer URL")
	fs.StringVarP(&cfg.Cookie, "cookie", "b", "", "Send cookies from string")
	fs.StringVarP(&cfg.User, "user", "u", "", "Server user and password (user:password)")
	fs.BoolVarP(&cfg.Get, "get", "G", false, "Put the post data in the URL and use GET")
	fs.BoolVarP(&cfg.Head, "head", "I", false, "Show document info only (HEAD)")

	// --- Redirects ---------------------------------------------------------
	fs.BoolVarP(&cfg.FollowRedirects, "location", "L", false, "Follow redirects")
	fs.IntVar(&cfg.MaxRedirs, "max-redirs", 50, "Maximum number of redirects allowed")

	// --- Output ------------------------------------------------------------
	fs.StringVarP(&cfg.OutputFile, "output", "o", "", "Write to file instead of stdout")
	fs.BoolVarP(&cfg.RemoteName, "remote-name", "O", false, "Write output to a file named as the remote file")
	fs.BoolVarP(&cfg.Silent, "silent", "s", false, "Silent mode")
	fs.BoolVarP(&cfg.Verbose, "verbose", "v", false, "Make the operation more talkative")
	fs.BoolVarP(&cfg.Include, "include", "i", false, "Include response headers in the output")
	fs.StringVarP(&cfg.WriteOut, "write-out", "w", "", "Use output FORMAT after completion")
	fs.BoolVar(&cfg.ShowTiming, "timings", false, "Print a detailed timing breakdown to stderr")

	// --- Output rendering (beautify + color) -------------------------------
	fs.BoolVar(&cfg.Color, "color", false, "Force ANSI color output (default: auto when stdout is a TTY)")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "Disable ANSI color output")
	fs.BoolVar(&cfg.Beautify, "beautify", false, "Force body beautify/pretty-print (default: auto when stdout is a TTY)")
	fs.BoolVar(&cfg.NoBeautify, "no-beautify", false, "Disable body beautification")
	fs.StringVar(&cfg.Theme, "theme", "dark", "UI theme for highlighting: dark or light (affects CLI and --html)")
	fs.StringVar(&cfg.Style, "style", "", "Explicit chroma style name (overrides --theme)")
	fs.BoolVar(&cfg.PrintBinary, "print-binary", false, "Print binary/image bodies raw instead of summarizing them")
	fs.Int64Var(&cfg.MaxRenderSize, "max-render-size", defaultMaxRenderSize, "Maximum body size (bytes) to beautify/highlight")

	// --- Connection / TLS --------------------------------------------------
	fs.BoolVarP(&cfg.Insecure, "insecure", "k", false, "Allow insecure server connections")
	fs.Float64Var(&cfg.ConnectTimeout, "connect-timeout", 0, "Maximum time allowed for connection in seconds (default: 10)")
	fs.Float64VarP(&cfg.MaxTime, "max-time", "m", 0, "Maximum time for the whole transfer in seconds (default: no limit, like curl)")
	fs.StringVarP(&cfg.Proxy, "proxy", "x", "", "Use this proxy ([scheme://]host[:port])")
	fs.StringVar(&cfg.ProxyUser, "proxy-user", "", "Proxy user and password (user:password)")
	fs.BoolVar(&cfg.HTTP11, "http1.1", false, "Use HTTP 1.1")
	fs.BoolVar(&cfg.HTTP2, "http2", false, "Try HTTP/2, fall back to HTTP/1.1 if unsupported (like curl --http2)")
	fs.BoolVar(&cfg.HTTP2Prior, "http2-prior-knowledge", false, "Force HTTP/2 with no fallback (error if the server doesn't support it)")
	fs.StringArrayVar(&cfg.Resolve, "resolve", nil, "Resolve the host+port to this address (host:port:addr)")
	fs.StringVar(&cfg.CertFile, "cert", "", "Client certificate file (mTLS)")
	fs.StringVar(&cfg.KeyFile, "key", "", "Private key file (mTLS)")
	fs.StringVar(&cfg.CACert, "cacert", "", "CA certificate to verify peer against")

	// --- Download manager --------------------------------------------------
	fs.BoolVar(&cfg.Download, "download", false, "Use the segmented download manager (progress bar, resume offsets)")
	fs.IntVarP(&cfg.Parallel, "parallel", "j", 1, "Number of parallel download connections (implies --download when >1)")
	fs.StringVar(&cfg.ChunkSize, "chunk-size", "4M", "Per-segment size for the download manager (e.g. 512K, 4M)")
	fs.BoolVar(&cfg.NoProgress, "no-progress", false, "Disable the download progress bar")

	// --- Library superpowers ----------------------------------------------
	fs.StringVar(&cfg.SNI, "sni", "", "Override the TLS SNI server name")
	fs.BoolVar(&cfg.DisableSNI, "disable-sni", false, "Disable the TLS SNI extension entirely")
	fs.StringArrayVar(&cfg.ConnectTo, "connect-to", nil, "Connect to host (HOST1:PORT1:HOST2:PORT2)")
	fs.StringVar(&cfg.ConnectIP, "connect-ip", "", "Connect to this IP directly, bypassing DNS")
	fs.StringVar(&cfg.RawRequest, "raw-request", "", "Send a raw HTTP request read from file ('-' = stdin)")
	fs.BoolVar(&cfg.Reuse, "reuse", false, "Enable keep-alive connection reuse / pooling")
	fs.StringVar(&cfg.TLSMin, "tls-min", "", "Minimum TLS version (1.0, 1.1, 1.2, 1.3)")
	fs.StringVar(&cfg.TLSMax, "tls-max", "", "Maximum TLS version (1.0, 1.1, 1.2, 1.3)")

	// --- Additional curl-compatible flags ----------------------------------
	fs.BoolVar(&cfg.PathAsIs, "path-as-is", false, "Do not squash /../ and /./ in the path (already the default)")
	fs.BoolVar(&cfg.Compressed, "compressed", true, "Request compressed responses (Accept-Encoding) and decompress them")
	fs.BoolVar(&cfg.NoCompressed, "no-compressed", false, "Do not send Accept-Encoding; leave the response body raw")
	fs.BoolVarP(&cfg.Fail, "fail", "f", false, "Fail silently (no body) on HTTP errors >= 400 (exit 22)")
	fs.StringVarP(&cfg.Range, "range", "r", "", "Request a byte range (sets the Range header)")
	fs.BoolVarP(&cfg.GlobOff, "globoff", "g", false, "Disable URL globbing (accepted; rawhttp never globs)")
	fs.BoolVarP(&cfg.NoBuffer, "no-buffer", "N", false, "Disable output buffering (accepted)")
	fs.BoolVar(&cfg.NoKeepalive, "no-keepalive", false, "Disable keep-alive (send Connection: close)")
	fs.BoolVar(&cfg.NoContentLength, "no-content-length", false, "Do not auto-add a Content-Length header (a user-supplied one is still sent)")

	// --- Structured output -------------------------------------------------
	fs.BoolVar(&cfg.JSONOut, "json", false, "Emit the whole transaction (request, response, connection, timing, error) as JSON")
	fs.BoolVar(&cfg.XMLOut, "xml", false, "Emit the whole transaction as XML")
	fs.StringVar(&cfg.HTMLOut, "html", "", "Write a styled, syntax-highlighted HTML report of the transaction to <file>")

	fs.BoolVarP(&cfg.showVersion, "version", "V", false, "Show version number and quit")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "rawhttp %s - a curl-like CLI powered by go-rawhttp\n\n", appVersion())
		fmt.Fprintf(os.Stderr, "Usage: rawhttp [options...] <url>\n\n")
		fmt.Fprint(os.Stderr, fs.FlagUsages())
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if cfg.showVersion {
		printVersion()
		os.Exit(0)
	}

	rest := fs.Args()
	if len(rest) > 0 {
		cfg.URL = rest[0]
	}

	return cfg, nil
}

// printVersion prints CLI + backend library version, mirroring `curl --version`.
func printVersion() {
	fmt.Printf("rawhttp %s (go-rawhttp/%s)\n", appVersion(), libVersion())
	fmt.Println("Protocols: http https")
	fmt.Println("Features: HTTP2 Proxy SOCKS5 SOCKS4 SNI mTLS raw-request connection-reuse")
}
