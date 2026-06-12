// Package status provides a dry-run preview of what changes a sync would make,
// without actually applying any changes.
package status

import (
	"context"
	"fmt"

	"github.com/hollen/remarker/internal/domain/document"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// StatusResult holds the planned sync actions without executing them.
type StatusResult struct {
	ToDevice  []document.SyncAction
	ToLocal   []document.SyncAction
	Conflicts []domainErrors.ConflictError
	Manifest  *document.Manifest
	Errors    []error
}

// HasChanges returns true if there are any pending sync actions or conflicts.
func (r *StatusResult) HasChanges() bool {
	return len(r.ToDevice) > 0 || len(r.ToLocal) > 0 || len(r.Conflicts) > 0
}

// Summary returns a human-readable summary of the planned sync changes.
func (r *StatusResult) Summary() string {
	if !r.HasChanges() {
		return "Already in sync.\n"
	}
	return fmt.Sprintf("Would sync:\n  → device: %d files\n  ← local: %d files\n  conflicts: %d\n",
		len(r.ToDevice), len(r.ToLocal), len(r.Conflicts))
}

// StatusUseCase orchestrates the status (dry-run) operation.
type StatusUseCase struct {
	deviceRepo   document.DeviceRepository
	localRepo    document.LocalRepository
	manifestRepo document.ManifestRepository
}

// NewStatusUseCase creates a new StatusUseCase with the given repositories.
func NewStatusUseCase(
	deviceRepo document.DeviceRepository,
	localRepo document.LocalRepository,
	manifestRepo document.ManifestRepository,
) *StatusUseCase {
	return &StatusUseCase{
		deviceRepo:   deviceRepo,
		localRepo:    localRepo,
		manifestRepo: manifestRepo,
	}
}

// Execute performs a dry-run sync analysis and returns the planned actions
// without applying any changes or saving the manifest.
func (uc *StatusUseCase) Execute(ctx context.Context) (*StatusResult, error) {
	// Load manifest
	manifest, err := uc.manifestRepo.Load(ctx)
	if err != nil {
		return nil, domainErrors.ManifestError{Reason: err}
	}

	// List device files
	deviceFiles, err := uc.deviceRepo.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list device files: %w", err)
	}

	// List local files
	localFiles, err := uc.localRepo.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local files: %w", err)
	}

	// Build maps for efficient lookup
	deviceMap := make(map[string]document.File, len(deviceFiles))
	for _, f := range deviceFiles {
		deviceMap[f.Path] = f
	}

	localMap := make(map[string]document.File, len(localFiles))
	for _, f := range localFiles {
		localMap[f.Path] = f
	}

	var toDevice []document.SyncAction
	var toLocal []document.SyncAction
	var conflicts []domainErrors.ConflictError

	// Collect all unique paths
	allPaths := make(map[string]bool)
	for path := range deviceMap {
		allPaths[path] = true
	}
	for path := range localMap {
		allPaths[path] = true
	}

	for path := range allPaths {
		onDevice, hasDevice := deviceMap[path]
		onLocal, hasLocal := localMap[path]
		entry, inManifest := manifest.Get(path)

		if !hasDevice && !hasLocal {
			// File in neither location — skip
			continue
		}

		if hasDevice && !hasLocal {
			// File only on device — pull to local (unless already in manifest)
			if !inManifest || entry.Hash != onDevice.Hash {
				toLocal = append(toLocal, document.SyncAction{
					ActionType: document.ActionPull,
					Path:       path,
					Source:     onDevice,
				})
			}
			continue
		}

		if !hasDevice && hasLocal {
			// File only on local — push to device (unless already in manifest)
			if !inManifest || entry.Hash != onLocal.Hash {
				toDevice = append(toDevice, document.SyncAction{
					ActionType: document.ActionPush,
					Path:       path,
					Source:     onLocal,
				})
			}
			continue
		}

		// File exists on both sides
		if onDevice.Hash == onLocal.Hash {
			// Already in sync
			continue
		}

		// Different hashes — conflict
		conflicts = append(conflicts, domainErrors.ConflictError{Path: path})
	}

	return &StatusResult{
		ToDevice:  toDevice,
		ToLocal:   toLocal,
		Conflicts: conflicts,
		Manifest:  manifest,
	}, nil
}
