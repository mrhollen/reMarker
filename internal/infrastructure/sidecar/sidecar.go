// Package sidecar provides a file-system implementation of the
// SidecarRepository interface for persisting reMarkable sidecar metadata
// files (.metadata, .content, .pagedata) per document.
package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hollen/remarker/internal/domain/document"
)

const (
	// DefaultDir is the top-level hidden directory for sidecar data.
	DefaultDir = ".remarker"

	// DefaultSubDir is the subdirectory under DefaultDir for device metadata.
	DefaultSubDir = "device-meta"

	// DefaultPageWidth is the default page width for a reMarkable document.
	DefaultPageWidth = 1404

	// DefaultPageHeight is the default page height for a reMarkable document.
	DefaultPageHeight = 1872
)

// Store is a file-system backed implementation of document.SidecarRepository.
type Store struct {
	baseDir string
}

// New creates a Store backed by the given base directory.
func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// NewDefault creates a Store in syncDir/.remarker/device-meta.
func NewDefault(syncDir string) *Store {
	return New(filepath.Join(syncDir, DefaultDir, DefaultSubDir))
}

// NewDefaultContent creates a Content with a single page at default dimensions.
func NewDefaultContent(fileType string) document.Content {
	return document.Content{
		Pages: []document.PageInfo{
			{Width: DefaultPageWidth, Height: DefaultPageHeight},
		},
		FileType: fileType,
	}
}

// SaveMetadata persists metadata and content for a document by writing
// .metadata and .content files into a device-specific subdirectory.
func (s *Store) SaveMetadata(_ context.Context, docUUID string, meta document.Metadata, content document.Content) error {
	docDir := filepath.Join(s.baseDir, docUUID)
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", docDir, err)
	}

	if err := atomicWriteJSON(filepath.Join(docDir, ".metadata"), meta); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	if err := atomicWriteJSON(filepath.Join(docDir, ".content"), content); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}

// GetMetadata reads metadata and content for a document.
func (s *Store) GetMetadata(_ context.Context, docUUID string) (document.Metadata, document.Content, error) {
	docDir := filepath.Join(s.baseDir, docUUID)

	meta, err := readJSON[document.Metadata](filepath.Join(docDir, ".metadata"))
	if err != nil {
		return document.Metadata{}, document.Content{}, fmt.Errorf("read metadata: %w", err)
	}

	content, err := readJSON[document.Content](filepath.Join(docDir, ".content"))
	if err != nil {
		return document.Metadata{}, document.Content{}, fmt.Errorf("read content: %w", err)
	}

	return meta, content, nil
}

// DeleteMetadata removes all sidecar files for a document and cleans up
// the device subdirectory if empty.
func (s *Store) DeleteMetadata(_ context.Context, docUUID string) error {
	docDir := filepath.Join(s.baseDir, docUUID)

	// Remove individual files (ignore errors for missing files).
	os.Remove(filepath.Join(docDir, ".metadata"))
	os.Remove(filepath.Join(docDir, ".content"))
	os.Remove(filepath.Join(docDir, ".pagedata"))

	// Remove device subdirectory (ignore errors for non-empty or missing).
	os.Remove(docDir)

	return nil
}

// Exists returns true if both .metadata and .content files are present.
func (s *Store) Exists(_ context.Context, docUUID string) (bool, error) {
	docDir := filepath.Join(s.baseDir, docUUID)

	metaPath := filepath.Join(docDir, ".metadata")
	contentPath := filepath.Join(docDir, ".content")

	metaExists := fileExists(metaPath)
	contentExists := fileExists(contentPath)

	return metaExists && contentExists, nil
}

// atomicWriteJSON marshals v to pretty-printed JSON and writes it atomically
// using a temp file + rename pattern.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// readJSON reads a file and unmarshals it into T.
func readJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return zero, fmt.Errorf("unmarshal: %w", err)
	}

	return v, nil
}

// fileExists returns true if the file at path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
