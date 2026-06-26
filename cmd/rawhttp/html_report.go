package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	rawhttp "github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/cmd/rawhttp/render"
)

// htmlData is the template model for the --html report.
type htmlData struct {
	Tool, Version, Timestamp, URL string
	Light                         bool
	Success                       bool
	Error                         string
	Redirects                     int
	Req                           htmlMsg
	Resp                          *htmlMsg
	Stats                         []kv
}

type htmlMsg struct {
	StartLine   string
	Note        string // optional clarification under the start line
	StatusClass string // "ok"/"redir"/"err" for the response status line
	Headers     []htmlHeader
	Body        template.HTML
	HasBody     bool
	Raw         string // raw copy text (HTTP/1.1 form for the request)
	RawH2       string // request only: the HTTP/2 form, when h2 was negotiated
}

type htmlHeader struct{ Name, Value string }
type kv struct{ K, V string }

// emitHTMLReport builds a styled, syntax-highlighted HTML report and writes it to
// cfg.HTMLOut. Nothing is printed to stdout.
func emitHTMLReport(cfg *Config, res *result, reqErr error) int {
	max := cfg.MaxRenderSize
	if max <= 0 {
		max = defaultMaxRenderSize
	}
	light := strings.EqualFold(strings.TrimSpace(cfg.Theme), "light")
	style := cfg.Style
	if style == "" {
		style = render.StyleForTheme(cfg.Theme)
	}

	data := &htmlData{
		Tool:      "rawhttp",
		Version:   appVersion(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		URL:       cfg.URL,
		Light:     light,
		Success:   reqErr == nil,
	}
	negProto := ""
	if res != nil && res.resp != nil {
		negProto = res.resp.HTTPVersion
	}
	if res != nil {
		data.Redirects = res.numRedirects
		if res.finalURL != nil {
			data.URL = res.finalURL.String()
		}
		// Represent the request on the protocol it actually travelled on; for HTTP/2
		// the display and the Copy button both show the real h2 form.
		data.Req = buildHTMLRequest(res, max, style, negProto)
		if strings.Contains(negProto, "2") {
			data.Req.Note = "Shown as the HTTP/2 request sent on the wire (HTTP/2 line, lowercase headers, Host → :authority). rawhttp constructs requests in HTTP/1.1 form internally."
		}
	}
	if reqErr != nil {
		data.Error = describeError(reqErr)
	}
	if res != nil && res.resp != nil {
		m := buildHTMLResponse(res.resp, max, style)
		data.Resp = &m
		data.Stats = htmlStats(res.resp)
	}

	f, err := os.Create(cfg.HTMLOut)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitGenericError
	}
	defer f.Close()
	if err := htmlTemplate.Execute(f, data); err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: could not write HTML report: %v\n", err)
		return exitGenericError
	}
	if !cfg.Silent {
		fmt.Fprintf(os.Stderr, "rawhttp: wrote HTML report to %s\n", cfg.HTMLOut)
	}

	if reqErr != nil {
		return classifyError(reqErr)
	}
	if cfg.Fail && res.resp != nil && res.resp.StatusCode >= 400 {
		return exitHTTPError
	}
	return exitOK
}

func buildHTMLRequest(res *result, max int64, style, proto string) htmlMsg {
	view := requestView(res.reqHead, proto)
	m := htmlMsg{}
	if len(view) > 0 {
		m.StartLine = view[0]
		for _, h := range view[1:] {
			if n, v, ok := cutHeaderLine(h); ok {
				m.Headers = append(m.Headers, htmlHeader{n, v})
			}
		}
	}
	ct := headerValue(view, "Content-Type")
	m.Body, m.HasBody = bodyToHTML(res.reqBody, ct, style, max)
	// Copy buttons: always the HTTP/1.1 form (universally replayable); plus the
	// HTTP/2 form when the wire negotiated h2, so the user can take either.
	m.Raw = strings.Join(res.reqHead, "\r\n") + "\r\n\r\n" + string(res.reqBody)
	if strings.Contains(proto, "2") {
		m.RawH2 = strings.Join(toHTTP2View(res.reqHead), "\r\n") + "\r\n\r\n" + string(res.reqBody)
	}
	return m
}

func buildHTMLResponse(resp *rawhttp.Response, max int64, style string) htmlMsg {
	m := htmlMsg{StartLine: resp.StatusLine, StatusClass: statusClass(resp.StatusCode)}
	for _, h := range flattenHeaders(resp.Headers) {
		m.Headers = append(m.Headers, htmlHeader{h.Name, h.Value})
	}
	ct := firstHeader(resp.Headers, "Content-Type")
	body := readBounded(resp.Body, resp.BodyBytes, max)
	body = render.Decompress(body, firstHeader(resp.Headers, "Content-Encoding"), max)
	m.Body, m.HasBody = bodyToHTML(body, ct, style, max)
	// Copy text = head + DECOMPRESSED, UTF-8 body, so the copy button never yields
	// gzipped bytes and the HTML document stays valid UTF-8 (resp.Raw holds the
	// still-compressed body, which we deliberately avoid).
	m.Raw = strings.Join(extractHeadLines(resp), "\r\n") + "\r\n\r\n" + string(render.DecodeCharset(body, ct))
	return m
}

// bodyToHTML beautifies and syntax-highlights a body to inline-styled HTML, or
// returns a placeholder for binary/oversized bodies.
func bodyToHTML(body []byte, contentType, style string, max int64) (template.HTML, bool) {
	if len(body) == 0 {
		return "", false
	}
	if int64(len(body)) > max {
		return template.HTML(template.HTMLEscapeString(fmt.Sprintf("[%d bytes — too large to render]", len(body)))), true
	}
	if render.LooksBinary(body, contentType) {
		return template.HTML(template.HTMLEscapeString(fmt.Sprintf("[%d bytes of binary data not shown]", len(body)))), true
	}
	lang := render.DetectLang(contentType, body)
	pretty := render.Beautify(render.DecodeCharset(body, contentType), lang)
	return template.HTML(render.HighlightHTML(string(pretty), lang, style)), true
}

func htmlStats(resp *rawhttp.Response) []kv {
	var s []kv
	add := func(k, v string) {
		if v != "" {
			s = append(s, kv{k, v})
		}
	}
	add("Connected IP", resp.ConnectedIP)
	if resp.ConnectedPort != 0 {
		add("Port", fmt.Sprintf("%d", resp.ConnectedPort))
	}
	add("Protocol", resp.NegotiatedProtocol)
	add("HTTP version", resp.HTTPVersion)
	if resp.ConnectionReused {
		add("Connection", "reused")
	}
	add("TLS", strings.TrimSpace(resp.TLSVersion+" "+resp.TLSCipherSuite))
	add("TLS SNI", resp.TLSServerName)
	if resp.ProxyUsed {
		add("Proxy", resp.ProxyType+" "+resp.ProxyAddr)
	}
	m := resp.Timings
	add("DNS", fmtMs(m.DNSLookup))
	add("TCP connect", fmtMs(m.TCPConnect))
	add("TLS handshake", fmtMs(m.TLSHandshake))
	add("TTFB", fmtMs(m.TTFB))
	add("Total", fmtMs(m.TotalTime))
	return s
}

func fmtMs(d interface{ Seconds() float64 }) string {
	ms := d.Seconds() * 1000
	if ms <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f ms", ms)
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "ok"
	case code >= 300 && code < 400:
		return "redir"
	case code >= 400:
		return "err"
	default:
		return ""
	}
}

func headerValue(headLines []string, name string) string {
	for _, h := range headLines {
		if n, v, ok := strings.Cut(h, ":"); ok && strings.EqualFold(strings.TrimSpace(n), name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func readBounded(buf *rawhttp.Buffer, size, max int64) []byte {
	if buf == nil || size == 0 || size > max {
		return nil
	}
	r, err := buf.Reader()
	if err != nil {
		return nil
	}
	defer r.Close()
	data, _ := io.ReadAll(io.LimitReader(r, max))
	return data
}
