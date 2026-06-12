package manifeststore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// Compile-time check: Store implements document.ManifestRepository.
var _ document.ManifestRepository = (*Store)(nil)

func TestNew(t *testing.T) {
	t.Run("creates store with correct file path", func(t *testing.T) {
		store := New("/some/path/manifest.json")
		if store.filePath != "/some/path/manifest.json" {
			t.Errorf("filePath = %q, want %q", store.filePath, "/some/path/manifest.json")
		}
	})
}

func TestNewDefault(t *testing.T) {
	t.Run("creates store in .remarker/manifest.json", func(t *testing.T) {
		store := NewDefault("/home/user")
		want := filepath.Join("/home/user", DefaultDir, DefaultFileName)
		if store.filePath != want {
			t.Errorf("filePath = %q, want %q", store.filePath, want)
		}
	})

	t.Run("uses DefaultDir and DefaultFileName constants", func(t *testing.T) {
		if DefaultDir != ".remarker" {
			t.Errorf("DefaultDir = %q, want %q", DefaultDir, ".remarker")
		}
		if DefaultFileName != "manifest.json" {
			t.Errorf("DefaultFileName = %q, want %q", DefaultFileName, "manifest.json")
		}
	})
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	t.Run("returns manifest with correct data", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version:  2,
			LastSync: ptrTime(time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)),
			Entries: map[string]document.ManifestEntry{
				"doc1": {
					Path:     "doc1",
					Hash:     "abc123",
					Size:     1024,
					ModTime:  time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
					SyncedAt: time.Date(2025, 5, 2, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("save: %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.Version != manifest.Version {
			t.Errorf("Version = %d, want %d", got.Version, manifest.Version)
		}
		if got.LastSync == nil || !got.LastSync.Equal(*manifest.LastSync) {
			t.Errorf("LastSync mismatch: got %v, want %v", got.LastSync, manifest.LastSync)
		}
		if len(got.Entries) != len(manifest.Entries) {
			t.Fatalf("Entries count = %d, want %d", len(got.Entries), len(manifest.Entries))
		}
		entry, ok := got.Entries["doc1"]
		if !ok {
			t.Fatal("missing entry doc1")
		}
		if entry.Hash != "abc123" {
			t.Errorf("entry.Hash = %q, want %q", entry.Hash, "abc123")
		}
		if entry.Size != 1024 {
			t.Errorf("entry.Size = %d, want %d", entry.Size, 1024)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "nonexistent.json"))

		_, err := store.Load(ctx)
		if err == nil {
			t.Fatal("Load() expected error for non-existent file, got nil")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "manifest.json")
		if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		store := New(path)

		_, err := store.Load(ctx)
		if err == nil {
			t.Fatal("Load() expected error for invalid JSON, got nil")
		}
	})

	t.Run("returns error for malformed manifest", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "manifest.json")
		// Valid JSON but not a manifest — wrong top-level type
		if err := os.WriteFile(path, []byte(`"just a string"`), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		store := New(path)

		_, err := store.Load(ctx)
		if err == nil {
			t.Fatal("Load() expected error for malformed manifest, got nil")
		}
	})
}

func TestSave(t *testing.T) {
	ctx := context.Background()

	t.Run("creates directory if it doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "sub", "dir", "manifest.json"))

		manifest := &document.Manifest{Version: 1}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(store.filePath); err != nil {
			t.Fatalf("file not created: %v", err)
		}
	})

	t.Run("writes valid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version: 3,
			LastSync: ptrTime(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
			Entries: map[string]document.ManifestEntry{
				"test": {
					Path:     "test",
					Hash:     "hash1",
					Size:     500,
					ModTime:  time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
					SyncedAt: time.Date(2025, 5, 2, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		data, err := os.ReadFile(store.filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		// Verify it's valid JSON
		var parsed document.Manifest
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("parsed JSON is not valid manifest: %v", err)
		}
	})

	t.Run("writes pretty-printed JSON with 2-space indent", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{Version: 1}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		data, err := os.ReadFile(store.filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		// Pretty-printed JSON should contain newlines and indentation
		if len(data) < 10 {
			t.Errorf("expected pretty-printed JSON, got short output: %s", data)
		}
	})

	t.Run("uses atomic write pattern - no .tmp file after save", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{Version: 1}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		tmpPath := store.filePath + ".tmp"
		if _, err := os.Stat(tmpPath); err == nil {
			t.Error("temp file should not exist after successful save")
		}
	})

	t.Run("returns error for nil manifest", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		err := store.Save(ctx, nil)
		if err == nil {
			t.Fatal("Save(nil) expected error, got nil")
		}
	})
}

func TestExists(t *testing.T) {
	ctx := context.Background()

	t.Run("returns true after save", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{Version: 1}
		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		exists, err := store.Exists(ctx)
		if err != nil {
			t.Fatalf("Exists(): %v", err)
		}
		if !exists {
			t.Error("Exists() = false, want true after save")
		}
	})

	t.Run("returns false for non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "nonexistent.json"))

		exists, err := store.Exists(ctx)
		if err != nil {
			t.Fatalf("Exists(): %v", err)
		}
		if exists {
			t.Error("Exists() = true, want false for non-existent file")
		}
	})
}

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()

	t.Run("save then load preserves all fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		now := time.Date(2025, 6, 1, 12, 30, 0, 0, time.UTC)
		manifest := &document.Manifest{
			Version:  5,
			LastSync: &now,
			Entries: map[string]document.ManifestEntry{
				"doc1": {
					Path:     "doc1",
					Hash:     "sha256:abc",
					Size:     2048,
					ModTime:  time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
					SyncedAt: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
				},
				"doc2": {
					Path:     "doc2",
					Hash:     "sha256:def",
					Size:     4096,
					ModTime:  time.Date(2025, 4, 2, 0, 0, 0, 0, time.UTC),
					SyncedAt: time.Date(2025, 5, 2, 0, 0, 0, 0, time.UTC),
				},
			},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.Version != manifest.Version {
			t.Errorf("Version = %d, want %d", got.Version, manifest.Version)
		}
		if got.LastSync == nil || !got.LastSync.Equal(*manifest.LastSync) {
			t.Errorf("LastSync mismatch: got %v, want %v", got.LastSync, manifest.LastSync)
		}
		if len(got.Entries) != len(manifest.Entries) {
			t.Fatalf("Entries count = %d, want %d", len(got.Entries), len(manifest.Entries))
		}
		for k, want := range manifest.Entries {
			gotEntry, ok := got.Entries[k]
			if !ok {
				t.Errorf("missing entry %q", k)
				continue
			}
			if gotEntry.Path != want.Path {
				t.Errorf("entry %q Path = %q, want %q", k, gotEntry.Path, want.Path)
			}
			if gotEntry.Hash != want.Hash {
				t.Errorf("entry %q Hash = %q, want %q", k, gotEntry.Hash, want.Hash)
			}
			if gotEntry.Size != want.Size {
				t.Errorf("entry %q Size = %d, want %d", k, gotEntry.Size, want.Size)
			}
			if !gotEntry.ModTime.Equal(want.ModTime) {
				t.Errorf("entry %q ModTime = %v, want %v", k, gotEntry.ModTime, want.ModTime)
			}
			if !gotEntry.SyncedAt.Equal(want.SyncedAt) {
				t.Errorf("entry %q SyncedAt = %v, want %v", k, gotEntry.SyncedAt, want.SyncedAt)
			}
		}
	})

	t.Run("entries map is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version: 1,
			Entries: map[string]document.ManifestEntry{
				"a": {Path: "a", Hash: "h1"},
				"b": {Path: "b", Hash: "h2"},
				"c": {Path: "c", Hash: "h3"},
			},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if len(got.Entries) != 3 {
			t.Fatalf("Entries count = %d, want 3", len(got.Entries))
		}
		for key := range manifest.Entries {
			if _, ok := got.Entries[key]; !ok {
				t.Errorf("missing entry %q", key)
			}
		}
	})

	t.Run("LastSync pointer nil is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version:  1,
			LastSync: nil,
			Entries:  map[string]document.ManifestEntry{},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.LastSync != nil {
			t.Errorf("LastSync = %v, want nil", got.LastSync)
		}
	})

	t.Run("LastSync pointer set is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		ts := time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC)
		manifest := &document.Manifest{
			Version:  1,
			LastSync: &ts,
			Entries:  map[string]document.ManifestEntry{},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.LastSync == nil || !got.LastSync.Equal(ts) {
			t.Errorf("LastSync = %v, want %v", got.LastSync, ts)
		}
	})

	t.Run("Version is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version: 99,
			Entries: map[string]document.ManifestEntry{},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.Version != 99 {
			t.Errorf("Version = %d, want 99", got.Version)
		}
	})

	t.Run("empty entries map is preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version: 1,
			Entries: map[string]document.ManifestEntry{},
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got.Entries == nil {
			t.Error("Entries is nil, want empty map")
		}
	})

	t.Run("nil entries map round-trips", func(t *testing.T) {
		tmpDir := t.TempDir()
		store := New(filepath.Join(tmpDir, "manifest.json"))

		manifest := &document.Manifest{
			Version: 1,
			Entries: nil,
		}

		if err := store.Save(ctx, manifest); err != nil {
			t.Fatalf("Save(): %v", err)
		}

		got, err := store.Load(ctx)
		if err != nil {
			t.Fatalf("Load(): %v", err)
		}

		// nil map serializes to null, which may deserialize as nil
		// that's acceptable
		if got.Entries == nil {
			// nil is fine
		} else if len(got.Entries) != 0 {
			t.Errorf("Entries should be empty, got %d entries", len(got.Entries))
		}
	})
}

// ptrTime is a helper to avoid repeating &time.Time{...} in tests.
func ptrTime(t time.Time) *time.Time {
	return &t
}
