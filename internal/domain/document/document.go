// Package document defines the core domain entities for reMarkable document
// synchronization. This is the innermost layer and depends only on the Go
// standard library.
package document

import (
	"github.com/hollen/remarker/internal/domain/errors"
	"time"
)

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

// ManifestEntry represents a single file entry tracked in the sync manifest.
type ManifestEntry struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	SyncedAt time.Time `json:"syncedAt"`
}

// Manifest tracks the sync state across all managed files.
// It provides the source of truth for what has been synced and when.
type Manifest struct {
	Version int                      `json:"version"`
	LastSync *time.Time              `json:"lastSync,omitempty"`
	Entries map[string]ManifestEntry `json:"entries"`
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

// Touch sets LastSync to the current time, marking the manifest as freshly
// synced.
func (m *Manifest) Touch() {
	now := time.Now()
	m.LastSync = &now
}

// ActionType represents what needs to happen for a file during a sync cycle.
type ActionType string

const (
	ActionNone         ActionType = "none"           // No action needed
	ActionPush         ActionType = "push"           // Local → Device
	ActionPull         ActionType = "pull"           // Device → Local
	ActionConflict     ActionType = "conflict"       // Both modified
	ActionDeleteLocal  ActionType = "delete_local"   // Delete on local
	ActionDeleteDevice ActionType = "delete_device"  // Delete on device
)

// SyncAction pairs a file path with its required action during a sync cycle.
type SyncAction struct {
	ActionType ActionType
	Path       string
	Source     File // Where to copy from
	Dest       File // Where to copy to (or conflict info)
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
