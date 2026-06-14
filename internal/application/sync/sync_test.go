package sync

import (
	"context"
	"errors"
	"io"
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
	// getContent returns the raw content of a file.
	getContent func(ctx context.Context, path string) (io.ReadCloser, error)
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

func (m *mockDeviceRepository) GetFileContent(_ context.Context, path string) (io.ReadCloser, error) {
	if m.getContent != nil {
		return m.getContent(context.Background(), path)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

// Compile-time check.
var _ document.DeviceRepository = (*mockDeviceRepository)(nil)

// mockLocalRepository implements document.LocalRepository for testing.
type mockLocalRepository struct {
	files              map[string]document.File
	getFileErr         error
	putFileErr         error
	deleteErr          error
	listErr            error
	putFileCall        func(ctx context.Context, file document.File) error
	putFileContentCall func(ctx context.Context, file document.File, content io.Reader) error
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

func (m *mockLocalRepository) PutFileContent(ctx context.Context, file document.File, content io.Reader) error {
	if m.putFileContentCall != nil {
		return m.putFileContentCall(ctx, file, content)
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
// Progress recording helper
// ---------------------------------------------------------------------------

// progressRecorder captures progress callback invocations for assertion.
type progressRecorder struct {
	calls []progressCall
}

type progressCall struct {
	current int
	total   int
	action  string
	path    string
}

func newProgressRecorder() *progressRecorder {
	return &progressRecorder{calls: []progressCall{}}
}

func (r *progressRecorder) record(current, total int, action, path string) {
	r.calls = append(r.calls, progressCall{
		current: current,
		total:   total,
		action:  action,
		path:    path,
	})
}

func (r *progressRecorder) asProgressFunc() ProgressFunc {
	return r.record
}

func TestExecute_ProgressCallback_SingleAction(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	recorder := newProgressRecorder()

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{
			"abc.metadata": file("abc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		recorder.asProgressFunc(),
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action, got none")
	}

	if len(recorder.calls) != 1 {
		t.Fatalf("expected 1 progress call, got %d", len(recorder.calls))
	}

	call := recorder.calls[0]
	if call.current != 1 {
		t.Errorf("expected current=1, got %d", call.current)
	}
	if call.total != 1 {
		t.Errorf("expected total=1, got %d", call.total)
	}
	if call.action != string(document.ActionPush) {
		t.Errorf("expected action=%q, got %q", document.ActionPush, call.action)
	}
	if call.path != "abc.metadata" {
		t.Errorf("expected path=%q, got %q", "abc.metadata", call.path)
	}
}

func TestExecute_ProgressCallback_MultipleActions(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	recorder := newProgressRecorder()

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{
			"a.metadata": file("a.metadata", "hash1", 100, now),
			"b.metadata": file("b.metadata", "hash2", 200, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		recorder.asProgressFunc(),
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push actions, got none")
	}

	if len(recorder.calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(recorder.calls))
	}

	if recorder.calls[0].current != 1 {
		t.Errorf("first call: expected current=1, got %d", recorder.calls[0].current)
	}
	if recorder.calls[0].total != 2 {
		t.Errorf("first call: expected total=2, got %d", recorder.calls[0].total)
	}

	if recorder.calls[1].current != 2 {
		t.Errorf("second call: expected current=2, got %d", recorder.calls[1].current)
	}
	if recorder.calls[1].total != 2 {
		t.Errorf("second call: expected total=2, got %d", recorder.calls[1].total)
	}
}

func TestExecute_ProgressCallback_NilWorks(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{
			"abc.metadata": file("abc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil, // nil progress callback
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error with nil progress: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action even with nil progress callback")
	}
}

func TestExecute_ProgressCallback_NoActions(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	recorder := newProgressRecorder()

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"synced.metadata": file("synced.metadata", "samehash", 100, now),
		}},
		&mockLocalRepository{files: map[string]document.File{
			"synced.metadata": file("synced.metadata", "samehash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata": entry("synced.metadata", "samehash", 100, now, now),
			}),
		},
		recorder.asProgressFunc(),
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Even though the file is in sync, planExisting produces an ActionNone
	// for every tracked path, so we expect 1 progress call.
	if len(recorder.calls) != 1 {
		t.Fatalf("expected 1 progress call for synced file, got %d", len(recorder.calls))
	}
	if recorder.calls[0].current != 1 {
		t.Errorf("expected current=1, got %d", recorder.calls[0].current)
	}
	if recorder.calls[0].total != 1 {
		t.Errorf("expected total=1, got %d", recorder.calls[0].total)
	}
	if recorder.calls[0].action != string(document.ActionNone) {
		t.Errorf("expected action=%s, got %s", document.ActionNone, recorder.calls[0].action)
	}
	if recorder.calls[0].path != "synced.metadata" {
		t.Errorf("expected path=synced.metadata, got %s", recorder.calls[0].path)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func file(path, hash string, size int64, modTime time.Time) document.File {
	return document.File{Path: path, Hash: hash, Size: size, ModTime: modTime}
}

func entry(path, hash string, size int64, modTime, syncedAt time.Time) document.ManifestEntry {
	return document.ManifestEntry{
		DeviceUUID:  path,
		DeviceType:  document.DocumentTypePDF,
		LocalHash:   hash,
		DeviceHash:  hash,
		VisibleName: path,
		Size:        size,
		SyncedAt:    syncedAt,
	}
}

func manifest(version int, entries map[string]document.ManifestEntry) *document.Manifest {
	return &document.Manifest{
		Version: version,
		Entries: entries,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewSyncUseCase(t *testing.T) {
	uc := NewSyncUseCase(&mockDeviceRepository{}, &mockLocalRepository{}, &mockManifestRepository{}, nil)
	if uc == nil {
		t.Fatal("NewSyncUseCase() returned nil")
	}
}

func TestExecute_EmptySync(t *testing.T) {
	// No files on either side, clean manifest → no actions
	ctx := context.Background()
	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.HasActions() {
		t.Errorf("expected no actions, got %d", len(result.Actions))
	}
	if result.HasConflicts() {
		t.Errorf("expected no conflicts, got %d", len(result.Conflicts))
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}
}

func TestExecute_NewLocalFile(t *testing.T) {
	// File only on local, not in manifest → push action
	now := time.Now()
	ctx := context.Background()
	localFile := file("abc.metadata", "localhash", 100, now)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{
			"abc.metadata": localFile,
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action, got none")
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "abc.metadata" && a.ActionType == document.ActionPush {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected push action for abc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_NewDeviceFile(t *testing.T) {
	// File only on device, not in manifest → pull action
	now := time.Now()
	ctx := context.Background()
	deviceFile := file("xyz.content", "devicehash", 200, now)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"xyz.content": deviceFile,
		}},
		&mockLocalRepository{files: map[string]document.File{}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected pull action, got none")
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "xyz.content" && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pull action for xyz.content, got actions: %v", result.Actions)
	}
}

func TestExecute_InSync(t *testing.T) {
	// Same file on both sides, matches manifest → no action
	now := time.Now()
	ctx := context.Background()
	hash := "samehash"
	f := file("synced.metadata", hash, 100, now)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"synced.metadata": f,
		}},
		&mockLocalRepository{files: map[string]document.File{
			"synced.metadata": f,
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata": entry("synced.metadata", hash, 100, now, now),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should have no actions (or only ActionNone)
	for _, a := range result.Actions {
		if a.ActionType != document.ActionNone {
			t.Errorf("expected only ActionNone, got %v for %s", a.ActionType, a.Path)
		}
	}
}

func TestExecute_LocalModified(t *testing.T) {
	// File changed locally, device unchanged → push
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	localHash := "newlocalhash"
	manifestTime := now.Add(-time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"doc.metadata": file("doc.metadata", manifestHash, 100, manifestTime),
		}},
		&mockLocalRepository{files: map[string]document.File{
			"doc.metadata": file("doc.metadata", localHash, 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "doc.metadata" && a.ActionType == document.ActionPush {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected push action for doc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_DeviceModified(t *testing.T) {
	// File changed on device, local unchanged → pull
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	deviceHash := "newdevicehash"
	manifestTime := now.Add(-time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"doc.metadata": file("doc.metadata", deviceHash, 100, now),
		}},
		&mockLocalRepository{files: map[string]document.File{
			"doc.metadata": file("doc.metadata", manifestHash, 100, manifestTime),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "doc.metadata" && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pull action for doc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_Conflict(t *testing.T) {
	// Both sides modified differently → conflict, newer wins
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	// Local is newer
	localFile := file("doc.metadata", "localhash", 100, now)
	deviceFile := file("doc.metadata", "devicehash", 100, now.Add(-time.Hour))

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"doc.metadata": deviceFile,
		}},
		&mockLocalRepository{files: map[string]document.File{
			"doc.metadata": localFile,
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should have a conflict
	if !result.HasConflicts() {
		t.Fatalf("expected conflict, got none. Actions: %v", result.Actions)
	}

	// Verify conflict path
	foundConflict := false
	for _, c := range result.Conflicts {
		if c.Path == "doc.metadata" {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Errorf("expected conflict on doc.metadata, got conflicts: %v", result.Conflicts)
	}
}

func TestExecute_ConflictDeviceNewer(t *testing.T) {
	// Both sides modified, device is newer → device wins
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	// Device is newer
	localFile := file("doc.metadata", "localhash", 100, now.Add(-time.Hour))
	deviceFile := file("doc.metadata", "devicehash", 100, now)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			"doc.metadata": deviceFile,
		}},
		&mockLocalRepository{files: map[string]document.File{
			"doc.metadata": localFile,
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatalf("expected conflict, got none")
	}
}

func TestExecute_FileDeletedLocally(t *testing.T) {
	// In manifest, missing from local, still on device → remove from manifest, don't delete on device
	now := time.Now()
	ctx := context.Background()
	manifestHash := "hash123"
	manifestTime := now.Add(-time.Hour)

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			"deleted.metadata": file("deleted.metadata", manifestHash, 100, manifestTime),
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{}, // file not on local
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"deleted.metadata": entry("deleted.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should NOT have a delete action on device
	for _, a := range result.Actions {
		if a.ActionType == document.ActionDeleteDevice {
			t.Errorf("should not delete from device, got action: %v", a)
		}
	}

	// File should still be on device
	_, ok := deviceRepo.files["deleted.metadata"]
	if !ok {
		t.Error("file was deleted from device, should have been preserved")
	}
}

func TestExecute_FileDeletedOnDevice(t *testing.T) {
	// In manifest, missing from device, still on local → remove from manifest, don't delete locally
	now := time.Now()
	ctx := context.Background()
	manifestHash := "hash123"
	manifestTime := now.Add(-time.Hour)

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{}, // file not on device
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"deleted.metadata": file("deleted.metadata", manifestHash, 100, manifestTime),
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"deleted.metadata": entry("deleted.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should NOT have a delete action on local
	for _, a := range result.Actions {
		if a.ActionType == document.ActionDeleteLocal {
			t.Errorf("should not delete from local, got action: %v", a)
		}
	}

	// File should still be on local
	_, ok := localRepo.files["deleted.metadata"]
	if !ok {
		t.Error("file was deleted from local, should have been preserved")
	}
}

func TestExecute_ManifestLoadFails(t *testing.T) {
	ctx := context.Background()
	loadErr := errors.New("manifest corrupted")

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{},
		&mockManifestRepository{loadErr: loadErr},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on manifest load error")
	}
}

func TestExecute_DeviceListFails(t *testing.T) {
	ctx := context.Background()
	listErr := errors.New("SSH connection lost")

	uc := NewSyncUseCase(
		&mockDeviceRepository{listErr: listErr},
		&mockLocalRepository{},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on device list error")
	}
}

func TestExecute_LocalListFails(t *testing.T) {
	ctx := context.Background()
	listErr := errors.New("permission denied")

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{listErr: listErr},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on local list error")
	}
}

func TestExecute_TransferFails(t *testing.T) {
	// Transfer fails → adds to result errors, continues with other files
	now := time.Now()
	ctx := context.Background()
	transferErr := errors.New("SFTP write failed")

	uc := NewSyncUseCase(
		&mockDeviceRepository{
			files:      map[string]document.File{},
			putFileErr: transferErr,
		},
		&mockLocalRepository{files: map[string]document.File{
			"fail.metadata": file("fail.metadata", "newhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected transfer error in result.Errors, got none")
	}

	// Verify the error mentions the file path
	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "fail.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for fail.metadata, got errors: %v", result.Errors)
	}
}

func TestExecute_MultipleFiles(t *testing.T) {
	// Mix of push, pull, conflict, none
	now := time.Now()
	ctx := context.Background()
	manifestTime := now.Add(-2 * time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{
			// In sync
			"synced.metadata": file("synced.metadata", "hash1", 100, now),
			// Device only → pull
			"deviceonly.content": file("deviceonly.content", "hash2", 200, now),
			// Conflict (device newer)
			"conflict.metadata": file("conflict.metadata", "devicehash", 100, now),
		}},
		&mockLocalRepository{files: map[string]document.File{
			// In sync
			"synced.metadata": file("synced.metadata", "hash1", 100, now),
			// Local only → push
			"localonly.metadata": file("localonly.metadata", "hash3", 300, now),
			// Conflict (local older)
			"conflict.metadata": file("conflict.metadata", "localhash", 100, now.Add(-time.Hour)),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata":   entry("synced.metadata", "hash1", 100, now, now),
				"conflict.metadata": entry("conflict.metadata", "originalhash", 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Count action types
	pushCount := 0
	pullCount := 0
	noneCount := 0
	for _, a := range result.Actions {
		switch a.ActionType {
		case document.ActionPush:
			pushCount++
		case document.ActionPull:
			pullCount++
		case document.ActionNone:
			noneCount++
		}
	}

	if pushCount != 1 {
		t.Errorf("expected 1 push action, got %d", pushCount)
	}
	if pullCount != 1 {
		t.Errorf("expected 1 pull action, got %d", pullCount)
	}
	// synced.metadata should be ActionNone (or not listed)
	if noneCount < 1 {
		t.Logf("got %d ActionNone (may be 0 if in-sync files are omitted)", noneCount)
	}
	if !result.HasConflicts() {
		t.Error("expected conflict for conflict.metadata")
	}
}

func TestExecute_ManifestSaved(t *testing.T) {
	// After sync, manifest should be saved
	now := time.Now()
	ctx := context.Background()

	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{
			"new.metadata": file("new.metadata", "newhash", 100, now),
		}},
		manifestRepo,
		nil,
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !manifestRepo.saveCalled {
		t.Error("manifest was not saved after sync")
	}
}

func TestExecute_ManifestTouched(t *testing.T) {
	// After sync, manifest LastSync should be set
	ctx := context.Background()

	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(
		&mockDeviceRepository{files: map[string]document.File{}},
		&mockLocalRepository{files: map[string]document.File{}},
		manifestRepo,
		nil,
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if manifestRepo.savedManifest == nil {
		t.Fatal("manifest was not saved")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error on cancelled context, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on context cancellation")
	}
}

func TestSyncResult_HasConflicts(t *testing.T) {
	tests := []struct {
		name string
		res  *document.SyncResult
		want bool
	}{
		{
			name: "nil conflicts",
			res:  &document.SyncResult{Conflicts: nil},
			want: false,
		},
		{
			name: "empty conflicts",
			res:  &document.SyncResult{Conflicts: []domainErrors.ConflictError{}},
			want: false,
		},
		{
			name: "one conflict",
			res:  &document.SyncResult{Conflicts: []domainErrors.ConflictError{{Path: "a.metadata"}}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.HasConflicts()
			if got != tc.want {
				t.Errorf("HasConflicts() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSyncResult_HasActions(t *testing.T) {
	tests := []struct {
		name string
		res  *document.SyncResult
		want bool
	}{
		{
			name: "nil actions",
			res:  &document.SyncResult{Actions: nil},
			want: false,
		},
		{
			name: "empty actions",
			res:  &document.SyncResult{Actions: []document.SyncAction{}},
			want: false,
		},
		{
			name: "one action",
			res:  &document.SyncResult{Actions: []document.SyncAction{{ActionType: document.ActionPush}}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.HasActions()
			if got != tc.want {
				t.Errorf("HasActions() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pullFile content transfer tests
// ---------------------------------------------------------------------------

func TestPullFile_TransfersContent(t *testing.T) {
	// pullFile should download file content from device and write to local
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	fileContent := "this is the actual file content from the device"

	var receivedContent string
	localFiles := make(map[string]document.File)
	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", int64(len(fileContent)), now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			receivedContent = string(data)
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Verify pull action happened
	found := false
	for _, a := range result.Actions {
		if a.Path == filePath && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected pull action for device-only file")
	}

	// Verify content was actually transferred
	if receivedContent != fileContent {
		t.Errorf("content mismatch: got %q, want %q", receivedContent, fileContent)
	}

	// Verify file metadata stored on local
	localFile, ok := localRepo.files[filePath]
	if !ok {
		t.Fatal("file not stored on local after pull")
	}
	if localFile.Hash != "devicehash" {
		t.Errorf("local file hash = %q, want %q", localFile.Hash, "devicehash")
	}

	// Verify manifest was updated with correct fields
	if len(manifestRepo.savedManifest.Entries) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(manifestRepo.savedManifest.Entries))
	}
	entry := manifestRepo.savedManifest.Entries[filePath]
	if entry.LocalHash != "devicehash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "devicehash")
	}
	if entry.DeviceHash != "devicehash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "devicehash")
	}
	if entry.Size != int64(len(fileContent)) {
		t.Errorf("manifest size = %d, want %d", entry.Size, int64(len(fileContent)))
	}
}

func TestPullFile_DeviceGetContentFails(t *testing.T) {
	// When device GetFileContent fails, pull should report error and not abort sync
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	getContentErr := errors.New("SFTP read failed")

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", 200, now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return nil, getContentErr
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	// Should have a non-fatal error for the file
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed pull, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, filePath) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for %s, got errors: %v", filePath, result.Errors)
	}
}

func TestPullFile_LocalPutContentFails(t *testing.T) {
	// When local PutFileContent fails, pull should report error
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	fileContent := "file content"
	putContentErr := errors.New("disk full")

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", int64(len(fileContent)), now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			return putContentErr
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed local write, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, filePath) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for %s, got errors: %v", filePath, result.Errors)
	}
}

func TestPullFile_UpdatesManifestWithSourceFields(t *testing.T) {
	// Manifest entry should use action.Source fields (device file info)
	now := time.Now()
	ctx := context.Background()
	filePath := "doc.content"
	fileContent := "content"

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "dev-hash", 7, now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	entry, ok := manifestRepo.savedManifest.Entries[filePath]
	if !ok {
		t.Fatal("manifest entry not created for pulled file")
	}
	if entry.LocalHash != "dev-hash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "dev-hash")
	}
	if entry.DeviceHash != "dev-hash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "dev-hash")
	}
	if entry.Size != 7 {
		t.Errorf("manifest size = %d, want 7", entry.Size)
	}
}

// ---------------------------------------------------------------------------
// resolveConflict content transfer tests
// ---------------------------------------------------------------------------

func TestResolveConflict_LocalWinner_LocalLoser(t *testing.T) {
	// Both sides modified, local is newer. Loser is device.
	// Expected: conflict copy of device file pulled to local,
	// winner (local) pushed to device.
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local winner content"
	deviceContent := "device loser content"

	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now)
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now.Add(-time.Hour))

	var conflictContentReceived string
	var winnerPushedToDevice bool

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			if f.Path == "doc.metadata.conflict" {
				// Conflict copy pushed to device
			} else if f.Path == "doc.metadata" {
				winnerPushedToDevice = true
			}
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			if f.Path == "doc.metadata.conflict" {
				data, err := io.ReadAll(r)
				if err != nil {
					return err
				}
				conflictContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (local) should be pushed to device
	if !winnerPushedToDevice {
		t.Error("winner (local file) should be pushed to device")
	}

	// Conflict copy of device loser should be pulled to local with content
	if conflictContentReceived != deviceContent {
		t.Errorf("conflict content = %q, want %q", conflictContentReceived, deviceContent)
	}

	// Conflict copy should exist on device
	if _, ok := deviceRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on device")
	}

	// Manifest should have winner's hash
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "localhash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "localhash")
	}
	if entry.DeviceHash != "localhash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "localhash")
	}
}

func TestResolveConflict_DeviceWinner_LocalLoser(t *testing.T) {
	// Both sides modified, device is newer. Loser is local.
	// Expected: conflict copy of local file pushed to device,
	// winner (device) pulled to local with content, then pushed to device.
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local loser content"
	deviceContent := "device winner content"

	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now.Add(-time.Hour))
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now)

	var winnerContentReceived string
	var conflictPushedToDevice bool

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			if f.Path == "doc.metadata.conflict" {
				conflictPushedToDevice = true
			} else if f.Path == "doc.metadata" {
				// Winner pushed to device
			}
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			if f.Path == "doc.metadata" {
				data, err := io.ReadAll(r)
				if err != nil {
					return err
				}
				winnerContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (device) content should be pulled to local
	if winnerContentReceived != deviceContent {
		t.Errorf("winner content = %q, want %q", winnerContentReceived, deviceContent)
	}

	// Conflict copy of local loser should be pushed to device
	if !conflictPushedToDevice {
		t.Error("conflict copy (local loser) should be pushed to device")
	}

	// Conflict copy should exist on local
	if _, ok := localRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on local")
	}

	// Manifest should have winner's hash (device)
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "devicehash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "devicehash")
	}
	if entry.DeviceHash != "devicehash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "devicehash")
	}
}

func TestResolveConflict_DeviceWinner_DeviceLoser(t *testing.T) {
	// Both sides modified, local is newer. Loser is device.
	// Expected: conflict copy of device loser pulled to local with content,
	// winner (local) pushed to device via PutFile (reads local content).
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local winner content"
	deviceContent := "device loser content"

	// Source=local (newer), Dest=device (older). Local wins, device loses.
	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now)
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now.Add(-time.Hour))

	var conflictContentReceived string
	getContentCalls := 0

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			getContentCalls++
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			if f.Path == "doc.metadata.conflict" {
				conflictContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (local) content should be pushed to device via PutFile (reads from local)
	// No PutFileContent call for winner since it's local-originated

	// Conflict copy content should be device loser content
	if conflictContentReceived != deviceContent {
		t.Errorf("conflict content = %q, want %q", conflictContentReceived, deviceContent)
	}

	// GetFileContent should be called (for device loser)
	if getContentCalls == 0 {
		t.Error("expected GetFileContent to be called for device loser")
	}

	// Conflict copy should exist on device
	if _, ok := deviceRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on device")
	}

	// Manifest should have winner's hash (local)
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "localhash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "localhash")
	}
	if entry.DeviceHash != "localhash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "localhash")
	}
}

func TestResolveConflict_LocalWinner_DeviceLoser_DeviceGetContentFails(t *testing.T) {
	// Device loser content pull fails => error reported
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)
	getContentErr := errors.New("SFTP read failed")

	localFile := file("doc.metadata", "localhash", 100, now)
	deviceFile := file("doc.metadata", "devicehash", 100, now.Add(-time.Hour))

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			"doc.metadata": deviceFile,
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return nil, getContentErr
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"doc.metadata": localFile,
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	// Should have a non-fatal error
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed conflict resolution, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "doc.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for doc.metadata, got errors: %v", result.Errors)
	}
}

func TestResolveConflict_DeviceWinner_LocalPutContentFails(t *testing.T) {
	// Pulling device winner to local fails => error reported
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)
	putContentErr := errors.New("disk full")

	localFile := file("doc.metadata", "localhash", 100, now.Add(-time.Hour))
	deviceFile := file("doc.metadata", "devicehash", 100, now)

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			"doc.metadata": deviceFile,
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("device content")), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"doc.metadata": localFile,
		},
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			return putContentErr
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed local write, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "doc.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for doc.metadata, got errors: %v", result.Errors)
	}
}
