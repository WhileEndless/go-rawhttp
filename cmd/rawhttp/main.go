// Command rawhttp is a curl-like command-line client built on top of the
// go-rawhttp library. It exposes the usual curl ergonomics (-X, -H, -d, -F,
// -L, -o, -w, -v, ...) plus the library's raw-HTTP superpowers: sending a
// verbatim raw request, SNI override/disable, direct-IP connect (--resolve /
// --connect-ip), every proxy type, explicit HTTP/2, mTLS, connection reuse and
// rich timing/TLS/connection metadata.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

// curl-compatible exit codes for the failure modes we can distinguish.
const (
	exitOK               = 0
	exitURLError         = 3
	exitCouldntResolve   = 6
	exitCouldntConnect   = 7
	exitOperationTimeout = 28
	exitTooManyRedirects = 47
	exitTLSError         = 60
	exitHTTPError        = 22
	exitGenericError     = 2
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitGenericError
	}

	if cfg.JSONOut && cfg.XMLOut {
		fmt.Fprintln(os.Stderr, "rawhttp: --json and --xml are mutually exclusive")
		return exitGenericError
	}

	if cfg.URL == "" && cfg.RawRequest == "" {
		fmt.Fprintln(os.Stderr, "rawhttp: no URL specified")
		fmt.Fprintln(os.Stderr, "Try 'rawhttp --help' for more information.")
		return exitGenericError
	}

	_, u, err := parseTarget(cfg.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		return exitURLError
	}

	// -m/--max-time bounds the whole transfer via context (the library has no
	// single total-timeout option).
	ctx := context.Background()
	var cancel context.CancelFunc
	if cfg.MaxTime > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTime*float64(time.Second)))
		defer cancel()
	}

	sender := rawhttp.NewSender()

	// Download manager mode: --download or -j/--parallel > 1.
	if cfg.Download || cfg.Parallel > 1 {
		return runDownload(ctx, sender, cfg, u)
	}

	tr := newTracer(cfg)

	res, err := DoWithRedirects(ctx, sender, cfg, u, tr)

	// Styled HTML report to a file (--html). Writes only to the file, not stdout.
	if cfg.HTMLOut != "" {
		code := emitHTMLReport(cfg, res, err)
		if res != nil {
			closeResp(res.resp)
		}
		return code
	}

	// Structured output (--json/--xml): emit the whole transaction — including the
	// request and the error — whether or not a response was received.
	if cfg.JSONOut || cfg.XMLOut {
		code := emitReport(cfg, res, err)
		if res != nil {
			closeResp(res.resp)
		}
		return code
	}

	if err != nil {
		reportError(cfg, err)
		return classifyError(err)
	}
	defer closeResp(res.resp)

	// -f/--fail: on an HTTP error, emit nothing and return curl's exit code 22.
	if cfg.Fail && res.resp.StatusCode >= 400 {
		if !cfg.Silent {
			fmt.Fprintf(os.Stderr, "rawhttp: the requested URL returned error: %d\n", res.resp.StatusCode)
		}
		return exitHTTPError
	}

	// Note a fallback: the user asked for HTTP/2 but the wire spoke something else
	// (--http2 falls back to HTTP/1.1 when the server doesn't support h2).
	if (cfg.HTTP2 || cfg.HTTP2Prior) && !cfg.Silent && !strings.Contains(res.resp.HTTPVersion, "2") {
		fmt.Fprintf(os.Stderr, "rawhttp: HTTP/2 not available, used %s\n", res.resp.HTTPVersion)
	}

	if err := writeOutput(cfg, res.resp, res.finalURL.String()); err != nil {
		if !cfg.Silent {
			fmt.Fprintf(os.Stderr, "rawhttp: %v\n", err)
		}
		return exitGenericError
	}

	if cfg.ShowTiming {
		printTimings(res.resp)
	}
	if cfg.WriteOut != "" {
		writeOut(cfg.WriteOut, res, cfg)
	}

	return exitOK
}

// reportError prints a clear, curl-like failure message. It is shown when not
// silent, OR when verbose is on (so `-s -v` still surfaces the cause instead of
// failing with no output at all).
func reportError(cfg *Config, err error) {
	if cfg.Silent && !cfg.Verbose {
		return
	}
	msg := describeError(err)
	if cfg.Verbose && msg != err.Error() {
		// In verbose mode include the underlying detail too.
		msg = fmt.Sprintf("%s (%v)", msg, err)
	}
	fmt.Fprintf(os.Stderr, "rawhttp: %s\n", msg)
}

// describeError maps a transport error to a human-friendly message.
func describeError(err error) string {
	switch {
	case rawhttp.IsTimeoutError(err) || errors.Is(err, context.DeadlineExceeded):
		return "operation timed out (no response from server)"
	case errors.Is(err, context.Canceled):
		return "request canceled"
	}
	switch rawhttp.GetErrorType(err) {
	case string(rawhttp.ErrorTypeDNS):
		return "could not resolve host"
	case string(rawhttp.ErrorTypeConnection):
		return "failed to connect to host"
	case string(rawhttp.ErrorTypeProxy):
		return "proxy connection failed"
	case string(rawhttp.ErrorTypeTLS):
		return "TLS/SSL handshake failed"
	case string(rawhttp.ErrorTypeProtocol):
		return "received no valid HTTP response (connection closed or malformed reply)"
	case string(rawhttp.ErrorTypeIO):
		return "connection error while reading the response"
	}
	return err.Error()
}

// classifyError maps library/transport errors onto curl-style exit codes.
func classifyError(err error) int {
	if errors.Is(err, errTooManyRedirects) {
		return exitTooManyRedirects
	}
	if rawhttp.IsTimeoutError(err) || errors.Is(err, context.DeadlineExceeded) {
		return exitOperationTimeout
	}
	switch rawhttp.GetErrorType(err) {
	case string(rawhttp.ErrorTypeDNS):
		return exitCouldntResolve
	case string(rawhttp.ErrorTypeConnection), string(rawhttp.ErrorTypeProxy):
		return exitCouldntConnect
	case string(rawhttp.ErrorTypeTLS):
		return exitTLSError
	}
	return exitGenericError
}
