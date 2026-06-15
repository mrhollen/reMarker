package document

import (
	"context"
	"io"
	"time"
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

	// ListDocuments walks the xochitl directory, finds all .metadata files,
	// parses them, and returns Document entities. Only non-deleted documents
	// are returned.
	ListDocuments(ctx context.Context) ([]Document, error)

	// PutDocument uploads a document's content file and generates .metadata
	// and .content sidecar files on the device. Uses atomic writes
	// (temp + rename) for all files.
	PutDocument(ctx context.Context, doc Document, content io.Reader) error

	// PutFolder creates a folder on the device by writing a .metadata file
	// with folder type.
	PutFolder(ctx context.Context, folder Folder) error

	// GetDocumentMetadata reads and parses .metadata and .content sidecar
	// files for a given device UUID.
	GetDocumentMetadata(ctx context.Context, deviceUUID string) (SidecarMetadata, error)
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

// Metadata represents the parsed contents of a reMarkable .metadata sidecar
// file. It captures document identification, naming, and organizational state.
type Metadata struct {
	DeviceID         string    `json:"DocumentID"`
	VisibleName      string    `json:"VisibleName"`
	FileFormat       string    `json:"FileFormat"`
	ParentFolderUUID string    `json:"ParentFolderUUID"`
	LastModified     time.Time `json:"LastModified"`
	IsDeleted        bool      `json:"IsDeleted"`
}

// PageInfo describes a single page within a reMarkable document.
type PageInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Content represents the parsed contents of a reMarkable .content sidecar
// file. It describes the document's page layout and file type.
type Content struct {
	Pages    []PageInfo `json:"pages"`
	FileType string     `json:"fileType"`
}

// SidecarRepository represents file-system operations for persisting and
// retrieving reMarkable sidecar metadata files (.metadata, .content, .pagedata)
// per document, organized under a device-scoped directory.
type SidecarRepository interface {
	// SaveMetadata writes the .metadata and .content files for a document.
	// Creates the device subdirectory if it does not exist. Uses atomic
	// writes (temp file + rename).
	SaveMetadata(ctx context.Context, documentUUID string, meta Metadata, content Content) error

	// GetMetadata reads the .metadata and .content files for a document.
	// Returns an error if the files do not exist.
	GetMetadata(ctx context.Context, documentUUID string) (Metadata, Content, error)

	// DeleteMetadata removes all sidecar files (.metadata, .content, .pagedata)
	// for a document. Returns nil if the files do not exist.
	DeleteMetadata(ctx context.Context, documentUUID string) error

	// Exists returns true if the .metadata and .content files exist for a
	// document.
	Exists(ctx context.Context, documentUUID string) (bool, error)
}
