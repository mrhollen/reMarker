// Package sync contains the use case for synchronizing files between the
// local filesystem and the reMarkable device.
package sync

import (
	"context"
	"fmt"

	"github.com/hollen/remarker/internal/domain/document"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// ProgressFunc is called after each sync action completes, providing
// the current action number (1-indexed), total number of actions,
// the action type string, and the file path.
// It may be nil if the caller does not need progress updates.
type ProgressFunc func(current, total int, action, path string)

// SyncUseCase orchestrates a full sync cycle: planning, execution,
// and manifest persistence.
type SyncUseCase struct {
	deviceRepo   document.DeviceRepository
	localRepo    document.LocalRepository
	manifestRepo document.ManifestRepository
	onProgress   ProgressFunc
}

// NewSyncUseCase creates a new SyncUseCase with the given repositories.
// The onProgress callback is optional and may be nil.
func NewSyncUseCase(
	deviceRepo document.DeviceRepository,
	localRepo document.LocalRepository,
	manifestRepo document.ManifestRepository,
	onProgress ProgressFunc,
) *SyncUseCase {
	return &SyncUseCase{
		deviceRepo:   deviceRepo,
		localRepo:    localRepo,
		manifestRepo: manifestRepo,
		onProgress:   onProgress,
	}
}

// Execute performs a complete sync cycle:
//
//  1. Load the manifest
//  2. List files on both sides
//  3. Plan actions by comparing local, device, and manifest
//  4. Execute actions (push, pull, conflict resolution, deletion handling)
//  5. Update and save the manifest
//
// Individual file transfer failures are collected in result.Errors and do not
// abort the entire sync. The manifest is always saved even if some transfers
// fail, so subsequent syncs can pick up where this one left off.
func (uc *SyncUseCase) Execute(ctx context.Context) (*document.SyncResult, error) {
	// Check context before starting any work
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Phase 0: Load manifest
	manifest, err := uc.manifestRepo.Load(ctx)
	if err != nil {
		return nil, domainErrors.NewManifestError(err)
	}

	// Phase 1: Gather state from both sides
	deviceFiles, err := uc.deviceRepo.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list device files: %w", err)
	}

	localFiles, err := uc.localRepo.ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local files: %w", err)
	}

	// Phase 2: Plan actions
	actions := planSync(deviceFiles, localFiles, manifest)

	// Phase 3: Execute actions
	result := &document.SyncResult{}
	for i, action := range actions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		transferErr := uc.executeAction(ctx, action, manifest)

		// Report progress after each action
		if uc.onProgress != nil {
			uc.onProgress(i+1, len(actions), string(action.ActionType), action.Path)
		}
		if transferErr != nil {
	result.Errors = append(result.Errors, domainErrors.NewSyncError(action.Path, transferErr))
			continue
		}

		switch action.ActionType {
		case document.ActionConflict:
			result.Conflicts = append(result.Conflicts, domainErrors.ConflictError{
				Path: action.Path,
			})
		default:
			result.Actions = append(result.Actions, action)
		}
	}

	// Phase 4: Update manifest and save
	manifest.Touch()
	if err := uc.manifestRepo.Save(ctx, manifest); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("save manifest: %w", err))
	}

	return result, nil
}

// executeAction performs a single sync action and updates the manifest
// accordingly. Returns an error if the action fails.
func (uc *SyncUseCase) executeAction(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	switch action.ActionType {
	case document.ActionPush:
		return uc.pushFile(ctx, action, manifest)
	case document.ActionPull:
		return uc.pullFile(ctx, action, manifest)
	case document.ActionConflict:
		return uc.resolveConflict(ctx, action, manifest)
	case document.ActionNone:
		return nil
	case document.ActionDeleteLocal:
		// Deletion policy: ignore deletions, only remove from manifest
		manifest.Delete(action.Path)
		return nil
	case document.ActionDeleteDevice:
		// Deletion policy: ignore deletions, only remove from manifest
		manifest.Delete(action.Path)
		return nil
	default:
		return fmt.Errorf("unknown action type: %s", action.ActionType)
	}
}

// pushFile transfers a file from local to device and updates the manifest.
func (uc *SyncUseCase) pushFile(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	if err := uc.deviceRepo.PutFile(ctx, action.Source); err != nil {
		return fmt.Errorf("push to device: %w", err)
	}
	manifest.Set(action.Path, document.ManifestEntry{
		Path:     action.Path,
		Hash:     action.Source.Hash,
		Size:     action.Source.Size,
		ModTime:  action.Source.ModTime,
		SyncedAt: action.Source.ModTime,
	})
	return nil
}

// pullFile transfers a file from device to local and updates the manifest.
func (uc *SyncUseCase) pullFile(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	content, err := uc.deviceRepo.GetFileContent(ctx, action.Path)
	if err != nil {
		return fmt.Errorf("get file content from device: %w", err)
	}
	defer content.Close()

	if err := uc.localRepo.PutFileContent(ctx, action.Source, content); err != nil {
		return fmt.Errorf("pull to local: %w", err)
	}
	manifest.Set(action.Path, document.ManifestEntry{
		Path:     action.Path,
		Hash:     action.Source.Hash,
		Size:     action.Source.Size,
		ModTime:  action.Source.ModTime,
		SyncedAt: action.Source.ModTime,
	})
	return nil
}

// resolveConflict handles a file that has been modified on both sides.
// The newer file wins; the loser is preserved with a .conflict suffix on
// both sides.
//
// Content-aware transfers: action.Source is the local file, action.Dest is
// the device file. When content originates from device, we must use
// GetFileContent + PutFileContent to transfer actual bytes. When content
// originates from local, PutFile reads from the local filesystem.
//
// Ordering matters: we must pull device content before the device file
// gets overwritten by a subsequent push.
func (uc *SyncUseCase) resolveConflict(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	// Determine winner and loser based on modification time
	var winner, loser document.File
	winnerIsLocal := false
	loserIsLocal := false

	if action.Source.IsNewer(action.Dest) {
		winner = action.Source
		loser = action.Dest
		winnerIsLocal = true
		loserIsLocal = false
	} else {
		winner = action.Dest
		loser = action.Source
		winnerIsLocal = false
		loserIsLocal = true
	}

	conflictPath := action.Path + ".conflict"
	conflictFile := document.File{
		Path:    conflictPath,
		Size:    loser.Size,
		ModTime: loser.ModTime,
		Hash:    loser.Hash,
	}

	// --- Create conflict copy of loser on both sides ---
	if loserIsLocal {
		// Loser content is in local filesystem — PutFile reads it correctly
		if err := uc.deviceRepo.PutFile(ctx, conflictFile); err != nil {
			return fmt.Errorf("create conflict copy on device: %w", err)
		}
		if err := uc.localRepo.PutFile(ctx, conflictFile); err != nil {
			return fmt.Errorf("create conflict copy on local: %w", err)
		}
	} else {
		// Loser content is on device — must pull via GetFileContent
		loserContent, err := uc.deviceRepo.GetFileContent(ctx, action.Path)
		if err != nil {
			return fmt.Errorf("get loser content from device: %w", err)
		}

		if err := uc.localRepo.PutFileContent(ctx, conflictFile, loserContent); err != nil {
			loserContent.Close()
			return fmt.Errorf("create conflict copy on local: %w", err)
		}
		loserContent.Close()

		// Now local has the conflict content — push to device
		if err := uc.deviceRepo.PutFile(ctx, conflictFile); err != nil {
			return fmt.Errorf("create conflict copy on device: %w", err)
		}
	}

	// --- Push winner to both sides ---
	if winnerIsLocal {
		// Winner content is in local filesystem — PutFile reads it correctly
		if err := uc.deviceRepo.PutFile(ctx, winner); err != nil {
			return fmt.Errorf("push winner to device: %w", err)
		}
		if err := uc.localRepo.PutFile(ctx, winner); err != nil {
			return fmt.Errorf("push winner to local: %w", err)
		}
	} else {
		// Winner content is on device — must pull via GetFileContent
		winnerContent, err := uc.deviceRepo.GetFileContent(ctx, action.Path)
		if err != nil {
			return fmt.Errorf("get winner content from device: %w", err)
		}

		if err := uc.localRepo.PutFileContent(ctx, winner, winnerContent); err != nil {
			winnerContent.Close()
			return fmt.Errorf("pull winner to local: %w", err)
		}
		winnerContent.Close()

		// Now local has the winner content — push to device
		if err := uc.deviceRepo.PutFile(ctx, winner); err != nil {
			return fmt.Errorf("push winner to device: %w", err)
		}
	}

	// Update manifest with winner
	manifest.Set(action.Path, document.ManifestEntry{
		Path:     action.Path,
		Hash:     winner.Hash,
		Size:     winner.Size,
		ModTime:  winner.ModTime,
		SyncedAt: winner.ModTime,
	})

	return nil
}
