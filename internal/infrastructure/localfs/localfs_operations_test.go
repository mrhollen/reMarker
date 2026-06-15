// Package localfs provides local filesystem operations for reading and writing
// files in the sync directory. It implements the document.LocalRepository
// interface for local-side file operations.
package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// Compile-time check: Client implements document.LocalRepository
var _ document.LocalRepository = (*Client)(nil)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
	}{
		{
			name:    "absolute path",
			baseDir: "/tmp/remarker-sync",
		},
		{
			name:    "relative path",
			baseDir: "./sync",
		},
		{
			name:    "nested path",
			baseDir: "/home/user/Documents/reMarkable/sync",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.baseDir)
			if c == nil {
				t.Fatal("New returned nil")
			}
			if c.baseDir != tc.baseDir {
				t.Errorf("baseDir = %q, want %q", c.baseDir, tc.baseDir)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ListFiles
// ---------------------------------------------------------------------------

func TestListFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	files, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListFiles_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a test file
	content := []byte("hello world")
	filePath := filepath.Join(tmpDir, "test.metadata")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	files, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Path != "test.metadata" {
		t.Errorf("Path = %q, want %q", f.Path, "test.metadata")
	}
	if f.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", f.Size, len(content))
	}

	wantHash := sha256hex(content)
	if f.Hash != wantHash {
		t.Errorf("Hash = %q, want %q", f.Hash, wantHash)
	}
}

func TestListFiles_NestedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create nested directory structure
	files := map[string][]byte{
		"doc-001.metadata": []byte("metadata content 1"),
		"doc-001.content":  []byte("content data 1"),
		"notes/notes-001.metadata": []byte("nested metadata"),
		"notes/notes-001.content":  []byte("nested content"),
		"deep/nested/dir/file.pdf": []byte("deep file"),
	}

	for relPath, content := range files {
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

	if len(result) != len(files) {
		t.Fatalf("expected %d files, got %d", len(files), len(result))
	}

	// Verify each file was found
	found := make(map[string]document.File)
	for _, f := range result {
		found[f.Path] = f
	}

	for relPath, content := range files {
		f, ok := found[relPath]
		if !ok {
			t.Errorf("file %q not found in results", relPath)
			continue
		}
		if f.Size != int64(len(content)) {
			t.Errorf("%s: Size = %d, want %d", relPath, f.Size, len(content))
		}
		wantHash := sha256hex(content)
		if f.Hash != wantHash {
			t.Errorf("%s: Hash = %q, want %q", relPath, f.Hash, wantHash)
		}
	}
}

func TestListFiles_SkipsGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a regular file
	if err := os.WriteFile(filepath.Join(tmpDir, "regular.metadata"), []byte("regular"), 0644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}

	// Create .git directory with files
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
		t.Fatalf("write git config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatalf("write git HEAD: %v", err)
	}

	// Create nested .git in subdirectory
	subGitDir := filepath.Join(tmpDir, "subdir", ".git")
	if err := os.MkdirAll(subGitDir, 0755); err != nil {
		t.Fatalf("mkdir subdir/.git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subGitDir, "config"), []byte("nested git config"), 0644); err != nil {
		t.Fatalf("write nested git config: %v", err)
	}

	// Create a regular file in subdirectory
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "subdir", "file.metadata"), []byte("subdir file"), 0644); err != nil {
		t.Fatalf("write subdir file: %v", err)
	}

	result, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	// Should only find 2 files, not the .git files
	if len(result) != 2 {
		t.Errorf("expected 2 files (skipping .git), got %d: ", len(result))
		for _, f := range result {
			t.Logf("  - %s", f.Path)
		}
	}

	// Verify no .git paths are in results
	for _, f := range result {
		if filepath.Base(filepath.Dir(f.Path)) == ".git" || filepath.Base(f.Path) == ".git" {
			t.Errorf("should not include .git file: %s", f.Path)
		}
	}
}

func TestListFiles_SkipsRemarkerDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a regular file
	if err := os.WriteFile(filepath.Join(tmpDir, "document.metadata"), []byte("document"), 0644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	// Create .remarker directory with files
	rmDir := filepath.Join(tmpDir, ".remarker")
	if err := os.MkdirAll(filepath.Join(rmDir, "cache"), 0755); err != nil {
		t.Fatalf("mkdir .remarker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rmDir, "manifest.json"), []byte(`{"version":1}`), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rmDir, "cache", "data"), []byte("cache data"), 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	result, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	// Should only find 1 file
	if len(result) != 1 {
		t.Errorf("expected 1 file (skipping .remarker), got %d", len(result))
		for _, f := range result {
			t.Logf("  - %s", f.Path)
		}
	}

	if result[0].Path != "document.metadata" {
		t.Errorf("Path = %q, want %q", result[0].Path, "document.metadata")
	}
}

func TestListFiles_SortedOutput(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create files in non-alphabetical order
	files := []string{
		"zebra.metadata",
		"alpha.content",
		"middle.pdf",
		"beta.metadata",
	}

	for _, name := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	result, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	// Verify sorted order
	for i := 1; i < len(result); i++ {
		if result[i].Path < result[i-1].Path {
			t.Errorf("files not sorted: %q should come before %q", result[i].Path, result[i-1].Path)
		}
	}

	// Expected order
	expected := []string{"alpha.content", "beta.metadata", "middle.pdf", "zebra.metadata"}
	for i, want := range expected {
		if result[i].Path != want {
			t.Errorf("result[%d].Path = %q, want %q", i, result[i].Path, want)
		}
	}
}

func TestListFiles_CreatesBaseDir(t *testing.T) {
	// Use a path that doesn't exist yet
	newDir := filepath.Join(t.TempDir(), "new-sync-dir", "nested")
	c := New(newDir)

	// Directory should not exist yet
	if _, err := os.Stat(newDir); err == nil {
		t.Fatal("directory should not exist before ListFiles")
	}

	// ListFiles should create it
	files, err := c.ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Verify directory was created
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("ListFiles should have created baseDir: %v", err)
	}
}

func TestListFiles_ContextCancellation_Immediate(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.ListFiles(ctx)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestListFiles_ContextCancellation_DuringWalk(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create many files across subdirectories so the walk takes meaningful time
	// and we can reliably cancel mid-traversal
	for i := 0; i < 500; i++ {
		dir := fmt.Sprintf("sub-%03d", i/10)
		name := fmt.Sprintf("file-%04d.metadata", i)
		fullPath := filepath.Join(tmpDir, dir, name)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("data"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a very short delay — walk should still be in progress
	done := make(chan error, 1)
	go func() {
		_, err := c.ListFiles(ctx)
		done <- err
	}()

	time.AfterFunc(1*time.Millisecond, cancel)

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ListFiles did not return after context was cancelled")
	}
}

// ---------------------------------------------------------------------------
// GetFile
// ---------------------------------------------------------------------------

func TestGetFile_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	content := []byte("file content for hashing")
	filePath := filepath.Join(tmpDir, "subdir", "test-file.metadata")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Set a specific modtime for testing
	specificTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(filePath, specificTime, specificTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	f, err := c.GetFile(context.Background(), "subdir/test-file.metadata")
	if err != nil {
		t.Fatalf("GetFile error: %v", err)
	}

	if f.Path != "subdir/test-file.metadata" {
		t.Errorf("Path = %q, want %q", f.Path, "subdir/test-file.metadata")
	}
	if f.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", f.Size, len(content))
	}

	wantHash := sha256hex(content)
	if f.Hash != wantHash {
		t.Errorf("Hash = %q, want %q", f.Hash, wantHash)
	}

	// ModTime should be close to the specific time (truncated to fs precision)
	if f.ModTime.Year() != specificTime.Year() {
		t.Errorf("ModTime = %v, want close to %v", f.ModTime, specificTime)
	}
}

func TestGetFile_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	_, err := c.GetFile(context.Background(), "nonexistent.metadata")
	if err == nil {
		t.Error("GetFile should return error for non-existent file")
	}
}

func TestGetFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a file so the path is valid
	filePath := filepath.Join(tmpDir, "test.metadata")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.GetFile(ctx, "test.metadata")
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestGetFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create empty file
	filePath := filepath.Join(tmpDir, "empty.metadata")
	if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	f, err := c.GetFile(context.Background(), "empty.metadata")
	if err != nil {
		t.Fatalf("GetFile error: %v", err)
	}

	if f.Size != 0 {
		t.Errorf("Size = %d, want 0", f.Size)
	}

	wantHash := sha256hex([]byte{})
	if f.Hash != wantHash {
		t.Errorf("Hash = %q, want %q", f.Hash, wantHash)
	}
}

// ---------------------------------------------------------------------------
// PutFile
// ---------------------------------------------------------------------------

func TestPutFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	file := document.File{
		Path:    "new-file.metadata",
		Size:    100,
		ModTime: time.Now(),
		Hash:    "somehash",
	}

	err := c.PutFile(ctx, file)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestPutFile_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	file := document.File{
		Path:    "new-file.metadata",
		Size:    100,
		ModTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Hash:    "somehash",
	}

	err := c.PutFile(context.Background(), file)
	if err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	// Verify file exists
	fullPath := filepath.Join(tmpDir, "new-file.metadata")
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("file should exist after PutFile: %v", err)
	}

	// File should be empty (content not transferred through this interface)
	if info.Size() != 0 {
		t.Errorf("file size = %d, want 0 (empty placeholder)", info.Size())
	}
}

func TestPutFile_CreatesParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	file := document.File{
		Path:    "deep/nested/path/document.metadata",
		Size:    100,
		ModTime: time.Date(2024, 3, 15, 9, 0, 0, 0, time.UTC),
		Hash:    "somehash",
	}

	err := c.PutFile(context.Background(), file)
	if err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	// Verify file exists with parent dirs created
	fullPath := filepath.Join(tmpDir, "deep/nested/path/document.metadata")
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("file should exist after PutFile with nested path: %v", err)
	}
}

func TestPutFile_SetsModTime(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	specificTime := time.Date(2024, 7, 20, 14, 30, 45, 0, time.UTC)
	file := document.File{
		Path:    "timed-file.metadata",
		Size:    50,
		ModTime: specificTime,
		Hash:    "somehash",
	}

	err := c.PutFile(context.Background(), file)
	if err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	fullPath := filepath.Join(tmpDir, "timed-file.metadata")
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	// ModTime should match (within 1 second tolerance for fs precision)
	diff := info.ModTime().Sub(specificTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("ModTime = %v, want %v (diff %v)", info.ModTime(), specificTime, diff)
	}
}

func TestPutFile_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create existing file
	existingPath := filepath.Join(tmpDir, "existing.metadata")
	if err := os.WriteFile(existingPath, []byte("old content"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	newTime := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
	file := document.File{
		Path:    "existing.metadata",
		Size:    200,
		ModTime: newTime,
		Hash:    "newhash",
	}

	err := c.PutFile(context.Background(), file)
	if err != nil {
		t.Fatalf("PutFile error: %v", err)
	}

	info, err := os.Stat(existingPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	diff := info.ModTime().Sub(newTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), newTime)
	}
}

// ---------------------------------------------------------------------------
// DeleteFile
// ---------------------------------------------------------------------------

func TestDeleteFile_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a file so the path is valid
	filePath := filepath.Join(tmpDir, "to-delete.metadata")
	if err := os.WriteFile(filePath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := c.DeleteFile(ctx, "to-delete.metadata")
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}

	// Verify file was NOT deleted (context checked before operation)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("file should not have been deleted when context was cancelled")
	}
}

func TestDeleteFile_RemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create a file to delete
	filePath := filepath.Join(tmpDir, "to-delete.metadata")
	if err := os.WriteFile(filePath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := c.DeleteFile(context.Background(), "to-delete.metadata")
	if err != nil {
		t.Fatalf("DeleteFile error: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist after DeleteFile")
	}
}

func TestDeleteFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	err := c.DeleteFile(context.Background(), "does-not-exist.metadata")
	if err == nil {
		t.Error("DeleteFile should return error for non-existent file")
	}
}

func TestDeleteFile_NestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	c := New(tmpDir)

	// Create nested file
	nestedPath := filepath.Join(tmpDir, "sub", "nested.metadata")
	if err := os.MkdirAll(filepath.Dir(nestedPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(nestedPath, []byte("nested content"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := c.DeleteFile(context.Background(), "sub/nested.metadata")
	if err != nil {
		t.Fatalf("DeleteFile error: %v", err)
	}

	if _, err := os.Stat(nestedPath); !os.IsNotExist(err) {
		t.Error("nested file should not exist after DeleteFile")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sha256hex returns the SHA256 hex digest of the given bytes.
func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
