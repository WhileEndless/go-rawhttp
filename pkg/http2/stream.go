package http2

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// StreamManager manages HTTP/2 streams
type StreamManager struct {
	streams       map[uint32]*Stream
	nextStreamID  uint32
	maxConcurrent uint32
	mu            sync.RWMutex
}

// NewStreamManager creates a new stream manager
func NewStreamManager(maxConcurrent uint32) *StreamManager {
	return &StreamManager{
		streams:       make(map[uint32]*Stream),
		nextStreamID:  1, // Client streams use odd IDs
		maxConcurrent: maxConcurrent,
	}
}

// NewStream creates a new stream
func (m *StreamManager) NewStream(request *Request) (*Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check total stream count to prevent resource exhaustion
	const maxTotalStreams = 10000 // Reasonable limit
	if len(m.streams) >= maxTotalStreams {
		// Cleanup old closed streams first
		m.cleanupClosedStreamsLocked()
		if len(m.streams) >= maxTotalStreams {
			return nil, fmt.Errorf("maximum total streams (%d) reached", maxTotalStreams)
		}
	}

	// Check concurrent stream limit
	activeCount := 0
	for _, stream := range m.streams {
		if stream.State == StateOpen || stream.State == StateHalfClosedLocal {
			activeCount++
		}
	}

	if uint32(activeCount) >= m.maxConcurrent {
		return nil, fmt.Errorf("maximum concurrent streams (%d) reached", m.maxConcurrent)
	}

	// Check for stream ID exhaustion (DEF-6)
	// Stream IDs are uint32, max value is 2^31-1 for client-initiated streams
	if m.nextStreamID > (1<<31 - 1) {
		return nil, fmt.Errorf("stream ID exhausted (max 2^31-1), connection must be recreated")
	}

	// Allocate stream ID
	streamID := m.nextStreamID
	m.nextStreamID += 2 // Client uses odd stream IDs

	// Create stream
	stream := &Stream{
		ID:             streamID,
		State:          StateIdle,
		Request:        request,
		WindowSize:     65535, // Default window size
		PeerWindowSize: 65535,
	}

	m.streams[streamID] = stream

	return stream, nil
}

// GetStream retrieves a stream by ID
func (m *StreamManager) GetStream(streamID uint32) (*Stream, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[streamID]
	return stream, exists
}

// GetStreamState returns the current state of a stream (BUG-7: thread-safe access)
func (m *StreamManager) GetStreamState(streamID uint32) (StreamState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return StateIdle, fmt.Errorf("stream %d not found", streamID)
	}

	return stream.State, nil
}

// UpdateStreamState updates the state of a stream
func (m *StreamManager) UpdateStreamState(streamID uint32, newState StreamState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return fmt.Errorf("stream %d not found", streamID)
	}

	// Validate state transition
	if !isValidStateTransition(stream.State, newState) {
		return fmt.Errorf("invalid state transition from %v to %v for stream %d",
			stream.State, newState, streamID)
	}

	stream.State = newState

	// Mark stream as closed if reaching closed state
	if newState == StateClosed {
		stream.Closed = true
	}

	return nil
}

// UpdateWindowSize updates the flow control window for a stream
func (m *StreamManager) UpdateWindowSize(streamID uint32, increment int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if streamID == 0 {
		// Connection-level window update
		for _, stream := range m.streams {
			stream.PeerWindowSize += increment
		}
		return nil
	}

	stream, exists := m.streams[streamID]
	if !exists {
		return fmt.Errorf("stream %d not found", streamID)
	}

	newSize := stream.WindowSize + increment
	if newSize > 2147483647 { // Max int32
		return fmt.Errorf("window size overflow for stream %d", streamID)
	}

	stream.WindowSize = newSize

	return nil
}

// CloseStream closes a stream
func (m *StreamManager) CloseStream(streamID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return fmt.Errorf("stream %d not found", streamID)
	}

	stream.State = StateClosed
	stream.Closed = true

	// Don't immediately delete the stream - keep it for a while
	// to handle late frames (as per HTTP/2 spec)

	return nil
}

// CleanupClosedStreams removes closed streams that are safe to delete
func (m *StreamManager) CleanupClosedStreams() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupClosedStreamsLocked()
}

// cleanupClosedStreamsLocked removes closed streams (must be called with lock held)
func (m *StreamManager) cleanupClosedStreamsLocked() {
	now := time.Now()
	for id, stream := range m.streams {
		// Remove closed streams that have been closed for a while
		// or streams that never properly opened
		if stream.Closed && stream.State == StateClosed {
			delete(m.streams, id)
		} else if stream.State == StateIdle && stream.Request != nil {
			// Remove idle streams that never progressed (likely abandoned)
			delete(m.streams, id)
		}
	}
	_ = now // Will be used for timeout-based cleanup in next iteration
}

// GetActiveStreams returns all active streams
func (m *StreamManager) GetActiveStreams() []*Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*Stream
	for _, stream := range m.streams {
		if !stream.Closed {
			active = append(active, stream)
		}
	}

	return active
}

// Reset resets a stream with an error code
func (m *StreamManager) Reset(streamID uint32, errorCode http2.ErrCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return fmt.Errorf("stream %d not found", streamID)
	}

	stream.State = StateClosed
	stream.Closed = true

	return nil
}

// isValidStateTransition checks if a state transition is valid
func isValidStateTransition(from, to StreamState) bool {
	switch from {
	case StateIdle:
		return to == StateReservedLocal || to == StateReservedRemote ||
			to == StateOpen || to == StateClosed
	case StateReservedLocal:
		return to == StateHalfClosedRemote || to == StateClosed
	case StateReservedRemote:
		return to == StateHalfClosedLocal || to == StateClosed
	case StateOpen:
		return to == StateHalfClosedLocal || to == StateHalfClosedRemote ||
			to == StateClosed
	case StateHalfClosedLocal:
		return to == StateClosed
	case StateHalfClosedRemote:
		return to == StateClosed
	case StateClosed:
		return false // No transitions from closed state
	default:
		return false
	}
}

// StreamProcessor processes frames for streams
type StreamProcessor struct {
	manager   *StreamManager
	converter *Converter
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(manager *StreamManager) *StreamProcessor {
	return &StreamProcessor{
		manager:   manager,
		converter: NewConverter(),
	}
}

// ProcessHeadersFrame processes a HEADERS frame
func (p *StreamProcessor) ProcessHeadersFrame(frame *HeadersFrame) error {
	stream, exists := p.manager.GetStream(frame.StreamId)
	if !exists {
		// Create new stream for server-initiated streams
		stream = &Stream{
			ID:    frame.StreamId,
			State: StateOpen,
		}
		p.manager.mu.Lock()
		p.manager.streams[frame.StreamId] = stream
		p.manager.mu.Unlock()
	}

	// Create response if not exists
	if stream.Response == nil {
		stream.Response = &Response{
			StreamID: frame.StreamId,
			Headers:  make(map[string][]string),
		}
	}

	// Process headers
	for name, value := range frame.Headers {
		if name == ":status" {
			// Parse status code
			var status int
			fmt.Sscanf(value, "%d", &status)
			stream.Response.Status = status
			stream.Response.HTTPVersion = "HTTP/2"
		} else if !isConnectionSpecificHeader(name) {
			// Add to response headers
			stream.Response.Headers[name] = append(stream.Response.Headers[name], value)
		}
	}

	stream.HeadersReceived = true

	// Update state (BUG-7: use thread-safe state access)
	if frame.EndStream {
		currentState, err := p.manager.GetStreamState(frame.StreamId)
		if err != nil {
			return err
		}
		if currentState == StateOpen {
			p.manager.UpdateStreamState(frame.StreamId, StateHalfClosedRemote)
		} else if currentState == StateHalfClosedLocal {
			p.manager.UpdateStreamState(frame.StreamId, StateClosed)
		}
	}

	return nil
}

// ProcessDataFrame processes a DATA frame
func (p *StreamProcessor) ProcessDataFrame(frame *DataFrame) error {
	stream, exists := p.manager.GetStream(frame.StreamId)
	if !exists {
		return fmt.Errorf("received DATA frame for unknown stream %d", frame.StreamId)
	}

	// Append data to response body
	if stream.Response != nil {
		stream.Response.Body = append(stream.Response.Body, frame.Data...)
	}

	stream.DataReceived = true

	// Update window size
	p.manager.UpdateWindowSize(frame.StreamId, -int32(len(frame.Data)))

	// Update state if END_STREAM flag is set (BUG-7: use thread-safe state access)
	if frame.EndStream {
		currentState, err := p.manager.GetStreamState(frame.StreamId)
		if err != nil {
			return err
		}
		if currentState == StateOpen {
			p.manager.UpdateStreamState(frame.StreamId, StateHalfClosedRemote)
		} else if currentState == StateHalfClosedLocal {
			p.manager.UpdateStreamState(frame.StreamId, StateClosed)
		}
	}

	return nil
}

// ProcessWindowUpdateFrame processes a WINDOW_UPDATE frame
func (p *StreamProcessor) ProcessWindowUpdateFrame(streamID uint32, increment uint32) error {
	return p.manager.UpdateWindowSize(streamID, int32(increment))
}

// ProcessResetFrame processes a RST_STREAM frame
func (p *StreamProcessor) ProcessResetFrame(streamID uint32, errorCode uint32) error {
	return p.manager.Reset(streamID, http2.ErrCode(errorCode))
}

func isConnectionSpecificHeader(name string) bool {
	connectionHeaders := []string{
		"connection", "keep-alive", "proxy-connection",
		"transfer-encoding", "upgrade", "te",
	}
	for _, h := range connectionHeaders {
		if name == h {
			return true
		}
	}
	return false
}
