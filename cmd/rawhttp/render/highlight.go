package render

import (
	"bytes"
	"context"
	"html"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// HighlightHTML returns a self-contained, inline-styled HTML fragment (a series
// of <span> elements) for the given language, suitable for embedding in a
// <pre>. On beautify the caller should pre-format the source. On any failure the
// HTML-escaped source is returned.
func HighlightHTML(src, lang, styleName string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return html.EscapeString(src)
	}
	f := chromahtml.New(chromahtml.WithClasses(false), chromahtml.PreventSurroundingPre(true))
	var buf bytes.Buffer
	if err := f.Format(&buf, style, it); err != nil {
		return html.EscapeString(src)
	}
	return buf.String()
}

// Highlight returns ANSI-colored source for the given chroma lexer name. It is
// deliberately defensive: a nil lexer falls back to plaintext, formatter panics
// are recovered, and a timeout guards against pathological backtracking. On any
// failure the original source is returned unchanged.
func Highlight(src, chromaLexer, styleName string) (out string) {
	defer func() {
		if recover() != nil {
			out = src
		}
	}()

	lexer := lexers.Get(chromaLexer)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	done := make(chan string, 1)
	go func() {
		defer func() {
			if recover() != nil {
				done <- src
			}
		}()
		it, err := lexer.Tokenise(nil, src)
		if err != nil {
			done <- src
			return
		}
		var buf bytes.Buffer
		if err := formatter.Format(&buf, style, it); err != nil {
			done <- src
			return
		}
		done <- buf.String()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	select {
	case r := <-done:
		return r
	case <-ctx.Done():
		return src
	}
}
