package unit

import (
	"io"
	"testing"

	"github.com/WhileEndless/go-rawhttp/pkg/buffer"
)

func TestBufferMemoryLimit(t *testing.T) {
	// Test with very small limit to force disk spilling
	buf := buffer.New(10)
	defer buf.Close()

	// Write small data (should stay in memory)
	data1 := []byte("small")
	_, err := buf.Write(data1)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if buf.IsSpilled() {
		t.Fatalf("expected data in memory")
	}

	if buf.Bytes() == nil {
		t.Fatalf("expected data in memory")
	}

	// Write more data (should spill to disk)
	data2 := []byte("this is much larger data that exceeds the limit")
	_, err = buf.Write(data2)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if !buf.IsSpilled() {
		t.Fatalf("expected data to spill to disk")
	}

	if buf.Path() == "" {
		t.Fatalf("expected temp file path")
	}

	if buf.Bytes() != nil {
		t.Fatalf("expected no data in memory after spill")
	}

	totalSize := int64(len(data1) + len(data2))
	if buf.Size() != totalSize {
		t.Fatalf("expected size %d, got %d", totalSize, buf.Size())
	}
}

func TestBufferReader(t *testing.T) {
	buf := buffer.New(1024)
	defer buf.Close()

	testData := []byte("test data for reader")
	_, err := buf.Write(testData)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	reader, err := buf.Reader()
	if err != nil {
		t.Fatalf("reader failed: %v", err)
	}
	defer reader.Close()

	readData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(readData) != string(testData) {
		t.Fatalf("data mismatch: expected %s, got %s", testData, readData)
	}
}

func TestBufferReset(t *testing.T) {
	buf := buffer.New(10)
	defer buf.Close()

	// Write data to spill to disk
	data := []byte("this will spill to disk because it's too large")
	_, err := buf.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if !buf.IsSpilled() {
		t.Fatalf("expected data to spill")
	}

	// Reset buffer
	err = buf.Reset()
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	if buf.Size() != 0 {
		t.Fatalf("expected size 0 after reset, got %d", buf.Size())
	}

	if buf.IsSpilled() {
		t.Fatalf("expected no spill after reset")
	}
}
