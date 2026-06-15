// Package sidecar provides a file-system implementation of the
// SidecarRepository interface for persisting reMarkable sidecar metadata
// files (.metadata, .content, .pagedata) per document.
package sidecar

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

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
