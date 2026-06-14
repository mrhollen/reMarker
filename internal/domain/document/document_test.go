package document

import (
	"testing"
	"time"

	"github.com/google/uuid"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// ============================================================================
// DocumentType tests
// ============================================================================

func TestDocumentType_Values(t *testing.T) {
	tests := []struct {
		docType DocumentType
		want    string
	}{
		{DocumentTypePDF, "pdf"},
		{DocumentTypeNotebook, "notebook"},
		{DocumentTypeEPub, "epub"},
		{DocumentTypeFolder, "folder"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.docType) != tc.want {
				t.Errorf("DocumentType = %q, want %q", tc.docType, tc.want)
			}
		})
	}
}

// ============================================================================
// Document tests
// ============================================================================

func TestNewDocument(t *testing.T) {
	tests := []struct {
		name        string
		localPath   string
		docType     DocumentType
		wantVisible string
		wantExt     string
	}{
		{
			name:        "simple pdf in root",
			localPath:   "documents/Annual Report.pdf",
			docType:     DocumentTypePDF,
			wantVisible: "Annual Report.pdf",
			wantExt:     "pdf",
		},
		{
			name:        "notebook in nested folder",
			localPath:   "documents/Projects/Meeting Notes.notebook",
			docType:     DocumentTypeNotebook,
			wantVisible: "Meeting Notes.notebook",
			wantExt:     "notebook",
		},
		{
			name:        "epub file",
			localPath:   "documents/Books/Design Patterns.epub",
			docType:     DocumentTypeEPub,
			wantVisible: "Design Patterns.epub",
			wantExt:     "epub",
		},
		{
			name:        "file with spaces in name",
			localPath:   "documents/My File (copy).pdf",
			docType:     DocumentTypePDF,
			wantVisible: "My File (copy).pdf",
			wantExt:     "pdf",
		},
		{
			name:        "file with dots in name",
			localPath:   "documents/version.2.final.pdf",
			docType:     DocumentTypePDF,
			wantVisible: "version.2.final.pdf",
			wantExt:     "pdf",
		},
		{
			name:        "file at root of sync dir",
			localPath:   "documents/readme.pdf",
			docType:     DocumentTypePDF,
			wantVisible: "readme.pdf",
			wantExt:     "pdf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := NewDocument(tc.localPath, tc.docType)

			// UUID must be valid
			if doc.ID == uuid.Nil {
				t.Error("NewDocument() ID is nil UUID")
			}

			// LocalPath must match
			if doc.LocalPath != tc.localPath {
				t.Errorf("LocalPath = %q, want %q", doc.LocalPath, tc.localPath)
			}

			// Type must match
			if doc.Type != tc.docType {
				t.Errorf("Type = %q, want %q", doc.Type, tc.docType)
			}

			// VisibleName must be derived from filename
			if doc.VisibleName != tc.wantVisible {
				t.Errorf("VisibleName = %q, want %q", doc.VisibleName, tc.wantVisible)
			}

			// Hashes should be empty initially
			if doc.LocalHash != "" {
				t.Errorf("LocalHash = %q, want empty", doc.LocalHash)
			}
			if doc.DeviceHash != "" {
				t.Errorf("DeviceHash = %q, want empty", doc.DeviceHash)
			}

			// ModTime should be zero
			if !doc.ModTime.IsZero() {
				t.Errorf("ModTime = %v, want zero time", doc.ModTime)
			}

			// SyncedAt should be zero
			if !doc.SyncedAt.IsZero() {
				t.Errorf("SyncedAt = %v, want zero time", doc.SyncedAt)
			}
		})
	}
}

func TestDocument_IsSynced(t *testing.T) {
	tests := []struct {
		name       string
		localHash  string
		deviceHash string
		wantSynced bool
	}{
		{
			name:       "matching hashes are synced",
			localHash:  "abc123",
			deviceHash: "abc123",
			wantSynced: true,
		},
		{
			name:       "different hashes are not synced",
			localHash:  "abc123",
			deviceHash: "def456",
			wantSynced: false,
		},
		{
			name:       "both empty hashes are synced",
			localHash:  "",
			deviceHash: "",
			wantSynced: true,
		},
		{
			name:       "empty local hash not synced with device hash",
			localHash:  "",
			deviceHash: "abc123",
			wantSynced: false,
		},
		{
			name:       "empty device hash not synced with local hash",
			localHash:  "abc123",
			deviceHash: "",
			wantSynced: false,
		},
		{
			name:       "full sha256 hashes matching",
			localHash:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			deviceHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantSynced: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{
				LocalHash:  tc.localHash,
				DeviceHash: tc.deviceHash,
			}
			got := doc.IsSynced()
			if got != tc.wantSynced {
				t.Errorf("IsSynced() = %v, want %v", got, tc.wantSynced)
			}
		})
	}
}

func TestDocument_IsNewer(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		modTime   time.Time
		otherTime time.Time
		want      bool
	}{
		{
			name:      "later mod time is newer",
			modTime:   now.Add(time.Hour),
			otherTime: now,
			want:      true,
		},
		{
			name:      "earlier mod time is not newer",
			modTime:   now,
			otherTime: now.Add(time.Hour),
			want:      false,
		},
		{
			name:      "same mod time is not newer",
			modTime:   now,
			otherTime: now,
			want:      false,
		},
		{
			name:      "zero time is not newer than real time",
			modTime:   time.Time{},
			otherTime: now,
			want:      false,
		},
		{
			name:      "real time is newer than zero time",
			modTime:   now,
			otherTime: time.Time{},
			want:      true,
		},
		{
			name:      "both zero times are not newer",
			modTime:   time.Time{},
			otherTime: time.Time{},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{ModTime: tc.modTime}
			other := Document{ModTime: tc.otherTime}
			got := doc.IsNewer(other)
			if got != tc.want {
				t.Errorf("IsNewer() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDocument_ToDevicePath(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name  string
		docID uuid.UUID
		doc   DocumentType
		want  string
	}{
		{
			name:  "pdf device path",
			docID: id,
			doc:   DocumentTypePDF,
			want:  id.String() + ".pdf",
		},
		{
			name:  "notebook device path",
			docID: id,
			doc:   DocumentTypeNotebook,
			want:  id.String() + ".notebook",
		},
		{
			name:  "epub device path",
			docID: id,
			doc:   DocumentTypeEPub,
			want:  id.String() + ".epub",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := Document{
				ID:   tc.docID,
				Type: tc.doc,
			}
			got := doc.ToDevicePath()
			if got != tc.want {
				t.Errorf("ToDevicePath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDocument_ToDevicePath_ExtensionMapping(t *testing.T) {
	// Verify that each document type maps to the correct file extension
	tests := []struct {
		docType DocumentType
		wantExt string
	}{
		{DocumentTypePDF, ".pdf"},
		{DocumentTypeNotebook, ".notebook"},
		{DocumentTypeEPub, ".epub"},
	}

	for _, tc := range tests {
		t.Run(tc.wantExt, func(t *testing.T) {
			doc := Document{
				ID:   uuid.New(),
				Type: tc.docType,
			}
			got := doc.ToDevicePath()
			// The path should end with the expected extension
			if len(got) < len(tc.wantExt) || got[len(got)-len(tc.wantExt):] != tc.wantExt {
				t.Errorf("ToDevicePath() = %q, should end with %q", got, tc.wantExt)
			}
		})
	}
}

func TestDocument_FullyPopulated(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	doc := Document{
		ID:          id,
		LocalPath:   "documents/test.pdf",
		Type:        DocumentTypePDF,
		VisibleName: "test.pdf",
		LocalHash:   "local123",
		DeviceHash:  "local123",
		Size:        4096,
		ModTime:     now,
		SyncedAt:    now,
	}

	if doc.ID != id {
		t.Errorf("ID = %v, want %v", doc.ID, id)
	}
	if doc.Size != 4096 {
		t.Errorf("Size = %d, want 4096", doc.Size)
	}
	if !doc.IsSynced() {
		t.Error("Document with matching hashes should be synced")
	}
}

// ============================================================================
// Folder tests
// ============================================================================

func TestNewFolder(t *testing.T) {
	rootID := uuid.New()
	tests := []struct {
		name        string
		localPath   string
		visibleName string
		parentID    uuid.UUID
	}{
		{
			name:        "root folder",
			localPath:   "documents/",
			visibleName: "Documents",
			parentID:    uuid.Nil,
		},
		{
			name:        "nested folder",
			localPath:   "documents/Projects/",
			visibleName: "Projects",
			parentID:    rootID,
		},
		{
			name:        "deeply nested folder",
			localPath:   "documents/Projects/2024/",
			visibleName: "2024",
			parentID:    rootID,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			folder := NewFolder(tc.localPath, tc.visibleName, tc.parentID)

			// UUID must be valid and non-nil
			if folder.ID == uuid.Nil {
				t.Error("NewFolder() ID is nil UUID")
			}

			if folder.LocalPath != tc.localPath {
				t.Errorf("LocalPath = %q, want %q", folder.LocalPath, tc.localPath)
			}

			if folder.VisibleName != tc.visibleName {
				t.Errorf("VisibleName = %q, want %q", folder.VisibleName, tc.visibleName)
			}

			if folder.ParentID != tc.parentID {
				t.Errorf("ParentID = %v, want %v", folder.ParentID, tc.parentID)
			}
		})
	}
}

func TestFolder_IsRoot(t *testing.T) {
	tests := []struct {
		name     string
		parentID uuid.UUID
		wantRoot bool
	}{
		{
			name:     "nil UUID is root",
			parentID: uuid.Nil,
			wantRoot: true,
		},
		{
			name:     "zero value UUID is root",
			parentID: uuid.UUID{},
			wantRoot: true,
		},
		{
			name:     "non-nil UUID is not root",
			parentID: uuid.New(),
			wantRoot: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			folder := Folder{
				ID:       uuid.New(),
				ParentID: tc.parentID,
			}
			got := folder.IsRoot()
			if got != tc.wantRoot {
				t.Errorf("IsRoot() = %v, want %v", got, tc.wantRoot)
			}
		})
	}
}

// ============================================================================
// SidecarMetadata tests
// ============================================================================

func TestSidecarMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadataFile string
		contentFile  string
		pageDataFile string
	}{
		{
			name:         "all files present",
			metadataFile: `{"id":"abc-123"}`,
			contentFile:  `{"path":"abc-123.pdf"}`,
			pageDataFile: `{"pageCount":10}`,
		},
		{
			name:         "empty page data",
			metadataFile: `{"id":"abc-123"}`,
			contentFile:  `{"path":"abc-123.pdf"}`,
			pageDataFile: "",
		},
		{
			name:         "minimal notebook metadata",
			metadataFile: `{"id":"xyz-789"}`,
			contentFile:  `{"path":"xyz-789.notebook"}`,
			pageDataFile: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sm := SidecarMetadata{
				MetadataFile: tc.metadataFile,
				ContentFile:  tc.contentFile,
				PageDataFile: tc.pageDataFile,
			}

			if sm.MetadataFile != tc.metadataFile {
				t.Errorf("MetadataFile = %q, want %q", sm.MetadataFile, tc.metadataFile)
			}
			if sm.ContentFile != tc.contentFile {
				t.Errorf("ContentFile = %q, want %q", sm.ContentFile, tc.contentFile)
			}
			if sm.PageDataFile != tc.pageDataFile {
				t.Errorf("PageDataFile = %q, want %q", sm.PageDataFile, tc.pageDataFile)
			}
		})
	}
}

func TestSidecarMetadata_ZeroValue(t *testing.T) {
	sm := SidecarMetadata{}

	if sm.MetadataFile != "" {
		t.Errorf("zero value MetadataFile = %q, want empty", sm.MetadataFile)
	}
	if sm.ContentFile != "" {
		t.Errorf("zero value ContentFile = %q, want empty", sm.ContentFile)
	}
	if sm.PageDataFile != "" {
		t.Errorf("zero value PageDataFile = %q, want empty", sm.PageDataFile)
	}
}

// ============================================================================
// File (legacy) tests — preserved for backward compatibility
// ============================================================================

func TestFile_IsNewer(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		f      File
		other  File
		want   bool
	}{
		{
			name: "file with later mod time is newer",
			f:    File{Path: "a.metadata", ModTime: now.Add(time.Hour)},
			other: File{Path: "a.metadata", ModTime: now},
			want: true,
		},
		{
			name: "file with earlier mod time is not newer",
			f:    File{Path: "a.metadata", ModTime: now},
			other: File{Path: "a.metadata", ModTime: now.Add(time.Hour)},
			want: false,
		},
		{
			name: "file with same mod time is not newer",
			f:    File{Path: "a.metadata", ModTime: now},
			other: File{Path: "a.metadata", ModTime: now},
			want: false,
		},
		{
			name: "file with zero time is not newer than file with real time",
			f:    File{Path: "a.metadata", ModTime: time.Time{}},
			other: File{Path: "a.metadata", ModTime: now},
			want: false,
		},
		{
			name: "file with real time is newer than file with zero time",
			f:    File{Path: "a.metadata", ModTime: now},
			other: File{Path: "a.metadata", ModTime: time.Time{}},
			want: true,
		},
		{
			name: "both zero times are not newer",
			f:    File{Path: "a.metadata", ModTime: time.Time{}},
			other: File{Path: "a.metadata", ModTime: time.Time{}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.f.IsNewer(tc.other)
			if got != tc.want {
				t.Errorf("IsNewer() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFile_Structure(t *testing.T) {
	now := time.Now()
	f := File{
		Path:    "abc-123.metadata",
		Size:    4096,
		ModTime: now,
		Hash:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	if f.Path != "abc-123.metadata" {
		t.Errorf("Path = %q, want %q", f.Path, "abc-123.metadata")
	}
	if f.Size != 4096 {
		t.Errorf("Size = %d, want %d", f.Size, 4096)
	}
	if f.ModTime != now {
		t.Errorf("ModTime = %v, want %v", f.ModTime, now)
	}
	if f.Hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("Hash = %q, want SHA256 hex", f.Hash)
	}
}

// ============================================================================
// Manifest v2 tests
// ============================================================================

func TestNewManifest(t *testing.T) {
	tests := []struct {
		name       string
		wantVersion int
	}{
		{
			name:       "creates version 2 manifest",
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
// SyncAction tests (with Document types)
// ============================================================================

func TestSyncAction_DocumentTypes(t *testing.T) {
	now := time.Now()
	sourceDoc := Document{
		ID:          uuid.New(),
		LocalPath:   "documents/test.pdf",
		Type:        DocumentTypePDF,
		VisibleName: "test.pdf",
		LocalHash:   "local_hash",
		DeviceHash:  "",
		Size:        1024,
		ModTime:     now,
	}

	destDoc := Document{
		ID:          uuid.New(),
		LocalPath:   "documents/test.pdf",
		Type:        DocumentTypePDF,
		VisibleName: "test.pdf",
		LocalHash:   "",
		DeviceHash:  "device_hash",
		Size:        1024,
		ModTime:     now.Add(-time.Hour),
	}

	tests := []struct {
		name       string
		actionType ActionType
	}{
		{
			name:       "push action with documents",
			actionType: ActionPush,
		},
		{
			name:       "pull action with documents",
			actionType: ActionPull,
		},
		{
			name:       "conflict action with documents",
			actionType: ActionConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sa := SyncAction{
				ActionType: tc.actionType,
				Path:       "documents/test.pdf",
				Source:     sourceDoc,
				Dest:       destDoc,
			}

			if sa.ActionType != tc.actionType {
				t.Errorf("ActionType = %v, want %v", sa.ActionType, tc.actionType)
			}
			if sa.Path != "documents/test.pdf" {
				t.Errorf("Path = %q, want %q", sa.Path, "documents/test.pdf")
			}
			if sa.Source.LocalHash != "local_hash" {
				t.Errorf("Source.LocalHash = %q, want %q", sa.Source.LocalHash, "local_hash")
			}
			if sa.Dest.DeviceHash != "device_hash" {
				t.Errorf("Dest.DeviceHash = %q, want %q", sa.Dest.DeviceHash, "device_hash")
			}
			if sa.Source.Type != DocumentTypePDF {
				t.Errorf("Source.Type = %q, want %q", sa.Source.Type, DocumentTypePDF)
			}
		})
	}
}

func TestSyncAction_ZeroValue(t *testing.T) {
	sa := SyncAction{}
	if sa.ActionType != "" {
		t.Errorf("zero value ActionType = %q, want empty", sa.ActionType)
	}
	if sa.Path != "" {
		t.Errorf("zero value Path = %q, want empty", sa.Path)
	}
	// Source and Dest should be zero-value Documents
	if sa.Source.ID != uuid.Nil {
		t.Error("zero value Source.ID should be nil UUID")
	}
	if sa.Dest.ID != uuid.Nil {
		t.Error("zero value Dest.ID should be nil UUID")
	}
}

// ============================================================================
// SyncResult tests
// ============================================================================

func TestSyncResult_HasConflicts(t *testing.T) {
	tests := []struct {
		name     string
		result   SyncResult
		wantTrue bool
	}{
		{
			name: "empty result has no conflicts",
			result: SyncResult{
				Actions:   []SyncAction{},
				Conflicts: []domainErrors.ConflictError{},
				Errors:    []error{},
			},
			wantTrue: false,
		},
		{
			name: "nil slices has no conflicts",
			result: SyncResult{
				Actions:   nil,
				Conflicts: nil,
				Errors:    nil,
			},
			wantTrue: false,
		},
		{
			name: "one conflict returns true",
			result: SyncResult{
				Conflicts: []domainErrors.ConflictError{{Path: "documents/test.pdf"}},
			},
			wantTrue: true,
		},
		{
			name: "multiple conflicts returns true",
			result: SyncResult{
				Conflicts: []domainErrors.ConflictError{
					{Path: "documents/a.pdf"},
					{Path: "documents/b.pdf"},
				},
			},
			wantTrue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.result.HasConflicts()
			if got != tc.wantTrue {
				t.Errorf("HasConflicts() = %v, want %v", got, tc.wantTrue)
			}
		})
	}
}

func TestSyncResult_HasActions(t *testing.T) {
	tests := []struct {
		name     string
		result   SyncResult
		wantTrue bool
	}{
		{
			name: "empty result has no actions",
			result: SyncResult{
				Actions: []SyncAction{},
			},
			wantTrue: false,
		},
		{
			name: "nil actions has no actions",
			result: SyncResult{
				Actions: nil,
			},
			wantTrue: false,
		},
		{
			name: "one action returns true",
			result: SyncResult{
				Actions: []SyncAction{{
					ActionType: ActionPush,
					Path:       "documents/test.pdf",
					Source:     Document{Type: DocumentTypePDF},
					Dest:       Document{Type: DocumentTypePDF},
				}},
			},
			wantTrue: true,
		},
		{
			name: "multiple actions returns true",
			result: SyncResult{
				Actions: []SyncAction{
					{ActionType: ActionPush, Path: "documents/a.pdf", Source: Document{Type: DocumentTypePDF}, Dest: Document{Type: DocumentTypePDF}},
					{ActionType: ActionPull, Path: "documents/b.notebook", Source: Document{Type: DocumentTypeNotebook}, Dest: Document{Type: DocumentTypeNotebook}},
				},
			},
			wantTrue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.result.HasActions()
			if got != tc.wantTrue {
				t.Errorf("HasActions() = %v, want %v", got, tc.wantTrue)
			}
		})
	}
}

func TestActionType_Values(t *testing.T) {
	// Verify all action type constants have expected string values
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionNone, "none"},
		{ActionPush, "push"},
		{ActionPull, "pull"},
		{ActionConflict, "conflict"},
		{ActionDeleteLocal, "delete_local"},
		{ActionDeleteDevice, "delete_device"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.action) != tc.want {
				t.Errorf("ActionType = %q, want %q", tc.action, tc.want)
			}
		})
	}
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
