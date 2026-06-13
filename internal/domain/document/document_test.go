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
// Existing tests (preserved for backward compatibility)
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

func TestManifest_Get(t *testing.T) {
	now := time.Now()
	entry := ManifestEntry{
		Path:     "abc.metadata",
		Hash:     "sha256hash",
		Size:     1024,
		ModTime:  now,
		SyncedAt: now,
	}

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
				"abc.metadata": entry,
			},
			path:   "abc.metadata",
			want:   entry,
			wantOK: true,
		},
		{
			name:    "missing entry returns zero value",
			entries: map[string]ManifestEntry{},
			path:    "missing.metadata",
			want:    ManifestEntry{},
			wantOK:  false,
		},
		{
			name: "nil entries map returns false",
			entries: nil,
			path:    "abc.metadata",
			want:    ManifestEntry{},
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: tc.entries}
			got, ok := m.Get(tc.path)
			if ok != tc.wantOK {
				t.Errorf("Get() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("Get() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestManifest_Set(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		path    string
		entry   ManifestEntry
		wantKey string
		wantVal ManifestEntry
	}{
		{
			name: "add new entry",
			path: "abc.metadata",
			entry: ManifestEntry{
				Path:    "abc.metadata",
				Hash:    "hash1",
				Size:    100,
				ModTime: now,
			},
			wantKey: "abc.metadata",
			wantVal: ManifestEntry{
				Path:    "abc.metadata",
				Hash:    "hash1",
				Size:    100,
				ModTime: now,
			},
		},
		{
			name: "overwrite existing entry",
			path: "abc.metadata",
			entry: ManifestEntry{
				Path:    "abc.metadata",
				Hash:    "hash2",
				Size:    200,
				ModTime: now,
			},
			wantKey: "abc.metadata",
			wantVal: ManifestEntry{
				Path:    "abc.metadata",
				Hash:    "hash2",
				Size:    200,
				ModTime: now,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: map[string]ManifestEntry{}}
			m.Set(tc.path, tc.entry)

			got, ok := m.Get(tc.path)
			if !ok {
				t.Fatalf("Get() after Set() returned false")
			}
			if got != tc.wantVal {
				t.Errorf("Get() = %+v, want %+v", got, tc.wantVal)
			}
		})
	}
}

func TestManifest_Delete(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		entries map[string]ManifestEntry
		delete  string
		wantOK  bool
	}{
		{
			name: "delete existing entry",
			entries: map[string]ManifestEntry{
				"abc.metadata": {Path: "abc.metadata", ModTime: now},
			},
			delete: "abc.metadata",
			wantOK: true,
		},
		{
			name: "delete non-existent entry",
			entries: map[string]ManifestEntry{
				"abc.metadata": {Path: "abc.metadata", ModTime: now},
			},
			delete: "other.metadata",
			wantOK: false,
		},
		{
			name:    "delete from nil entries",
			entries: nil,
			delete:  "abc.metadata",
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manifest{Entries: tc.entries}
			ok := m.Delete(tc.delete)
			if ok != tc.wantOK {
				t.Errorf("Delete() = %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

func TestManifest_Touch(t *testing.T) {
	tests := []struct {
		name     string
		lastSync *time.Time
	}{
		{
			name:     "sets LastSync when nil",
			lastSync: nil,
		},
		{
			name: "updates LastSync when already set",
			lastSync: func() *time.Time {
				t := time.Now().Add(-time.Hour)
				return &t
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now().Add(-time.Second)
			m := &Manifest{LastSync: tc.lastSync}
			m.Touch()
			after := time.Now()

			if m.LastSync == nil {
				t.Fatal("Touch() did not set LastSync")
			}
			if m.LastSync.Before(before) || m.LastSync.After(after) {
				t.Errorf("Touch() LastSync = %v, should be between %v and %v", m.LastSync, before, after)
			}
		})
	}
}

func TestSyncResult_HasConflicts(t *testing.T) {
	tests := []struct {
		name      string
		result    SyncResult
		wantTrue  bool
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
				Conflicts: []domainErrors.ConflictError{{Path: "abc.metadata"}},
			},
			wantTrue: true,
		},
		{
			name: "multiple conflicts returns true",
			result: SyncResult{
				Conflicts: []domainErrors.ConflictError{
					{Path: "a.metadata"},
					{Path: "b.metadata"},
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
				Actions: []SyncAction{{ActionType: ActionPush, Path: "abc.metadata"}},
			},
			wantTrue: true,
		},
		{
			name: "multiple actions returns true",
			result: SyncResult{
				Actions: []SyncAction{
					{ActionType: ActionPush, Path: "a.metadata"},
					{ActionType: ActionPull, Path: "b.content"},
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

func TestManifest_Version(t *testing.T) {
	m := &Manifest{Version: 1}
	if m.Version != 1 {
		t.Errorf("Version = %d, want %d", m.Version, 1)
	}
}

func TestManifest_ZeroValue(t *testing.T) {
	m := &Manifest{}

	// Zero value manifest should work with all methods
	if m.Version != 0 {
		t.Errorf("zero value Version = %d, want 0", m.Version)
	}
	if m.LastSync != nil {
		t.Error("zero value LastSync should be nil")
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

func TestManifestEntry_Structure(t *testing.T) {
	now := time.Now()
	e := ManifestEntry{
		Path:     "abc.metadata",
		Hash:     "hash123",
		Size:     1024,
		ModTime:  now,
		SyncedAt: now,
	}

	if e.Path != "abc.metadata" {
		t.Errorf("Path = %q, want %q", e.Path, "abc.metadata")
	}
	if e.Hash != "hash123" {
		t.Errorf("Hash = %q, want %q", e.Hash, "hash123")
	}
	if e.Size != 1024 {
		t.Errorf("Size = %d, want %d", e.Size, 1024)
	}
}

func TestSyncAction_Structure(t *testing.T) {
	now := time.Now()
	sa := SyncAction{
		ActionType: ActionPush,
		Path:       "abc.metadata",
		Source: File{
			Path:    "abc.metadata",
			Size:    100,
			ModTime: now,
			Hash:    "h1",
		},
		Dest: File{
			Path:    "abc.metadata",
			Size:    0,
			ModTime: time.Time{},
			Hash:    "",
		},
	}

	if sa.ActionType != ActionPush {
		t.Errorf("ActionType = %v, want %v", sa.ActionType, ActionPush)
	}
	if sa.Path != "abc.metadata" {
		t.Errorf("Path = %q, want %q", sa.Path, "abc.metadata")
	}
	if sa.Source.Hash != "h1" {
		t.Errorf("Source.Hash = %q, want %q", sa.Source.Hash, "h1")
	}
}
