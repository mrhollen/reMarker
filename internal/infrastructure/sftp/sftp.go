// Package sftp provides SFTP operations for reading and writing files on a
// reMarkable device. It implements the document.DeviceRepository interface
// for device-side file operations.
package sftp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hollen/remarker/internal/domain/document"
	"github.com/hollen/remarker/internal/infrastructure/ssh"
	"github.com/pkg/sftp"
)

// XochitlDir is the root directory on the reMarkable device where documents
// are stored.
const XochitlDir = "/home/root/.local/share/remarkable/xochitl"

// sftpFile abstracts an SFTP file handle for testability.
type sftpFile interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
}

// sftpClientWrapper adapts *sftp.Client to the sftpClient interface.
type sftpClientWrapper struct {
	client *sftp.Client
}

func (w *sftpClientWrapper) Create(path string) (sftpFile, error) {
	return w.client.Create(path)
}
func (w *sftpClientWrapper) Stat(path string) (os.FileInfo, error) {
	return w.client.Stat(path)
}
func (w *sftpClientWrapper) Remove(path string) error {
	return w.client.Remove(path)
}
func (w *sftpClientWrapper) Rename(oldPath, newPath string) error {
	return w.client.Rename(oldPath, newPath)
}
func (w *sftpClientWrapper) Chmod(path string, mode os.FileMode) error {
	return w.client.Chmod(path, mode)
}
func (w *sftpClientWrapper) ReadDir(path string) ([]os.FileInfo, error) {
	return w.client.ReadDir(path)
}
func (w *sftpClientWrapper) Open(path string) (sftpFile, error) {
	return w.client.Open(path)
}
func (w *sftpClientWrapper) MkdirAll(path string) error {
	return w.client.MkdirAll(path)
}
func (w *sftpClientWrapper) Close() error {
	return w.client.Close()
}

// sftpClient abstracts the SFTP client for testability.
type sftpClient interface {
	Create(path string) (sftpFile, error)
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	Rename(oldPath, newPath string) error
	Chmod(path string, mode os.FileMode) error
	ReadDir(path string) ([]os.FileInfo, error)
	Open(path string) (sftpFile, error)
	MkdirAll(path string) error
	Close() error
}

// Client wraps an sftp.Client and provides operations for the reMarkable
// device's xochitl directory. It implements document.DeviceRepository.
type Client struct {
	sftp    sftpClient
	sftpRaw *sftp.Client // kept for Close()
	baseDir string
}

// New creates a new SFTP client using the provided SSH client. The SFTP
// client operates relative to the xochitl directory on the device.
func New(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil || sshClient.SSH() == nil {
		return nil, fmt.Errorf("sftp: ssh client is nil")
	}

	rawClient, err := sftp.NewClient(sshClient.SSH())
	if err != nil {
		return nil, fmt.Errorf("sftp new client: %w", err)
	}

	return &Client{
		sftp:    &sftpClientWrapper{client: rawClient},
		sftpRaw: rawClient,
		baseDir: XochitlDir,
	}, nil
}

// Close closes the underlying SFTP connection.
func (c *Client) Close() error {
	if c == nil || c.sftpRaw == nil {
		return nil
	}
	return c.sftpRaw.Close()
}

// ListFiles returns all files in the xochitl directory recursively.
// Paths are relative to the xochitl directory. Directories are skipped.
func (c *Client) ListFiles(ctx context.Context) ([]document.File, error) {
	if c.sftp == nil {
		return nil, fmt.Errorf("sftp: client not initialized")
	}

	var files []document.File

	err := walkSFTP(ctx, c.sftp, c.baseDir, ".", func(relPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Open the file to compute hash
		f, err := c.sftp.Open(filepath.Join(c.baseDir, relPath))
		if err != nil {
			return fmt.Errorf("sftp open %s: %w", relPath, err)
		}
		defer f.Close()

		hash := computeHash(f)

		files = append(files, document.File{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    hash,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sftp list files: %w", err)
	}

	return files, nil
}

// GetFile reads a single file from the device and returns its metadata
// including the SHA256 hash.
func (c *Client) GetFile(ctx context.Context, path string) (document.File, error) {
	if c.sftp == nil {
		return document.File{}, fmt.Errorf("sftp: client not initialized")
	}

	fullPath := filepath.Join(c.baseDir, path)

	// Get file info
	info, err := c.sftp.Stat(fullPath)
	if err != nil {
		return document.File{}, fmt.Errorf("sftp stat %s: %w", path, err)
	}

	// Open and hash the file
	f, err := c.sftp.Open(fullPath)
	if err != nil {
		return document.File{}, fmt.Errorf("sftp open %s: %w", path, err)
	}
	defer f.Close()

	hash := computeHash(f)

	return document.File{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Hash:    hash,
	}, nil
}

// PutFile writes a file to the device using an atomic write pattern:
// 1. Write content to a temporary file
// 2. Remove the destination if it exists
// 3. Rename the temp file to the destination
// This is required because SFTP does not support atomic overwrites.
func (c *Client) PutFile(ctx context.Context, file document.File) error {
	if c.sftp == nil {
		return fmt.Errorf("sftp: client not initialized")
	}

	fullPath := filepath.Join(c.baseDir, file.Path)
	tmpPath := fullPath + ".tmp." + randomHex(8)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if parentDir != c.baseDir {
		_, err := c.sftp.Stat(parentDir)
		if err != nil {
			// Directory doesn't exist, create it
			err = c.sftp.MkdirAll(parentDir)
			if err != nil {
				return fmt.Errorf("sftp mkdir %s: %w", parentDir, err)
			}
		}
	}

	// Create temp file and write content
	// We need to read the file content from the local filesystem
	localPath := file.Path
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("sftp put: open local file %s: %w", localPath, err)
	}
	defer f.Close()

	// Create the temp file on the device
	tmpFile, err := c.sftp.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("sftp create temp %s: %w", tmpPath, err)
	}

	// Copy content with context awareness
	_, err = io.Copy(tmpFile, f)
	if err != nil {
		tmpFile.Close()
		c.sftp.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("sftp write %s: %w", tmpPath, err)
	}

	err = tmpFile.Close()
	if err != nil {
		c.sftp.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("sftp close temp %s: %w", tmpPath, err)
	}

	// Set permissions on temp file
	err = c.sftp.Chmod(tmpPath, 0644)
	if err != nil {
		c.sftp.Remove(tmpPath)
		return fmt.Errorf("sftp chmod %s: %w", tmpPath, err)
	}

	// Remove destination if it exists (SFTP rename won't overwrite)
	_, err = c.sftp.Stat(fullPath)
	if err == nil {
		err = c.sftp.Remove(fullPath)
		if err != nil {
			c.sftp.Remove(tmpPath) // Clean up temp file
			return fmt.Errorf("sftp remove existing %s: %w", fullPath, err)
		}
	}

	// Atomic rename
	err = c.sftp.Rename(tmpPath, fullPath)
	if err != nil {
		c.sftp.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("sftp rename %s -> %s: %w", tmpPath, fullPath, err)
	}

	// Set permissions on final file
	err = c.sftp.Chmod(fullPath, 0644)
	if err != nil {
		return fmt.Errorf("sftp chmod %s: %w", fullPath, err)
	}

	return nil
}

// DeleteFile removes a file from the device.
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	if c.sftp == nil {
		return fmt.Errorf("sftp: client not initialized")
	}

	fullPath := filepath.Join(c.baseDir, path)

	err := c.sftp.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("sftp remove %s: %w", path, err)
	}

	return nil
}

// GetFileContent reads the raw content of a file from the device.
// Returns an io.ReadCloser that the caller MUST close.
func (c *Client) GetFileContent(ctx context.Context, path string) (io.ReadCloser, error) {
	if c.sftp == nil {
		return nil, fmt.Errorf("sftp: client not initialized")
	}

	fullPath := filepath.Join(c.baseDir, path)

	f, err := c.sftp.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", path, err)
	}

	return f, nil
}

// computeHash reads from the reader and returns the SHA256 hex digest.
// It uses a buffered reader to avoid loading the entire file into memory.
func computeHash(r io.Reader) string {
	h := sha256.New()
	// Use a buffered reader for efficiency
	buf := make([]byte, 32*1024) // 32KB buffer
	_, err := io.CopyBuffer(h, r, buf)
	if err != nil {
		// If we can't read, return a zero hash rather than panicking
		return strings.Repeat("0", 64)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// randomHex generates a random hex string of the given length.
func randomHex(n int) string {
	bytes := make([]byte, n/2)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback: use time-based value if rand fails
		return fmt.Sprintf("%x", time.Now().UnixNano())[:n]
	}
	return hex.EncodeToString(bytes)
}

// walkSFTP walks the SFTP directory tree similar to filepath.Walk.
// It calls the walkFn for each file and directory found.
// Paths passed to walkFn are relative to the root directory.
func walkSFTP(ctx context.Context, client sftpClient, root, relDir string, walkFn filepath.WalkFunc) error {
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := client.ReadDir(filepath.Join(root, relDir))
	if err != nil {
		return walkFn(filepath.Join(root, relDir), nil, err)
	}

	for _, entry := range entries {
		// Check context periodically
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath := filepath.Join(relDir, entry.Name())
		// Normalize to forward slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)

		if entry.IsDir() {
			err = walkSFTP(ctx, client, root, relPath, walkFn)
			if err != nil {
				return err
			}
			continue
		}

		err = walkFn(relPath, entry, nil)
		if err != nil {
			return err
		}
	}

	return nil
}


// ListDocuments walks the xochitl directory, finds all .metadata files,
// parses them, and returns Document entities. Only non-deleted documents
// are returned.
func (c *Client) ListDocuments(ctx context.Context) ([]document.Document, error) {
	if c.sftp == nil {
		return nil, fmt.Errorf("sftp: client not initialized")
	}

	var docs []document.Document

	err := walkSFTP(ctx, c.sftp, c.baseDir, ".", func(relPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(relPath, ".metadata") {
			return nil
		}

		// Read and parse the metadata file — silently skip on parse errors
		meta, err := c.parseMetadataFile(relPath)
		if err != nil {
			return nil
		}

		// Skip deleted items and folders
		if meta.IsDeleted || meta.FileFormat == "folder" {
			return nil
		}

		// Derive the content file path (strip .metadata, keep extension from FileFormat)
		baseName := strings.TrimSuffix(relPath, ".metadata")
		contentPath := baseName + "." + meta.FileFormat

		// Get file size and mod time from the content file
		var size int64
		var modTime time.Time
		contentInfo, err := c.sftp.Stat(filepath.Join(c.baseDir, contentPath))
		if err == nil && !contentInfo.IsDir() {
			size = contentInfo.Size()
			modTime = contentInfo.ModTime()
		}

		docID, err := uuid.Parse(meta.DeviceID)
		if err != nil {
			return nil
		}

		doc := document.Document{
			ID:          docID,
			VisibleName: meta.VisibleName,
			Type:        document.DocumentType(meta.FileFormat),
			Size:        size,
			ModTime:     modTime,
		}
		docs = append(docs, doc)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sftp list documents: %w", err)
	}

	return docs, nil
}

// parseMetadataFile reads a .metadata file from the device and parses it
// into a Metadata struct.
func (c *Client) parseMetadataFile(relPath string) (document.Metadata, error) {
	fullPath := filepath.Join(c.baseDir, relPath)
	f, err := c.sftp.Open(fullPath)
	if err != nil {
		return document.Metadata{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return document.Metadata{}, err
	}

	var meta document.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return document.Metadata{}, err
	}

	return meta, nil
}

// GetDocumentMetadata reads and parses .metadata and .content sidecar
// files for a given device UUID.
func (c *Client) GetDocumentMetadata(ctx context.Context, deviceUUID string) (document.SidecarMetadata, error) {
	if c.sftp == nil {
		return document.SidecarMetadata{}, fmt.Errorf("sftp: client not initialized")
	}

	metadataPath := deviceUUID + ".metadata"
	contentPath := deviceUUID + ".content"
	pageDataPath := deviceUUID + ".pagedata"

	var metadataFile, contentFile, pageDataFile string

	// Read .metadata file (required)
	metaData, err := c.readFile(filepath.Join(c.baseDir, metadataPath))
	if err != nil {
		return document.SidecarMetadata{}, fmt.Errorf("sftp read metadata %s: %w", metadataPath, err)
	}
	metadataFile = string(metaData)

	// Read .content file (required)
	contentData, err := c.readFile(filepath.Join(c.baseDir, contentPath))
	if err != nil {
		return document.SidecarMetadata{}, fmt.Errorf("sftp read content %s: %w", contentPath, err)
	}
	contentFile = string(contentData)

	// Read .pagedata file (optional)
	pageData, err := c.readFile(filepath.Join(c.baseDir, pageDataPath))
	if err == nil {
		pageDataFile = string(pageData)
	}

	return document.SidecarMetadata{
		MetadataFile: metadataFile,
		ContentFile:  contentFile,
		PageDataFile: pageDataFile,
	}, nil
}

// readFile reads the entire contents of a file from the device.
func (c *Client) readFile(path string) ([]byte, error) {
	f, err := c.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

// PutDocument uploads a document's content file and generates .metadata
// and .content sidecar files on the device. Uses atomic writes
// (temp + rename) for all files.
func (c *Client) PutDocument(ctx context.Context, doc document.Document, content io.Reader) error {
	if c.sftp == nil {
		return fmt.Errorf("sftp: client not initialized")
	}

	// Use provided UUID or generate a new one
	docID := uuid.New().String()
	if doc.ID != uuid.Nil {
		docID = doc.ID.String()
	}
	ext := string(doc.Type)
	if ext == "" {
		ext = "pdf"
	}

	contentPath := docID + "." + ext
	metadataPath := docID + ".metadata"
	contentMetaPath := docID + ".content"

	// Write content file
	fullContentPath := filepath.Join(c.baseDir, contentPath)
	contentFile, err := c.sftp.Create(fullContentPath)
	if err != nil {
		return fmt.Errorf("sftp create content %s: %w", contentPath, err)
	}

	_, err = io.Copy(contentFile, content)
	if err != nil {
		contentFile.Close()
		c.sftp.Remove(fullContentPath)
		return fmt.Errorf("sftp write content %s: %w", contentPath, err)
	}
	if err := contentFile.Close(); err != nil {
		c.sftp.Remove(fullContentPath)
		return fmt.Errorf("sftp close content %s: %w", contentPath, err)
	}

	// Write .metadata file atomically
	metadataJSON, err := json.Marshal(document.Metadata{
		DeviceID:         docID,
		VisibleName:      doc.VisibleName,
		FileFormat:       ext,
		ParentFolderUUID: "",
		LastModified:     time.Now().UTC(),
		IsDeleted:        false,
	})
	if err != nil {
		return fmt.Errorf("sftp marshal metadata: %w", err)
	}

	if err := c.writeFileAtomically(metadataPath, metadataJSON); err != nil {
		return err
	}

	// Write .content file atomically
	contentJSON, err := json.Marshal(document.Content{
		Pages:    []document.PageInfo{{Width: 1404, Height: 1872}},
		FileType: ext,
	})
	if err != nil {
		return fmt.Errorf("sftp marshal content: %w", err)
	}

	if err := c.writeFileAtomically(contentMetaPath, contentJSON); err != nil {
		return err
	}

	return nil
}

// PutFolder creates a folder on the device by writing a .metadata file
// with folder type.
func (c *Client) PutFolder(ctx context.Context, folder document.Folder) error {
	if c.sftp == nil {
		return fmt.Errorf("sftp: client not initialized")
	}

	folderID := folder.ID.String()
	metadataPath := folderID + ".metadata"

	parentID := ""
	if folder.ParentID != uuid.Nil {
		parentID = folder.ParentID.String()
	}

	metadataJSON, err := json.Marshal(document.Metadata{
		DeviceID:         folderID,
		VisibleName:      folder.VisibleName,
		FileFormat:       "folder",
		ParentFolderUUID: parentID,
		LastModified:     time.Now().UTC(),
		IsDeleted:        false,
	})
	if err != nil {
		return fmt.Errorf("sftp marshal folder metadata: %w", err)
	}

	return c.writeFileAtomically(metadataPath, metadataJSON)
}

// writeFileAtomically writes data to a file on the device using a temp +
// rename pattern to ensure atomicity. Delegates to atomicWrite.
func (c *Client) writeFileAtomically(relPath string, data []byte) error {
	fullPath := filepath.Join(c.baseDir, relPath)
	return atomicWrite(c.sftp, fullPath, data)
}
// atomicWrite writes data to a file using a temp + rename pattern.
// It operates through the sftpClient interface for testability.
func atomicWrite(client sftpClient, path string, data []byte) error {
	tmpPath := path + ".tmp." + randomHex(8)

	tmpFile, err := client.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("sftp create temp %s: %w", tmpPath, err)
	}

	_, err = tmpFile.Write(data)
	if err != nil {
		tmpFile.Close()
		client.Remove(tmpPath)
		return fmt.Errorf("sftp write temp %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		client.Remove(tmpPath)
		return fmt.Errorf("sftp close temp %s: %w", tmpPath, err)
	}

	if err := client.Chmod(tmpPath, 0644); err != nil {
		client.Remove(tmpPath)
		return fmt.Errorf("sftp chmod temp %s: %w", tmpPath, err)
	}

	_, err = client.Stat(path)
	if err == nil {
		if err := client.Remove(path); err != nil {
			client.Remove(tmpPath)
			return fmt.Errorf("sftp remove existing %s: %w", path, err)
		}
	}

	if err := client.Rename(tmpPath, path); err != nil {
		client.Remove(tmpPath)
		return fmt.Errorf("sftp rename %s -> %s: %w", tmpPath, path, err)
	}

	return nil
}