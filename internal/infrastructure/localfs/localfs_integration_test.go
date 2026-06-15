// Package localfs provides local filesystem operations for reading and writing
// files in the sync directory. It implements the document.LocalRepository
// interface for local-side file operations.
package localfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// ---------------------------------------------------------------------------
// Round-trip
// ---------------------------------------------------------------------------

func TestRoundTrip_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Put a file
	originalTime := time.Date(2024, 11, 5, 16, 45, 30, 0, time.UTC)
	original := document.File{
		Path:    "roundtrip/document.metadata",
		Size:    42,
		ModTime: originalTime,
		Hash:    "abc123",
	}

	err := c.PutFile(context.Background(), original)
	if err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	// Get it back
	retrieved, err := c.GetFile(context.Background(), "roundtrip/document.metadata")
	if err != nil {
		t.Fatalf("GetFile error: %v", err)
	}

	// Path should match
	if retrieved.Path != original.Path {
		t.Errorf("Path = %q, want %q", retrieved.Path, original.Path)
	}

	// ModTime should be close
	diff := retrieved.ModTime.Sub(originalTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("ModTime diff = %v, want within 1s", diff)
	}

	// Verify it shows up in ListFiles
	files, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Path == original.Path {
			found = true
			break
		}
	}
	if !found {
		t.Error("PutFile'd file should appear in ListFiles")
	}
}

func TestRoundTrip_PutDeleteList(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Put a file
	file := document.File{
		Path:    "temp.md",
		Size:    10,
		ModTime: time.Now(),
		Hash:    "hash1",
	}
	if err := c.PutFile(context.Background(), file); err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	// Verify it exists
	files, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	// Delete it
	if err := c.DeleteFile(context.Background(), "temp.md"); err != nil {
		t.Fatalf("DeleteFile error: %v", err)
	}

	// Verify it's gone
	files, err = c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files after delete, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// ListFiles with mixed content (files + dirs + skipped dirs)
// ---------------------------------------------------------------------------

func TestListFiles_MixedContent(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a realistic directory structure
	items := map[string][]byte{
		// Regular files
		"doc-001.metadata": []byte("meta1"),
		"doc-001.content":  []byte("content1"),
		"notes/001.metadata": []byte("note meta"),
		// .git should be skipped
		".git/objects/pack": []byte("git pack data"),
		// .remarker should be skipped
		".remarker/state.json": []byte(`{"synced":true}`),
		// Regular nested directory
		"archive/2024/january/scan.pdf": []byte("scanned pdf"),
	}

	for relPath, content := range items {
		fullPath := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("write %s: %v", relPath, err)
		}
	}

	result, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	// Should find 4 files (skipping .git and .remarker)
	// doc-001.metadata, doc-001.content, notes/001.metadata, archive/2024/january/scan.pdf
	expectedCount := 4
	if len(result) != expectedCount {
		t.Errorf("expected %d files, got %d:", expectedCount, len(result))
		for _, f := range result {
			t.Logf("  - %s", f.Path)
		}
	}

	// Verify no skipped paths leaked through
	for _, f := range result {
		parts := filepath.SplitList(f.Path)
		for _, part := range parts {
			if part == ".git" || part == ".remarker" {
				t.Errorf("should not include path with .git or .remarker: %s", f.Path)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sha256hex is defined in localfs_operations_test.go. Both files share the
// same package so the helper is available to integration tests as well.
