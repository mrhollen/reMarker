// Package status provides a dry-run preview of what changes a sync would make,
// without actually applying any changes.
package status

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// Mock repositories
// ---------------------------------------------------------------------------

// mockDeviceRepository implements document.DeviceRepository for testing.
type mockDeviceRepository struct {
	files       map[string]document.File
	getFileErr  error
	putFileErr  error
	deleteErr   error
	listErr     error
	putFileCall func(ctx context.Context, file document.File) error
}

func (m *mockDeviceRepository) ListFiles(_ context.Context) ([]document.File, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var files []document.File
	for _, f := range m.files {
		files = append(files, f)
	}
	return files, nil
}

func (m *mockDeviceRepository) GetFile(_ context.Context, path string) (document.File, error) {
	if m.getFileErr != nil {
		return document.File{}, m.getFileErr
	}
	f, ok := m.files[path]
	if !ok {
		return document.File{}, nil
	}
	return f, nil
}

func (m *mockDeviceRepository) PutFile(ctx context.Context, file document.File) error {
	if m.putFileCall != nil {
		return m.putFileCall(ctx, file)
	}
	if m.putFileErr != nil {
		return m.putFileErr
	}
	if m.files == nil {
		m.files = make(map[string]document.File)
	}
	m.files[file.Path] = file
	return nil
}

func (m *mockDeviceRepository) DeleteFile(_ context.Context, path string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.files, path)
	return nil
}

// Compile-time check.
var _ document.DeviceRepository = (*mockDeviceRepository)(nil)

// mockLocalRepository implements document.LocalRepository for testing.
type mockLocalRepository struct {
	files       map[string]document.File
	getFileErr  error
	putFileErr  error
	deleteErr   error
	listErr     error
	putFileCall func(ctx context.Context, file document.File) error
}

func (m *mockLocalRepository) ListFiles(_ context.Context) ([]document.File, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var files []document.File
	for _, f := range m.files {
		files = append(files, f)
	}
	return files, nil
}

func (m *mockLocalRepository) GetFile(_ context.Context, path string) (document.File, error) {
	if m.getFileErr != nil {
		return document.File{}, m.getFileErr
	}
	f, ok := m.files[path]
	if !ok {
		return document.File{}, nil
	}
	return f, nil
}

func (m *mockLocalRepository) PutFile(ctx context.Context, file document.File) error {
	if m.putFileCall != nil {
		return m.putFileCall(ctx, file)
	}
	if m.putFileErr != nil {
		return m.putFileErr
	}
	if m.files == nil {
		m.files = make(map[string]document.File)
	}
	m.files[file.Path] = file
	return nil
}

func (m *mockLocalRepository) DeleteFile(_ context.Context, path string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.files, path)
	return nil
}

// Compile-time check.
var _ document.LocalRepository = (*mockLocalRepository)(nil)

// mockManifestRepository implements document.ManifestRepository for testing.
type mockManifestRepository struct {
	manifest    *document.Manifest
	loadErr     error
	saveErr     error
	exists      bool
	existsErr   error
	saveCalled  bool
	savedManifest *document.Manifest
}

func (m *mockManifestRepository) Load(_ context.Context) (*document.Manifest, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.manifest, nil
}

func (m *mockManifestRepository) Save(_ context.Context, manifest *document.Manifest) error {
	m.saveCalled = true
	m.savedManifest = manifest
	if m.saveErr != nil {
		return m.saveErr
	}
	return nil
}

func (m *mockManifestRepository) Exists(_ context.Context) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	return m.exists, nil
}

// Compile-time check.
var _ document.ManifestRepository = (*mockManifestRepository)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func file(path, hash string, size int64, modTime time.Time) document.File {
	return document.File{Path: path, Hash: hash, Size: size, ModTime: modTime}
}

func entry(path, hash string, size int64, modTime, syncedAt time.Time) document.ManifestEntry {
	return document.ManifestEntry{
		Path:     path,
		Hash:     hash,
		Size:     size,
		ModTime:  modTime,
		SyncedAt: syncedAt,
	}
}

func manifest(version int, entries map[string]document.ManifestEntry) *document.Manifest {
	return &document.Manifest{
		Version: version,
		Entries: entries,
	}
}

// ---------------------------------------------------------------------------
// Tests: StatusUseCase
// ---------------------------------------------------------------------------

func TestNewStatusUseCase(t *testing.T) {
	uc := NewStatusUseCase(&mockDeviceRepository{}, &mockLocalRepository{}, &mockManifestRepository{})
	if uc == nil {
		t.Fatal("NewStatusUseCase() returned nil")
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name              string
		deviceRepo        *mockDeviceRepository
		localRepo         *mockLocalRepository
		manifestRepo      *mockManifestRepository
		wantErr           bool
		wantErrContains   string
		wantToDevice      int
		wantToLocal       int
		wantConflicts     int
		wantHasChanges    bool
		wantSummaryPrefix string
		wantNoSave        bool // manifest should NOT be saved
	}{
		{
			name:       "empty directories, no manifest",
			deviceRepo: &mockDeviceRepository{files: map[string]document.File{}},
			localRepo:  &mockLocalRepository{files: map[string]document.File{}},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantToDevice:    0,
			wantToLocal:     0,
			wantConflicts:   0,
			wantHasChanges:  false,
			wantSummaryPrefix: "Already in sync",
			wantNoSave:      true,
		},
		{
			name:       "files only on local",
			deviceRepo: &mockDeviceRepository{files: map[string]document.File{}},
			localRepo: &mockLocalRepository{files: map[string]document.File{
				"a.metadata": file("a.metadata", "hashA", 100, time.Now()),
				"b.content":  file("b.content", "hashB", 200, time.Now()),
				"c.note":     file("c.note", "hashC", 300, time.Now()),
			}},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantToDevice:    3,
			wantToLocal:     0,
			wantConflicts:   0,
			wantHasChanges:  true,
			wantSummaryPrefix: "Would sync",
			wantNoSave:      true,
		},
		{
			name:       "files only on device",
			deviceRepo: &mockDeviceRepository{files: map[string]document.File{
				"x.metadata": file("x.metadata", "hashX", 100, time.Now()),
				"y.content":  file("y.content", "hashY", 200, time.Now()),
			}},
			localRepo:  &mockLocalRepository{files: map[string]document.File{}},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantToDevice:    0,
			wantToLocal:     2,
			wantConflicts:   0,
			wantHasChanges:  true,
			wantSummaryPrefix: "Would sync",
			wantNoSave:      true,
		},
		{
			name:       "file changed on both sides",
			deviceRepo: &mockDeviceRepository{files: map[string]document.File{
				"doc.metadata": file("doc.metadata", "deviceHash", 100, time.Now()),
			}},
			localRepo: &mockLocalRepository{files: map[string]document.File{
				"doc.metadata": file("doc.metadata", "localHash", 100, time.Now().Add(-time.Hour)),
			}},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantToDevice:      0,
			wantToLocal:       0,
			wantConflicts:     1,
			wantHasChanges:    true,
			wantSummaryPrefix: "Would sync",
			wantNoSave:        true,
		},
		{
			name:       "already in sync",
			deviceRepo: &mockDeviceRepository{files: map[string]document.File{
				"synced.metadata": file("synced.metadata", "sameHash", 100, time.Now()),
			}},
			localRepo: &mockLocalRepository{files: map[string]document.File{
				"synced.metadata": file("synced.metadata", "sameHash", 100, time.Now()),
			}},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{
					"synced.metadata": entry("synced.metadata", "sameHash", 100, time.Now(), time.Now()),
				}),
			},
			wantToDevice:      0,
			wantToLocal:       0,
			wantConflicts:     0,
			wantHasChanges:    false,
			wantSummaryPrefix: "Already in sync",
			wantNoSave:        true,
		},
		{
			name:       "manifest load error",
			deviceRepo: &mockDeviceRepository{},
			localRepo:  &mockLocalRepository{},
			manifestRepo: &mockManifestRepository{
				loadErr: errors.New("manifest corrupted"),
			},
			wantErr:         true,
			wantErrContains: "manifest error",
		},
		{
			name:       "device list error",
			deviceRepo: &mockDeviceRepository{listErr: errors.New("SSH connection lost")},
			localRepo:  &mockLocalRepository{},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantErr:         true,
			wantErrContains: "list device files",
		},
		{
			name:       "local list error",
			deviceRepo: &mockDeviceRepository{},
			localRepo:  &mockLocalRepository{listErr: errors.New("permission denied")},
			manifestRepo: &mockManifestRepository{
				manifest: manifest(1, map[string]document.ManifestEntry{}),
			},
			wantErr:         true,
			wantErrContains: "list local files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewStatusUseCase(tc.deviceRepo, tc.localRepo, tc.manifestRepo)
			result, err := uc.Execute(context.Background())

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Errorf("expected error containing %q, got %q", tc.wantErrContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			// Check ToDevice count
			if len(result.ToDevice) != tc.wantToDevice {
				t.Errorf("ToDevice: got %d, want %d", len(result.ToDevice), tc.wantToDevice)
			}

			// Check ToLocal count
			if len(result.ToLocal) != tc.wantToLocal {
				t.Errorf("ToLocal: got %d, want %d", len(result.ToLocal), tc.wantToLocal)
			}

			// Check Conflicts count
			if len(result.Conflicts) != tc.wantConflicts {
				t.Errorf("Conflicts: got %d, want %d", len(result.Conflicts), tc.wantConflicts)
			}

			// Check HasChanges
			gotHasChanges := result.HasChanges()
			if gotHasChanges != tc.wantHasChanges {
				t.Errorf("HasChanges(): got %v, want %v", gotHasChanges, tc.wantHasChanges)
			}

			// Check Summary prefix
			summary := result.Summary()
			if !strings.HasPrefix(summary, tc.wantSummaryPrefix) {
				t.Errorf("Summary() = %q, want prefix %q", summary, tc.wantSummaryPrefix)
			}

			// Verify manifest was NOT saved (key difference from sync)
			if tc.wantNoSave && tc.manifestRepo.saveCalled {
				t.Error("manifest should NOT be saved by status use case")
			}
		})
	}
}

func TestExecute_NoPutFileCalled(t *testing.T) {
	// Status should never call PutFile on either repository
	now := time.Now()
	putFileCalled := false

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{},
		putFileCall: func(_ context.Context, _ document.File) error {
			putFileCalled = true
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"new.metadata": file("new.metadata", "newhash", 100, now),
		},
		putFileCall: func(_ context.Context, _ document.File) error {
			putFileCalled = true
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewStatusUseCase(deviceRepo, localRepo, manifestRepo)
	result, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if putFileCalled {
		t.Error("status should not call PutFile on any repository")
	}

	// It should still identify the file as needing to be pushed
	if len(result.ToDevice) != 1 {
		t.Errorf("expected 1 ToDevice action, got %d", len(result.ToDevice))
	}
}

func TestExecute_MixedScenario(t *testing.T) {
	// Complex scenario: push, pull, conflict, and in-sync all at once
	now := time.Now()
	manifestTime := now.Add(-2 * time.Hour)

	uc := NewStatusUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"synced.metadata":    file("synced.metadata", "hash1", 100, now),
			"deviceonly.content": file("deviceonly.content", "hash2", 200, now),
			"conflict.metadata":  file("conflict.metadata", "deviceHash", 100, now),
		}},
		&mockLocalRepository{files: map[string]document.File{
			"synced.metadata":    file("synced.metadata", "hash1", 100, now),
			"localonly.metadata": file("localonly.metadata", "hash3", 300, now),
			"conflict.metadata":  file("conflict.metadata", "localHash", 100, now.Add(-time.Hour)),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata":   entry("synced.metadata", "hash1", 100, now, now),
				"conflict.metadata": entry("conflict.metadata", "originalHash", 100, manifestTime, manifestTime),
			}),
		},
	)

	result, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 push (localonly.metadata)
	if len(result.ToDevice) != 1 {
		t.Errorf("ToDevice: got %d, want 1", len(result.ToDevice))
	}

	// 1 pull (deviceonly.content)
	if len(result.ToLocal) != 1 {
		t.Errorf("ToLocal: got %d, want 1", len(result.ToLocal))
	}

	// 1 conflict (conflict.metadata)
	if len(result.Conflicts) != 1 {
		t.Errorf("Conflicts: got %d, want 1", len(result.Conflicts))
	}

	if !result.HasChanges() {
		t.Error("HasChanges() should be true for mixed scenario")
	}

	summary := result.Summary()
	if !strings.Contains(summary, "→ device: 1") {
		t.Errorf("summary should mention 1 device file, got: %s", summary)
	}
	if !strings.Contains(summary, "← local: 1") {
		t.Errorf("summary should mention 1 local file, got: %s", summary)
	}
	if !strings.Contains(summary, "conflicts: 1") {
		t.Errorf("summary should mention 1 conflict, got: %s", summary)
	}
}

// ---------------------------------------------------------------------------
// Tests: StatusResult
// ---------------------------------------------------------------------------

func TestStatusResult_HasChanges(t *testing.T) {
	tests := []struct {
		name string
		res  *StatusResult
		want bool
	}{
		{
			name: "all nil",
			res:  &StatusResult{},
			want: false,
		},
		{
			name: "all empty slices",
			res: &StatusResult{
				ToDevice:  []document.SyncAction{},
				ToLocal:   []document.SyncAction{},
				Conflicts: []domainErrors.ConflictError{},
			},
			want: false,
		},
		{
			name: "has device actions",
			res: &StatusResult{
				ToDevice: []document.SyncAction{{ActionType: document.ActionPush, Path: "a.metadata"}},
			},
			want: true,
		},
		{
			name: "has local actions",
			res: &StatusResult{
				ToLocal: []document.SyncAction{{ActionType: document.ActionPull, Path: "b.content"}},
			},
			want: true,
		},
		{
			name: "has conflicts",
			res: &StatusResult{
				Conflicts: []domainErrors.ConflictError{{Path: "c.metadata"}},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.HasChanges()
			if got != tc.want {
				t.Errorf("HasChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatusResult_Summary(t *testing.T) {
	tests := []struct {
		name string
		res  *StatusResult
		want string
	}{
		{
			name: "no changes",
			res:  &StatusResult{},
			want: "Already in sync.\n",
		},
		{
			name: "device and local actions with conflicts",
			res: &StatusResult{
				ToDevice: []document.SyncAction{
					{ActionType: document.ActionPush, Path: "a.metadata"},
					{ActionType: document.ActionPush, Path: "b.content"},
					{ActionType: document.ActionPush, Path: "c.note"},
				},
				ToLocal: []document.SyncAction{
					{ActionType: document.ActionPull, Path: "d.metadata"},
					{ActionType: document.ActionPull, Path: "e.content"},
				},
				Conflicts: []domainErrors.ConflictError{{Path: "f.metadata"}},
			},
			want: "Would sync:\n  → device: 3 files\n  ← local: 2 files\n  conflicts: 1\n",
		},
		{
			name: "only device actions",
			res: &StatusResult{
				ToDevice: []document.SyncAction{
					{ActionType: document.ActionPush, Path: "a.metadata"},
				},
			},
			want: "Would sync:\n  → device: 1 files\n  ← local: 0 files\n  conflicts: 0\n",
		},
		{
			name: "only conflicts",
			res: &StatusResult{
				Conflicts: []domainErrors.ConflictError{
					{Path: "a.metadata"},
					{Path: "b.content"},
				},
			},
			want: "Would sync:\n  → device: 0 files\n  ← local: 0 files\n  conflicts: 2\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.Summary()
			if got != tc.want {
				t.Errorf("Summary() = %q, want %q", got, tc.want)
			}
		})
	}
}
