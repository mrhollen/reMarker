// Package manifeststore provides a file-based implementation of the
// ManifestRepository interface for persisting sync manifests as JSON.
package manifeststore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hollen/remarker/internal/domain/document"
)

// Compile-time check: Store implements document.ManifestRepository.
var _ document.ManifestRepository = (*Store)(nil)

const (
	// DefaultDir is the default directory name for reMarker data.
	DefaultDir = ".remarker"
	// DefaultFileName is the default manifest file name.
	DefaultFileName = "manifest.json"
)

// Store is a file-based manifest repository that reads and writes
// JSON manifest files to the local filesystem.
type Store struct {
	filePath string
}

// New creates a Store with the given file path.
func New(path string) *Store {
	return &Store{filePath: path}
}

// NewDefault creates a Store with the default path under the given base directory.
// The path is constructed as baseDir/.remarker/manifest.json.
func NewDefault(baseDir string) *Store {
	return New(filepath.Join(baseDir, DefaultDir, DefaultFileName))
}

// Load reads the manifest from the stored file path.
// Returns an error if the file does not exist, contains invalid JSON,
// or is malformed (not a valid manifest object).
func (s *Store) Load(_ context.Context) (*document.Manifest, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	var manifest document.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// Save writes the manifest to the stored file path using an atomic write
// pattern (write to .tmp, then rename). Creates parent directories if needed.
// Returns an error if manifest is nil.
func (s *Store) Save(_ context.Context, manifest *document.Manifest) error {
	if manifest == nil {
		return fmt.Errorf("cannot save nil manifest")
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest tmp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		// Clean up tmp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("rename manifest file: %w", err)
	}

	return nil
}

// Exists returns true if the manifest file exists at the stored path.
func (s *Store) Exists(_ context.Context) (bool, error) {
	_, err := os.Stat(s.filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat manifest file: %w", err)
}
