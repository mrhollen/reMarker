package document

import (
	"context"
	"io"
)

// Compile-time interface checks ensure implementations in infrastructure
// satisfy the domain contracts.

var (
	_ DeviceRepository    = (*deviceRepositoryImpl)(nil)
	_ LocalRepository     = (*localRepositoryImpl)(nil)
	_ ManifestRepository  = (*manifestRepositoryImpl)(nil)
	_ SidecarRepository   = (*sidecarRepositoryImpl)(nil)
)

// deviceRepositoryImpl is a dummy type used solely for compile-time
// verification that DeviceRepository is a valid interface with the
// expected method signatures.
type deviceRepositoryImpl struct{}

func (d *deviceRepositoryImpl) ListFiles(ctx context.Context) ([]File, error) {
	return nil, nil
}

func (d *deviceRepositoryImpl) GetFile(ctx context.Context, path string) (File, error) {
	return File{}, nil
}

func (d *deviceRepositoryImpl) PutFile(ctx context.Context, file File) error {
	return nil
}

func (d *deviceRepositoryImpl) DeleteFile(ctx context.Context, path string) error {
	return nil
}

func (d *deviceRepositoryImpl) GetFileContent(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, nil
}

func (d *deviceRepositoryImpl) ListDocuments(ctx context.Context) ([]Document, error) {
	return nil, nil
}

func (d *deviceRepositoryImpl) PutDocument(ctx context.Context, doc Document, content io.Reader) error {
	return nil
}

func (d *deviceRepositoryImpl) PutFolder(ctx context.Context, folder Folder) error {
	return nil
}

func (d *deviceRepositoryImpl) GetDocumentMetadata(ctx context.Context, deviceUUID string) (SidecarMetadata, error) {
	return SidecarMetadata{}, nil
}

// localRepositoryImpl is a dummy type used solely for compile-time
// verification that LocalRepository is a valid interface with the
// expected method signatures.
type localRepositoryImpl struct{}

func (l *localRepositoryImpl) ListFiles(ctx context.Context) ([]File, error) {
	return nil, nil
}

func (l *localRepositoryImpl) GetFile(ctx context.Context, path string) (File, error) {
	return File{}, nil
}

func (l *localRepositoryImpl) PutFile(ctx context.Context, file File) error {
	return nil
}

func (l *localRepositoryImpl) DeleteFile(ctx context.Context, path string) error {
	return nil
}

func (l *localRepositoryImpl) PutFileContent(ctx context.Context, file File, content io.Reader) error {
	return nil
}

// manifestRepositoryImpl is a dummy type used solely for compile-time
// verification that ManifestRepository is a valid interface with the
// expected method signatures.
type manifestRepositoryImpl struct{}

func (m *manifestRepositoryImpl) Load(ctx context.Context) (*Manifest, error) {
	return nil, nil
}

func (m *manifestRepositoryImpl) Save(ctx context.Context, manifest *Manifest) error {
	return nil
}

func (m *manifestRepositoryImpl) Exists(ctx context.Context) (bool, error) {
	return false, nil
}

// sidecarRepositoryImpl is a dummy type used solely for compile-time
// verification that SidecarRepository is a valid interface with the
// expected method signatures.
type sidecarRepositoryImpl struct{}

func (s *sidecarRepositoryImpl) SaveMetadata(ctx context.Context, documentUUID string, meta Metadata, content Content) error {
	return nil
}

func (s *sidecarRepositoryImpl) GetMetadata(ctx context.Context, documentUUID string) (Metadata, Content, error) {
	return Metadata{}, Content{}, nil
}

func (s *sidecarRepositoryImpl) DeleteMetadata(ctx context.Context, documentUUID string) error {
	return nil
}

func (s *sidecarRepositoryImpl) Exists(ctx context.Context, documentUUID string) (bool, error) {
	return false, nil
}
