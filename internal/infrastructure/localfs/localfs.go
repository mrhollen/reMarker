// Package localfs provides local filesystem operations for reading and writing
// files in the sync directory. It implements the document.LocalRepository
// interface for local-side file operations.
package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hollen/remarker/internal/domain/document"
)

// Client wraps a local directory path and provides file operations for the
// sync directory. It implements document.LocalRepository.
type Client struct {
	baseDir string
}

// New creates a new Client for the given base directory. The base directory
// is the root of the local sync area.
func New(baseDir string) *Client {
	return &Client{baseDir: baseDir}
}

// ListFiles returns all regular files in the base directory recursively.
// Paths are relative to the base directory. Directories named ".git" and
// ".remarker" are skipped entirely. Results are sorted by path for
// deterministic output. If the base directory does not exist, it is created.
func (c *Client) ListFiles(ctx context.Context) ([]document.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("localfs list: %w", err)
	}

	// Create baseDir if it doesn't exist
	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("localfs mkdir %s: %w", c.baseDir, err)
	}

	var files []document.File

	err := filepath.WalkDir(c.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check context before processing each entry
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("localfs list: %w", err)
		}

		// Skip directories (we only want regular files)
		if d.IsDir() {
			// Skip .git and .remarker directories entirely
			if d.Name() == ".git" || d.Name() == ".remarker" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip non-regular files (symlinks, etc.)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}

		// Compute relative path
		relPath, err := filepath.Rel(c.baseDir, path)
		if err != nil {
			return fmt.Errorf("localfs rel path %s: %w", path, err)
		}
		// Normalize to forward slashes for cross-platform consistency
		relPath = filepath.ToSlash(relPath)

		// Open file to compute hash
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("localfs open %s: %w", relPath, err)
		}

		hash := computeHash(f)
		f.Close()

		files = append(files, document.File{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Hash:    hash,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("localfs walk %s: %w", c.baseDir, err)
	}

	// Sort by path for deterministic output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

// GetFile reads a single file from the local filesystem and returns its
// metadata including the SHA256 hash. The path is relative to the base
// directory.
func (c *Client) GetFile(ctx context.Context, path string) (document.File, error) {
	if err := ctx.Err(); err != nil {
		return document.File{}, fmt.Errorf("localfs get: %w", err)
	}

	fullPath := filepath.Join(c.baseDir, path)

	info, err := os.Stat(fullPath)
	if err != nil {
		return document.File{}, fmt.Errorf("localfs stat %s: %w", path, err)
	}

	if info.IsDir() {
		return document.File{}, fmt.Errorf("localfs: %s is a directory, not a file", path)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return document.File{}, fmt.Errorf("localfs open %s: %w", path, err)
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

// PutFile creates a file in the local sync directory with the given metadata.
// Parent directories are created as needed. The file is created as an empty
// placeholder; the actual content transfer is handled at the application layer.
// The modification time is set to the value provided in the File struct.
func (c *Client) PutFile(ctx context.Context, file document.File) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("localfs put: %w", err)
	}

	fullPath := filepath.Join(c.baseDir, file.Path)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("localfs mkdir %s: %w", parentDir, err)
	}

	// Create the file (empty placeholder)
	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("localfs create %s: %w", file.Path, err)
	}
	f.Close()

	// Set the modification time
	if err := os.Chtimes(fullPath, file.ModTime, file.ModTime); err != nil {
		return fmt.Errorf("localfs chtimes %s: %w", file.Path, err)
	}

	return nil
}

// PutFileContent writes file content from an io.Reader to the local filesystem
// using an atomic write pattern (temp file + rename). Parent directories are
// created as needed. The modification time is set to the value provided in the
// File struct. Use this method when you need to transfer actual file content
// (e.g., during pull sync) rather than creating an empty placeholder.
func (c *Client) PutFileContent(ctx context.Context, file document.File, content io.Reader) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("localfs put content: %w", err)
	}

	fullPath := filepath.Join(c.baseDir, file.Path)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("localfs mkdir %s: %w", parentDir, err)
	}

	// Create temp file in the same directory for atomic rename
	tmpFile, err := os.CreateTemp(parentDir, ".remarker-tmp-*")
	if err != nil {
		return fmt.Errorf("localfs create temp %s: %w", parentDir, err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content to temp file
	_, err = io.Copy(tmpFile, content)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("localfs write %s: %w", file.Path, err)
	}

	// Close temp file before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("localfs close temp %s: %w", tmpPath, err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return fmt.Errorf("localfs rename %s: %w", file.Path, err)
	}
	success = true

	// Set the modification time
	if err := os.Chtimes(fullPath, file.ModTime, file.ModTime); err != nil {
		return fmt.Errorf("localfs chtimes %s: %w", file.Path, err)
	}

	return nil
}

// DeleteFile removes a file from the local filesystem. The path is relative
// to the base directory.
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("localfs delete: %w", err)
	}

	fullPath := filepath.Join(c.baseDir, path)

	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("localfs remove %s: %w", path, err)
	}

	return nil
}

// computeHash reads from the reader and returns the SHA256 hex digest.
// It uses a buffered reader to avoid loading the entire file into memory.
func computeHash(r io.Reader) string {
	h := sha256.New()
	buf := make([]byte, 32*1024) // 32KB buffer
	_, err := io.CopyBuffer(h, r, buf)
	if err != nil {
		// If we can't read, return a zero hash rather than panicking
		return strings.Repeat("0", 64)
	}
	return hex.EncodeToString(h.Sum(nil))
}
