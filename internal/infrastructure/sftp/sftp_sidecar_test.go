package sftp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hollen/remarker/internal/domain/document"
)

// mockSFTPClient implements sftpClient interface for unit tests.
// It uses a temp directory to simulate the SFTP filesystem.
type mockSFTPClient struct {
	rootDir string
}

func newMockSFTP(t *testing.T) *mockSFTPClient {
	t.Helper()
	dir := t.TempDir()
	return &mockSFTPClient{rootDir: dir}
}

func (m *mockSFTPClient) ReadDir(path string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(m.resolve(path))
	if err != nil {
		return nil, err
	}
	var infos []os.FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (m *mockSFTPClient) Open(path string) (sftpFile, error) {
	f, err := os.Open(m.resolve(path))
	if err != nil {
		return nil, err
	}
	return &mockSFTPFile{File: f}, nil
}

func (m *mockSFTPClient) Create(path string) (sftpFile, error) {
	f, err := os.Create(m.resolve(path))
	if err != nil {
		return nil, err
	}
	return &mockSFTPFile{File: f}, nil
}

func (m *mockSFTPClient) MkdirAll(path string) error {
	return os.MkdirAll(m.resolve(path), 0755)
}

func (m *mockSFTPClient) Stat(path string) (os.FileInfo, error) {
	return os.Stat(m.resolve(path))
}

func (m *mockSFTPClient) Remove(path string) error {
	return os.Remove(m.resolve(path))
}

func (m *mockSFTPClient) Rename(oldPath, newPath string) error {
	return os.Rename(m.resolve(oldPath), m.resolve(newPath))
}

func (m *mockSFTPClient) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(m.resolve(path), mode)
}

func (m *mockSFTPClient) Close() error {
	return nil
}

func (m *mockSFTPClient) resolve(path string) string {
	// If path is absolute, use it directly relative to rootDir
	if filepath.IsAbs(path) {
		return filepath.Join(m.rootDir, strings.TrimPrefix(path, "/home/root/.local/share/remarkable/xochitl"))
	}
	return filepath.Join(m.rootDir, path)
}

// mockSFTPFile wraps an os.File to satisfy the file operations used by the SFTP client.
type mockSFTPFile struct {
	*os.File
}

// newTestClient creates an SFTP Client with a mock backend for testing.
func newTestClient(t *testing.T, mock *mockSFTPClient) *Client {
	t.Helper()
	return &Client{
		sftp:    mock,
		baseDir: XochitlDir,
	}
}

// --- ListDocuments tests ---

func TestListDocuments_NilSFTP(t *testing.T) {
	c := &Client{}
	_, err := c.ListDocuments(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "sftp: client not initialized") {
		t.Errorf("error = %q, want substring %q", err.Error(), "sftp: client not initialized")
	}
}

func TestListDocuments_EmptyDirectory(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents, got %d", len(docs))
	}
}

func TestListDocuments_SingleDocument(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create a document with all sidecar files
	uuid := "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d"
	meta := map[string]interface{}{
		"DocumentID":       uuid,
		"VisibleName":      "Annual Report",
		"FileFormat":       "pdf",
		"ParentFolderUUID": "",
		"LastModified":     "2026-06-13T12:00:00Z",
		"IsDeleted":        false,
	}
	metaJSON, _ := json.Marshal(meta)

	files := map[string][]byte{
		uuid + ".pdf":       []byte("pdf content"),
		uuid + ".metadata":  metaJSON,
		uuid + ".content":   []byte(`{"pages":[{"width":1404,"height":1872}],"fileType":"pdf"}`),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(mock.rootDir, name), content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	doc := docs[0]
	if doc.ID.String() != uuid {
		t.Errorf("doc.ID = %s, want %s", doc.ID, uuid)
	}
	if doc.VisibleName != "Annual Report" {
		t.Errorf("doc.VisibleName = %q, want %q", doc.VisibleName, "Annual Report")
	}
	if doc.Type != document.DocumentTypePDF {
		t.Errorf("doc.Type = %q, want %q", doc.Type, document.DocumentTypePDF)
	}
}

func TestListDocuments_MultipleDocuments(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create two documents
	docs := []struct {
		uuid   string
		name   string
		format string
	}{
		{"11111111-1111-1111-1111-111111111111", "Doc One", "pdf"},
		{"22222222-2222-2222-2222-222222222222", "Doc Two", "epub"},
	}

	for _, d := range docs {
		meta := map[string]interface{}{
			"DocumentID":       d.uuid,
			"VisibleName":      d.name,
			"FileFormat":       d.format,
			"ParentFolderUUID": "",
			"LastModified":     "2026-06-13T12:00:00Z",
			"IsDeleted":        false,
		}
		metaJSON, _ := json.Marshal(meta)
		contentJSON, _ := json.Marshal(map[string]interface{}{
			"pages":    []map[string]int{{"width": 1404, "height": 1872}},
			"fileType": d.format,
		})

		files := map[string][]byte{
			d.uuid + "." + d.format: []byte("content"),
			d.uuid + ".metadata":    metaJSON,
			d.uuid + ".content":     contentJSON,
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(mock.rootDir, name), content, 0644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}

	found, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(found))
	}

	// Build a map for easier lookup
	foundMap := make(map[string]document.Document)
	for _, d := range found {
		foundMap[d.VisibleName] = d
	}

	if doc, ok := foundMap["Doc One"]; !ok {
		t.Error("Doc One not found")
	} else if doc.Type != document.DocumentTypePDF {
		t.Errorf("Doc One type = %q, want %q", doc.Type, document.DocumentTypePDF)
	}

	if doc, ok := foundMap["Doc Two"]; !ok {
		t.Error("Doc Two not found")
	} else if doc.Type != document.DocumentTypeEPub {
		t.Errorf("Doc Two type = %q, want %q", doc.Type, document.DocumentTypeEPub)
	}
}

func TestListDocuments_SkipsDeleted(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create a deleted document
	uuid := "33333333-3333-3333-3333-333333333333"
	meta := map[string]interface{}{
		"DocumentID":       uuid,
		"VisibleName":      "Deleted Doc",
		"FileFormat":       "pdf",
		"ParentFolderUUID": "",
		"LastModified":     "2026-06-13T12:00:00Z",
		"IsDeleted":        true,
	}
	metaJSON, _ := json.Marshal(meta)

	files := map[string][]byte{
		uuid + ".pdf":       []byte("content"),
		uuid + ".metadata":  metaJSON,
		uuid + ".content":   []byte(`{"pages":[],"fileType":"pdf"}`),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(mock.rootDir, name), content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents (deleted should be skipped), got %d", len(docs))
	}
}

func TestListDocuments_SkipsFolders(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create a folder (FileFormat = "folder")
	uuid := "44444444-4444-4444-4444-444444444444"
	meta := map[string]interface{}{
		"DocumentID":       uuid,
		"VisibleName":      "My Folder",
		"FileFormat":       "folder",
		"ParentFolderUUID": "",
		"LastModified":     "2026-06-13T12:00:00Z",
		"IsDeleted":        false,
	}
	metaJSON, _ := json.Marshal(meta)

	if err := os.WriteFile(filepath.Join(mock.rootDir, uuid+".metadata"), metaJSON, 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 documents (folders should be skipped), got %d", len(docs))
	}
}

func TestListDocuments_IgnoresParseErrors(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create a valid document
	uuid1 := "55555555-5555-5555-5555-555555555555"
	meta1 := map[string]interface{}{
		"DocumentID":       uuid1,
		"VisibleName":      "Valid Doc",
		"FileFormat":       "pdf",
		"ParentFolderUUID": "",
		"LastModified":     "2026-06-13T12:00:00Z",
		"IsDeleted":        false,
	}
	metaJSON1, _ := json.Marshal(meta1)

	// Create an invalid metadata file (bad JSON)
	uuid2 := "66666666-6666-6666-6666-666666666666"

	files := map[string][]byte{
		uuid1 + ".pdf":        []byte("content"),
		uuid1 + ".metadata":   metaJSON1,
		uuid1 + ".content":    []byte(`{"pages":[],"fileType":"pdf"}`),
		uuid2 + ".metadata":   []byte("not valid json {{{"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(mock.rootDir, name), content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document (invalid metadata should be skipped), got %d", len(docs))
	}
	if docs[0].VisibleName != "Valid Doc" {
		t.Errorf("doc.VisibleName = %q, want %q", docs[0].VisibleName, "Valid Doc")
	}
}

func TestListDocuments_MixedDocumentsAndFolders(t *testing.T) {
	mock := newMockSFTP(t)
	c := newTestClient(t, mock)

	// Create documents and folders
	items := []struct {
		uuid   string
		name   string
		format string
	}{
		{"11111111-1111-1111-1111-111111111111", "PDF Doc", "pdf"},
		{"22222222-2222-2222-2222-222222222222", "My Folder", "folder"},
		{"33333333-3333-3333-3333-333333333333", "EPUB Doc", "epub"},
		{"44444444-4444-4444-4444-444444444444", "Another Folder", "folder"},
		{"55555555-5555-5555-5555-555555555555", "Notebook", "notebook"},
	}

	for _, item := range items {
		meta := map[string]interface{}{
			"DocumentID":       item.uuid,
			"VisibleName":      item.name,
			"FileFormat":       item.format,
			"ParentFolderUUID": "",
			"LastModified":     "2026-06-13T12:00:00Z",
			"IsDeleted":        false,
		}
		metaJSON, _ := json.Marshal(meta)
		if err := os.WriteFile(filepath.Join(mock.rootDir, item.uuid+".metadata"), metaJSON, 0644); err != nil {
			t.Fatalf("write %s metadata: %v", item.uuid, err)
		}
		// Only create content file for non-folders
		if item.format != "folder" {
			if err := os.WriteFile(filepath.Join(mock.rootDir, item.uuid+"."+item.format), []byte("content"), 0644); err != nil {
				t.Fatalf("write %s content: %v", item.uuid, err)
			}
			if err := os.WriteFile(filepath.Join(mock.rootDir, item.uuid+".content"),
				[]byte(fmt.Sprintf(`{"pages":[{"width":1404,"height":1872}],"fileType":"%s"}`, item.format)), 0644); err != nil {
				t.Fatalf("write %s content sidecar: %v", item.uuid, err)
			}
		}
	}

	docs, err := c.ListDocuments(context.Background())
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}

	// Should only return documents, not folders
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents (excluding 2 folders), got %d", len(docs))
	}

	// Verify no folders in results
	for _, doc := range docs {
		if doc.Type == document.DocumentTypeFolder {
			t.Errorf("ListDocuments should not return folders, got: %s", doc.VisibleName)
		}
	}
}
