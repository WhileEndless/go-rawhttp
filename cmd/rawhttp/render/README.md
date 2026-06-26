# render

A self-contained terminal rendering layer for HTTP traffic. It:

- **Colorizes** HTTP headers and structured fields (status line, header
  name/value, `Set-Cookie`/`Cookie`, `Authorization`, `Location`, request-line
  query parameters, urlencoded form bodies) via a small `Painter`.
- **Beautifies + syntax-highlights** response bodies by content type: JSON, XML,
  HTML (including embedded `<script>` JS and `<style>` CSS, reformatted with
  esbuild and re-indented under their tag), JavaScript, CSS, and plain text.
- **Decompresses** `Content-Encoding` (gzip/deflate/br) — both buffered and
  streaming.
- **Safely summarizes binary/image bodies** instead of corrupting the terminal
  (with image dimensions read from the header only).
- **Tolerates malformed input**: when the strict formatter (encoding/json,
  esbuild) rejects truncated/broken JSON/CSS/JS, a syntax-tolerant best-effort
  re-indenter still makes it readable; highlighting (chroma) is best-effort too.
- Is **safe**: control/ESC characters are stripped from terminal-bound text,
  highlighting runs under a timeout, decompression output is size-capped, and
  every failure falls back to the original bytes.

It has **no dependency on any HTTP client or CLI** — callers pass raw bytes and
an `Options` value — so it is reusable and easy to split into its own module.

## Usage

```go
import "github.com/WhileEndless/go-rawhttp/cmd/rawhttp/render"

out := render.Render(body, contentType, contentEncoding, render.Options{
    Color:      true,
    Beautify:   true,
    Style:      "monokai", // chroma style
    MaxSize:    5 << 20,
    Decompress: true,
})
os.Stdout.Write(out)
```

Header/structured colorizing:

```go
p := render.Painter{On: true}
fmt.Println(render.ColorizeStatusLine("HTTP/1.1 200 OK", p))
fmt.Println(render.ColorizeHeaderLine("Set-Cookie: id=abc; Path=/", p))
```

Primitives are also exported: `Beautify`, `Highlight`, `DetectLang`,
`Decompress`, `DecompressStream`, `DecodeCharset`, `LooksBinary`,
`BinarySummary`, `Sanitize`.

## Dependencies

`chroma/v2` (highlight), `esbuild` (JS/CSS reformat), `go-xmlfmt` (XML),
`gohtml` (HTML), `andybalholm/brotli` (br), `golang.org/x/text` (charset).
