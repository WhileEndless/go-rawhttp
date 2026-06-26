// Package render is a self-contained terminal rendering layer for HTTP traffic:
// it colorizes headers and structured fields and beautifies + syntax-highlights
// response bodies by content type (JSON, XML, HTML with embedded JS/CSS, JS, CSS,
// form data), decompresses Content-Encoding, and safely summarizes binary bodies.
//
// It has no dependency on any particular HTTP client or CLI: callers pass raw
// bytes and an Options value. This makes it reusable and easy to split into its
// own module/repository.
package render

import "strings"

// ANSI SGR codes used for header / structured colorization. Body syntax
// highlighting is handled separately by chroma (see Highlight).
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
	ansiBrCyan  = "\033[96m"
)

// Painter wraps strings in ANSI color codes when On is true. When On is false
// every method is an identity function, so call sites stay branch-free.
type Painter struct{ On bool }

func (p Painter) Wrap(code, s string) string {
	if !p.On || s == "" {
		return s
	}
	return code + s + ansiReset
}

// Role helpers — the palette lives in one place.
func (p Painter) Status(code int, s string) string {
	switch {
	case code >= 200 && code < 300:
		return p.Wrap(ansiGreen+ansiBold, s)
	case code >= 300 && code < 400:
		return p.Wrap(ansiCyan, s)
	case code >= 400:
		return p.Wrap(ansiRed+ansiBold, s)
	default:
		return p.Wrap(ansiBold, s)
	}
}

func (p Painter) HeaderName(s string) string  { return p.Wrap(ansiCyan, s) }
func (p Painter) HeaderValue(s string) string { return s }
func (p Painter) Punct(s string) string       { return p.Wrap(ansiGray, s) }
func (p Painter) Key(s string) string         { return p.Wrap(ansiBlue, s) }
func (p Painter) Value(s string) string       { return p.Wrap(ansiGreen, s) }
func (p Painter) Method(s string) string      { return p.Wrap(ansiBold+ansiYellow, s) }
func (p Painter) URL(s string) string         { return p.Wrap(ansiBrCyan, s) }
func (p Painter) Meta(s string) string        { return p.Wrap(ansiDim, s) }
func (p Painter) Warn(s string) string        { return p.Wrap(ansiYellow, s) }
func (p Painter) Number(s string) string      { return p.Wrap(ansiMagenta, s) }
func (p Painter) Proto(s string) string       { return p.Wrap(ansiBlue, s) }
func (p Painter) Label(s string) string       { return p.Wrap(ansiCyan, s) }

// Sanitize removes control characters (except newline and tab) from text
// destined for the terminal, so hostile response bytes cannot inject ANSI escape
// sequences and hijack the terminal. ESC (0x1b) and other C0 controls plus DEL
// are dropped.
func Sanitize(s string) string {
	if !strings.ContainsFunc(s, isUnsafeControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isUnsafeControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isUnsafeControl(r rune) bool {
	if r == '\n' || r == '\t' {
		return false
	}
	return r < 0x20 || r == 0x7f
}

// ColorizeStatusLine colours an HTTP status line, e.g. "HTTP/2 301 Moved".
func ColorizeStatusLine(line string, p Painter) string {
	if !p.On {
		return line
	}
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 2 {
		return line
	}
	code := parseStatusCode(fields[1])
	rest := strings.Join(fields[1:], " ")
	return p.Proto(fields[0]) + " " + p.Status(code, rest)
}

// ColorizeHeaderLine colours a "Name: value" header (request or response),
// special-casing cookies, auth and URL-bearing headers.
func ColorizeHeaderLine(line string, p Painter) string {
	if !p.On {
		return line
	}
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return line
	}
	value = strings.TrimLeft(value, " ")
	return p.HeaderName(name) + p.Punct(": ") + colorizeHeaderValue(name, value, p)
}

func colorizeHeaderValue(name, value string, p Painter) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "set-cookie", "cookie":
		return colorizeCookie(value, p)
	case "authorization", "proxy-authorization":
		return colorizeAuth(value, p)
	case "location", "content-location", "referer":
		return p.URL(value)
	default:
		return p.HeaderValue(value)
	}
}

// ColorizeRequestStartLine colours a request line, e.g. "GET /a?x=1 HTTP/1.1",
// including its query parameters.
func ColorizeRequestStartLine(line string, p Painter) string {
	if !p.On {
		return line
	}
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 3 {
		return line
	}
	return p.Method(fields[0]) + " " + colorizeURLTarget(fields[1], p) + " " + p.Proto(fields[2])
}

func colorizeURLTarget(target string, p Painter) string {
	path, query, ok := strings.Cut(target, "?")
	if !ok {
		return p.URL(path)
	}
	return p.URL(path) + p.Punct("?") + colorizeQuery(query, p)
}

func colorizeQuery(query string, p Painter) string {
	pairs := strings.Split(query, "&")
	for i, pair := range pairs {
		if k, v, ok := strings.Cut(pair, "="); ok {
			pairs[i] = p.Key(k) + p.Punct("=") + p.Value(v)
		} else {
			pairs[i] = p.Key(pair)
		}
	}
	return strings.Join(pairs, p.Punct("&"))
}

func colorizeCookie(value string, p Painter) string {
	parts := strings.Split(value, ";")
	out := make([]string, 0, len(parts))
	for i, part := range parts {
		seg := strings.TrimSpace(part)
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		switch {
		case ok && i == 0:
			out = append(out, p.Key(k)+p.Punct("=")+p.Value(v))
		case ok:
			out = append(out, p.Meta(k)+p.Punct("=")+p.Meta(v))
		default:
			out = append(out, p.Meta(seg))
		}
	}
	return strings.Join(out, p.Punct("; "))
}

func colorizeAuth(value string, p Painter) string {
	scheme, token, ok := strings.Cut(value, " ")
	if !ok {
		return p.Value(value)
	}
	return p.Method(scheme) + " " + p.Meta(token)
}

// ColorizeForm colours an application/x-www-form-urlencoded body (a=1&b=2).
func ColorizeForm(s string, p Painter) string {
	if !p.On {
		return s
	}
	lines := strings.Split(s, "\n")
	for li, line := range lines {
		if line == "" {
			continue
		}
		pairs := strings.Split(line, "&")
		for i, pair := range pairs {
			if k, v, ok := strings.Cut(pair, "="); ok {
				pairs[i] = p.Key(k) + p.Punct("=") + p.Value(v)
			} else {
				pairs[i] = p.Value(pair)
			}
		}
		lines[li] = strings.Join(pairs, p.Punct("&"))
	}
	return strings.Join(lines, "\n")
}

func parseStatusCode(s string) int {
	code := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		code = code*10 + int(c-'0')
	}
	return code
}
