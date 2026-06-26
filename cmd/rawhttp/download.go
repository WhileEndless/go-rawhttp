package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

// segment is one byte range [start, end] of the target file.
type segment struct {
	start int64
	end   int64
}

// downloadInfo is the result of probing the remote resource.
type downloadInfo struct {
	total        int64 // total size in bytes, -1 if unknown
	acceptRanges bool
}

// runDownload drives the segmented, multi-connection download manager. It is
// triggered by --download or -j/--parallel > 1.
func runDownload(ctx context.Context, sender *rawhttp.Sender, cfg *Config, u *url.URL) int {
	t, _, err := parseTarget(cfg.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitURLError
	}

	outName := downloadFilename(cfg, u)
	parallel := cfg.Parallel
	if parallel < 1 {
		parallel = 1
	}
	chunkSize, err := parseSize(cfg.ChunkSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitGenericError
	}

	// Resolve redirects (when -L) to the final URL before probing/segmenting, so
	// a download URL that 302s to a CDN works instead of failing on the 3xx.
	info, ft, fu, err := resolveAndProbe(ctx, sender, cfg, t, u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: download probe failed: %v\n", err)
		return classifyError(err)
	}
	t, u = ft, fu

	progressOn := !cfg.Silent && !cfg.NoProgress
	if !cfg.Silent {
		fmt.Fprintf(os.Stderr, "Downloading %s -> %s\n", cfg.URL, outName)
		if info.total >= 0 {
			fmt.Fprintf(os.Stderr, "Size: %s  Ranges: %v  Connections: %d\n",
				humanBytes(info.total), info.acceptRanges, effectiveConnections(info, parallel))
		}
	}

	f, err := os.Create(outName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitGenericError
	}
	defer f.Close()

	bar := newProgressBar(info.total, progressOn)
	bar.startRender()

	// No range support or unknown size → single streamed download.
	if !info.acceptRanges || info.total < 0 || parallel == 1 {
		err = downloadSingle(ctx, sender, cfg, t, u, f, bar)
	} else {
		if info.total > 0 {
			_ = f.Truncate(info.total)
		}
		err = downloadSegmented(ctx, sender, cfg, t, u, f, bar, info.total, chunkSize, parallel)
	}
	bar.finish()

	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: download failed: %v\n", err)
		return classifyError(err)
	}

	if !cfg.Silent {
		fmt.Fprintf(os.Stderr, "Saved %s (%s)\n", outName, humanBytes(atomicTotal(bar)))
	}
	return exitOK
}

// downloadSegmented splits the file into chunkSize segments and downloads them
// with a pool of `parallel` workers, each writing to its own file offset.
func downloadSegmented(ctx context.Context, sender *rawhttp.Sender, cfg *Config, t *target, u *url.URL,
	f *os.File, bar *progressBar, total, chunkSize int64, parallel int) error {

	segs := make(chan segment)
	var firstErr atomic.Value
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for seg := range segs {
			if firstErr.Load() != nil {
				return
			}
			if err := fetchSegment(ctx, sender, cfg, t, u, f, bar, seg); err != nil {
				firstErr.CompareAndSwap(nil, err)
				return
			}
		}
	}

	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		for start := int64(0); start < total; start += chunkSize {
			end := start + chunkSize - 1
			if end >= total {
				end = total - 1
			}
			segs <- segment{start: start, end: end}
		}
		close(segs)
	}()

	wg.Wait()
	if v := firstErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// fetchSegment downloads one byte range and writes it at the correct offset.
func fetchSegment(ctx context.Context, sender *rawhttp.Sender, cfg *Config, t *target, u *url.URL,
	f *os.File, bar *progressBar, seg segment) error {

	rangeHeader := fmt.Sprintf("bytes=%d-%d", seg.start, seg.end)
	resp, err := doRangeRequest(ctx, sender, cfg, t, u, rangeHeader)
	if err != nil {
		return err
	}
	defer closeResp(resp)

	// Only 206 is safe in segmented mode. A 200 means the server returned the full
	// body for a ranged request; writing it at this segment's offset would corrupt
	// the file, so fail loudly instead.
	if resp.StatusCode != 206 {
		return fmt.Errorf("server did not honour Range (status %d for %s); cannot segment", resp.StatusCode, rangeHeader)
	}

	r, err := resp.Body.Reader()
	if err != nil {
		return err
	}
	defer r.Close()

	ow := &offsetWriter{f: f, off: seg.start, bar: bar}
	_, err = io.Copy(ow, r)
	return err
}

// downloadSingle streams the whole resource in one request (used when the
// server does not support ranges or the user requested a single connection).
func downloadSingle(ctx context.Context, sender *rawhttp.Sender, cfg *Config, t *target, u *url.URL,
	f *os.File, bar *progressBar) error {

	resp, err := doRangeRequest(ctx, sender, cfg, t, u, "")
	if err != nil {
		return err
	}
	defer closeResp(resp)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	r, err := resp.Body.Reader()
	if err != nil {
		return err
	}
	defer r.Close()

	ow := &offsetWriter{f: f, off: 0, bar: bar}
	_, err = io.Copy(ow, r)
	return err
}

// doRangeRequest builds and sends a GET for the given Range (empty = full body),
// reusing the user's headers, proxy, TLS and protocol options.
func doRangeRequest(ctx context.Context, sender *rawhttp.Sender, cfg *Config, t *target, u *url.URL, rangeHeader string) (*rawhttp.Response, error) {
	rc := cloneConfig(cfg)
	rc.Method = "GET"
	rc.Get = false
	rc.Data = nil
	rc.DataBinary = nil
	rc.DataRaw = nil
	rc.Forms = nil
	rc.Head = false
	if rangeHeader != "" {
		rc.Headers = append(rc.Headers, "Range: "+rangeHeader)
	}

	req, _, err := BuildRequest(rc, t, u)
	if err != nil {
		return nil, err
	}
	opts, err := BuildOptions(rc, t)
	if err != nil {
		return nil, err
	}
	return sender.Do(ctx, req, opts)
}

// resolveAndProbe follows redirects (when -L) using lightweight Range probes and
// returns the final target plus its download info. The returned target/URL are
// what every subsequent range/segment request must use.
func resolveAndProbe(ctx context.Context, sender *rawhttp.Sender, cfg *Config, t *target, u *url.URL) (*downloadInfo, *target, *url.URL, error) {
	maxHops := cfg.MaxRedirs
	if maxHops <= 0 {
		maxHops = 50
	}
	for hop := 0; ; hop++ {
		resp, err := doRangeRequest(ctx, sender, cfg, t, u, "bytes=0-0")
		if err != nil {
			return nil, nil, nil, err
		}

		if cfg.FollowRedirects && isRedirect(resp.StatusCode) {
			loc := firstHeader(resp.Headers, "Location")
			if loc != "" {
				if hop >= maxHops {
					closeResp(resp)
					return nil, nil, nil, errTooManyRedirects
				}
				next, perr := u.Parse(loc)
				closeResp(resp)
				if perr != nil {
					return nil, nil, nil, fmt.Errorf("invalid redirect target %q: %w", loc, perr)
				}
				nt, nu, terr := parseTarget(next.String())
				if terr != nil {
					return nil, nil, nil, terr
				}
				t, u = nt, nu
				continue
			}
		}

		info := classifyProbe(resp)
		closeResp(resp)
		return info, t, u, nil
	}
}

// classifyProbe extracts size and range support from a probe response. Range
// support is trusted ONLY when the server actually answered the bytes=0-0 probe
// with 206 — a bare Accept-Ranges header is advisory and some servers still
// return a full 200 per range, which would corrupt a segmented download.
func classifyProbe(resp *rawhttp.Response) *downloadInfo {
	info := &downloadInfo{total: -1}

	switch resp.StatusCode {
	case 206:
		info.acceptRanges = true
		// Content-Range: bytes 0-0/12345
		if cr := firstHeader(resp.Headers, "Content-Range"); cr != "" {
			if i := strings.LastIndex(cr, "/"); i >= 0 {
				if n, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64); err == nil {
					info.total = n
				}
			}
		}
	default:
		// 200 (or anything else): the server did not honour our range probe, so we
		// must NOT segment. Use Content-Length only for the progress total.
		if cl := firstHeader(resp.Headers, "Content-Length"); cl != "" {
			if n, err := strconv.ParseInt(strings.TrimSpace(cl), 10, 64); err == nil {
				info.total = n
			}
		}
	}
	return info
}

// offsetWriter writes sequential data to a fixed file offset and reports
// progress to the bar.
type offsetWriter struct {
	f   *os.File
	off int64
	bar *progressBar
}

func (w *offsetWriter) Write(p []byte) (int, error) {
	n, err := w.f.WriteAt(p, w.off)
	w.off += int64(n)
	if w.bar != nil {
		w.bar.add(int64(n))
	}
	return n, err
}

// downloadFilename resolves the output path for a download.
func downloadFilename(cfg *Config, u *url.URL) string {
	if cfg.OutputFile != "" && cfg.OutputFile != "-" {
		return cfg.OutputFile
	}
	return remoteName(u.String())
}

func effectiveConnections(info *downloadInfo, parallel int) int {
	if !info.acceptRanges || info.total < 0 {
		return 1
	}
	return parallel
}

func atomicTotal(bar *progressBar) int64 {
	return atomic.LoadInt64(&bar.done)
}

// parseSize parses a human size like "512K", "4M", "1G" or a plain byte count.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 4 << 20, nil
	}
	mult := int64(1)
	switch last := s[len(s)-1]; last {
	case 'k', 'K':
		mult = 1 << 10
		s = s[:len(s)-1]
	case 'm', 'M':
		mult = 1 << 20
		s = s[:len(s)-1]
	case 'g', 'G':
		mult = 1 << 30
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid chunk size %q", s)
	}
	return n * mult, nil
}
