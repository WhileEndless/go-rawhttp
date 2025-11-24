package unit

import (
	"sync"
	"testing"

	"github.com/WhileEndless/go-rawhttp/v2/pkg/buffer"
)

func TestBufferConcurrentClose(t *testing.T) {
	// Test concurrent Close() calls for race conditions
	buf := buffer.New(1024)

	// Write some data to potentially trigger disk spill
	data := []byte("test data for concurrent close")
	_, err := buf.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Concurrent Close() calls
	var wg sync.WaitGroup
	errorCount := 0
	mu := sync.Mutex{}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := buf.Close(); err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// No errors should occur - Close() should be idempotent
	if errorCount > 0 {
		t.Errorf("expected no errors from concurrent Close(), got %d errors", errorCount)
	}
}

func TestBufferDoubleClose(t *testing.T) {
	// Test that double Close() is idempotent
	buf := buffer.New(1024)

	data := []byte("test data")
	_, err := buf.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// First close
	err = buf.Close()
	if err != nil {
		t.Errorf("first Close() failed: %v", err)
	}

	// Second close should not error
	err = buf.Close()
	if err != nil {
		t.Errorf("second Close() should not error, got: %v", err)
	}

	// Third close should also not error
	err = buf.Close()
	if err != nil {
		t.Errorf("third Close() should not error, got: %v", err)
	}
}

func TestBufferResetAfterClose(t *testing.T) {
	// Test that Reset() allows reuse after Close()
	buf := buffer.New(1024)

	// Write and close
	data := []byte("initial data")
	_, err := buf.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Reset should allow reuse
	err = buf.Reset()
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Should be able to write again
	newData := []byte("new data after reset")
	_, err = buf.Write(newData)
	if err != nil {
		t.Errorf("write after reset failed: %v", err)
	}

	// Size should reflect new data only
	if buf.Size() != int64(len(newData)) {
		t.Errorf("expected size %d after reset, got %d", len(newData), buf.Size())
	}
}

func TestBufferConcurrentWriteAndClose(t *testing.T) {
	// Test concurrent writes and closes
	buf := buffer.New(10) // Small limit to force disk spill

	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte("data from writer")
			buf.Write(data) // Ignore error as close might happen
		}(i)
	}

	// Closers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.Close() // Should be safe
		}()
	}

	wg.Wait()

	// Final close to clean up
	buf.Close()

	// Test passes if no panic or race detected
	t.Log("Concurrent write and close handled safely")
}
