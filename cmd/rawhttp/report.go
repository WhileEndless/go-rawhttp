package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	rawhttp "github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/cmd/rawhttp/render"
)

// txReport is the structured record of a single request/response transaction,
// emitted by --json / --xml. It is designed to round-trip cleanly through both
// encoding/json and encoding/xml (no maps; headers are ordered key/value lists).
type txReport struct {
	XMLName   xml.Name    `json:"-" xml:"transaction"`
	Tool      string      `json:"tool" xml:"tool"`
	Version   string      `json:"version" xml:"version"`
	Timestamp string      `json:"timestamp" xml:"timestamp"`
	URL       string      `json:"url" xml:"url"`
	Success   bool        `json:"success" xml:"success"`
	Redirects int         `json:"redirects" xml:"redirects"`
	Error     string      `json:"error,omitempty" xml:"error,omitempty"`
	Request   *txRequest  `json:"request,omitempty" xml:"request,omitempty"`
	Response  *txResponse `json:"response,omitempty" xml:"response,omitempty"`
	TLS       *txTLS      `json:"tls,omitempty" xml:"tls,omitempty"`
	Proxy     *txProxy    `json:"proxy,omitempty" xml:"proxy,omitempty"`
	Conn      *txConn     `json:"connection,omitempty" xml:"connection,omitempty"`
	Timing    *txTiming   `json:"timing,omitempty" xml:"timing,omitempty"`
}

type txHeader struct {
	Name  string `json:"name" xml:"name,attr"`
	Value string `json:"value" xml:",chardata"`
}

type txBody struct {
	Size     int64  `json:"size" xml:"size,attr"`
	Encoding string `json:"encoding" xml:"encoding,attr"` // text | base64 | omitted | none
	Data     string `json:"data,omitempty" xml:",chardata"`
	Note     string `json:"note,omitempty" xml:"note,attr,omitempty"`
}

type txRequest struct {
	Method      string     `json:"method" xml:"method,attr"`
	Target      string     `json:"target" xml:"target,attr"`
	HTTPVersion string     `json:"httpVersion" xml:"httpVersion,attr"`
	Headers     []txHeader `json:"headers" xml:"headers>header"`
	Body        txBody     `json:"body" xml:"body"`
}

type txResponse struct {
	StatusCode  int        `json:"statusCode" xml:"statusCode,attr"`
	StatusLine  string     `json:"statusLine" xml:"statusLine,attr"`
	HTTPVersion string     `json:"httpVersion" xml:"httpVersion,attr"`
	Headers     []txHeader `json:"headers" xml:"headers>header"`
	Body        txBody     `json:"body" xml:"body"`
}

type txTLS struct {
	Version    string `json:"version,omitempty" xml:"version,attr,omitempty"`
	Cipher     string `json:"cipher,omitempty" xml:"cipher,attr,omitempty"`
	ServerName string `json:"serverName,omitempty" xml:"serverName,attr,omitempty"`
	Resumed    bool   `json:"resumed" xml:"resumed,attr"`
}

type txProxy struct {
	Used bool   `json:"used" xml:"used,attr"`
	Type string `json:"type,omitempty" xml:"type,attr,omitempty"`
	Addr string `json:"addr,omitempty" xml:"addr,attr,omitempty"`
}

type txConn struct {
	ConnectedIP        string `json:"connectedIP,omitempty" xml:"connectedIP,attr,omitempty"`
	ConnectedPort      int    `json:"connectedPort,omitempty" xml:"connectedPort,attr,omitempty"`
	LocalAddr          string `json:"localAddr,omitempty" xml:"localAddr,attr,omitempty"`
	RemoteAddr         string `json:"remoteAddr,omitempty" xml:"remoteAddr,attr,omitempty"`
	NegotiatedProtocol string `json:"negotiatedProtocol,omitempty" xml:"negotiatedProtocol,attr,omitempty"`
	Reused             bool   `json:"reused" xml:"reused,attr"`
}

type txTiming struct {
	DNSMs   float64 `json:"dnsMs" xml:"dnsMs,attr"`
	TCPMs   float64 `json:"tcpMs" xml:"tcpMs,attr"`
	TLSMs   float64 `json:"tlsMs" xml:"tlsMs,attr"`
	TTFBMs  float64 `json:"ttfbMs" xml:"ttfbMs,attr"`
	TotalMs float64 `json:"totalMs" xml:"totalMs,attr"`
}

// emitReport builds and writes the structured transaction report. res may be a
// partial result (request only) when the request failed. Returns the process
// exit code.
func emitReport(cfg *Config, res *result, reqErr error) int {
	report := buildReport(cfg, res, reqErr)

	var (
		out []byte
		err error
	)
	if cfg.XMLOut {
		out, err = xml.MarshalIndent(report, "", "  ")
		out = append([]byte(xml.Header), append(out, '\n')...)
	} else {
		out, err = json.MarshalIndent(report, "", "  ")
		out = append(out, '\n')
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: could not encode report: %v\n", err)
		return exitGenericError
	}

	w, _, closeFn, oerr := outputWriter(cfg, reportFinalURL(res))
	if oerr != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", oerr)
		return exitGenericError
	}
	defer closeFn()
	if _, werr := w.Write(out); werr != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", werr)
		return exitGenericError
	}

	if reqErr != nil {
		return classifyError(reqErr)
	}
	if cfg.Fail && res != nil && res.resp != nil && res.resp.StatusCode >= 400 {
		return exitHTTPError
	}
	return exitOK
}

func buildReport(cfg *Config, res *result, reqErr error) *txReport {
	max := cfg.MaxRenderSize
	if max <= 0 {
		max = defaultMaxRenderSize
	}

	rep := &txReport{
		Tool:      "rawhttp",
		Version:   appVersion(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		URL:       cfg.URL,
		Success:   reqErr == nil,
	}
	negProto := ""
	if res != nil && res.resp != nil {
		negProto = res.resp.HTTPVersion
	}
	if res != nil {
		rep.Redirects = res.numRedirects
		if res.finalURL != nil {
			rep.URL = res.finalURL.String()
		}
		if len(res.reqHead) > 0 {
			// Represent the request on the protocol it actually travelled on (for
			// HTTP/2 this is the h2 form: HTTP/2 line, lowercase headers, :authority).
			rep.Request = buildReqReport(requestView(res.reqHead, negProto), res.reqBody, max)
		}
	}
	if reqErr != nil {
		rep.Error = describeError(reqErr)
	}

	if res != nil && res.resp != nil {
		resp := res.resp
		rep.Response = buildRespReport(resp, max)
		rep.Conn = &txConn{
			ConnectedIP:        resp.ConnectedIP,
			ConnectedPort:      resp.ConnectedPort,
			LocalAddr:          resp.LocalAddr,
			RemoteAddr:         resp.RemoteAddr,
			NegotiatedProtocol: resp.NegotiatedProtocol,
			Reused:             resp.ConnectionReused,
		}
		if resp.TLSVersion != "" || resp.TLSCipherSuite != "" {
			rep.TLS = &txTLS{
				Version:    resp.TLSVersion,
				Cipher:     resp.TLSCipherSuite,
				ServerName: resp.TLSServerName,
				Resumed:    resp.TLSResumed,
			}
		}
		if resp.ProxyUsed {
			rep.Proxy = &txProxy{Used: true, Type: resp.ProxyType, Addr: resp.ProxyAddr}
		}
		m := resp.Timings
		rep.Timing = &txTiming{
			DNSMs:   ms(m.DNSLookup),
			TCPMs:   ms(m.TCPConnect),
			TLSMs:   ms(m.TLSHandshake),
			TTFBMs:  ms(m.TTFB),
			TotalMs: ms(m.TotalTime),
		}
	}
	return rep
}

func buildReqReport(headLines []string, body []byte, max int64) *txRequest {
	r := &txRequest{HTTPVersion: "HTTP/1.1"}
	if len(headLines) > 0 {
		fields := strings.SplitN(headLines[0], " ", 3)
		if len(fields) >= 1 {
			r.Method = fields[0]
		}
		if len(fields) >= 2 {
			r.Target = fields[1]
		}
		if len(fields) >= 3 {
			r.HTTPVersion = fields[2]
		}
		r.Headers = parseHeaderLines(headLines[1:])
	}
	r.Body = captureBody(body, max)
	return r
}

func buildRespReport(resp *rawhttp.Response, max int64) *txResponse {
	r := &txResponse{
		StatusCode:  resp.StatusCode,
		StatusLine:  resp.StatusLine,
		HTTPVersion: resp.HTTPVersion,
		Headers:     flattenHeaders(resp.Headers),
	}
	r.Body = captureResponseBody(resp, max)
	return r
}

// captureBody encodes a request body for the report, capping by size and
// base64-encoding non-text data so the document stays valid.
func captureBody(data []byte, max int64) txBody {
	if len(data) == 0 {
		return txBody{Size: 0, Encoding: "none"}
	}
	if int64(len(data)) > max {
		return txBody{Size: int64(len(data)), Encoding: "omitted", Note: "larger than max-render-size"}
	}
	if utf8.Valid(data) && !hasNUL(data) {
		return txBody{Size: int64(len(data)), Encoding: "text", Data: string(data)}
	}
	return txBody{Size: int64(len(data)), Encoding: "base64", Data: base64.StdEncoding.EncodeToString(data)}
}

func captureResponseBody(resp *rawhttp.Response, max int64) txBody {
	if resp.Body == nil || resp.BodyBytes == 0 {
		return txBody{Size: 0, Encoding: "none"}
	}
	if resp.BodyBytes > max {
		return txBody{Size: resp.BodyBytes, Encoding: "omitted", Note: "larger than max-render-size"}
	}
	r, err := resp.Body.Reader()
	if err != nil {
		return txBody{Size: resp.BodyBytes, Encoding: "omitted", Note: "could not read body"}
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, max))
	if err != nil {
		return txBody{Size: resp.BodyBytes, Encoding: "omitted", Note: "could not read body"}
	}
	data = render.Decompress(data, firstHeader(resp.Headers, "Content-Encoding"), max)
	return captureBody(data, max)
}

func parseHeaderLines(lines []string) []txHeader {
	out := make([]txHeader, 0, len(lines))
	for _, ln := range lines {
		if name, value, ok := cutHeaderLine(ln); ok {
			out = append(out, txHeader{Name: name, Value: value})
		}
	}
	return out
}

func flattenHeaders(h map[string][]string) []txHeader {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sortStrings(keys)
	out := make([]txHeader, 0, len(h))
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, txHeader{Name: k, Value: v})
		}
	}
	return out
}

func reportFinalURL(res *result) string {
	if res != nil && res.finalURL != nil {
		return res.finalURL.String()
	}
	return ""
}

func ms(d interface{ Seconds() float64 }) float64 {
	return d.Seconds() * 1000
}

func hasNUL(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
