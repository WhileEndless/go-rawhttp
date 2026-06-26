package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	rawhttp "github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/cmd/rawhttp/render"
)

// tracer renders curl-style verbose output (>, <, *) to stderr. It is a no-op
// when verbose is disabled.
type tracer struct {
	enabled bool
	w       io.Writer
	p       render.Painter
}

func newTracer(cfg *Config) *tracer {
	// Verbose goes to stderr, so gate its color on stderr being a TTY.
	color := colorEnabledFor(cfg, stderrIsTTY(), false)
	return &tracer{enabled: cfg.Verbose, w: os.Stderr, p: render.Painter{On: color}}
}

// star writes a plain "* ..." diagnostic line (message in the label color).
func (t *tracer) star(format string, args ...any) {
	fmt.Fprintf(t.w, "%s %s\n", t.p.Punct("*"), t.p.Label(fmt.Sprintf(format, args...)))
}

// starKV writes a "* <label> <value>" diagnostic line: the label is colored and
// the value is rendered with the given role function so it stands out.
func (t *tracer) starKV(label, value string, role func(string) string) {
	fmt.Fprintf(t.w, "%s %s %s\n", t.p.Punct("*"), t.p.Label(label), role(value))
}

// requestLines prints the request head (already in the right per-protocol view —
// see requestView) with ">" prefixes.
func (t *tracer) requestLines(headLines []string, body []byte) {
	if !t.enabled {
		return
	}
	for i, ln := range headLines {
		if i == 0 {
			ln = render.ColorizeRequestStartLine(ln, t.p)
		} else {
			ln = render.ColorizeHeaderLine(ln, t.p)
		}
		fmt.Fprintf(t.w, "%s %s\n", t.p.Punct(">"), ln)
	}
	fmt.Fprintln(t.w, t.p.Punct(">"))
	t.requestBody(body)
}

// requestBody shows what was sent in the request body. Textual bodies are
// printed (sanitized) prefixed with "}", like curl's trace; binary bodies are
// not dumped to the terminal — a labelled summary is shown instead.
func (t *tracer) requestBody(body []byte) {
	if len(body) == 0 {
		return
	}
	if render.LooksBinary(body, "") {
		t.star("[%d bytes of binary request body, not shown]", len(body))
		return
	}
	const maxShown = 64 * 1024
	shown, truncated := body, false
	if len(shown) > maxShown {
		shown, truncated = shown[:maxShown], true
	}
	text := render.Sanitize(string(shown))
	for _, ln := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintf(t.w, "%s %s\n", t.p.Punct("}"), ln)
	}
	if truncated {
		t.star("[request body truncated for display: showing %d of %d bytes]", maxShown, len(body))
	}
}

// connInfo prints the "*" connection/TLS/proxy/ALPN diagnostics (printed before
// the request lines, like curl).
func (t *tracer) connInfo(resp *rawhttp.Response) {
	if !t.enabled {
		return
	}

	p := t.p
	if resp.ProxyUsed {
		fmt.Fprintf(t.w, "%s %s %s %s\n", p.Punct("*"), p.Label("Via"),
			p.Value(resp.ProxyType), p.URL(resp.ProxyAddr))
	}
	if resp.ConnectedIP != "" {
		fmt.Fprintf(t.w, "%s %s %s %s %s\n", p.Punct("*"), p.Label("Connected to"),
			p.URL(resp.ConnectedIP), p.Label("port"), p.Number(strconv.Itoa(resp.ConnectedPort)))
	}
	if resp.ConnectionReused {
		t.star("Re-using existing connection")
	}
	if resp.TLSVersion != "" {
		fmt.Fprintf(t.w, "%s %s %s %s %s\n", p.Punct("*"), p.Label("SSL connection using"),
			p.Value(resp.TLSVersion), p.Punct("/"), p.HeaderName(resp.TLSCipherSuite))
	}
	if resp.TLSServerName != "" {
		t.starKV("TLS SNI:", resp.TLSServerName, p.URL)
	}
	if resp.NegotiatedProtocol != "" {
		t.starKV("ALPN: negotiated", resp.NegotiatedProtocol, p.Proto)
	}
	// Report the protocol the response actually arrived on, so the user can tell
	// at a glance what the wire spoke regardless of the raw request formatting.
	if resp.HTTPVersion != "" {
		t.starKV("using", resp.HTTPVersion, p.Proto)
	}
}

// responseHead prints the "<" response status line and headers.
func (t *tracer) responseHead(resp *rawhttp.Response) {
	if !t.enabled {
		return
	}
	for i, ln := range extractHeadLines(resp) {
		if i == 0 {
			ln = render.ColorizeStatusLine(ln, t.p)
		} else {
			ln = render.ColorizeHeaderLine(ln, t.p)
		}
		fmt.Fprintf(t.w, "%s %s\n", t.p.Punct("<"), ln)
	}
	fmt.Fprintln(t.w, t.p.Punct("<"))
}

func (t *tracer) redirect(url string) {
	if !t.enabled {
		return
	}
	t.star("Issue another request to this URL: '%s'", url)
}

// writeOutput renders the final response to the chosen destination, honouring
// -i/-I (include headers) and -o/-O (write to file). Headers and body are
// colorized / beautified for an interactive terminal; output to a file or pipe
// stays raw (see resolveColor / resolveBeautify). Disk-spilled buffers are
// streamed, never fully loaded for the raw path.
func writeOutput(cfg *Config, resp *rawhttp.Response, finalURL string) error {
	out, toFile, closeFn, err := outputWriter(cfg, finalURL)
	if err != nil {
		return err
	}
	defer closeFn()

	bw := bufio.NewWriter(out)
	defer bw.Flush()

	color := resolveColor(cfg, toFile)
	beautifyOn := resolveBeautify(cfg, toFile)

	if cfg.Include || cfg.Head {
		writeHeadBlock(bw, resp, color)
	}

	// HEAD requests have no body to print.
	if cfg.Head {
		return nil
	}
	return renderBody(cfg, resp, bw, toFile, color, beautifyOn)
}

// writeHeadBlock writes the response head (status line + headers). When colored,
// lines use LF and ANSI; otherwise the raw CRLF form is preserved.
func writeHeadBlock(w io.Writer, resp *rawhttp.Response, color bool) {
	p := render.Painter{On: color}
	for i, ln := range extractHeadLines(resp) {
		if color {
			if i == 0 {
				ln = render.ColorizeStatusLine(ln, p)
			} else {
				ln = render.ColorizeHeaderLine(ln, p)
			}
			fmt.Fprintf(w, "%s\n", ln)
		} else {
			fmt.Fprintf(w, "%s\r\n", ln)
		}
	}
	if color {
		fmt.Fprint(w, "\n")
	} else {
		fmt.Fprint(w, "\r\n")
	}
}

// outputWriter resolves the destination for the response body and reports
// whether it is a real file (-o/-O) rather than stdout.
func outputWriter(cfg *Config, finalURL string) (io.Writer, bool, func(), error) {
	name := cfg.OutputFile
	if cfg.RemoteName {
		name = remoteName(finalURL)
	}
	if name == "" || name == "-" {
		return os.Stdout, false, func() {}, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, false, nil, fmt.Errorf("could not open output file %q: %w", name, err)
	}
	return f, true, func() { f.Close() }, nil
}

// remoteName derives the -O filename from the URL path basename.
func remoteName(rawURL string) string {
	p := rawURL
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	base := path.Base(p)
	if base == "" || base == "." || base == "/" {
		return "index.html"
	}
	return base
}

// extractHeadLines pulls the status line + header lines out of resp.Raw,
// preserving the server's original ordering. It reads only up to the blank
// line so large/disk-spilled bodies are not loaded.
func extractHeadLines(resp *rawhttp.Response) []string {
	var lines []string
	if resp.Raw == nil {
		if resp.StatusLine != "" {
			lines = append(lines, resp.StatusLine)
		}
		return lines
	}
	r, err := resp.Raw.Reader()
	if err != nil {
		return lines
	}
	defer r.Close()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		ln := strings.TrimRight(sc.Text(), "\r")
		if ln == "" {
			break
		}
		lines = append(lines, ln)
	}
	return lines
}
