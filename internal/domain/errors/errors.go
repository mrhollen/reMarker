// Package errors defines domain-specific error types used throughout the reMarker
// application. These errors represent business-level failures and are not tied to
// any particular infrastructure concern.
package errors

import "fmt"

// ConflictError indicates that a file exists with different content on both
// the local filesystem and the device, requiring manual resolution.
//
// Note: ConflictError lives in the document package alongside File, since it
// carries File references. Use this type for conflict detection across sync
// operations.
type ConflictError struct {
	Path string
}

// Error implements the error interface.
func (e ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s", e.Path)
}

// SyncError indicates a failure during a sync operation for a specific file path.
type SyncError struct {
	Path   string
	Reason error
}

// Error implements the error interface.
func (e SyncError) Error() string {
	if e.Reason == nil {
		return fmt.Sprintf("sync error on %s", e.Path)
	}
	return fmt.Sprintf("sync error on %s: %s", e.Path, e.Reason.Error())
}

// ManifestError indicates a failure reading, writing, or parsing the sync manifest.
type ManifestError struct {
	Reason error
}

// Error implements the error interface.
func (e ManifestError) Error() string {
	if e.Reason == nil {
		return "manifest error"
	}
	return fmt.Sprintf("manifest error: %s", e.Reason.Error())
}
