package render

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-xmlfmt/xmlfmt"
	"github.com/yosssi/gohtml"
)

func jsonValid(b []byte) bool { return json.Valid(b) }

// Beautify reindents/pretty-prints the body for the given language. Unsupported
// languages and any error return the input unchanged.
//
// JSON/XML use the stdlib / a formatter; JavaScript and CSS are reformatted with
// esbuild (it parses minified code and re-prints it indented); HTML is reindented
// with gohtml and its embedded <script>/<style> blocks are then reformatted with
// esbuild and re-indented under their tag.
func Beautify(body []byte, lang string) []byte {
	switch lang {
	case LangJSON:
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err == nil {
			return buf.Bytes()
		}
		// Malformed/truncated JSON: indent best-effort so it stays readable.
		return lenientIndent(body, lenientJSONCfg)
	case LangXML:
		out := xmlfmt.FormatXML(string(body), "", "  ")
		return []byte(strings.TrimLeft(out, "\r\n"))
	case LangJS:
		return beautifyCode(body, api.LoaderJS, lenientJSCfg)
	case LangCSS:
		return beautifyCode(body, api.LoaderCSS, lenientCSSCfg)
	case LangHTML:
		return []byte(reindentEmbedded(gohtml.Format(string(body))))
	default:
		return body
	}
}

// beautifyCode reformats JavaScript or CSS with esbuild. On a parse error (or
// empty output) it falls back to a best-effort, syntax-tolerant re-indent so
// malformed/truncated code still becomes readable instead of staying minified.
func beautifyCode(body []byte, loader api.Loader, lenient lenientConfig) []byte {
	src := strings.TrimSpace(string(body))
	if src == "" {
		return body
	}
	res := api.Transform(src, api.TransformOptions{Loader: loader})
	if len(res.Errors) > 0 || len(res.Code) == 0 {
		return lenientIndent(body, lenient)
	}
	return bytes.TrimRight(res.Code, "\n")
}

// embeddedBlock matches a gohtml-formatted <script>/<style> block.
var embeddedBlock = regexp.MustCompile(`(?is)([ \t]*)(<(script|style)\b[^>]*>)\n(.*?)\n[ \t]*(</(?:script|style)>)`)

var scriptTypeRE = regexp.MustCompile(`(?i)\btype\s*=\s*["']?([^"'>\s]+)`)

// reindentEmbedded reformats the contents of every <script>/<style> block in
// gohtml-formatted HTML and re-indents the result one level under the opening
// tag. The right beautifier is chosen per block: CSS for <style>, JSON for
// <script type="...json">, JavaScript otherwise. Unparseable blocks are kept.
func reindentEmbedded(html string) string {
	return embeddedBlock.ReplaceAllStringFunc(html, func(m string) string {
		sub := embeddedBlock.FindStringSubmatch(m)
		if sub == nil {
			return m
		}
		indent, open, tagName, inner, closeTag := sub[1], sub[2], strings.ToLower(sub[3]), sub[4], sub[5]
		trimmed := strings.TrimSpace(inner)
		if trimmed == "" {
			return m
		}

		var beaut string
		switch {
		case tagName == "style":
			beaut = string(beautifyCode([]byte(trimmed), api.LoaderCSS, lenientCSSCfg))
		default:
			switch embeddedScriptKind(open) {
			case "json":
				beaut = string(Beautify([]byte(trimmed), LangJSON))
			case "css":
				beaut = string(beautifyCode([]byte(trimmed), api.LoaderCSS, lenientCSSCfg))
			default:
				beaut = string(beautifyCode([]byte(trimmed), api.LoaderJS, lenientJSCfg))
			}
		}

		var b strings.Builder
		b.WriteString(indent + open + "\n")
		for _, ln := range strings.Split(beaut, "\n") {
			if ln == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString(indent + "  " + ln + "\n")
		}
		b.WriteString(indent + closeTag)
		return b.String()
	})
}

func embeddedScriptKind(openTag string) string {
	mt := scriptTypeRE.FindStringSubmatch(openTag)
	if mt == nil {
		return "js"
	}
	t := strings.ToLower(mt[1])
	switch {
	case strings.Contains(t, "json"):
		return "json"
	case strings.Contains(t, "css"):
		return "css"
	default:
		return "js"
	}
}
