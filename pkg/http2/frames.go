package http2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/net/http2"
)

// FrameHandler handles raw HTTP/2 frame operations
type FrameHandler struct {
	framer    *http2.Framer
	converter *Converter
}

// NewFrameHandler creates a new frame handler
func NewFrameHandler(rw io.ReadWriter) *FrameHandler {
	return &FrameHandler{
		framer:    http2.NewFramer(rw, rw),
		converter: NewConverter(),
	}
}

// SendFrame sends a single frame
func (h *FrameHandler) SendFrame(frame Frame) error {
	switch f := frame.(type) {
	case *HeadersFrame:
		return h.sendHeadersFrame(f)
	case *DataFrame:
		return h.sendDataFrame(f)
	default:
		return fmt.Errorf("unsupported frame type: %T", frame)
	}
}

// SendFrames sends multiple frames
func (h *FrameHandler) SendFrames(frames []Frame) error {
	for _, frame := range frames {
		if err := h.SendFrame(frame); err != nil {
			return err
		}
	}
	return nil
}

// ReadFrame reads a single frame
func (h *FrameHandler) ReadFrame() (Frame, error) {
	rawFrame, err := h.framer.ReadFrame()
	if err != nil {
		return nil, err
	}

	switch f := rawFrame.(type) {
	case *http2.HeadersFrame:
		return h.convertHeadersFrame(f)
	case *http2.DataFrame:
		return h.convertDataFrame(f)
	default:
		// Return as generic frame for other types
		return &GenericFrame{
			frameType: f.Header().Type,
			streamId:  f.Header().StreamID,
			flags:     f.Header().Flags,
			payload:   nil, // Could extract if needed
		}, nil
	}
}

// sendHeadersFrame sends a HEADERS frame
func (h *FrameHandler) sendHeadersFrame(frame *HeadersFrame) error {
	// Encode headers using HPACK
	encodedHeaders, err := h.converter.EncodeHeaders(frame.Headers)
	if err != nil {
		return fmt.Errorf("failed to encode headers: %w", err)
	}

	// Calculate frame size
	frameSize := len(encodedHeaders)
	if frame.Priority != nil {
		frameSize += 5 // Priority fields
	}
	if frame.PadLength > 0 {
		frameSize += int(frame.PadLength) + 1
	}

	// Write frame using raw frame method
	if err := h.framer.WriteRawFrame(http2.FrameHeaders, frame.Flags(), frame.StreamId, encodedHeaders); err != nil {
		return fmt.Errorf("failed to write headers frame: %w", err)
	}

	return nil
}

// sendDataFrame sends a DATA frame
func (h *FrameHandler) sendDataFrame(frame *DataFrame) error {
	// Calculate frame size
	frameSize := len(frame.Data)
	if frame.PadLength > 0 {
		frameSize += int(frame.PadLength) + 1
	}

	// Write frame
	if err := h.framer.WriteData(frame.StreamId, frame.EndStream, frame.Data); err != nil {
		return fmt.Errorf("failed to write data frame: %w", err)
	}

	return nil
}

// convertHeadersFrame converts http2.HeadersFrame to our HeadersFrame
func (h *FrameHandler) convertHeadersFrame(f *http2.HeadersFrame) (*HeadersFrame, error) {
	// Decode headers
	headers, err := h.converter.DecodeHeaders(f.HeaderBlockFragment())
	if err != nil {
		return nil, fmt.Errorf("failed to decode headers: %w", err)
	}

	frame := &HeadersFrame{
		StreamId:   f.StreamID,
		Headers:    headers,
		EndStream:  f.StreamEnded(),
		EndHeaders: f.HeadersEnded(),
	}

	// Extract priority if present
	if f.HasPriority() {
		priority := f.Priority
		frame.Priority = &PriorityParam{
			StreamDependency: priority.StreamDep,
			Exclusive:        priority.Exclusive,
			Weight:           priority.Weight,
		}
	}

	return frame, nil
}

// convertDataFrame converts http2.DataFrame to our DataFrame
func (h *FrameHandler) convertDataFrame(f *http2.DataFrame) (*DataFrame, error) {
	data := f.Data()

	return &DataFrame{
		StreamId:  f.StreamID,
		Data:      data,
		EndStream: f.StreamEnded(),
	}, nil
}

// GenericFrame represents a generic HTTP/2 frame
type GenericFrame struct {
	frameType http2.FrameType
	streamId  uint32
	flags     http2.Flags
	payload   []byte
}

func (f *GenericFrame) Type() http2.FrameType { return f.frameType }
func (f *GenericFrame) StreamID() uint32      { return f.streamId }
func (f *GenericFrame) Flags() http2.Flags    { return f.flags }
func (f *GenericFrame) Payload() []byte       { return f.payload }

// RawFrameBuilder builds raw HTTP/2 frames at the byte level
type RawFrameBuilder struct {
	buf bytes.Buffer
}

// NewRawFrameBuilder creates a new raw frame builder
func NewRawFrameBuilder() *RawFrameBuilder {
	return &RawFrameBuilder{}
}

// BuildFrame builds a raw frame from components
func (b *RawFrameBuilder) BuildFrame(frameType http2.FrameType, flags http2.Flags, streamID uint32, payload []byte) []byte {
	b.buf.Reset()

	// Frame header (9 bytes)
	header := make([]byte, 9)

	// Length (24 bits)
	length := uint32(len(payload))
	header[0] = byte(length >> 16)
	header[1] = byte(length >> 8)
	header[2] = byte(length)

	// Type (8 bits)
	header[3] = byte(frameType)

	// Flags (8 bits)
	header[4] = byte(flags)

	// Stream ID (31 bits, R bit = 0)
	binary.BigEndian.PutUint32(header[5:9], streamID&0x7fffffff)

	// Combine header and payload
	b.buf.Write(header)
	if len(payload) > 0 {
		b.buf.Write(payload)
	}

	return b.buf.Bytes()
}

// BuildSettingsFrame builds a SETTINGS frame
func (b *RawFrameBuilder) BuildSettingsFrame(settings map[http2.SettingID]uint32, ack bool) []byte {
	var payload bytes.Buffer

	for id, value := range settings {
		binary.Write(&payload, binary.BigEndian, uint16(id))
		binary.Write(&payload, binary.BigEndian, value)
	}

	flags := http2.Flags(0)
	if ack {
		flags = http2.FlagSettingsAck
	}

	return b.BuildFrame(http2.FrameSettings, flags, 0, payload.Bytes())
}

// BuildPingFrame builds a PING frame
func (b *RawFrameBuilder) BuildPingFrame(data [8]byte, ack bool) []byte {
	flags := http2.Flags(0)
	if ack {
		flags = http2.FlagPingAck
	}

	return b.BuildFrame(http2.FramePing, flags, 0, data[:])
}

// BuildWindowUpdateFrame builds a WINDOW_UPDATE frame
func (b *RawFrameBuilder) BuildWindowUpdateFrame(streamID uint32, increment uint32) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, increment&0x7fffffff)

	return b.BuildFrame(http2.FrameWindowUpdate, 0, streamID, payload)
}

// BuildGoAwayFrame builds a GOAWAY frame
func (b *RawFrameBuilder) BuildGoAwayFrame(lastStreamID uint32, errorCode uint32, debugData []byte) []byte {
	var payload bytes.Buffer

	binary.Write(&payload, binary.BigEndian, lastStreamID&0x7fffffff)
	binary.Write(&payload, binary.BigEndian, errorCode)
	if len(debugData) > 0 {
		payload.Write(debugData)
	}

	return b.BuildFrame(http2.FrameGoAway, 0, 0, payload.Bytes())
}

// ParseFrame parses a raw frame
func ParseFrame(data []byte) (*http2.FrameHeader, []byte, error) {
	if len(data) < 9 {
		return nil, nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}

	// Parse header
	length := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	frameType := http2.FrameType(data[3])
	flags := http2.Flags(data[4])
	streamID := binary.BigEndian.Uint32(data[5:9]) & 0x7fffffff

	header := &http2.FrameHeader{
		Length:   length,
		Type:     frameType,
		Flags:    flags,
		StreamID: streamID,
	}

	// Extract payload
	if len(data) < int(9+length) {
		return nil, nil, fmt.Errorf("incomplete frame: expected %d bytes, got %d", 9+length, len(data))
	}

	payload := data[9 : 9+length]

	return header, payload, nil
}
