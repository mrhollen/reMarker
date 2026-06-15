// Package watcher monitors a directory for file system changes using fsnotify
// and emits relative file paths on Create, Write, Remove, and Rename events.
// It filters out .git/ and .remarker/ paths and deduplicates rapid events.
package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// File change events
// ---------------------------------------------------------------------------

func TestEvents_FileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// Allow watcher to initialize
	time.Sleep(50 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "new-document.metadata")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Wait for event
	select {
	case relPath := <-w.Events():
		if relPath != "new-document.metadata" {
			t.Errorf("event path = %q, want %q", relPath, "new-document.metadata")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file creation event")
	}
}

func TestEvents_FileModified(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// Create a file first
	testFile := filepath.Join(tmpDir, "existing.metadata")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Wait longer than the dedup window so the next write isn't suppressed
	time.Sleep(1200 * time.Millisecond)
	drainEvents(w)

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if relPath != "existing.metadata" {
			t.Errorf("event path = %q, want %q", relPath, "existing.metadata")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file modification event")
	}
}

func TestEvents_FileRemoved(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// Create a file first
	testFile := filepath.Join(tmpDir, "to-remove.content")
	if err := os.WriteFile(testFile, []byte("remove me"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Wait longer than the dedup window so the remove isn't suppressed
	time.Sleep(1200 * time.Millisecond)
	drainEvents(w)

	// Remove the file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if relPath != "to-remove.content" {
			t.Errorf("event path = %q, want %q", relPath, "to-remove.content")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file removal event")
	}
}

func TestEvents_NestedFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a nested directory
	nestedDir := filepath.Join(tmpDir, "sub", "deep")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Wait for the watcher to process the directory creation events and
	// add the new directories to its watch list.
	time.Sleep(200 * time.Millisecond)

	testFile := filepath.Join(nestedDir, "nested-document.content")
	if err := os.WriteFile(testFile, []byte("nested content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		// Normalize to forward slashes
		want := "sub/deep/nested-document.content"
		if relPath != want {
			t.Errorf("event path = %q, want %q", relPath, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for nested file creation event")
	}
}

// ---------------------------------------------------------------------------
// Skip rules: .git and .remarker
// ---------------------------------------------------------------------------

func TestEvents_SkipsGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a file in .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	gitFile := filepath.Join(gitDir, "config")
	if err := os.WriteFile(gitFile, []byte("git config"), 0644); err != nil {
		t.Fatalf("write git file: %v", err)
	}

	// Should NOT receive an event for .git files
	select {
	case relPath := <-w.Events():
		if strings.Contains(relPath, ".git") {
			t.Errorf("should not emit events for .git paths, got: %q", relPath)
		}
	case <-time.After(500 * time.Millisecond):
		// No event is the expected behavior
	}
}

func TestEvents_SkipsGitNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a file in a nested .git directory
	nestedGitDir := filepath.Join(tmpDir, "subdir", ".git")
	if err := os.MkdirAll(nestedGitDir, 0755); err != nil {
		t.Fatalf("mkdir subdir/.git: %v", err)
	}
	gitFile := filepath.Join(nestedGitDir, "HEAD")
	if err := os.WriteFile(gitFile, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatalf("write git file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if strings.Contains(relPath, ".git") {
			t.Errorf("should not emit events for nested .git paths, got: %q", relPath)
		}
	case <-time.After(500 * time.Millisecond):
		// No event is the expected behavior
	}
}

func TestEvents_SkipsRemarkerDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a file in .remarker directory
	rmDir := filepath.Join(tmpDir, ".remarker")
	if err := os.MkdirAll(rmDir, 0755); err != nil {
		t.Fatalf("mkdir .remarker: %v", err)
	}
	rmFile := filepath.Join(rmDir, "manifest.json")
	if err := os.WriteFile(rmFile, []byte(`{"version":1}`), 0644); err != nil {
		t.Fatalf("write remarker file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if strings.Contains(relPath, ".remarker") {
			t.Errorf("should not emit events for .remarker paths, got: %q", relPath)
		}
	case <-time.After(500 * time.Millisecond):
		// No event is the expected behavior
	}
}

func TestEvents_SkipsRemarkerNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a file in a nested .remarker directory
	nestedRmDir := filepath.Join(tmpDir, "subdir", ".remarker")
	if err := os.MkdirAll(nestedRmDir, 0755); err != nil {
		t.Fatalf("mkdir subdir/.remarker: %v", err)
	}
	rmFile := filepath.Join(nestedRmDir, "cache")
	if err := os.WriteFile(rmFile, []byte("cache data"), 0644); err != nil {
		t.Fatalf("write remarker file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if strings.Contains(relPath, ".remarker") {
			t.Errorf("should not emit events for nested .remarker paths, got: %q", relPath)
		}
	case <-time.After(500 * time.Millisecond):
		// No event is the expected behavior
	}
}

func TestEvents_SkipsButStillEmitsRegularFiles(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git"), 0644); err != nil {
		t.Fatalf("write git file: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	drainEvents(w)

	// Create a regular file - this should trigger an event
	regularFile := filepath.Join(tmpDir, "document.metadata")
	if err := os.WriteFile(regularFile, []byte("document content"), 0644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}

	select {
	case relPath := <-w.Events():
		if relPath != "document.metadata" {
			t.Errorf("event path = %q, want %q", relPath, "document.metadata")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for regular file event")
	}
}

// ---------------------------------------------------------------------------
// Deduplication
// ---------------------------------------------------------------------------

func TestEvents_DeduplicatesRapidWrites(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	// Rapidly write to the same file multiple times
	testFile := filepath.Join(tmpDir, "rapid-file.metadata")
	for i := 0; i < 10; i++ {
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatalf("write file iteration %d: %v", i, err)
		}
	}

	// Collect events within a window
	var events []string
	timeout := time.After(1 * time.Second)
	for {
		select {
		case relPath := <-w.Events():
			events = append(events, relPath)
		case <-timeout:
			goto done
		}
	}
done:

	// Should have at most 1-2 events (deduplication), not 10+
	if len(events) > 3 {
		t.Errorf("expected at most 3 deduplicated events, got %d: %v", len(events), events)
	}

	// All events should be for the same file
	for _, e := range events {
		if e != "rapid-file.metadata" {
			t.Errorf("unexpected event path: %q", e)
		}
	}
}

// ---------------------------------------------------------------------------
// Multiple files produce separate events
// ---------------------------------------------------------------------------

func TestEvents_MultipleFilesProduceSeparateEvents(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	// Create notes directory first
	if err := os.MkdirAll(filepath.Join(tmpDir, "notes"), 0755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	drainEvents(w)

	// Create multiple files with spacing to avoid dedup
	files := []string{
		"doc-001.content",
		"doc-001.metadata",
		"notes/scan-001.pdf",
	}

	// Collect all events in a goroutine
	var received []string
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		for relPath := range w.Events() {
			mu.Lock()
			received = append(received, relPath)
			mu.Unlock()
		}
		close(done)
	}()

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("content"), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
		// Small delay between files to avoid dedup
		time.Sleep(200 * time.Millisecond)
	}

	// Give events time to arrive, then stop
	time.Sleep(300 * time.Millisecond)
	w.Stop()
	<-done

	// Each file should produce at least one event
	for _, f := range files {
		found := false
		for _, r := range received {
			if r == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event for %q, not found in events: %v", f, received)
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestEvents_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	w := New(tmpDir, 100)

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup

	// Multiple goroutines creating files
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			testFile := filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d.metadata", n))
			os.WriteFile(testFile, []byte("content"), 0644)
		}(i)
	}

	// Read events concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.After(2 * time.Second)
		for {
			select {
			case <-w.Events():
				// Just consume events
			case <-timeout:
				return
			}
		}
	}()

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// drainEvents consumes all pending events from the watcher's channel within a short timeout.
func drainEvents(w *Watcher) {
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case <-w.Events():
			// discard
		case <-timeout:
			return
		}
	}
}
