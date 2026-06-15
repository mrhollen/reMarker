// Package status provides a dry-run preview of what changes a sync would make,
// without actually applying any changes.
package status

import (
	"context"
	"io"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
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

func (m *mockDeviceRepository) GetFileContent(_ context.Context, path string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDeviceRepository) ListDocuments(_ context.Context) ([]document.Document, error) {
	return nil, nil
}

func (m *mockDeviceRepository) PutDocument(_ context.Context, _ document.Document, _ io.Reader) error {
	return nil
}

func (m *mockDeviceRepository) PutFolder(_ context.Context, _ document.Folder) error {
	return nil
}

func (m *mockDeviceRepository) GetDocumentMetadata(_ context.Context, _ string) (document.SidecarMetadata, error) {
	return document.SidecarMetadata{}, nil
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

func (m *mockLocalRepository) PutFileContent(_ context.Context, file document.File, _ io.Reader) error {
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
	manifest      *document.Manifest
	loadErr       error
	saveErr       error
	exists        bool
	existsErr     error
	saveCalled    bool
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
