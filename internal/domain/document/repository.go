package document

import (
	"context"
	"io"
)

// DeviceRepository represents operations on the reMarkable device.
// Implementations typically use SSH/SFTP to communicate with the device.
// This interface belongs in the domain layer; the implementation lives in
// the infrastructure layer.
type DeviceRepository interface {
	// ListFiles returns all files in the xochitl directory.
	ListFiles(ctx context.Context) ([]File, error)

	// GetFile reads a single file from the device.
	GetFile(ctx context.Context, path string) (File, error)

	// PutFile writes a file to the device (atomic: temp + rename).
	PutFile(ctx context.Context, file File) error

	// DeleteFile removes a file from the device.
	DeleteFile(ctx context.Context, path string) error

	// GetFileContent reads the raw content of a file from the device.
	// Returns an io.ReadCloser that the caller MUST close.
	GetFileContent(ctx context.Context, path string) (io.ReadCloser, error)
}

// LocalRepository represents operations on the local filesystem.
// Implementations use the standard os package to manage local files.
type LocalRepository interface {
	// ListFiles returns all files in the local sync directory.
	ListFiles(ctx context.Context) ([]File, error)

	// GetFile reads a single file from the local filesystem.
	GetFile(ctx context.Context, path string) (File, error)

	// PutFile writes a file to the local filesystem.
	PutFile(ctx context.Context, file File) error

	// DeleteFile removes a file from the local filesystem.
	DeleteFile(ctx context.Context, path string) error

	// PutFileContent writes file content from an io.Reader to the local filesystem
	// using an atomic write pattern (temp file + rename). Parent directories are
	// created as needed. The modification time is set to the value provided in the
	// File struct. Use this method instead of PutFile when you need to transfer
	// actual file content (e.g., during pull sync).
	PutFileContent(ctx context.Context, file File, content io.Reader) error
}

// ManifestRepository represents operations on the sync manifest.
// The manifest is the source of truth for sync state.
type ManifestRepository interface {
	// Load reads the manifest from storage.
	Load(ctx context.Context) (*Manifest, error)

	// Save writes the manifest to storage.
	Save(ctx context.Context, manifest *Manifest) error

	// Exists returns true if the manifest file exists.
	Exists(ctx context.Context) (bool, error)
}
