// Package sftp provides SFTP operations for reading and writing files on a
// reMarkable device. It implements the document.DeviceRepository interface
// for device-side file operations.
package sftp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
	"github.com/hollen/remarker/internal/infrastructure/ssh"
	"github.com/pkg/sftp"
)

// XochitlDir is the root directory on the reMarkable device where documents
// are stored.
const XochitlDir = "/home/root/.local/share/remarkable/xochitl"

// Client wraps an sftp.Client and provides operations for the reMarkable
// device's xochitl directory. It implements document.DeviceRepository.
type Client struct {
	sftp    *sftp.Client
	baseDir string
}

// New creates a new SFTP client using the provided SSH client. The SFTP
// client operates relative to the xochitl directory on the device.
func New(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil || sshClient.SSH() == nil {
		return nil, fmt.Errorf("sftp: ssh client is nil")
	}

	sftpClient, err := sftp.NewClient(sshClient.SSH())
	if err != nil {
		return nil, fmt.Errorf("sftp new client: %w", err)
	}

	return &Client{
		sftp:    sftpClient,
		baseDir: XochitlDir,
	}, nil
}

// Close closes the underlying SFTP connection.
func (c *Client) Close() error {
	if c == nil || c.sftp == nil {
		return nil
	}
	return c.sftp.Close()
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
func walkSFTP(ctx context.Context, client *sftp.Client, root, relDir string, walkFn filepath.WalkFunc) error {
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
