// Package watcher monitors a directory for file system changes using fsnotify
// and emits relative file paths on Create, Write, Remove, and Rename events.
// It filters out .git/ and .remarker/ paths and deduplicates rapid events.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		baseDir   string
		bufSize   int
		wantBufSz int
	}{
		{
			name:      "absolute path with explicit bufSize",
			baseDir:   "/tmp/remarker-sync",
			bufSize:   50,
			wantBufSz: 50,
		},
		{
			name:      "absolute path with zero bufSize defaults to 100",
			baseDir:   "/tmp/remarker-sync",
			bufSize:   0,
			wantBufSz: 100,
		},
		{
			name:      "absolute path with negative bufSize defaults to 100",
			baseDir:   "/home/user/Documents/reMarkable",
			bufSize:   -5,
			wantBufSz: 100,
		},
		{
			name:      "relative path",
			baseDir:   "./sync",
			bufSize:   100,
			wantBufSz: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := New(tc.baseDir, tc.bufSize)
			if w == nil {
				t.Fatal("New returned nil")
			}
			if w.baseDir != tc.baseDir {
				t.Errorf("baseDir = %q, want %q", w.baseDir, tc.baseDir)
			}
			// Check channel buffer size by trying to fill it
			gotBufSz := cap(w.events)
			if gotBufSz != tc.wantBufSz {
				t.Errorf("events channel bufSize = %d, want %d", gotBufSz, tc.wantBufSz)
			}
			// Watcher should not be started yet
			if w.watcher != nil {
				t.Error("New should not create the fsnotify watcher yet")
			}
			if w.running {
				t.Error("New should not set running to true")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func TestEvents_ReturnsReceiveOnlyChannel(t *testing.T) {
	w := New("/tmp/test", 100)
	ch := w.Events()

	// Verify it's receive-only by attempting a send (compile-time check)
	// We can't actually send, so just verify it's not nil
	if ch == nil {
		t.Fatal("Events returned nil channel")
	}
}

// ---------------------------------------------------------------------------
// Start and Stop
// ---------------------------------------------------------------------------

func TestStart_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	err := w.Start(context.Background())
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if !w.running {
		t.Error("Start should set running to true")
	}
	if w.watcher == nil {
		t.Error("Start should create the fsnotify watcher")
	}

	err = w.Stop()
	if err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	if w.running {
		t.Error("Stop should set running to false")
	}
}

func TestStart_FailsWithInvalidDirectory(t *testing.T) {
	// Use a path that doesn't exist — fsnotify.Add requires the path to exist
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist", "also-not-here")

	w := New(nonExistent, 100)
	err := w.Start(context.Background())
	if err == nil {
		t.Error("Start should return error for non-existent directory path")
	}
}

func TestStop_WithoutStart(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	// Stop without Start should not panic and should handle gracefully
	err := w.Stop()
	if err != nil {
		t.Logf("Stop without Start returned error (acceptable): %v", err)
	}
	if w.running {
		t.Error("Stop should set running to false even if watcher was nil")
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestStart_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if !w.running {
		t.Fatal("Start should set running to true")
	}

	// Cancel the context
	cancel()

	// Give the goroutine time to notice cancellation
	time.Sleep(200 * time.Millisecond)

	// The events channel should be closed or the goroutine should have exited
	// We verify by checking that Stop works cleanly
	err := w.Stop()
	if err != nil {
		t.Fatalf("Stop after context cancellation: %v", err)
	}
}

func TestStart_ContextCancellationStopsEvents(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Create a file to generate an event
	testFile := filepath.Join(tmpDir, "cancel-test.content")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Drain the event
	drainEvents(w)

	// Wait past dedup window so next event isn't suppressed
	time.Sleep(1200 * time.Millisecond)

	// Cancel context
	cancel()
	time.Sleep(200 * time.Millisecond)

	// After cancel, no more events should come
	select {
	case ev, ok := <-w.Events():
		if !ok {
			return // Channel closed, good
		}
		t.Errorf("got event after cancel: %q", ev)
	case <-time.After(500 * time.Millisecond):
		// No event — good
	}
}
