package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	rawhttp "github.com/WhileEndless/go-rawhttp"
)

// writeOut expands a curl-compatible -w format string and prints it to stdout.
// Supported variables map onto the library's Response metadata and Timings.
func writeOut(format string, res *result, cfg *Config) {
	resp := res.resp
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for i := 0; i < len(format); i++ {
		c := format[i]
		switch c {
		case '\\':
			if i+1 < len(format) {
				i++
				switch format[i] {
				case 'n':
					out.WriteString("\n")
				case 't':
					out.WriteString("\t")
				case 'r':
					out.WriteString("\r")
				case '\\':
					out.WriteString("\\")
				default:
					out.WriteString(string(format[i]))
				}
			}
		case '%':
			if i+1 < len(format) && format[i+1] == '%' {
				out.WriteString("%")
				i++
				continue
			}
			if i+1 < len(format) && format[i+1] == '{' {
				end := strings.IndexByte(format[i+2:], '}')
				if end >= 0 {
					name := format[i+2 : i+2+end]
					out.WriteString(writeOutVar(name, res, resp, cfg))
					i = i + 2 + end
					continue
				}
			}
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
}

// writeOutVar resolves a single %{name} variable.
func writeOutVar(name string, res *result, resp *rawhttp.Response, cfg *Config) string {
	m := resp.Timings
	switch name {
	case "http_code", "response_code":
		return fmt.Sprintf("%d", resp.StatusCode)
	case "http_version":
		return normalizeHTTPVersion(resp.HTTPVersion)
	case "scheme":
		if res.finalURL != nil {
			return strings.ToUpper(res.finalURL.Scheme)
		}
		return ""
	case "content_type":
		return firstHeader(resp.Headers, "Content-Type")
	case "num_redirects":
		return fmt.Sprintf("%d", res.numRedirects)
	case "num_connects":
		if resp.ConnectionReused {
			return "0"
		}
		return "1"
	case "size_download":
		return fmt.Sprintf("%d", resp.BodyBytes)
	case "remote_ip":
		return resp.ConnectedIP
	case "remote_port":
		return fmt.Sprintf("%d", resp.ConnectedPort)
	case "local_ip":
		ip, _ := splitHostPort(resp.LocalAddr)
		return ip
	case "local_port":
		_, port := splitHostPort(resp.LocalAddr)
		return port
	case "ssl_verify_result":
		if cfg.Insecure {
			return "1"
		}
		return "0"
	case "time_namelookup":
		return secs(m.DNSLookup)
	case "time_connect":
		return secs(m.DNSLookup + m.TCPConnect)
	case "time_appconnect":
		// curl reports 0 for plain HTTP (no TLS handshake stage).
		if resp.TLSVersion == "" {
			return "0.000000"
		}
		return secs(m.DNSLookup + m.TCPConnect + m.TLSHandshake)
	case "time_pretransfer":
		return secs(m.DNSLookup + m.TCPConnect + m.TLSHandshake)
	case "time_starttransfer":
		return secs(m.DNSLookup + m.TCPConnect + m.TLSHandshake + m.TTFB)
	case "time_total":
		return secs(m.TotalTime)
	default:
		if !cfg.Silent {
			fmt.Fprintf(os.Stderr, "rawhttp: warning: unknown --write-out variable '%s'\n", name)
		}
		return ""
	}
}

// printTimings writes a human-readable timing breakdown to stderr (--timings).
func printTimings(resp *rawhttp.Response) {
	m := resp.Timings
	fmt.Fprintln(os.Stderr, "* Timing breakdown:")
	fmt.Fprintf(os.Stderr, "*   DNS lookup:     %s\n", m.DNSLookup)
	fmt.Fprintf(os.Stderr, "*   TCP connect:    %s\n", m.TCPConnect)
	fmt.Fprintf(os.Stderr, "*   TLS handshake:  %s\n", m.TLSHandshake)
	fmt.Fprintf(os.Stderr, "*   TTFB:           %s\n", m.TTFB)
	fmt.Fprintf(os.Stderr, "*   Total:          %s\n", m.TotalTime)
}

func normalizeHTTPVersion(v string) string {
	switch {
	case strings.Contains(v, "2"):
		return "2"
	case strings.Contains(v, "1.1"):
		return "1.1"
	case strings.Contains(v, "1.0"):
		return "1.0"
	default:
		return v
	}
}

func splitHostPort(addr string) (host, port string) {
	if addr == "" {
		return "", ""
	}
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return h, p
}

// secs formats a duration as fractional seconds with 6 decimals, like curl.
func secs(d interface{ Seconds() float64 }) string {
	return fmt.Sprintf("%.6f", d.Seconds())
}
