// Package buffer provides memory-efficient data storage with disk spilling.
package buffer

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/WhileEndless/go-rawhttp/pkg/errors"
)

const (
	// DefaultMemoryLimit is the default memory threshold before spilling to disk.
	DefaultMemoryLimit = 4 * 1024 * 1024 // 4MB
)

// Buffer stores data either in memory or spooled to a temporary file when
// exceeding a threshold.
type Buffer struct {
	buf    bytes.Buffer
	file   *os.File
	path   string
	size   int64
	limit  int64
	mu     sync.Mutex // Protects Close() and other operations
	closed bool       // Track if already closed
}

// New creates a new Buffer with the provided memory limit.
func New(limit int64) *Buffer {
	if limit <= 0 {
		limit = DefaultMemoryLimit
	}
	return &Buffer{limit: limit}
}

// NewWithData creates a new buffer with existing data.
func NewWithData(data []byte) *Buffer {
	b := &Buffer{
		limit: DefaultMemoryLimit,
		size:  int64(len(data)),
	}
	b.buf.Write(data)
	return b
}

// Write stores the provided bytes, spilling to disk once above the configured
// memory threshold.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if closed
	if b.closed {
		return 0, errors.NewIOError("buffer is closed", nil)
	}

	b.size += int64(len(p))

	// If still under limit and no file yet, write to memory
	if b.file == nil && int64(b.buf.Len()+len(p)) <= b.limit {
		return b.buf.Write(p)
	}

	// Need to spill to disk
	if b.file == nil {
		tmp, err := os.CreateTemp("", "rawhttp-buffer-*.tmp")
		if err != nil {
			return 0, errors.NewIOError("creating temp file", err)
		}

		// Store file reference immediately to ensure cleanup if Close() is called
		b.file = tmp
		b.path = tmp.Name()

		// Write existing buffer content to file
		if b.buf.Len() > 0 {
			if _, err := tmp.Write(b.buf.Bytes()); err != nil {
				// Close will clean up the file
				b.Close()
				return 0, errors.NewIOError("writing to temp file", err)
			}
		}

		b.buf.Reset()
	}

	// Write new data to file
	n, err := b.file.Write(p)
	if err != nil {
		return n, errors.NewIOError("writing to temp file", err)
	}
	return n, nil
}

// Bytes returns the in-memory data. If the payload spilled to disk this will be
// empty.
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.file != nil {
		return nil
	}
	return b.buf.Bytes()
}

// Path returns the filesystem path backing the spilled payload.
func (b *Buffer) Path() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.path
}

// Size returns the total number of bytes written.
func (b *Buffer) Size() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

// IsSpilled returns true if the buffer has spilled to disk.
func (b *Buffer) IsSpilled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.file != nil
}

// Reader provides a fresh reader for the stored data.
func (b *Buffer) Reader() (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, errors.NewIOError("buffer is closed", nil)
	}

	if b.file != nil {
		// Sync file to ensure all data is written
		if err := b.file.Sync(); err != nil {
			return nil, errors.NewIOError("syncing temp file", err)
		}

		// Open a new reader
		f, err := os.Open(b.path)
		if err != nil {
			return nil, errors.NewIOError("opening temp file for reading", err)
		}
		return f, nil
	}

	// Return in-memory data
	return io.NopCloser(bytes.NewReader(b.buf.Bytes())), nil
}

// Close flushes and closes the underlying file, if any, and removes the temp file.
// Safe for concurrent calls and idempotent.
func (b *Buffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Already closed, make it idempotent
	if b.closed {
		return nil
	}

	b.closed = true

	if b.file != nil {
		err := b.file.Close()
		// Always try to remove the temp file
		if removeErr := os.Remove(b.path); removeErr != nil && err == nil {
			err = errors.NewIOError("removing temp file", removeErr)
		}
		b.file = nil
		b.path = ""
		if err != nil {
			return errors.NewIOError("closing temp file", err)
		}
	}
	return nil
}

// Reset clears the buffer and prepares it for reuse.
func (b *Buffer) Reset() error {
	if err := b.Close(); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf.Reset()
	b.size = 0
	b.closed = false // Allow reuse after reset
	return nil
}
