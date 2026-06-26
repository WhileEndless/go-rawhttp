package render

import "strings"

// Virtual (recoverable) beautification.
//
// Unlike Beautify — which re-prints the input and therefore discards the
// original formatting — BeautifyVirtual is PURELY ADDITIVE: it inserts virtual
// "newline" and "indent" marker sequences at structural boundaries WITHOUT
// changing, removing or reordering any existing byte. Because only the markers
// are added, the exact original bytes are recovered by StripVirtual, and a
// human-readable form is produced by RenderVirtual.
//
// This is intended for callers that must keep the original bytes intact (e.g. to
// replay or re-sign a request) while still showing an indented view: format for
// display, then strip the markers to get the original back.
//
// The marker sequences MUST NOT occur in the input. The defaults use Unicode
// Private-Use-Area runes, which never appear in valid JSON/CSS source.
type VirtualConfig struct {
	Indent  string // one indent level
	Newline string // a line break
}

// DefaultVirtual returns markers that are safe for text/JSON/CSS (PUA runes).
func DefaultVirtual() VirtualConfig {
	return VirtualConfig{Indent: "", Newline: ""}
}

// BeautifyVirtual inserts virtual indentation markers for the given language
// without modifying any existing byte. Supported: json, css. Other languages
// (where an additive, recoverable transform is not safe — e.g. JavaScript) are
// returned unchanged.
//
// To preserve the recoverability guarantee absolutely, if the input already
// contains either marker (so stripping could not distinguish original from
// inserted), or the config is empty, the input is returned unchanged.
func BeautifyVirtual(src []byte, lang string, vc VirtualConfig) []byte {
	if vc.Indent == "" || vc.Newline == "" || vc.Indent == vc.Newline {
		return src
	}
	if bytesContains(src, vc.Indent) || bytesContains(src, vc.Newline) {
		return src
	}
	switch lang {
	case LangJSON:
		return virtualJSON(src, vc)
	case LangCSS:
		return virtualCSS(src, vc)
	default:
		return src
	}
}

func bytesContains(b []byte, s string) bool {
	return s != "" && strings.Contains(string(b), s)
}

// StripVirtual removes all virtual markers, recovering the exact original bytes.
func StripVirtual(b []byte, vc VirtualConfig) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, vc.Newline, "")
	s = strings.ReplaceAll(s, vc.Indent, "")
	return []byte(s)
}

// RenderVirtual replaces virtual markers with real whitespace for display:
// each Newline marker becomes "\n" and each Indent marker becomes indentUnit
// (e.g. "  " or "\t").
func RenderVirtual(b []byte, vc VirtualConfig, indentUnit string) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, vc.Newline, "\n")
	s = strings.ReplaceAll(s, vc.Indent, indentUnit)
	return []byte(s)
}

// nl writes a virtual newline followed by depth indent markers.
func nl(b *strings.Builder, vc VirtualConfig, depth int) {
	b.WriteString(vc.Newline)
	for i := 0; i < depth; i++ {
		b.WriteString(vc.Indent)
	}
}

func nextNonSpaceIdx(src []byte, from int) int {
	for j := from; j < len(src); j++ {
		switch src[j] {
		case ' ', '\t', '\r', '\n':
		default:
			return j
		}
	}
	return -1
}

// virtualJSON inserts markers around {}, [] and after commas, string-aware.
func virtualJSON(src []byte, vc VirtualConfig) []byte {
	var b strings.Builder
	depth := 0
	inStr, esc := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inStr {
			b.WriteByte(c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
			b.WriteByte(c)
		case '{', '[':
			closer := byte('}')
			if c == '[' {
				closer = ']'
			}
			if j := nextNonSpaceIdx(src, i+1); j >= 0 && src[j] == closer {
				// Empty container: emit verbatim (preserving original spacing).
				b.Write(src[i : j+1])
				i = j
				continue
			}
			b.WriteByte(c)
			depth++
			nl(&b, vc, depth)
		case '}', ']':
			depth--
			nl(&b, vc, depth)
			b.WriteByte(c)
		case ',':
			b.WriteByte(c)
			nl(&b, vc, depth)
		default:
			b.WriteByte(c)
		}
	}
	return []byte(b.String())
}

// virtualCSS inserts markers around {} blocks and after ; declarations,
// string- and comment-aware. Note: [...] in selectors is NOT treated as a block.
func virtualCSS(src []byte, vc VirtualConfig) []byte {
	var b strings.Builder
	depth := 0
	var strDelim byte // 0 when not in a string
	esc := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if strDelim != 0 {
			b.WriteByte(c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == strDelim:
				strDelim = 0
			}
			continue
		}
		// Block comment /* ... */ — copy verbatim.
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			end := strings.Index(string(src[i+2:]), "*/")
			if end < 0 {
				b.Write(src[i:])
				break
			}
			b.Write(src[i : i+2+end+2])
			i = i + 2 + end + 1
			continue
		}
		switch c {
		case '"', '\'':
			strDelim = c
			b.WriteByte(c)
		case '{':
			b.WriteByte(c)
			depth++
			nl(&b, vc, depth)
		case '}':
			depth--
			nl(&b, vc, depth)
			b.WriteByte(c)
			nl(&b, vc, depth)
		case ';':
			b.WriteByte(c)
			nl(&b, vc, depth)
		default:
			b.WriteByte(c)
		}
	}
	return []byte(b.String())
}
