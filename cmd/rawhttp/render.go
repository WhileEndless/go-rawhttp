package main

import (
	"fmt"
	"io"
	"os"

	rawhttp "github.com/WhileEndless/go-rawhttp"
	"github.com/WhileEndless/go-rawhttp/cmd/rawhttp/render"
)

const defaultMaxRenderSize = render.DefaultMaxSize

// renderBody writes the response body to w, delegating beautify/highlight/binary
// handling to the render package. It keeps the streaming + size-gate policy here
// (the render package operates on a bounded byte slice).
func renderBody(cfg *Config, resp *rawhttp.Response, w io.Writer, toFile, color, beautifyOn bool) error {
	if resp.Body == nil {
		return nil
	}

	// Fast raw path: writing to a file, or no rendering requested at all.
	if toFile || (!color && !beautifyOn) {
		return streamRaw(cfg, resp, w)
	}

	max := cfg.MaxRenderSize
	if max <= 0 {
		max = defaultMaxRenderSize
	}
	if resp.BodyBytes > max {
		if !cfg.Silent {
			fmt.Fprintf(os.Stderr, "rawhttp: body too large to render (%s); printing raw\n", humanBytes(resp.BodyBytes))
		}
		return streamRaw(cfg, resp, w)
	}

	r, err := resp.Body.Reader()
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(r, max))
	r.Close()
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}

	out := render.Render(body,
		firstHeader(resp.Headers, "Content-Type"),
		firstHeader(resp.Headers, "Content-Encoding"),
		render.Options{
			Color:       color,
			Beautify:    beautifyOn,
			Theme:       cfg.Theme,
			Style:       cfg.Style,
			MaxSize:     max,
			PrintBinary: cfg.PrintBinary,
			Decompress:  cfg.compressEnabled(),
		})

	if _, err := w.Write(out); err != nil {
		return err
	}
	if len(out) > 0 && out[len(out)-1] != '\n' {
		_, _ = io.WriteString(w, "\n")
	}
	return nil
}

func streamRaw(cfg *Config, resp *rawhttp.Response, w io.Writer) error {
	r, err := resp.Body.Reader()
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}
	defer r.Close()
	var src io.Reader = r
	if cfg.compressEnabled() {
		// Hand back decompressed bytes even in raw output (browser/--compressed).
		src = render.DecompressStream(r, firstHeader(resp.Headers, "Content-Encoding"))
	}
	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("could not write response body: %w", err)
	}
	return nil
}

// compressEnabled reports whether to advertise Accept-Encoding and decompress
// the response. On by default (browser-like); --no-compressed turns it off.
func (c *Config) compressEnabled() bool {
	return c.Compressed && !c.NoCompressed
}
