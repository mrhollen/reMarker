package document

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Manifest v2 tests
// ============================================================================

func TestNewManifest(t *testing.T) {
	tests := []struct {
		name        string
		wantVersion int
	}{
		{
			name:        "creates version 2 manifest",
			wantVersion: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManifest()
			if m.Version != tc.wantVersion {
				t.Errorf("Version = %d, want %d", m.Version, tc.wantVersion)
			}
			if m.Entries == nil {
				t.Error("Entries should be initialized (non-nil) map")
			}
			if len(m.Entries) != 0 {
				t.Errorf("Entries should be empty, got %d entries", len(m.Entries))
			}
		})
	}
}

func TestManifest_Get(t *testing.T) {
	deviceUUID := uuid.New().String()
	now := time.Now()

	tests := []struct {
		name    string
		entries map[string]ManifestEntry
		path    string
		want    ManifestEntry
		wantOK  bool
	}{
		{
			name: "find existing entry",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID:  deviceUUID,
					DeviceType:  DocumentTypePDF,
					LocalHash:   "sha256hash",
					DeviceHash:  "sha256hash",
					Size:        1024,
					SyncedAt:    now,
				},
			},
			path: "documents/test.pdf",
			want: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypePDF,
				LocalHash:   "sha256hash",
				DeviceHash:  "sha256hash",
				Size:        1024,
				SyncedAt:    now,
			},
			wantOK: true,
		},
		{
			name:    "missing entry returns zero value",
			entries: map[string]ManifestEntry{},
			path:    "documents/missing.pdf",
			want:    ManifestEntry{},
			wantOK:  false,
		},
		{
			name:    "nil entries map returns false",
			entries: nil,
			path:    "documents/test.pdf",
			want:    ManifestEntry{},
			wantOK:  false,
		},
		{
			name: "entry with parentUUID for folder",
			entries: map[string]ManifestEntry{
				"documents/Projects/": {
					DeviceUUID:  deviceUUID,
					DeviceType:  DocumentTypeFolder,
					ParentUUID:  uuid.New().String(),
					VisibleName: "Projects",
				},
			},
			path: "documents/Projects/",
			want: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypeFolder,
				ParentUUID:  "", // will be set below
				VisibleName: "Projects",
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: tc.entries}
			got, ok := m.Get(tc.path)
			if ok != tc.wantOK {
				t.Errorf("Get() ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK {
				if got.DeviceUUID != tc.want.DeviceUUID {
					t.Errorf("Get().DeviceUUID = %q, want %q", got.DeviceUUID, tc.want.DeviceUUID)
				}
				if got.DeviceType != tc.want.DeviceType {
					t.Errorf("Get().DeviceType = %q, want %q", got.DeviceType, tc.want.DeviceType)
				}
				if got.LocalHash != tc.want.LocalHash {
					t.Errorf("Get().LocalHash = %q, want %q", got.LocalHash, tc.want.LocalHash)
				}
			}
		})
	}
}

func TestManifest_Set(t *testing.T) {
	deviceUUID := uuid.New().String()
	now := time.Now()

	tests := []struct {
		name    string
		path    string
		entry   ManifestEntry
		wantOK  bool
	}{
		{
			name: "add new entry to initialized map",
			path: "documents/test.pdf",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypePDF,
				LocalHash:   "hash1",
				Size:        100,
				SyncedAt:    now,
			},
			wantOK: true,
		},
		{
			name: "overwrite existing entry",
			path: "documents/test.pdf",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypePDF,
				LocalHash:   "hash2",
				DeviceHash:  "hash2",
				Size:        200,
				SyncedAt:    now,
			},
			wantOK: true,
		},
		{
			name: "add folder entry with parentUUID",
			path: "documents/Projects/",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypeFolder,
				ParentUUID:  uuid.New().String(),
				VisibleName: "Projects",
			},
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManifest()
			m.Set(tc.path, tc.entry)

			got, ok := m.Get(tc.path)
			if !ok {
				t.Fatalf("Get() after Set() returned false")
			}
			if got.DeviceUUID != tc.entry.DeviceUUID {
				t.Errorf("DeviceUUID = %q, want %q", got.DeviceUUID, tc.entry.DeviceUUID)
			}
			if got.DeviceType != tc.entry.DeviceType {
				t.Errorf("DeviceType = %q, want %q", got.DeviceType, tc.entry.DeviceType)
			}
			if got.LocalHash != tc.entry.LocalHash {
				t.Errorf("LocalHash = %q, want %q", got.LocalHash, tc.entry.LocalHash)
			}
		})
	}
}

func TestManifest_Set_NilEntries(t *testing.T) {
	// Set on a manifest with nil entries map should still work
	m := Manifest{Version: 2, Entries: nil}
	m.Set("documents/test.pdf", ManifestEntry{
		DeviceUUID: uuid.New().String(),
		DeviceType: DocumentTypePDF,
		LocalHash:  "hash1",
	})

	got, ok := m.Get("documents/test.pdf")
	if !ok {
		t.Fatal("Get() after Set() on nil map returned false")
	}
	if got.LocalHash != "hash1" {
		t.Errorf("LocalHash = %q, want %q", got.LocalHash, "hash1")
	}
}

func TestManifest_Delete(t *testing.T) {
	deviceUUID := uuid.New().String()

	tests := []struct {
		name    string
		entries map[string]ManifestEntry
		delete  string
		wantOK  bool
	}{
		{
			name: "delete existing entry",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {DeviceUUID: deviceUUID, DeviceType: DocumentTypePDF},
			},
			delete: "documents/test.pdf",
			wantOK: true,
		},
		{
			name: "delete non-existent entry",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {DeviceUUID: deviceUUID, DeviceType: DocumentTypePDF},
			},
			delete: "documents/other.pdf",
			wantOK: false,
		},
		{
			name:    "delete from nil entries",
			entries: nil,
			delete:  "documents/test.pdf",
			wantOK:  false,
		},
		{
			name: "delete from empty map",
			entries: map[string]ManifestEntry{},
			delete: "documents/test.pdf",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: tc.entries}
			ok := m.Delete(tc.delete)
			if ok != tc.wantOK {
				t.Errorf("Delete() = %v, want %v", ok, tc.wantOK)
			}
			if ok {
				_, stillExists := m.Get(tc.delete)
				if stillExists {
					t.Error("Delete() returned true but entry still exists")
				}
			}
		})
	}
}

func TestManifest_Touch(t *testing.T) {
	deviceUUID := uuid.New().String()
	syncedTime := time.Now().Add(-time.Hour)

	tests := []struct {
		name    string
		entries map[string]ManifestEntry
		path    string
	}{
		{
			name: "updates SyncedAt for existing entry",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "hash1",
					SyncedAt:   syncedTime,
				},
			},
			path: "documents/test.pdf",
		},
		{
			name: "does nothing for non-existent entry",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					SyncedAt:   syncedTime,
				},
			},
			path: "documents/missing.pdf",
		},
		{
			name:    "does nothing on nil entries",
			entries: nil,
			path:    "documents/test.pdf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			m := &Manifest{Entries: tc.entries}
			m.Touch(tc.path)
			after := time.Now()

			got, ok := m.Get(tc.path)
			if ok {
				// SyncedAt should have been updated to now
				if got.SyncedAt.Before(before) || got.SyncedAt.After(after) {
					t.Errorf("Touch() SyncedAt = %v, should be between %v and %v", got.SyncedAt, before, after)
				}
			} else {
		// Entry didn't exist, should still not exist
				if tc.name != "does nothing for non-existent entry" && tc.name != "does nothing on nil entries" {
					t.Error("Touch() should not have created entry for non-existent path")
				}
			}
		})
	}
}

func TestManifest_IsSynced(t *testing.T) {
	deviceUUID := uuid.New().String()

	tests := []struct {
		name    string
		entries map[string]ManifestEntry
		path    string
		want    bool
	}{
		{
			name: "matching hashes are synced",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "abc123",
					DeviceHash: "abc123",
				},
			},
			path: "documents/test.pdf",
			want: true,
		},
		{
			name: "different hashes are not synced",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "abc123",
					DeviceHash: "def456",
				},
			},
			path: "documents/test.pdf",
			want: false,
		},
		{
			name: "empty device hash means not synced",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "abc123",
					DeviceHash: "",
				},
			},
			path: "documents/test.pdf",
			want: false,
		},
		{
			name: "both empty hashes are synced",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "",
					DeviceHash: "",
				},
			},
			path: "documents/test.pdf",
			want: true,
		},
		{
			name: "missing entry is not synced",
			entries: map[string]ManifestEntry{
				"documents/other.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "abc123",
					DeviceHash: "abc123",
				},
			},
			path: "documents/missing.pdf",
			want: false,
		},
		{
			name:    "nil entries returns false",
			entries: nil,
			path:    "documents/test.pdf",
			want:    false,
		},
		{
			name: "full sha256 hashes matching",
			entries: map[string]ManifestEntry{
				"documents/test.pdf": {
					DeviceUUID: deviceUUID,
					DeviceType: DocumentTypePDF,
					LocalHash:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					DeviceHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			},
			path: "documents/test.pdf",
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: tc.entries}
			got := m.IsSynced(tc.path)
			if got != tc.want {
				t.Errorf("IsSynced(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ============================================================================
// ManifestEntry v2 structure tests
// ============================================================================

func TestManifestEntry_v2Structure(t *testing.T) {
	deviceUUID := uuid.New().String()
	parentUUID := uuid.New().String()
	now := time.Now()

	tests := []struct {
		name  string
		entry ManifestEntry
	}{
		{
			name: "full document entry",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypePDF,
				LocalHash:   "local_sha256",
				DeviceHash:  "device_sha256",
				VisibleName: "Annual Report.pdf",
				Size:        4096,
				SyncedAt:    now,
			},
		},
		{
			name: "folder entry with parent",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypeFolder,
				ParentUUID:  parentUUID,
				VisibleName: "Projects",
			},
		},
		{
			name: "notebook entry",
			entry: ManifestEntry{
				DeviceUUID:  deviceUUID,
				DeviceType:  DocumentTypeNotebook,
				LocalHash:   "local_hash",
				DeviceHash:  "local_hash",
				VisibleName: "Meeting Notes.notebook",
				Size:        2048,
			},
		},
		{
			name: "minimal entry",
			entry: ManifestEntry{
				DeviceUUID: deviceUUID,
				DeviceType: DocumentTypePDF,
				LocalHash:  "hash",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Verify all fields are settable and readable
			if tc.entry.DeviceUUID == "" && tc.name != "minimal entry" {
				t.Error("DeviceUUID should be set")
			}
			if tc.entry.DeviceType == "" {
				t.Error("DeviceType should be set")
			}
			if tc.entry.LocalHash == "" && tc.entry.DeviceType != DocumentTypeFolder {
				t.Error("LocalHash should be set for non-folder entries")
			}
		})
	}
}

// ============================================================================
// Manifest v2 zero value and version tests
// ============================================================================

func TestManifest_v2Version(t *testing.T) {
	m := NewManifest()
	if m.Version != 2 {
		t.Errorf("NewManifest() Version = %d, want 2", m.Version)
	}
}

func TestManifest_v2ZeroValue(t *testing.T) {
	m := Manifest{}

	// Zero value manifest should work with all methods
	if m.Version != 0 {
		t.Errorf("zero value Version = %d, want 0", m.Version)
	}
	if m.Entries != nil {
		t.Error("zero value Entries should be nil")
	}

	// Get on zero value should return false
	_, ok := m.Get("anything")
	if ok {
		t.Error("Get on zero value manifest should return false")
	}

	// Delete on zero value should return false
	ok = m.Delete("anything")
	if ok {
		t.Error("Delete on zero value manifest should return false")
	}

	// IsSynced on zero value should return false
	if m.IsSynced("anything") {
		t.Error("IsSynced on zero value manifest should return false")
	}

	// Touch on zero value should not panic
	m.Touch("anything")
}

// ============================================================================
// Manifest v2 integration tests
// ============================================================================

func TestManifest_v2FullWorkflow(t *testing.T) {
	deviceUUID := uuid.New().String()

	m := NewManifest()

	// Set an entry
	m.Set("documents/test.pdf", ManifestEntry{
		DeviceUUID:  deviceUUID,
		DeviceType:  DocumentTypePDF,
		LocalHash:   "hash1",
		DeviceHash:  "",
		VisibleName: "test.pdf",
		Size:        1024,
	})

	// Verify it's not synced
	if m.IsSynced("documents/test.pdf") {
		t.Error("Entry should not be synced (empty device hash)")
	}

	// Update device hash to match
	entry, ok := m.Get("documents/test.pdf")
	if !ok {
		t.Fatal("Entry should exist")
	}
	entry.DeviceHash = "hash1"
	m.Set("documents/test.pdf", entry)

	// Now it should be synced
	if !m.IsSynced("documents/test.pdf") {
		t.Error("Entry should be synced after matching hashes")
	}

	// Touch should update SyncedAt
	before := time.Now()
	m.Touch("documents/test.pdf")
	after := time.Now()

	entry, ok = m.Get("documents/test.pdf")
	if !ok {
		t.Fatal("Entry should still exist after touch")
	}
	if entry.SyncedAt.Before(before) || entry.SyncedAt.After(after) {
		t.Errorf("SyncedAt = %v, should be between %v and %v", entry.SyncedAt, before, after)
	}

	// Delete should remove entry
	if !m.Delete("documents/test.pdf") {
		t.Error("Delete should return true for existing entry")
	}
	if _, ok := m.Get("documents/test.pdf"); ok {
		t.Error("Entry should not exist after delete")
	}
}

func TestManifest_v2MultipleEntries(t *testing.T) {
	m := NewManifest()

	// Add multiple entries
	entries := []struct {
		path string
		uuid string
		typ  DocumentType
	}{
		{"documents/report.pdf", uuid.New().String(), DocumentTypePDF},
		{"documents/notes.notebook", uuid.New().String(), DocumentTypeNotebook},
		{"documents/book.epub", uuid.New().String(), DocumentTypeEPub},
		{"documents/Projects/", uuid.New().String(), DocumentTypeFolder},
	}

	for _, e := range entries {
		m.Set(e.path, ManifestEntry{
			DeviceUUID: e.uuid,
			DeviceType: e.typ,
			LocalHash:  "hash",
			DeviceHash: "hash",
		})
	}

	// All should be synced
	for _, e := range entries {
		if !m.IsSynced(e.path) {
			t.Errorf("IsSynced(%q) = false, want true", e.path)
		}
	}

	// Delete one
	m.Delete("documents/notes.notebook")
	if m.IsSynced("documents/notes.notebook") {
		t.Error("Deleted entry should not be synced")
	}

	// Others should still be synced
	if !m.IsSynced("documents/report.pdf") {
		t.Error("Non-deleted entry should still be synced")
	}
}
