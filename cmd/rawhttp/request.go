package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// target captures everything about the connection endpoint derived from a URL.
type target struct {
	scheme string
	host   string
	port   int
	path   string // path + query, ready for the request line
}

// parseTarget parses a raw URL string into a target, applying curl-style
// defaults (scheme http, path "/", default ports).
func parseTarget(raw string) (*target, *url.URL, error) {
	if raw == "" {
		return nil, nil, fmt.Errorf("no URL specified")
	}
	// curl lets you omit the scheme; default to http://
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse URL %q: %w", raw, err)
	}
	if u.Host == "" {
		return nil, nil, fmt.Errorf("could not parse host from URL %q", raw)
	}

	t := &target{scheme: strings.ToLower(u.Scheme), host: u.Hostname()}
	if t.scheme == "" {
		t.scheme = "http"
	}
	t.port = defaultPort(t.scheme)
	if p := u.Port(); p != "" {
		fmt.Sscanf(p, "%d", &t.port)
	}

	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	t.path = path
	return t, u, nil
}

func defaultPort(scheme string) int {
	if scheme == "https" {
		return 443
	}
	return 80
}

// hostHeader returns the value for the Host header, omitting the port when it
// is the default for the scheme (matching curl).
func (t *target) hostHeader() string {
	if t.port == defaultPort(t.scheme) {
		return t.host
	}
	return fmt.Sprintf("%s:%d", t.host, t.port)
}

// BuildRequest assembles the raw HTTP/1.1 request bytes for the given config
// and target. It returns the full request bytes plus the "head" lines (request
// line + headers) for verbose output. When --raw-request is set the file is
// sent verbatim.
func BuildRequest(cfg *Config, t *target, u *url.URL) (full []byte, headLines []string, err error) {
	if cfg.RawRequest != "" {
		return buildRawRequest(cfg.RawRequest)
	}

	// 1. Build the body first so we know Content-Length / Content-Type.
	body, autoContentType, err := buildBody(cfg)
	if err != nil {
		return nil, nil, err
	}

	// 2. -G: move data into the query string and drop the body.
	path := t.path
	method := cfg.Method
	if cfg.Get && len(body) > 0 {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		path += sep + string(body)
		body = nil
		autoContentType = ""
		if method == "" {
			method = "GET"
		}
	}

	// 3. Resolve method.
	if method == "" {
		switch {
		case cfg.Head:
			method = "HEAD"
		case len(body) > 0:
			method = "POST"
		default:
			method = "GET"
		}
	}

	// 4. Header assembly. userSet tracks which header names the user provided
	//    so we can suppress the corresponding defaults.
	userSet := map[string]bool{}
	for _, h := range cfg.Headers {
		if name, _, ok := splitHeader(h); ok {
			userSet[strings.ToLower(name)] = true
		}
	}

	var head bytes.Buffer
	fmt.Fprintf(&head, "%s %s HTTP/1.1\r\n", method, path)

	emit := func(name, value string) {
		fmt.Fprintf(&head, "%s: %s\r\n", name, value)
	}
	emitDefault := func(name, value string) {
		if !userSet[strings.ToLower(name)] {
			emit(name, value)
		}
	}

	// Host is mandatory and always emitted unless the user overrides it.
	emitDefault("Host", t.hostHeader())

	ua := cfg.UserAgent
	if ua == "" {
		ua = "rawhttp/" + appVersion()
	}
	emitDefault("User-Agent", ua)
	emitDefault("Accept", "*/*")

	if cfg.User != "" {
		token := base64.StdEncoding.EncodeToString([]byte(cfg.User))
		emitDefault("Authorization", "Basic "+token)
	}
	if cfg.Cookie != "" {
		emitDefault("Cookie", cfg.Cookie)
	}
	if cfg.Referer != "" {
		emitDefault("Referer", cfg.Referer)
	}
	if cfg.compressEnabled() {
		emitDefault("Accept-Encoding", "gzip, deflate, br")
	}
	if cfg.Range != "" {
		emitDefault("Range", "bytes="+normalizeRange(cfg.Range))
	}
	if cfg.NoKeepalive {
		emitDefault("Connection", "close")
	}

	// Body framing headers. A user-supplied Content-Length is sent verbatim (like
	// curl) — we never silently override it. Content-Length is auto-computed only
	// when the user did not provide one (and not for chunked bodies), unless
	// --no-content-length suppresses the automatic header entirely.
	if len(body) > 0 {
		if autoContentType != "" {
			emitDefault("Content-Type", autoContentType)
		}
		if !userSet["content-length"] && !userSet["transfer-encoding"] && !cfg.NoContentLength {
			emit("Content-Length", fmt.Sprintf("%d", len(body)))
		}
	}

	// Surface a user-supplied Content-Length that disagrees with the real body
	// size: the server will likely reject the request, and the usual cause is the
	// body being truncated at a NUL byte on the command line (the @file hint fixes
	// it). We do NOT override the value — a raw tool must send exactly what was asked.
	if len(body) > 0 && userSet["content-length"] && (!cfg.Silent || cfg.Verbose) {
		if cl, ok := headerInt(cfg.Headers, "Content-Length"); ok && cl != int64(len(body)) {
			fmt.Fprintf(os.Stderr,
				"rawhttp: warning: Content-Length (%d) does not match the request body size (%d) — the body is likely truncated (e.g. a NUL byte cut it on the command line). Send the body from a file: --data-binary @file\n",
				cl, len(body))
		}
	}

	// User headers last, in the order given. An empty value ("Name:") removes a header.
	for _, h := range cfg.Headers {
		name, value, ok := splitHeader(h)
		if !ok {
			continue
		}
		if value == "" {
			continue // header removal
		}
		emit(name, value)
	}

	head.WriteString("\r\n")
	headBytes := head.Bytes()

	fullBuf := append([]byte{}, headBytes...)
	fullBuf = append(fullBuf, body...)

	headLines = splitCRLFLines(string(headBytes))
	return fullBuf, headLines, nil
}

// buildRawRequest reads a complete raw HTTP request from a file (or stdin when
// path is "-"), normalises line endings, and guarantees a header terminator.
func buildRawRequest(path string) ([]byte, []string, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("could not read raw request: %w", err)
	}
	// Normalise bare LF to CRLF without doubling existing CRLF.
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", "\r\n")
	if !strings.Contains(s, "\r\n\r\n") {
		s += "\r\n\r\n"
	}
	full := []byte(s)

	headEnd := bytes.Index(full, []byte("\r\n\r\n"))
	headLines := splitCRLFLines(string(full[:headEnd+2]))
	return full, headLines, nil
}

// buildBody assembles the request body from -d / --data-binary / --data-raw or
// -F multipart forms, returning the body and the implied Content-Type.
func buildBody(cfg *Config) ([]byte, string, error) {
	if len(cfg.Forms) > 0 {
		return buildMultipart(cfg.Forms)
	}

	var parts []string
	// -d : strip newlines/CR, support @file
	for _, d := range cfg.Data {
		v, err := readDataArg(d, true)
		if err != nil {
			return nil, "", err
		}
		parts = append(parts, v)
	}
	// --data-binary : raw, support @file
	for _, d := range cfg.DataBinary {
		v, err := readDataArg(d, false)
		if err != nil {
			return nil, "", err
		}
		parts = append(parts, v)
	}
	// --data-raw : literal, '@' not special
	parts = append(parts, cfg.DataRaw...)
	// --data-hex / --data-base64 : decoded to raw bytes; safe for NUL/binary
	// bodies that the shell would otherwise truncate at a NUL byte.
	for _, d := range cfg.DataHex {
		v, err := readEncodedDataArg(d, decodeHex)
		if err != nil {
			return nil, "", err
		}
		parts = append(parts, v)
	}
	for _, d := range cfg.DataBase64 {
		v, err := readEncodedDataArg(d, decodeBase64)
		if err != nil {
			return nil, "", err
		}
		parts = append(parts, v)
	}
	// --data-urlencode : URL-encode per curl rules
	for _, d := range cfg.DataURLEnc {
		v, err := urlencodeDataArg(d)
		if err != nil {
			return nil, "", err
		}
		parts = append(parts, v)
	}

	if len(parts) == 0 {
		return nil, "", nil
	}
	return []byte(strings.Join(parts, "&")), "application/x-www-form-urlencoded", nil
}

// urlencodeDataArg implements curl's --data-urlencode forms:
//
//	content        -> urlencode(content)
//	=content       -> urlencode(content)
//	name=content   -> name=urlencode(content)
//	@file          -> urlencode(file contents)
//	name@file      -> name=urlencode(file contents)
func urlencodeDataArg(arg string) (string, error) {
	i := strings.IndexAny(arg, "=@")
	if i < 0 {
		return url.QueryEscape(arg), nil
	}
	name, sep, rest := arg[:i], arg[i], arg[i+1:]
	if sep == '@' {
		data, err := os.ReadFile(rest)
		if err != nil {
			return "", fmt.Errorf("could not read data file %q: %w", rest, err)
		}
		rest = string(data)
	}
	if name == "" {
		return url.QueryEscape(rest), nil
	}
	return name + "=" + url.QueryEscape(rest), nil
}

// normalizeRange strips an optional leading "bytes=" so callers can always
// prefix it themselves.
func normalizeRange(r string) string {
	return strings.TrimPrefix(strings.TrimSpace(r), "bytes=")
}

// readDataArg resolves a -d/--data-binary argument. A leading '@' reads a file.
// When strip is true, CR and LF are removed (curl's -d behaviour).
func readDataArg(arg string, strip bool) (string, error) {
	if strings.HasPrefix(arg, "@") {
		path := arg[1:]
		var data []byte
		var err error
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return "", fmt.Errorf("could not read data file %q: %w", path, err)
		}
		s := string(data)
		if strip {
			s = strings.NewReplacer("\r", "", "\n", "").Replace(s)
		}
		return s, nil
	}
	return arg, nil
}

// readEncodedDataArg resolves a --data-hex / --data-base64 argument. A leading
// '@' reads the encoded text from a file ('-' = stdin); otherwise the argument
// itself is the encoded text. The decoded raw bytes are returned as a string so
// they can flow through the normal body-assembly path. Because the value is
// transported as ASCII hex/base64, NUL and other binary bytes survive the shell
// (which would truncate a literal $'...\x00...' argument at the first NUL).
func readEncodedDataArg(arg string, decode func(string) ([]byte, error)) (string, error) {
	enc := arg
	if strings.HasPrefix(arg, "@") {
		path := arg[1:]
		var data []byte
		var err error
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return "", fmt.Errorf("could not read data file %q: %w", path, err)
		}
		enc = string(data)
	}
	raw, err := decode(enc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// decodeHex decodes hex text, tolerating surrounding whitespace, newlines and an
// optional "0x" prefix so pasted/file input "just works".
func decodeHex(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid --data-hex value: %w", err)
	}
	return b, nil
}

// decodeBase64 decodes base64 text, tolerating whitespace/newlines and accepting
// both standard and URL alphabets, with or without padding.
func decodeBase64(s string) ([]byte, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, s)
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("invalid --data-base64 value")
}

// buildMultipart constructs a multipart/form-data body from -F specs. Each spec
// is "name=value", "name=@file" (file upload) or "name=<file" (file as value).
func buildMultipart(forms []string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, f := range forms {
		name, value, found := strings.Cut(f, "=")
		if !found {
			return nil, "", fmt.Errorf("invalid --form value %q (expected name=value)", f)
		}
		switch {
		case strings.HasPrefix(value, "@"):
			spec := value[1:]
			// Support optional ;type=... suffix (curl-compatible).
			path := spec
			ctype := ""
			if i := strings.Index(spec, ";"); i >= 0 {
				path = spec[:i]
				if t, ok := parseFormTypeSuffix(spec[i:]); ok {
					ctype = t
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", fmt.Errorf("could not read form file %q: %w", path, err)
			}
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeFormQuotes(name), escapeFormQuotes(filepath.Base(path))))
			if ctype != "" {
				h.Set("Content-Type", ctype)
			}
			fw, err := w.CreatePart(h)
			if err != nil {
				return nil, "", err
			}
			if _, err := fw.Write(data); err != nil {
				return nil, "", err
			}
		case strings.HasPrefix(value, "<"):
			path := value[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", fmt.Errorf("could not read form file %q: %w", path, err)
			}
			if err := w.WriteField(name, string(data)); err != nil {
				return nil, "", err
			}
		default:
			if err := w.WriteField(name, value); err != nil {
				return nil, "", err
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// parseFormTypeSuffix extracts the MIME type from a curl -F suffix like
// ";type=text/csv". Returns ("", false) if no type is present.
func parseFormTypeSuffix(suffix string) (string, bool) {
	for _, part := range strings.Split(suffix, ";") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "type="); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// escapeFormQuotes escapes characters that would break a multipart
// Content-Disposition header, matching mime/multipart's own escaping.
func escapeFormQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"", "\r", "", "\n", "").Replace(s)
}

// headerInt finds a header in the user's -H list (case-insensitive) and parses
// its value as an integer.
func headerInt(headers []string, name string) (int64, bool) {
	for _, h := range headers {
		n, v, ok := splitHeader(h)
		if ok && strings.EqualFold(n, name) {
			i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

// requestView returns the request head lines as they should be shown for the
// protocol the request actually travelled on. For HTTP/2 it converts the
// HTTP/1.1-format head into the conventional HTTP/2 text representation; for
// HTTP/1.1 the lines are returned unchanged.
func requestView(headLines []string, proto string) []string {
	if strings.Contains(proto, "2") {
		return toHTTP2View(headLines)
	}
	return headLines
}

// toHTTP2View rewrites an HTTP/1.1-format request head into the conventional
// HTTP/2 text form (as Burp / browser devtools show it): "METHOD path HTTP/2",
// lowercase header names, Host -> :authority, and hop-by-hop headers (which
// HTTP/2 forbids) removed. This mirrors how the library frames the request on an
// h2 connection.
func toHTTP2View(headLines []string) []string {
	if len(headLines) == 0 {
		return headLines
	}
	out := make([]string, 0, len(headLines)+1)
	if fields := strings.SplitN(headLines[0], " ", 3); len(fields) >= 2 {
		out = append(out, fields[0]+" "+fields[1]+" HTTP/2")
	} else {
		out = append(out, headLines[0])
	}

	var authority string
	var rest []string
	for _, h := range headLines[1:] {
		name, value, ok := splitHeader(h)
		if !ok {
			continue
		}
		switch strings.ToLower(name) {
		case "host":
			authority = strings.TrimSpace(value)
		case "connection", "keep-alive", "proxy-connection", "transfer-encoding", "upgrade", "te":
			// Illegal in HTTP/2 — the library drops these when framing.
		default:
			rest = append(rest, strings.ToLower(name)+": "+value)
		}
	}
	if authority != "" {
		out = append(out, ":authority: "+authority)
	}
	return append(out, rest...)
}

// requestBodyBytes returns the body portion (after the CRLF header terminator)
// of a raw request, or nil if there is none.
func requestBodyBytes(req []byte) []byte {
	if i := bytes.Index(req, []byte("\r\n\r\n")); i >= 0 {
		return req[i+4:]
	}
	return nil
}

// cutHeaderLine splits a header line "Name: value" into its parts, correctly
// handling HTTP/2 pseudo-headers such as ":authority: host" (which begin with a
// colon). Header names never contain ": ", so the first ": " is the separator.
func cutHeaderLine(h string) (name, value string, ok bool) {
	if i := strings.Index(h, ": "); i > 0 {
		return strings.TrimSpace(h[:i]), strings.TrimSpace(h[i+2:]), true
	}
	if i := strings.IndexByte(h, ':'); i > 0 {
		return strings.TrimSpace(h[:i]), strings.TrimSpace(h[i+1:]), true
	}
	return "", "", false
}

// splitHeader splits "Name: value" (or "Name:" for removal) into its parts.
func splitHeader(h string) (name, value string, ok bool) {
	i := strings.IndexByte(h, ':')
	if i < 0 {
		return "", "", false
	}
	name = strings.TrimSpace(h[:i])
	value = strings.TrimSpace(h[i+1:])
	if name == "" {
		return "", "", false
	}
	return name, value, true
}

// splitCRLFLines splits a CRLF-delimited block into non-empty lines.
func splitCRLFLines(s string) []string {
	var lines []string
	for _, ln := range strings.Split(s, "\r\n") {
		if ln == "" {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}
