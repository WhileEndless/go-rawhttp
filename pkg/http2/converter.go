package http2

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"golang.org/x/net/http2/hpack"
)

// Converter handles conversion between HTTP/1.1-style raw text and HTTP/2 frames
type Converter struct {
	encoder *hpack.Encoder
	decoder *hpack.Decoder
	encBuf  bytes.Buffer
	decBuf  bytes.Buffer
}

// NewConverter creates a new HTTP/1.1 to HTTP/2 converter
func NewConverter() *Converter {
	c := &Converter{}
	c.encoder = hpack.NewEncoder(&c.encBuf)
	c.encoder.SetMaxDynamicTableSize(4096)
	c.decoder = hpack.NewDecoder(4096, nil)
	return c
}

// TextToFrames converts HTTP/1.1-style raw request to HTTP/2 frames
func (c *Converter) TextToFrames(rawRequest []byte, streamID uint32) ([]Frame, error) {
	req, err := c.parseHTTP11Request(rawRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTP/1.1 request: %w", err)
	}

	// Create pseudo-headers
	pseudoHeaders := []hpack.HeaderField{
		{Name: ":method", Value: req.Method},
		{Name: ":path", Value: req.Path},
		{Name: ":scheme", Value: req.Scheme},
		{Name: ":authority", Value: req.Authority},
	}

	// Add regular headers (convert to lowercase)
	var regularHeaders []hpack.HeaderField
	for name, value := range req.Headers {
		lowerName := strings.ToLower(name)

		// Skip connection-specific headers
		if c.isConnectionSpecificHeader(lowerName) {
			continue
		}

		// Skip pseudo-headers that were already added
		if strings.HasPrefix(lowerName, ":") {
			continue
		}

		// Skip 'host' header - it's replaced by ':authority' in HTTP/2
		if lowerName == "host" {
			continue
		}

		regularHeaders = append(regularHeaders, hpack.HeaderField{
			Name:  lowerName,
			Value: value,
		})
	}

	// Combine pseudo-headers and regular headers
	allHeaders := append(pseudoHeaders, regularHeaders...)

	// Create HEADERS frame
	headerFrame := &HeadersFrame{
		StreamId:   streamID,
		Headers:    c.headerFieldsToMap(allHeaders),
		EndHeaders: true,
		EndStream:  len(req.Body) == 0,
	}

	frames := []Frame{headerFrame}

	// Create DATA frame if body exists
	if len(req.Body) > 0 {
		dataFrame := &DataFrame{
			StreamId:  streamID,
			Data:      req.Body,
			EndStream: true,
		}
		frames = append(frames, dataFrame)
	}

	return frames, nil
}

// FramesToText converts HTTP/2 frames to HTTP/1.1-style representation
func (c *Converter) FramesToText(frames []Frame, isRequest bool) ([]byte, error) {
	var buf bytes.Buffer
	var headers map[string]string
	var body []byte
	var status int
	var method, path, authority string

	for _, frame := range frames {
		switch f := frame.(type) {
		case *HeadersFrame:
			headers = f.Headers

			// Extract pseudo-headers
			if isRequest {
				method = headers[":method"]
				path = headers[":path"]
				authority = headers[":authority"]
			} else {
				if statusStr, ok := headers[":status"]; ok {
					status, _ = strconv.Atoi(statusStr)
				}
			}

		case *DataFrame:
			body = append(body, f.Data...)
		}
	}

	if isRequest {
		// Format as HTTP/1.1-style request
		buf.WriteString(fmt.Sprintf("%s %s HTTP/2\r\n", method, path))
		buf.WriteString(fmt.Sprintf("Host: %s\r\n", authority))

		// Add regular headers
		for name, value := range headers {
			if !strings.HasPrefix(name, ":") && !c.isConnectionSpecificHeader(name) {
				buf.WriteString(fmt.Sprintf("%s: %s\r\n", c.normalizeHeaderName(name), value))
			}
		}

		buf.WriteString("\r\n")
		if len(body) > 0 {
			buf.Write(body)
		}
	} else {
		// Format as HTTP/1.1-style response
		statusText := http.StatusText(status)
		if statusText == "" {
			statusText = "Unknown"
		}
		buf.WriteString(fmt.Sprintf("HTTP/2 %d %s\r\n", status, statusText))

		// Add regular headers
		for name, value := range headers {
			if !strings.HasPrefix(name, ":") && !c.isConnectionSpecificHeader(name) {
				buf.WriteString(fmt.Sprintf("%s: %s\r\n", c.normalizeHeaderName(name), value))
			}
		}

		buf.WriteString("\r\n")
		if len(body) > 0 {
			buf.Write(body)
		}
	}

	return buf.Bytes(), nil
}

// EncodeHeaders encodes headers using HPACK
func (c *Converter) EncodeHeaders(headers map[string]string) ([]byte, error) {
	// If we have our own encoder, use it with our buffer
	if c.encoder != nil {
		c.encBuf.Reset()

		// Encode pseudo-headers first (in order)
		pseudoOrder := []string{":method", ":path", ":scheme", ":authority", ":status"}
		for _, name := range pseudoOrder {
			if value, ok := headers[name]; ok {
				err := c.encoder.WriteField(hpack.HeaderField{Name: name, Value: value})
				if err != nil {
					return nil, err
				}
			}
		}

		// Encode regular headers
		for name, value := range headers {
			if !strings.HasPrefix(name, ":") {
				err := c.encoder.WriteField(hpack.HeaderField{Name: strings.ToLower(name), Value: value})
				if err != nil {
					return nil, err
				}
			}
		}

		return c.encBuf.Bytes(), nil
	}

	// Otherwise, encode headers directly with a new buffer
	var buf bytes.Buffer
	encoder := hpack.NewEncoder(&buf)

	// Encode pseudo-headers first (in order)
	pseudoOrder := []string{":method", ":path", ":scheme", ":authority", ":status"}
	for _, name := range pseudoOrder {
		if value, ok := headers[name]; ok {
			err := encoder.WriteField(hpack.HeaderField{Name: name, Value: value})
			if err != nil {
				return nil, err
			}
		}
	}

	// Encode regular headers
	for name, value := range headers {
		if !strings.HasPrefix(name, ":") {
			err := encoder.WriteField(hpack.HeaderField{Name: strings.ToLower(name), Value: value})
			if err != nil {
				return nil, err
			}
		}
	}

	return buf.Bytes(), nil
}

// DecodeHeaders decodes HPACK-encoded headers
func (c *Converter) DecodeHeaders(data []byte) (map[string]string, error) {
	headers := make(map[string]string)
	fields, err := c.decoder.DecodeFull(data)
	if err != nil {
		return nil, err
	}

	for _, field := range fields {
		headers[field.Name] = field.Value
	}

	return headers, nil
}

// ParseHTTP11Request parses a raw HTTP/1.1 request (public for debugging)
func (c *Converter) ParseHTTP11Request(rawRequest []byte) (*Request, error) {
	return c.parseHTTP11Request(rawRequest)
}

// parseHTTP11Request parses an HTTP/1.1-style raw request
func (c *Converter) parseHTTP11Request(raw []byte) (*Request, error) {
	reader := bufio.NewReader(bytes.NewReader(raw))

	// Parse request line
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read request line: %w", err)
	}

	requestLine = strings.TrimSpace(requestLine)
	parts := strings.Fields(requestLine)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid request line: %s", requestLine)
	}

	method := parts[0]
	path := parts[1]

	// Parse headers
	tp := textproto.NewReader(reader)
	mimeHeaders, err := tp.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	headers := make(map[string]string)
	for name, values := range mimeHeaders {
		headers[name] = values[0] // Take first value for simplicity
	}

	// Extract authority from Host header
	authority := headers["Host"]
	if authority == "" {
		authority = "localhost"
	}

	// Determine scheme (default to https for HTTP/2)
	reqScheme := "https"
	if xScheme, ok := headers["X-Scheme"]; ok {
		reqScheme = xScheme
		delete(headers, "X-Scheme")
	}

	// Read body
	body, _ := io.ReadAll(reader)

	return &Request{
		Method:    method,
		Path:      path,
		Authority: authority,
		Scheme:    reqScheme,
		Headers:   headers,
		Body:      body,
		RawText:   raw,
	}, nil
}

// isConnectionSpecificHeader checks if a header is connection-specific
func (c *Converter) isConnectionSpecificHeader(name string) bool {
	connectionHeaders := []string{
		"connection",
		"keep-alive",
		"proxy-connection",
		"transfer-encoding",
		"upgrade",
		"te", // except "trailers"
	}

	for _, h := range connectionHeaders {
		if name == h {
			return true
		}
	}

	return false
}

// headerFieldsToMap converts HPACK header fields to a map
func (c *Converter) headerFieldsToMap(fields []hpack.HeaderField) map[string]string {
	headers := make(map[string]string)
	for _, field := range fields {
		headers[field.Name] = field.Value
	}
	return headers
}

// normalizeHeaderName capitalizes header names for display
func (c *Converter) normalizeHeaderName(name string) string {
	return textproto.CanonicalMIMEHeaderKey(name)
}
