# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-06-26

First public release of **go-rawhttp** — a raw, socket-level HTTP client library
for Go (HTTP/1.1 and HTTP/2) plus a curl-compatible command-line client.

### Library

- **Raw HTTP/1.1 and HTTP/2.** Requests are written in familiar HTTP/1.1 text
  form and carried over either protocol. HTTP/2 supports true stream
  multiplexing, HPACK, flow control, and H2C (cleartext upgrade).
- **Connection pooling / Keep-Alive** with per-host idle pools, LIFO reuse,
  stale-connection detection with transparent retry on a fresh connection, and
  pool observability (`PoolStats`, per-host stats).
- **Upstream proxies** for both protocols — HTTP, HTTPS (CONNECT tunnel), SOCKS4,
  and SOCKS5 — with authentication, proxy-aware connection pooling, and a
  `ParseProxyURL` helper.
- **Full TLS control**: custom `tls.Config` passthrough, min/max TLS version,
  cipher-suite selection, SNI override / disable, custom CA roots, and mutual TLS
  (client certificates via PEM bytes or file paths).
- **Structured errors** classified by phase (DNS, connection, TLS, timeout,
  protocol, I/O) with operation context for smart retries.
- **Timing metrics**: DNS, TCP, TLS handshake, TTFB, and total time.
- **Memory-efficient bodies**: kept in memory up to a limit, then spilled to disk.

### CLI (`cmd/rawhttp`)

- **curl-compatible flags** (`-X -H -d --data-binary -F -A -e -b -u -G -I -L
  -o -O -s -v -i -k -m -x -w --http1.1 --http2 --resolve --cert --key --cacert`)
  plus raw-HTTP extras: `--raw-request`, `--sni`/`--disable-sni`,
  `--connect-ip`/`--connect-to`, `--reuse`, `--tls-min`/`--tls-max`, `--timings`.
- **Binary-safe request bodies** via `--data-hex` / `--data-base64` (both accept
  `@file`), the reliable way to send bodies containing NUL or other binary bytes.
- **Multi-connection download manager** (`--download` / `-j`): segmented range
  downloads with a live progress bar and automatic single-stream fallback.
- **Beautify + syntax highlighting + color**, on by default for a terminal:
  headers and structured fields colorized; bodies pretty-printed and highlighted
  by content type (JSON/XML/HTML/JS/CSS, including embedded `<script>`/`<style>`);
  `gzip`/`deflate`/`br` decompressed; binary/image bodies summarized. Output to a
  pipe or file stays raw.
- **Structured reports**: `--json` / `--xml` emit the whole transaction (request,
  response, connection, TLS, proxy, timing, and any error) as one document;
  `--html` writes a self-contained, highlighted report with copy buttons.
