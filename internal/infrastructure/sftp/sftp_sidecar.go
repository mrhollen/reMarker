package sftp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hollen/remarker/internal/domain/document"
)

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
	var docID string
	if doc.ID != uuid.Nil {
		docID = doc.ID.String()
	} else {
		docID = uuid.New().String()
	}
	ext := string(doc.Type)
	if ext == "" {
		ext = "pdf"
	}

	contentPath := docID + "." + ext
	metadataPath := docID + ".metadata"
	contentMetaPath := docID + ".content"

	// Write content file atomically
	contentData, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("sftp read content: %w", err)
	}
	if err := c.writeFileAtomically(contentPath, contentData); err != nil {
		return err
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

	// Create the actual folder directory on the device
	if err := c.sftp.MkdirAll(folderID); err != nil {
		return fmt.Errorf("sftp mkdir %s: %w", folderID, err)
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
