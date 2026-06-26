package render

import "strings"

// Best-effort ("lenient") re-indentation for malformed / truncated code.
//
// The strict formatters (encoding/json, esbuild) reject invalid input and we
// then fall back to these walkers, which insert real newlines and 2-space
// indentation at structural boundaries WITHOUT requiring valid syntax. They are
// string- and comment-aware and tolerate unbalanced brackets, so a truncated
// JSON/CSS/JS body still becomes readable instead of staying on one line.
//
// Existing whitespace is preserved (never dropped), so tokens are never merged
// (e.g. CSS "1px solid red" stays intact); only extra indentation is added.

type lenientConfig struct {
	openers      string // bracket openers that increase depth
	closers      string // matching closers
	separators   string // chars after which to break the line
	strDelims    string // string delimiters
	lineComment  bool   // // ... to end of line
	blockComment bool   // /* ... */
}

var (
	lenientJSONCfg = lenientConfig{openers: "{[", closers: "}]", separators: ",", strDelims: `"`}
	lenientCSSCfg  = lenientConfig{openers: "{", closers: "}", separators: ";", strDelims: `"'`, blockComment: true}
	// JS: break on {}, [] and ; only — not () so function params / calls stay inline.
	lenientJSCfg = lenientConfig{openers: "{[", closers: "}]", separators: ";", strDelims: "\"'`", lineComment: true, blockComment: true}
)

func matchingCloser(c lenientConfig, opener byte) byte {
	if i := strings.IndexByte(c.openers, opener); i >= 0 && i < len(c.closers) {
		return c.closers[i]
	}
	return 0
}

func lenientIndent(src []byte, c lenientConfig) []byte {
	var b strings.Builder
	b.Grow(len(src) + len(src)/4 + 16)
	depth := 0
	indent := func(d int) {
		b.WriteByte('\n')
		for i := 0; i < d; i++ {
			b.WriteString("  ")
		}
	}
	var strDelim byte
	esc := false

	for i := 0; i < len(src); i++ {
		ch := src[i]

		if strDelim != 0 { // inside a string
			b.WriteByte(ch)
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == strDelim:
				strDelim = 0
			}
			continue
		}

		// Comments — copy verbatim.
		if c.blockComment && ch == '/' && i+1 < len(src) && src[i+1] == '*' {
			if end := strings.Index(string(src[i+2:]), "*/"); end >= 0 {
				b.Write(src[i : i+2+end+2])
				i = i + 2 + end + 1
				continue
			}
			b.Write(src[i:])
			break
		}
		if c.lineComment && ch == '/' && i+1 < len(src) && src[i+1] == '/' {
			if end := strings.IndexByte(string(src[i:]), '\n'); end >= 0 {
				b.Write(src[i : i+end])
				i = i + end - 1
				continue
			}
			b.Write(src[i:])
			break
		}

		if strings.IndexByte(c.strDelims, ch) >= 0 {
			strDelim = ch
			b.WriteByte(ch)
			continue
		}

		switch {
		case strings.IndexByte(c.openers, ch) >= 0:
			if closer := matchingCloser(c, ch); closer != 0 {
				if j := nextNonSpaceIdx(src, i+1); j >= 0 && src[j] == closer {
					// Empty container — keep compact, verbatim.
					b.Write(src[i : j+1])
					i = j
					continue
				}
			}
			b.WriteByte(ch)
			depth++
			indent(depth)
		case strings.IndexByte(c.closers, ch) >= 0:
			if depth > 0 {
				depth--
			}
			indent(depth)
			b.WriteByte(ch)
		case strings.IndexByte(c.separators, ch) >= 0:
			b.WriteByte(ch)
			indent(depth)
		default:
			b.WriteByte(ch)
		}
	}
	return []byte(b.String())
}
