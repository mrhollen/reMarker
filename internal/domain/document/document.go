// Package document defines the core domain entities for reMarkable document
// synchronization. This is the innermost layer and depends only on the Go
// standard library and google/uuid.
package document

import (
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hollen/remarker/internal/domain/errors"
)

// ============================================================================
// DocumentType
// ============================================================================

// DocumentType represents the type of content a document holds on the device.
type DocumentType string

const (
	// DocumentTypePDF represents a PDF document.
	DocumentTypePDF DocumentType = "pdf"
	// DocumentTypeNotebook represents a reMarkable notebook (handwritten notes).
	DocumentTypeNotebook DocumentType = "notebook"
	// DocumentTypeEPub represents an EPUB ebook.
	DocumentTypeEPub DocumentType = "epub"
	// DocumentTypeFolder represents a folder on the device.
	DocumentTypeFolder DocumentType = "folder"
)

// ============================================================================
// Document
// ============================================================================

// Document represents a syncable document on the reMarkable device.
// It carries the business rules for comparing local and device state,
// deriving device paths, and determining sync status.
type Document struct {
	ID          uuid.UUID    // Device UUID - unique identifier
	LocalPath   string       // Relative path from sync root, e.g. "documents/Annual Report.pdf"
	Type        DocumentType // pdf, notebook, epub
	VisibleName string       // Human-readable name on device (derived from LocalPath filename)
	LocalHash   string       // SHA256 of local content file
	DeviceHash  string       // SHA256 of device content file
	Size        int64        // File size in bytes
	ModTime     time.Time    // Last modification time
	SyncedAt    time.Time    // Last successful sync time
}

// NewDocument creates a new Document with a generated UUID and derived VisibleName.
func NewDocument(localPath string, docType DocumentType) Document {
	return Document{
		ID:          uuid.New(),
		LocalPath:   localPath,
		Type:        docType,
		VisibleName: filepath.Base(localPath),
	}
}

// IsSynced returns true if the local and device content hashes match,
// indicating the document is in sync.
func (d Document) IsSynced() bool {
	return d.LocalHash == d.DeviceHash
}

// IsNewer returns true if this document's modification time is strictly after
// the other document's modification time.
func (d Document) IsNewer(other Document) bool {
	return d.ModTime.After(other.ModTime)
}

// ToDevicePath returns the device-side filename in the form "<uuid>.<ext>",
// e.g. "a1b2c3d4-5e6f-7a8b-9c0d-1e2f3a4b5c6d.pdf".
func (d Document) ToDevicePath() string {
	return d.ID.String() + "." + string(d.Type)
}

// ============================================================================
// Folder
// ============================================================================

// Folder represents a folder on the reMarkable device.
type Folder struct {
	ID          uuid.UUID // Device UUID
	LocalPath   string    // Relative path, e.g. "documents/Projects/"
	VisibleName string    // Folder name visible on device
	ParentID    uuid.UUID // Parent folder UUID, zero UUID if root
}

// NewFolder creates a new Folder with a generated UUID.
func NewFolder(localPath, visibleName string, parentID uuid.UUID) Folder {
	return Folder{
		ID:          uuid.New(),
		LocalPath:   localPath,
		VisibleName: visibleName,
		ParentID:    parentID,
	}
}

// IsRoot returns true if the folder has no parent (zero UUID).
func (f Folder) IsRoot() bool {
	return f.ParentID == uuid.Nil
}

// ============================================================================
// SidecarMetadata
// ============================================================================

// SidecarMetadata represents the device metadata files associated with a
// document. The reMarkable device stores document metadata in companion
// files alongside the content file.
type SidecarMetadata struct {
	MetadataFile string // Content of .metadata file (JSON)
	ContentFile  string // Content of .content file (JSON)
	PageDataFile string // Content of .pagedata file (if any)
}

// ============================================================================
// Legacy types (preserved for backward compatibility)
// ============================================================================

// File represents any file in the sync scope.
// It carries the metadata needed to compare, transfer, and track files
// between the local filesystem and the reMarkable device.
type File struct {
	Path    string    // Relative path from sync root (e.g., "abc-123.metadata")
	Size    int64
	ModTime time.Time
	Hash    string // SHA256 hex string
}

// IsNewer returns true if this file's modification time is strictly after
// the other file's modification time.
func (f File) IsNewer(other File) bool {
	return f.ModTime.After(other.ModTime)
}

// ============================================================================
// Manifest v2
// ============================================================================

// ManifestEntry represents a single document entry tracked in the sync manifest.
// It maps a local path to its corresponding device state using UUIDs.
type ManifestEntry struct {
	DeviceUUID  string       `json:"deviceUUID"`
	DeviceType  DocumentType `json:"deviceType"`
	LocalHash   string       `json:"localHash"`
	DeviceHash  string       `json:"deviceHash,omitempty"`
	ParentUUID  string       `json:"parentUUID,omitempty"`
	VisibleName string       `json:"visibleName,omitempty"`
	Size        int64        `json:"size,omitempty"`
	SyncedAt    time.Time    `json:"syncedAt,omitempty"`
}

// Manifest tracks the sync state across all managed documents.
// It provides the source of truth for what has been synced and when.
type Manifest struct {
	Version int                        `json:"version"`
	Entries map[string]ManifestEntry   `json:"entries"`
}

// NewManifest creates a new version 2 manifest with an initialized entries map.
func NewManifest() Manifest {
	return Manifest{
		Version: 2,
		Entries: make(map[string]ManifestEntry),
	}
}

// Get returns the manifest entry for the given path and true if found,
// or a zero-value entry and false if not found.
func (m *Manifest) Get(path string) (ManifestEntry, bool) {
	if m.Entries == nil {
		return ManifestEntry{}, false
	}
	entry, ok := m.Entries[path]
	return entry, ok
}

// Set adds or updates a manifest entry for the given path.
func (m *Manifest) Set(path string, entry ManifestEntry) {
	if m.Entries == nil {
		m.Entries = make(map[string]ManifestEntry)
	}
	m.Entries[path] = entry
}

// Delete removes a manifest entry for the given path.
// Returns true if the entry existed and was removed, false otherwise.
func (m *Manifest) Delete(path string) bool {
	if m.Entries == nil {
		return false
	}
	_, existed := m.Entries[path]
	if existed {
		delete(m.Entries, path)
	}
	return existed
}

// Touch updates the SyncedAt timestamp for a specific entry to the current time.
// If the entry does not exist, it is a no-op.
func (m *Manifest) Touch(path string) {
	if m.Entries == nil {
		return
	}
	entry, ok := m.Entries[path]
	if !ok {
		return
	}
	entry.SyncedAt = time.Now()
	m.Entries[path] = entry
}

// IsSynced returns true if the entry at the given path has matching
// LocalHash and DeviceHash, indicating it is in sync.
// Returns false if the entry does not exist.
func (m *Manifest) IsSynced(path string) bool {
	if m.Entries == nil {
		return false
	}
	entry, ok := m.Entries[path]
	if !ok {
		return false
	}
	return entry.LocalHash == entry.DeviceHash
}

// ============================================================================
// Sync actions
// ============================================================================

// ActionType represents what needs to happen for a document during a sync cycle.
type ActionType string

const (
	ActionNone         ActionType = "none"           // No action needed
	ActionPush         ActionType = "push"           // Local → Device
	ActionPull         ActionType = "pull"           // Device → Local
	ActionConflict     ActionType = "conflict"       // Both modified
	ActionDeleteLocal  ActionType = "delete_local"   // Delete on local
	ActionDeleteDevice ActionType = "delete_device"  // Delete on device
)

// SyncAction pairs a document path with its required action during a sync cycle.
type SyncAction struct {
	ActionType ActionType
	Path       string
	Source     Document // Where to copy from
	Dest       Document // Where to copy to (or conflict info)
}

// SyncResult aggregates all actions, conflicts, and errors from a sync operation.
type SyncResult struct {
	Actions   []SyncAction
	Conflicts []errors.ConflictError
	Errors    []error
}

// HasConflicts returns true if the sync result contains any unresolved conflicts.
func (s *SyncResult) HasConflicts() bool {
	return len(s.Conflicts) > 0
}

// HasActions returns true if the sync result contains any actions to execute.
func (s *SyncResult) HasActions() bool {
	return len(s.Actions) > 0
}
