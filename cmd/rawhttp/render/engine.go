package render

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"image"
	// Register decoders so image.DecodeConfig can read dimensions.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"golang.org/x/text/encoding/htmlindex"
)

// Language keys used across detection, beautify and highlight.
const (
	LangJSON = "json"
	LangXML  = "xml"
	LangHTML = "html"
	LangJS   = "javascript"
	LangCSS  = "css"
	LangForm = "form"
	LangText = "text"
)

// DefaultMaxSize caps how much body the engine will buffer/transform.
const DefaultMaxSize = 5 << 20 // 5 MiB

// Options controls how a body is rendered.
type Options struct {
	Color       bool   // emit ANSI color (syntax highlight + structured colorizing)
	Beautify    bool   // reindent / pretty-print by content type
	Theme       string // "dark" (default) or "light" — selects a default style
	Style       string // explicit chroma style name (overrides Theme when set)
	MaxSize     int64  // max bytes to transform (0 => DefaultMaxSize)
	PrintBinary bool   // return raw bytes for binary bodies instead of a summary
	Decompress  bool   // decode Content-Encoding (gzip/deflate/br) first
}

func (o Options) maxSize() int64 {
	if o.MaxSize > 0 {
		return o.MaxSize
	}
	return DefaultMaxSize
}

// Style returns the effective chroma style: an explicit Style wins, otherwise a
// theme-appropriate default.
func (o Options) ResolvedStyle() string {
	if s := strings.TrimSpace(o.Style); s != "" {
		return s
	}
	return StyleForTheme(o.Theme)
}

// StyleForTheme returns a good chroma style for the given UI theme.
func StyleForTheme(theme string) string {
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		return "github"
	}
	return "monokai"
}

// Render transforms a response body for terminal display: optional decompression
// and charset decode, binary censoring, beautify and syntax highlight. The
// returned bytes are sanitized and ready to print. For a binary/image body it
// returns a labelled summary unless PrintBinary is set (then the raw bytes).
func Render(body []byte, contentType, contentEncoding string, opt Options) []byte {
	max := opt.maxSize()
	if opt.Decompress {
		body = Decompress(body, contentEncoding, max)
	}
	body = DecodeCharset(body, contentType)

	p := Painter{On: opt.Color}
	if LooksBinary(body, contentType) {
		if !opt.PrintBinary {
			return []byte(BinarySummary(body, contentType, p))
		}
		return body
	}

	text := Sanitize(string(body))
	lang := DetectLang(contentType, body)
	if opt.Beautify {
		text = string(Beautify([]byte(text), lang))
	}
	if opt.Color {
		if lang == LangForm {
			text = ColorizeForm(text, p)
		} else {
			text = Highlight(text, chromaLang(lang), opt.ResolvedStyle())
		}
	}
	return []byte(text)
}

// DetectLang maps a content-type (with a content-sniffing fallback) to a
// language key.
func DetectLang(contentType string, body []byte) string {
	ct := strings.ToLower(stripParams(contentType))
	switch {
	case strings.Contains(ct, "json"):
		return LangJSON
	case strings.Contains(ct, "xml"):
		return LangXML
	case strings.Contains(ct, "html"):
		return LangHTML
	case strings.Contains(ct, "javascript"), strings.Contains(ct, "ecmascript"):
		return LangJS
	case strings.Contains(ct, "css"):
		return LangCSS
	case strings.Contains(ct, "x-www-form-urlencoded"):
		return LangForm
	case strings.HasPrefix(ct, "text/"):
		return LangText
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 {
		head := trimmed
		if len(head) > 256 {
			head = head[:256]
		}
		switch trimmed[0] {
		case '{', '[':
			if jsonValid(trimmed) {
				return LangJSON
			}
		case '<':
			lower := bytes.ToLower(head)
			if bytes.Contains(lower, []byte("<html")) || bytes.Contains(lower, []byte("<!doctype html")) {
				return LangHTML
			}
			return LangXML
		}
	}
	return LangText
}

func chromaLang(lang string) string {
	switch lang {
	case LangForm, LangText:
		return ""
	default:
		return lang
	}
}

// ---- Content-Encoding decompression -------------------------------------

// Decompress decodes a Content-Encoding (gzip/deflate/br, possibly stacked) so
// the renderer operates on the real payload. Output is capped at limit bytes
// (decompression-bomb guard). On any failure it returns the input unchanged.
func Decompress(body []byte, contentEncoding string, limit int64) []byte {
	enc := strings.TrimSpace(strings.ToLower(contentEncoding))
	if enc == "" || enc == "identity" {
		return body
	}
	parts := strings.Split(enc, ",")
	cur := body
	for i := len(parts) - 1; i >= 0; i-- {
		dec, ok := decodeOne(cur, strings.TrimSpace(parts[i]), limit)
		if !ok {
			return body
		}
		cur = dec
	}
	return cur
}

func decodeOne(data []byte, name string, limit int64) ([]byte, bool) {
	var r io.Reader
	switch name {
	case "", "identity":
		return data, true
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		defer zr.Close()
		r = zr
	case "deflate":
		if zr, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
			defer zr.Close()
			r = zr
		} else {
			fr := flate.NewReader(bytes.NewReader(data))
			defer fr.Close()
			r = fr
		}
	case "br":
		r = brotli.NewReader(bytes.NewReader(data))
	default:
		return nil, false
	}
	out, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, false
	}
	return out, true
}

// DecompressStream wraps r with a streaming decompressor for the given
// Content-Encoding (gzip/deflate/br). Unknown/empty encodings (or an init error)
// return r as-is.
func DecompressStream(r io.Reader, contentEncoding string) io.Reader {
	switch strings.TrimSpace(strings.ToLower(contentEncoding)) {
	case "gzip", "x-gzip":
		if zr, err := gzip.NewReader(r); err == nil {
			return zr
		}
	case "deflate":
		if zr, err := zlib.NewReader(r); err == nil {
			return zr
		}
	case "br":
		return brotli.NewReader(r)
	}
	return r
}

// DecodeCharset converts body to UTF-8 based on the Content-Type charset param.
// UTF-8 / US-ASCII / unknown charsets pass through unchanged.
func DecodeCharset(body []byte, contentType string) []byte {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return body
	}
	cs := strings.ToLower(strings.TrimSpace(params["charset"]))
	switch cs {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return body
	}
	enc, err := htmlindex.Get(cs)
	if err != nil || enc == nil {
		return body
	}
	out, err := enc.NewDecoder().Bytes(body)
	if err != nil {
		return body
	}
	return out
}

// ---- Binary detection & summary -----------------------------------------

// LooksBinary reports whether a body should be treated as binary and therefore
// NOT printed raw to the terminal.
func LooksBinary(body []byte, contentType string) bool {
	ct := strings.ToLower(stripParams(contentType))
	if isTextualType(ct) {
		return false
	}
	if ct != "" && isBinaryType(ct) {
		return true
	}
	if ct == "" {
		sniff := strings.ToLower(stripParams(http.DetectContentType(body)))
		if strings.HasPrefix(sniff, "text/") || isTextualType(sniff) {
			return false
		}
		if isBinaryType(sniff) {
			return true
		}
	}
	sample := body
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	return !utf8.Valid(sample)
}

func isTextualType(ct string) bool {
	return strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "json") || strings.Contains(ct, "xml") ||
		strings.Contains(ct, "javascript") || strings.Contains(ct, "ecmascript") ||
		strings.Contains(ct, "html") || strings.Contains(ct, "css") ||
		strings.Contains(ct, "x-www-form-urlencoded") || strings.Contains(ct, "csv") ||
		strings.Contains(ct, "yaml")
}

func isBinaryType(ct string) bool {
	return strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") ||
		strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "font/") ||
		ct == "application/octet-stream" || strings.HasPrefix(ct, "application/pdf") ||
		strings.Contains(ct, "zip") || strings.Contains(ct, "gzip") ||
		strings.Contains(ct, "protobuf") || strings.Contains(ct, "msgpack") ||
		strings.Contains(ct, "font") || strings.Contains(ct, "octet")
}

// BinarySummary renders a concise, censored placeholder for a binary/image body.
// For images it includes the pixel dimensions (read from the header only).
func BinarySummary(body []byte, contentType string, p Painter) string {
	ct := stripParams(contentType)
	if ct == "" {
		ct = stripParams(http.DetectContentType(body))
	}
	dims := ""
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(body)); err == nil {
		dims = fmt.Sprintf(", %dx%d %s", cfg.Width, cfg.Height, strings.ToUpper(format))
	}
	msg := fmt.Sprintf("[%d bytes of %s binary data not shown%s]", len(body), ct, dims)
	hint := "use --print-binary to print it raw, or -o <file> to save it"
	return p.Warn(msg) + "\n" + p.Meta(hint) + "\n"
}

func stripParams(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}
