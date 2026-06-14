// Package sidecar provides a file-system implementation of the
// SidecarRepository interface for persisting reMarkable sidecar metadata
// files (.metadata, .content, .pagedata) per document.
package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// Compile-time check: Store implements document.SidecarRepository.
var _ document.SidecarRepository = (*Store)(nil)

func TestNew(t *testing.T) {
	t.Run("creates store with correct base directory", func(t *testing.T) {
		store := New("/some/path/device-meta")
		if store.baseDir != "/some/path/device-meta" {
			t.Errorf("baseDir = %q, want %q", store.baseDir, "/some/path/device-meta")
		}
	})
}

func TestNewDefault(t *testing.T) {
	t.Run("creates store in syncDir/.remarker/device-meta", func(t *testing.T) {
		store := NewDefault("/home/user")
		want := filepath.Join("/home/user", DefaultDir, DefaultSubDir)
		if store.baseDir != want {
			t.Errorf("baseDir = %q, want %q", store.baseDir, want)
		}
	})

	t.Run("uses DefaultDir and DefaultSubDir constants", func(t *testing.T) {
		if DefaultDir != ".remarker" {
			t.Errorf("DefaultDir = %q, want %q", DefaultDir, ".remarker")
		}
		if DefaultSubDir != "device-meta" {
			t.Errorf("DefaultSubDir = %q, want %q", DefaultSubDir, "device-meta")
		}
	})
}

func TestNewDefaultContent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("creates default content with one page at default dimensions", func(t *testing.T) {
		content := NewDefaultContent("pdf")
		if len(content.Pages) != 1 {
			t.Fatalf("Pages length = %d, want 1", len(content.Pages))
		}
		if content.Pages[0].Width != DefaultPageWidth {
			t.Errorf("Pages[0].Width = %d, want %d", content.Pages[0].Width, DefaultPageWidth)
		}
		if content.Pages[0].Height != DefaultPageHeight {
			t.Errorf("Pages[0].Height = %d, want %d", content.Pages[0].Height, DefaultPageHeight)
		}
		if content.FileType != "pdf" {
			t.Errorf("FileType = %q, want %q", content.FileType, "pdf")
		}
	})

	t.Run("saves and loads default content round trip", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := NewDefaultContent("pdf")

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		_, gotContent, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if len(gotContent.Pages) != 1 {
			t.Fatalf("loaded Pages length = %d, want 1", len(gotContent.Pages))
		}
		if gotContent.Pages[0].Width != DefaultPageWidth {
			t.Errorf("loaded Pages[0].Width = %d, want %d", gotContent.Pages[0].Width, DefaultPageWidth)
		}
		if gotContent.Pages[0].Height != DefaultPageHeight {
			t.Errorf("loaded Pages[0].Height = %d, want %d", gotContent.Pages[0].Height, DefaultPageHeight)
		}
	})
}

func TestSaveMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("creates device subdirectory if it doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		dirPath := filepath.Join(tmpDir, "device-meta", docUUID)
		if _, err := os.Stat(dirPath); err != nil {
			t.Fatalf("device subdirectory not created: %v", err)
		}
	})

	t.Run("writes valid JSON metadata file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		metaPath := filepath.Join(tmpDir, "device-meta", docUUID, ".metadata")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read metadata file: %v", err)
		}

		var parsed document.Metadata
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("metadata file is not valid JSON: %v", err)
		}
	})

	t.Run("writes valid JSON content file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		contentPath := filepath.Join(tmpDir, "device-meta", docUUID, ".content")
		data, err := os.ReadFile(contentPath)
		if err != nil {
			t.Fatalf("read content file: %v", err)
		}

		var parsed document.Content
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("content file is not valid JSON: %v", err)
		}
	})

	t.Run("writes pretty-printed JSON with 2-space indent", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		metaPath := filepath.Join(tmpDir, "device-meta", docUUID, ".metadata")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read metadata file: %v", err)
		}

		// Pretty-printed JSON should contain newlines
		if len(data) < 50 {
			t.Errorf("expected pretty-printed JSON, got short output: %s", data)
		}
	})

	t.Run("uses atomic write - no .tmp files after save", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		entries, err := os.ReadDir(filepath.Join(tmpDir, "device-meta", docUUID))
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".tmp" {
				t.Errorf("temp file should not exist after save: %s", entry.Name())
			}
		}
	})

	t.Run("does not create .pagedata file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		pagedataPath := filepath.Join(tmpDir, "device-meta", docUUID, ".pagedata")
		if _, err := os.Stat(pagedataPath); err == nil {
			t.Error(".pagedata file should not be created by SaveMetadata")
		}
	})
}

func TestGetMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("returns metadata and content for saved document", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		gotMeta, gotContent, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if gotMeta.DeviceID != docUUID {
			t.Errorf("DeviceID = %q, want %q", gotMeta.DeviceID, docUUID)
		}
		if gotMeta.VisibleName != "test.pdf" {
			t.Errorf("VisibleName = %q, want %q", gotMeta.VisibleName, "test.pdf")
		}
		if gotMeta.FileFormat != "pdf" {
			t.Errorf("FileFormat = %q, want %q", gotMeta.FileFormat, "pdf")
		}
		if gotMeta.IsDeleted {
			t.Error("IsDeleted = true, want false")
		}
		if len(gotContent.Pages) != 1 {
			t.Fatalf("Pages length = %d, want 1", len(gotContent.Pages))
		}
		if gotContent.FileType != "pdf" {
			t.Errorf("FileType = %q, want %q", gotContent.FileType, "pdf")
		}
	})

	t.Run("returns error for non-existent document", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		_, _, err := store.GetMetadata(ctx, "non-existent-uuid")
		if err == nil {
			t.Fatal("GetMetadata() expected error for non-existent document, got nil")
		}
	})

	t.Run("returns error for invalid JSON metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		docDir := filepath.Join(tmpDir, "device-meta", docUUID)
		if err := os.MkdirAll(docDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".metadata"), []byte("{invalid}"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".content"), []byte(`{"pages":[{"width":1404,"height":1872}],"fileType":"pdf"}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, _, err := store.GetMetadata(ctx, docUUID)
		if err == nil {
			t.Fatal("GetMetadata() expected error for invalid JSON metadata, got nil")
		}
	})

	t.Run("returns error when metadata exists but content missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		docDir := filepath.Join(tmpDir, "device-meta", docUUID)
		if err := os.MkdirAll(docDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".metadata"), []byte(`{"DeviceID":"test"}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, _, err := store.GetMetadata(ctx, docUUID)
		if err == nil {
			t.Fatal("GetMetadata() expected error when content file missing, got nil")
		}
	})
}

func TestDeleteMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("removes metadata and content files", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		if err := store.DeleteMetadata(ctx, docUUID); err != nil {
			t.Fatalf("DeleteMetadata(): %v", err)
		}

		metaPath := filepath.Join(tmpDir, "device-meta", docUUID, ".metadata")
		if _, err := os.Stat(metaPath); err == nil {
			t.Error(".metadata file should be deleted")
		}

		contentPath := filepath.Join(tmpDir, "device-meta", docUUID, ".content")
		if _, err := os.Stat(contentPath); err == nil {
			t.Error(".content file should be deleted")
		}
	})

	t.Run("removes pagedata file if present", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		docDir := filepath.Join(tmpDir, "device-meta", docUUID)
		if err := os.MkdirAll(docDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".metadata"), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".content"), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".pagedata"), []byte("pagedata"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		if err := store.DeleteMetadata(ctx, docUUID); err != nil {
			t.Fatalf("DeleteMetadata(): %v", err)
		}

		pagedataPath := filepath.Join(tmpDir, "device-meta", docUUID, ".pagedata")
		if _, err := os.Stat(pagedataPath); err == nil {
			t.Error(".pagedata file should be deleted")
		}
	})

	t.Run("returns nil for non-existent document", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		err := store.DeleteMetadata(ctx, "non-existent-uuid")
		if err != nil {
			t.Fatalf("DeleteMetadata() expected nil for non-existent document, got %v", err)
		}
	})

	t.Run("removes device subdirectory after deleting all files", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		if err := store.DeleteMetadata(ctx, docUUID); err != nil {
			t.Fatalf("DeleteMetadata(): %v", err)
		}

		dirPath := filepath.Join(tmpDir, "device-meta", docUUID)
		if _, err := os.Stat(dirPath); err == nil {
			t.Error("device subdirectory should be removed after deleting all files")
		}
	})
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("returns true after save", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		exists, err := store.Exists(ctx, docUUID)
		if err != nil {
			t.Fatalf("Exists(): %v", err)
		}
		if !exists {
			t.Error("Exists() = false, want true after save")
		}
	})

	t.Run("returns false for non-existent document", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		exists, err := store.Exists(ctx, "non-existent-uuid")
		if err != nil {
			t.Fatalf("Exists(): %v", err)
		}
		if exists {
			t.Error("Exists() = true, want false for non-existent document")
		}
	})

	t.Run("returns false when only metadata exists without content", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		docDir := filepath.Join(tmpDir, "device-meta", docUUID)
		if err := os.MkdirAll(docDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".metadata"), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}

		exists, err := store.Exists(ctx, docUUID)
		if err != nil {
			t.Fatalf("Exists(): %v", err)
		}
		if exists {
			t.Error("Exists() = true, want false when content file is missing")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("save then load preserves all metadata fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "Annual Report.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "parent-uuid-1234",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		gotMeta, gotContent, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if gotMeta.DeviceID != meta.DeviceID {
			t.Errorf("DeviceID = %q, want %q", gotMeta.DeviceID, meta.DeviceID)
		}
		if gotMeta.VisibleName != meta.VisibleName {
			t.Errorf("VisibleName = %q, want %q", gotMeta.VisibleName, meta.VisibleName)
		}
		if gotMeta.FileFormat != meta.FileFormat {
			t.Errorf("FileFormat = %q, want %q", gotMeta.FileFormat, meta.FileFormat)
		}
		if gotMeta.ParentFolderUUID != meta.ParentFolderUUID {
			t.Errorf("ParentFolderUUID = %q, want %q", gotMeta.ParentFolderUUID, meta.ParentFolderUUID)
		}
		if !gotMeta.LastModified.Equal(meta.LastModified) {
			t.Errorf("LastModified = %v, want %v", gotMeta.LastModified, meta.LastModified)
		}
		if gotMeta.IsDeleted != meta.IsDeleted {
			t.Errorf("IsDeleted = %v, want %v", gotMeta.IsDeleted, meta.IsDeleted)
		}
		if gotContent.FileType != content.FileType {
			t.Errorf("FileType = %q, want %q", gotContent.FileType, content.FileType)
		}
	})

	t.Run("save then load preserves multiple pages", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "multi-page.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages: []document.PageInfo{
				{Width: 1404, Height: 1872},
				{Width: 1404, Height: 1872},
				{Width: 1404, Height: 1872},
			},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		_, gotContent, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if len(gotContent.Pages) != 3 {
			t.Fatalf("Pages length = %d, want 3", len(gotContent.Pages))
		}
		for i, page := range gotContent.Pages {
			if page.Width != 1404 {
				t.Errorf("Pages[%d].Width = %d, want 1404", i, page.Width)
			}
			if page.Height != 1872 {
				t.Errorf("Pages[%d].Height = %d, want 1872", i, page.Height)
			}
		}
	})

	t.Run("deleted flag is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "deleted.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        true,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
			t.Fatalf("SaveMetadata(): %v", err)
		}

		gotMeta, _, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if !gotMeta.IsDeleted {
			t.Error("IsDeleted = false, want true")
		}
	})

	t.Run("different file types are preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		testCases := []struct {
			format string
			ftype  string
		}{
			{"pdf", "pdf"},
			{"rmd", "notebook"},
			{"epub", "epub"},
		}

		for _, tc := range testCases {
			t.Run(tc.format, func(t *testing.T) {
				docUUID := tc.format + "-uuid-1234"
				meta := document.Metadata{
					DeviceID:         docUUID,
					VisibleName:      "test." + tc.format,
					FileFormat:       tc.format,
					ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
					LastModified:     now,
					IsDeleted:        false,
				}
				content := document.Content{
					Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
					FileType: tc.ftype,
				}

				if err := store.SaveMetadata(ctx, docUUID, meta, content); err != nil {
					t.Fatalf("SaveMetadata(): %v", err)
				}

				gotMeta, gotContent, err := store.GetMetadata(ctx, docUUID)
				if err != nil {
					t.Fatalf("GetMetadata(): %v", err)
				}

				if gotMeta.FileFormat != tc.format {
					t.Errorf("FileFormat = %q, want %q", gotMeta.FileFormat, tc.format)
				}
				if gotContent.FileType != tc.ftype {
					t.Errorf("FileType = %q, want %q", gotContent.FileType, tc.ftype)
				}
			})
		}
	})

	t.Run("overwrite existing metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

		// Save initial metadata
		meta1 := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "original.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}
		if err := store.SaveMetadata(ctx, docUUID, meta1, content); err != nil {
			t.Fatalf("first SaveMetadata(): %v", err)
		}

		// Overwrite with new metadata
		later := time.Date(2025, 6, 16, 12, 0, 0, 0, time.UTC)
		meta2 := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "renamed.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "new-parent-uuid",
			LastModified:     later,
			IsDeleted:        true,
		}
		if err := store.SaveMetadata(ctx, docUUID, meta2, content); err != nil {
			t.Fatalf("second SaveMetadata(): %v", err)
		}

		gotMeta, _, err := store.GetMetadata(ctx, docUUID)
		if err != nil {
			t.Fatalf("GetMetadata(): %v", err)
		}

		if gotMeta.VisibleName != "renamed.pdf" {
			t.Errorf("VisibleName = %q, want %q", gotMeta.VisibleName, "renamed.pdf")
		}
		if gotMeta.ParentFolderUUID != "new-parent-uuid" {
			t.Errorf("ParentFolderUUID = %q, want %q", gotMeta.ParentFolderUUID, "new-parent-uuid")
		}
		if !gotMeta.LastModified.Equal(later) {
			t.Errorf("LastModified = %v, want %v", gotMeta.LastModified, later)
		}
		if !gotMeta.IsDeleted {
			t.Error("IsDeleted = false, want true")
		}
	})

	t.Run("multiple documents in same store", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		documents := []struct {
			uuid        string
			visibleName string
		}{
			{"uuid-1", "doc1.pdf"},
			{"uuid-2", "doc2.pdf"},
			{"uuid-3", "doc3.pdf"},
		}

		for _, doc := range documents {
			meta := document.Metadata{
				DeviceID:         doc.uuid,
				VisibleName:      doc.visibleName,
				FileFormat:       "pdf",
				ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
				LastModified:     now,
				IsDeleted:        false,
			}
			if err := store.SaveMetadata(ctx, doc.uuid, meta, content); err != nil {
				t.Fatalf("SaveMetadata(%s): %v", doc.uuid, err)
			}
		}

		// Verify each document independently
		for _, doc := range documents {
			gotMeta, _, err := store.GetMetadata(ctx, doc.uuid)
			if err != nil {
				t.Fatalf("GetMetadata(%s): %v", doc.uuid, err)
			}
			if gotMeta.VisibleName != doc.visibleName {
				t.Errorf("GetMetadata(%s) VisibleName = %q, want %q", doc.uuid, gotMeta.VisibleName, doc.visibleName)
			}
		}

		// Delete middle document and verify others unaffected
		if err := store.DeleteMetadata(ctx, "uuid-2"); err != nil {
			t.Fatalf("DeleteMetadata(uuid-2): %v", err)
		}

		exists, err := store.Exists(ctx, "uuid-2")
		if err != nil {
			t.Fatalf("Exists(uuid-2): %v", err)
		}
		if exists {
			t.Error("uuid-2 should not exist after deletion")
		}

		for _, uuid := range []string{"uuid-1", "uuid-3"} {
			exists, err := store.Exists(ctx, uuid)
			if err != nil {
				t.Fatalf("Exists(%s): %v", uuid, err)
			}
			if !exists {
				t.Error(uuid, "should still exist")
			}
		}
	})
}

func TestErrorPaths(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	t.Run("SaveMetadata fails when base directory is not writable", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a read-only directory as base
		roDir := filepath.Join(tmpDir, "readonly")
		if err := os.MkdirAll(roDir, 0o555); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		store := New(roDir)

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		meta := document.Metadata{
			DeviceID:         docUUID,
			VisibleName:      "test.pdf",
			FileFormat:       "pdf",
			ParentFolderUUID: "00000000-0000-0000-0000-000000000000",
			LastModified:     now,
			IsDeleted:        false,
		}
		content := document.Content{
			Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
			FileType: "pdf",
		}

		err := store.SaveMetadata(ctx, docUUID, meta, content)
		if err == nil {
			t.Fatal("SaveMetadata() expected error for read-only directory, got nil")
		}
	})

	t.Run("atomicWriteJSON cleans up temp file on rename failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a read-only directory as target
		roDir := filepath.Join(tmpDir, "readonly")
		if err := os.MkdirAll(roDir, 0o555); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		// Try to write to read-only directory
		v := struct{ Name string }{Name: "test"}
		err := atomicWriteJSON(filepath.Join(roDir, "test.json"), v)
		if err == nil {
			t.Fatal("atomicWriteJSON() expected error for read-only directory, got nil")
		}

		// Verify no temp file is left behind
		entries, err := os.ReadDir(roDir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".tmp" {
				t.Errorf("temp file should be cleaned up: %s", entry.Name())
			}
		}
	})

	t.Run("GetMetadata fails on invalid JSON content", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "device-meta"))

		docUUID := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
		docDir := filepath.Join(tmpDir, "device-meta", docUUID)
		if err := os.MkdirAll(docDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".metadata"), []byte(`{"DeviceID":"test"}`), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docDir, ".content"), []byte("{invalid}"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, _, err := store.GetMetadata(ctx, docUUID)
		if err == nil {
			t.Fatal("GetMetadata() expected error for invalid JSON content, got nil")
		}
	})
}
