// Package errors defines domain-specific error types used throughout the reMarker
// application. These errors represent business-level failures and are not tied to
// any particular infrastructure concern.
package errors

import (
	"fmt"
	"strings"
)

// ConflictError indicates that a file exists with different content on both
// the local filesystem and the device, requiring manual resolution.
type ConflictError struct {
	Path  string
	cause error
}

// NewConflictError creates a new ConflictError for the given path and cause.
func NewConflictError(path string, cause error) *ConflictError {
	return &ConflictError{Path: path, cause: cause}
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on %s", e.Path)
}

// Unwrap implements the unwrappable error interface, allowing errors.Is and
// errors.As to traverse through to the underlying cause.
func (e *ConflictError) Unwrap() error {
	return e.cause
}

// SyncError indicates a failure during a sync operation for a specific file path.
type SyncError struct {
	Path  string
	cause error
}

// NewSyncError creates a new SyncError for the given path and cause.
func NewSyncError(path string, cause error) *SyncError {
	return &SyncError{Path: path, cause: cause}
}

// Error implements the error interface.
func (e *SyncError) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("sync error on %s", e.Path)
	}
	return fmt.Sprintf("sync error on %s: %s", e.Path, e.cause.Error())
}

// Unwrap implements the unwrappable error interface, allowing errors.Is and
// errors.As to traverse through to the underlying cause.
func (e *SyncError) Unwrap() error {
	return e.cause
}

// ManifestError indicates a failure reading, writing, or parsing the sync manifest.
type ManifestError struct {
	cause error
}

// NewManifestError creates a new ManifestError wrapping the given cause.
func NewManifestError(cause error) *ManifestError {
	return &ManifestError{cause: cause}
}

// Error implements the error interface.
func (e *ManifestError) Error() string {
	if e.cause == nil {
		return "manifest error"
	}
	return fmt.Sprintf("manifest error: %s", e.cause.Error())
}

// Unwrap implements the unwrappable error interface, allowing errors.Is and
// errors.As to traverse through to the underlying cause.
func (e *ManifestError) Unwrap() error {
	return e.cause
}

// HasPath returns true if the error is a SyncError or ConflictError that
// references the given file path.
func HasPath(err error, path string) bool {
	switch e := err.(type) {
	case *SyncError:
		return e.Path == path
	case *ConflictError:
		return e.Path == path
	default:
		return strings.Contains(err.Error(), path)
	}
}
