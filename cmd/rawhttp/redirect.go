package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

// result bundles the final response with the redirect count for -w.
type result struct {
	resp         *rawhttp.Response
	numRedirects int
	finalURL     *url.URL
	reqHead      []string // final request head lines (request line + headers)
	reqBody      []byte   // final request body
}

// errTooManyRedirects signals that --max-redirs was exceeded (curl exit 47).
var errTooManyRedirects = fmt.Errorf("maximum number of redirects reached")

// DoWithRedirects executes the request and, when -L is set, follows 3xx
// responses. The library never auto-follows, so the loop rebuilds the request
// and options for each hop. cfg is cloned per hop so cross-host header drops do
// not leak back to the caller.
func DoWithRedirects(ctx context.Context, sender *rawhttp.Sender, cfg *Config, u *url.URL, tr *tracer) (*result, error) {
	cur := cloneConfig(cfg)
	curURL := u

	for redirects := 0; ; redirects++ {
		t, parsedURL, err := parseTarget(curURL.String())
		if err != nil {
			return nil, err
		}

		req, headLines, err := BuildRequest(cur, t, parsedURL)
		if err != nil {
			return nil, err
		}
		opts, err := BuildOptions(cur, t)
		if err != nil {
			return nil, err
		}

		reqBody := requestBodyBytes(req)
		resp, err := sender.Do(ctx, req, opts)
		if err != nil {
			// Show the request we attempted (HTTP/1.1 form, no wire info), then
			// return a partial result so callers (e.g. --json) can still report it.
			tr.requestLines(headLines, reqBody)
			return &result{numRedirects: redirects, finalURL: parsedURL, reqHead: headLines, reqBody: reqBody}, err
		}
		// Print after the exchange so the request can be shown in the protocol form
		// actually negotiated on the wire (known once the response is in):
		// connection/ALPN info, then the request, then the response.
		tr.connInfo(resp)
		tr.requestLines(requestView(headLines, resp.HTTPVersion), reqBody)
		tr.responseHead(resp)

		if !cur.FollowRedirects || !isRedirect(resp.StatusCode) {
			return &result{resp: resp, numRedirects: redirects, finalURL: parsedURL, reqHead: headLines, reqBody: reqBody}, nil
		}

		loc := firstHeader(resp.Headers, "Location")
		if loc == "" {
			return &result{resp: resp, numRedirects: redirects, finalURL: parsedURL, reqHead: headLines, reqBody: reqBody}, nil
		}

		if redirects >= cur.MaxRedirs {
			closeResp(resp)
			return nil, errTooManyRedirects
		}

		next, err := parsedURL.Parse(loc)
		if err != nil {
			closeResp(resp)
			return nil, fmt.Errorf("invalid redirect target %q: %w", loc, err)
		}

		applyRedirect(cur, resp.StatusCode, parsedURL, next)
		tr.redirect(next.String())
		closeResp(resp)
		curURL = next
	}
}

// applyRedirect mutates cfg for the next hop following curl semantics: 301/302/303
// turn non-GET/HEAD into GET (dropping the body), 307/308 preserve the method;
// cross-origin hops drop credentials.
func applyRedirect(cfg *Config, status int, from, to *url.URL) {
	switch status {
	case 301, 302, 303:
		if cfg.Method != "GET" && cfg.Method != "HEAD" {
			cfg.Method = "GET"
		}
		cfg.Data = nil
		cfg.DataBinary = nil
		cfg.DataRaw = nil
		cfg.Forms = nil
		cfg.Get = false
	}

	if !sameOrigin(from, to) {
		// Drop credentials when the origin changes (curl default).
		cfg.User = ""
		cfg.Cookie = ""
		cfg.Headers = dropHeaders(cfg.Headers, "authorization", "cookie")
		// A pinned --connect-ip was meant for the original host only; do not force
		// an unrelated redirect target onto it (correctness + SSRF safety).
		// --resolve/--connect-to are re-evaluated per hop against the new host.
		cfg.ConnectIP = ""
	}
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func isRedirect(code int) bool {
	switch code {
	case 301, 302, 303, 307, 308:
		return true
	}
	return false
}

// firstHeader returns the first value of a header, case-insensitively.
func firstHeader(h map[string][]string, name string) string {
	for k, v := range h {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func dropHeaders(headers []string, names ...string) []string {
	drop := map[string]bool{}
	for _, n := range names {
		drop[n] = true
	}
	var out []string
	for _, h := range headers {
		name, _, ok := splitHeader(h)
		if ok && drop[strings.ToLower(name)] {
			continue
		}
		out = append(out, h)
	}
	return out
}

// cloneConfig makes a shallow copy with independent slices so per-hop mutations
// do not affect the original.
func cloneConfig(cfg *Config) *Config {
	c := *cfg
	c.Headers = append([]string(nil), cfg.Headers...)
	c.Data = append([]string(nil), cfg.Data...)
	c.DataBinary = append([]string(nil), cfg.DataBinary...)
	c.DataRaw = append([]string(nil), cfg.DataRaw...)
	c.Forms = append([]string(nil), cfg.Forms...)
	return &c
}

func closeResp(resp *rawhttp.Response) {
	if resp == nil {
		return
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.Raw != nil {
		resp.Raw.Close()
	}
}
