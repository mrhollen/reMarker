package document

import (
	"testing"
	"time"

	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

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
