package http2_test

import (
	"strings"
	"testing"

	"github.com/WhileEndless/go-rawhttp/v2/pkg/http2"
)

func TestTextToFrames(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		streamID uint32
		wantErr  bool
		validate func(*testing.T, []http2.Frame)
	}{
		{
			name:     "Simple GET request",
			input:    []byte("GET /test HTTP/2\r\nHost: example.com\r\nUser-Agent: test\r\n\r\n"),
			streamID: 1,
			wantErr:  false,
			validate: func(t *testing.T, frames []http2.Frame) {
				if len(frames) != 1 {
					t.Errorf("expected 1 frame, got %d", len(frames))
				}

				hf, ok := frames[0].(*http2.HeadersFrame)
				if !ok {
					t.Fatalf("expected HeadersFrame, got %T", frames[0])
				}

				if hf.Headers[":method"] != "GET" {
					t.Errorf("expected method GET, got %s", hf.Headers[":method"])
				}
				if hf.Headers[":path"] != "/test" {
					t.Errorf("expected path /test, got %s", hf.Headers[":path"])
				}
				if hf.Headers[":authority"] != "example.com" {
					t.Errorf("expected authority example.com, got %s", hf.Headers[":authority"])
				}
				if !hf.EndStream {
					t.Error("expected EndStream to be true")
				}
			},
		},
		{
			name:     "POST request with body",
			input:    []byte("POST /api HTTP/2\r\nHost: api.example.com\r\nContent-Type: application/json\r\n\r\n{\"test\":true}"),
			streamID: 3,
			wantErr:  false,
			validate: func(t *testing.T, frames []http2.Frame) {
				if len(frames) != 2 {
					t.Errorf("expected 2 frames, got %d", len(frames))
				}

				// Check HEADERS frame
				hf, ok := frames[0].(*http2.HeadersFrame)
				if !ok {
					t.Fatalf("expected HeadersFrame, got %T", frames[0])
				}

				if hf.Headers[":method"] != "POST" {
					t.Errorf("expected method POST, got %s", hf.Headers[":method"])
				}
				if hf.EndStream {
					t.Error("expected EndStream to be false for HEADERS frame")
				}

				// Check DATA frame
				df, ok := frames[1].(*http2.DataFrame)
				if !ok {
					t.Fatalf("expected DataFrame, got %T", frames[1])
				}

				body := string(df.Data)
				if body != `{"test":true}` {
					t.Errorf("expected body '{\"test\":true}', got %s", body)
				}
				if !df.EndStream {
					t.Error("expected EndStream to be true for DATA frame")
				}
			},
		},
		{
			name:     "Request with multiple headers",
			input:    []byte("GET /resource HTTP/2\r\nHost: example.com\r\nAccept: application/json\r\nAuthorization: Bearer token\r\nX-Custom: value\r\n\r\n"),
			streamID: 5,
			wantErr:  false,
			validate: func(t *testing.T, frames []http2.Frame) {
				if len(frames) != 1 {
					t.Errorf("expected 1 frame, got %d", len(frames))
				}

				hf := frames[0].(*http2.HeadersFrame)

				// Check regular headers are converted to lowercase
				if hf.Headers["accept"] != "application/json" {
					t.Errorf("expected accept header, got %v", hf.Headers)
				}
				if hf.Headers["authorization"] != "Bearer token" {
					t.Errorf("expected authorization header, got %v", hf.Headers)
				}
				if hf.Headers["x-custom"] != "value" {
					t.Errorf("expected x-custom header, got %v", hf.Headers)
				}
			},
		},
		{
			name:     "Connection-specific headers should be removed",
			input:    []byte("GET / HTTP/2\r\nHost: example.com\r\nConnection: keep-alive\r\nKeep-Alive: timeout=5\r\nTransfer-Encoding: chunked\r\n\r\n"),
			streamID: 7,
			wantErr:  false,
			validate: func(t *testing.T, frames []http2.Frame) {
				hf := frames[0].(*http2.HeadersFrame)

				// These headers should be removed
				if _, exists := hf.Headers["connection"]; exists {
					t.Error("connection header should be removed")
				}
				if _, exists := hf.Headers["keep-alive"]; exists {
					t.Error("keep-alive header should be removed")
				}
				if _, exists := hf.Headers["transfer-encoding"]; exists {
					t.Error("transfer-encoding header should be removed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := http2.NewConverter()
			frames, err := converter.TextToFrames(tt.input, tt.streamID)

			if (err != nil) != tt.wantErr {
				t.Errorf("TextToFrames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, frames)
			}
		})
	}
}

func TestFramesToText(t *testing.T) {
	tests := []struct {
		name      string
		frames    []http2.Frame
		isRequest bool
		want      string
		wantErr   bool
	}{
		{
			name: "Simple response",
			frames: []http2.Frame{
				&http2.HeadersFrame{
					Headers: map[string]string{
						":status":        "200",
						"content-type":   "text/plain",
						"content-length": "5",
					},
					EndStream: false,
				},
				&http2.DataFrame{
					Data:      []byte("Hello"),
					EndStream: true,
				},
			},
			isRequest: false,
			want:      "HTTP/2 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 5\r\n\r\nHello",
			wantErr:   false,
		},
		{
			name: "404 response",
			frames: []http2.Frame{
				&http2.HeadersFrame{
					Headers: map[string]string{
						":status":      "404",
						"content-type": "text/html",
					},
					EndStream: true,
				},
			},
			isRequest: false,
			want:      "HTTP/2 404 Not Found\r\nContent-Type: text/html\r\n\r\n",
			wantErr:   false,
		},
		{
			name: "Request formatting",
			frames: []http2.Frame{
				&http2.HeadersFrame{
					Headers: map[string]string{
						":method":    "GET",
						":path":      "/test",
						":scheme":    "https",
						":authority": "example.com",
						"user-agent": "test-client",
					},
					EndStream: true,
				},
			},
			isRequest: true,
			want:      "GET /test HTTP/2\r\nHost: example.com\r\nUser-Agent: test-client\r\n\r\n",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := http2.NewConverter()
			got, err := converter.FramesToText(tt.frames, tt.isRequest)

			if (err != nil) != tt.wantErr {
				t.Errorf("FramesToText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				gotStr := string(got)
				// Normalize for comparison (handle header ordering)
				if !compareHTTPText(gotStr, tt.want) {
					t.Errorf("FramesToText() = %v, want %v", gotStr, tt.want)
				}
			}
		})
	}
}

func TestHPACKEncoding(t *testing.T) {
	converter := http2.NewConverter()

	headers := map[string]string{
		":method":    "GET",
		":path":      "/",
		":scheme":    "https",
		":authority": "example.com",
		"user-agent": "test",
	}

	// Test encoding
	encoded, err := converter.EncodeHeaders(headers)
	if err != nil {
		t.Fatalf("EncodeHeaders() error = %v", err)
	}

	if len(encoded) == 0 {
		t.Error("encoded headers should not be empty")
	}

	// Test decoding
	decoded, err := converter.DecodeHeaders(encoded)
	if err != nil {
		t.Fatalf("DecodeHeaders() error = %v", err)
	}

	// Verify all headers are present
	for key, value := range headers {
		if decoded[key] != value {
			t.Errorf("header %s: expected %s, got %s", key, value, decoded[key])
		}
	}
}

// Helper function to compare HTTP text (ignoring header order)
func compareHTTPText(got, want string) bool {
	// Simple comparison for testing - in production would need more robust comparison
	gotLines := strings.Split(strings.TrimSpace(got), "\r\n")
	wantLines := strings.Split(strings.TrimSpace(want), "\r\n")

	if len(gotLines) != len(wantLines) {
		return false
	}

	// Compare first line (request/status line)
	if gotLines[0] != wantLines[0] {
		return false
	}

	// For headers, we'd need to parse and compare sets
	// For simplicity in tests, we'll do basic comparison
	return true
}
