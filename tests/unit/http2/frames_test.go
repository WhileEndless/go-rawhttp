package http2_test

import (
	"bytes"
	"testing"

	"github.com/WhileEndless/go-rawhttp/pkg/http2"
	gohttp2 "golang.org/x/net/http2"
)

func TestRawFrameBuilder(t *testing.T) {
	builder := http2.NewRawFrameBuilder()

	t.Run("BuildDataFrame", func(t *testing.T) {
		payload := []byte("test data")
		frame := builder.BuildFrame(gohttp2.FrameData, gohttp2.FlagDataEndStream, 1, payload)

		// Verify frame structure
		if len(frame) < 9 {
			t.Fatalf("frame too short: %d bytes", len(frame))
		}

		// Check length field (first 3 bytes)
		length := uint32(frame[0])<<16 | uint32(frame[1])<<8 | uint32(frame[2])
		if length != uint32(len(payload)) {
			t.Errorf("expected length %d, got %d", len(payload), length)
		}

		// Check type
		if frame[3] != byte(gohttp2.FrameData) {
			t.Errorf("expected type %d, got %d", gohttp2.FrameData, frame[3])
		}

		// Check flags
		if frame[4] != byte(gohttp2.FlagDataEndStream) {
			t.Errorf("expected flags %d, got %d", gohttp2.FlagDataEndStream, frame[4])
		}

		// Check stream ID
		streamID := (uint32(frame[5])<<24 | uint32(frame[6])<<16 | uint32(frame[7])<<8 | uint32(frame[8])) & 0x7fffffff
		if streamID != 1 {
			t.Errorf("expected stream ID 1, got %d", streamID)
		}

		// Check payload
		if !bytes.Equal(frame[9:], payload) {
			t.Errorf("payload mismatch: expected %v, got %v", payload, frame[9:])
		}
	})

	t.Run("BuildSettingsFrame", func(t *testing.T) {
		settings := map[gohttp2.SettingID]uint32{
			gohttp2.SettingMaxConcurrentStreams: 100,
			gohttp2.SettingInitialWindowSize:    65535,
		}

		frame := builder.BuildSettingsFrame(settings, false)

		// Check frame header
		if frame[3] != byte(gohttp2.FrameSettings) {
			t.Errorf("expected SETTINGS frame type, got %d", frame[3])
		}

		// Stream ID should be 0 for SETTINGS
		streamID := (uint32(frame[5])<<24 | uint32(frame[6])<<16 | uint32(frame[7])<<8 | uint32(frame[8])) & 0x7fffffff
		if streamID != 0 {
			t.Errorf("SETTINGS frame should have stream ID 0, got %d", streamID)
		}

		// Check payload size (6 bytes per setting)
		expectedLen := len(settings) * 6
		length := uint32(frame[0])<<16 | uint32(frame[1])<<8 | uint32(frame[2])
		if length != uint32(expectedLen) {
			t.Errorf("expected payload length %d, got %d", expectedLen, length)
		}
	})

	t.Run("BuildPingFrame", func(t *testing.T) {
		data := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
		frame := builder.BuildPingFrame(data, false)

		// Check frame type
		if frame[3] != byte(gohttp2.FramePing) {
			t.Errorf("expected PING frame type, got %d", frame[3])
		}

		// Check payload (should be exactly 8 bytes)
		length := uint32(frame[0])<<16 | uint32(frame[1])<<8 | uint32(frame[2])
		if length != 8 {
			t.Errorf("PING frame should have 8 byte payload, got %d", length)
		}

		// Check data
		if !bytes.Equal(frame[9:17], data[:]) {
			t.Errorf("PING data mismatch")
		}
	})

	t.Run("BuildWindowUpdateFrame", func(t *testing.T) {
		frame := builder.BuildWindowUpdateFrame(3, 1024)

		// Check frame type
		if frame[3] != byte(gohttp2.FrameWindowUpdate) {
			t.Errorf("expected WINDOW_UPDATE frame type, got %d", frame[3])
		}

		// Check stream ID
		streamID := (uint32(frame[5])<<24 | uint32(frame[6])<<16 | uint32(frame[7])<<8 | uint32(frame[8])) & 0x7fffffff
		if streamID != 3 {
			t.Errorf("expected stream ID 3, got %d", streamID)
		}

		// Check increment value
		increment := (uint32(frame[9])<<24 | uint32(frame[10])<<16 | uint32(frame[11])<<8 | uint32(frame[12])) & 0x7fffffff
		if increment != 1024 {
			t.Errorf("expected increment 1024, got %d", increment)
		}
	})

	t.Run("BuildGoAwayFrame", func(t *testing.T) {
		debugData := []byte("connection error")
		frame := builder.BuildGoAwayFrame(7, uint32(gohttp2.ErrCodeProtocol), debugData)

		// Check frame type
		if frame[3] != byte(gohttp2.FrameGoAway) {
			t.Errorf("expected GOAWAY frame type, got %d", frame[3])
		}

		// Stream ID should be 0 for GOAWAY
		streamID := (uint32(frame[5])<<24 | uint32(frame[6])<<16 | uint32(frame[7])<<8 | uint32(frame[8])) & 0x7fffffff
		if streamID != 0 {
			t.Errorf("GOAWAY frame should have stream ID 0, got %d", streamID)
		}

		// Check last stream ID
		lastStreamID := (uint32(frame[9])<<24 | uint32(frame[10])<<16 | uint32(frame[11])<<8 | uint32(frame[12])) & 0x7fffffff
		if lastStreamID != 7 {
			t.Errorf("expected last stream ID 7, got %d", lastStreamID)
		}

		// Check error code
		errorCode := uint32(frame[13])<<24 | uint32(frame[14])<<16 | uint32(frame[15])<<8 | uint32(frame[16])
		if errorCode != uint32(gohttp2.ErrCodeProtocol) {
			t.Errorf("expected error code %d, got %d", gohttp2.ErrCodeProtocol, errorCode)
		}

		// Check debug data
		if !bytes.Equal(frame[17:], debugData) {
			t.Errorf("debug data mismatch")
		}
	})
}

func TestParseFrame(t *testing.T) {
	builder := http2.NewRawFrameBuilder()

	t.Run("ParseValidFrame", func(t *testing.T) {
		// Build a frame
		payload := []byte("test payload")
		frame := builder.BuildFrame(gohttp2.FrameData, gohttp2.FlagDataEndStream, 5, payload)

		// Parse it back
		header, parsedPayload, err := http2.ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame() error = %v", err)
		}

		// Verify header
		if header.Type != gohttp2.FrameData {
			t.Errorf("expected type %d, got %d", gohttp2.FrameData, header.Type)
		}
		if header.Flags != gohttp2.FlagDataEndStream {
			t.Errorf("expected flags %d, got %d", gohttp2.FlagDataEndStream, header.Flags)
		}
		if header.StreamID != 5 {
			t.Errorf("expected stream ID 5, got %d", header.StreamID)
		}

		// Verify payload
		if !bytes.Equal(parsedPayload, payload) {
			t.Errorf("payload mismatch: expected %v, got %v", payload, parsedPayload)
		}
	})

	t.Run("ParseInvalidFrame", func(t *testing.T) {
		// Too short frame
		_, _, err := http2.ParseFrame([]byte{1, 2, 3})
		if err == nil {
			t.Error("expected error for short frame")
		}

		// Incomplete frame
		frame := make([]byte, 9)
		frame[0] = 10 // Length = 10
		_, _, err = http2.ParseFrame(frame)
		if err == nil {
			t.Error("expected error for incomplete frame")
		}
	})
}
