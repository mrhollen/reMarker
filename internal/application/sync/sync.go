// Package sync contains the use case for synchronizing files between the
// local filesystem and the reMarkable device.
package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	deviceDocs, err := uc.deviceRepo.ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list device documents: %w", err)
	}

	localDocs, err := uc.localRepo.ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local documents: %w", err)
	}

	// Phase 2: Plan actions
	actions := planSync(deviceDocs, localDocs, manifest)

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
	for _, action := range result.Actions {
		manifest.Touch(action.Path)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
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
	entry, exists := manifest.Get(action.Path)
	var docID uuid.UUID
	if exists && entry.DeviceUUID != "" {
		var err error
		docID, err = uuid.Parse(entry.DeviceUUID)
		if err != nil {
			docID = uuid.New()
		}
	} else {
		docID = uuid.New()
	}

	doc := action.Source
	doc.ID = docID

	content, err := uc.localRepo.GetFileContent(ctx, action.Path)
	if err != nil {
		return fmt.Errorf("read local file: %w", err)
	}
	defer content.Close()

	if err := uc.deviceRepo.PutDocument(ctx, doc, content); err != nil {
		return fmt.Errorf("push to device: %w", err)
	}

	manifest.Set(action.Path, document.ManifestEntry{
		DeviceUUID:  docID.String(),
		DeviceType:  doc.Type,
		LocalHash:   action.Source.LocalHash,
		DeviceHash:  action.Source.LocalHash,
		VisibleName: doc.VisibleName,
		Size:        action.Source.Size,
		SyncedAt:    time.Now(),
	})
	return nil
}

// pullFile transfers a file from device to local and updates the manifest.
func (uc *SyncUseCase) pullFile(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	// action.Source is the device document with DeviceUUID from ListDocuments()
	doc := action.Source

	// Check context before reading file content
	if err := ctx.Err(); err != nil {
		return err
	}
	// Get file content from device using device path
	content, err := uc.deviceRepo.GetFileContent(ctx, action.Source.ToDevicePath())
	if err != nil {
		return fmt.Errorf("get file content from device: %w", err)
	}
	defer content.Close()

	// Write to local filesystem
	localFile := document.File{
		Path:    action.Path,
		Hash:    doc.LocalHash,
		ModTime: doc.ModTime,
		Size:    doc.Size,
	}
	if err := uc.localRepo.PutFileContent(ctx, localFile, content); err != nil {
		return fmt.Errorf("pull to local: %w", err)
	}

	// Update manifest with full device metadata
	manifest.Set(action.Path, document.ManifestEntry{
		DeviceUUID:  doc.ID.String(),
		DeviceType:  doc.Type,
		LocalHash:   doc.LocalHash,
		DeviceHash:  doc.LocalHash,
		VisibleName: doc.VisibleName,
		Size:        doc.Size,
		SyncedAt:    time.Now(),
	})
	return nil
}

// resolveConflict handles a file that has been modified on both sides.
// The newer file wins; the loser is preserved with a .conflict suffix on
// both sides.
//
// Content-aware transfers: action.Source is the local document, action.Dest is
// the device document. Device-side writes use PutDocument (with sidecar files).
// Local-side writes use PutFile/PutFileContent (no sidecars on local FS).
// When content originates from device, we pull via GetFileContent and push
// back via PutDocument.
//
// Ordering matters: we must pull device content before the device file
// gets overwritten by a subsequent push.
func (uc *SyncUseCase) resolveConflict(ctx context.Context, action document.SyncAction, manifest *document.Manifest) error {
	// Determine winner and loser based on modification time
	var winnerDoc, loserDoc document.Document
	winnerIsLocal := false
	loserIsLocal := false

	if action.Source.IsNewer(action.Dest) {
		winnerDoc = action.Source
		loserDoc = action.Dest
		winnerIsLocal = true
		loserIsLocal = false
	} else {
		winnerDoc = action.Dest
		loserDoc = action.Source
		winnerIsLocal = false
		loserIsLocal = true
	}

	conflictPath := action.Path + ".conflict"

	// --- Create conflict copy of loser on both sides ---
	if loserIsLocal {
		// Loser content is in local filesystem
		loserContent, err := uc.localRepo.GetFileContent(ctx, action.Path)
		if err != nil {
			return fmt.Errorf("get loser content from local: %w", err)
		}
		defer loserContent.Close()
		conflictDoc := document.Document{
			ID:          uuid.New(),
			LocalPath:   conflictPath,
			Type:        loserDoc.Type,
			VisibleName: loserDoc.VisibleName,
			Size:        loserDoc.Size,
			ModTime:     loserDoc.ModTime,
		}
		if err := uc.deviceRepo.PutDocument(ctx, conflictDoc, loserContent); err != nil {
			return fmt.Errorf("create conflict copy on device: %w", err)
		}

		if err := uc.localRepo.PutFile(ctx, document.File{
			Path:    conflictPath,
			Size:    loserDoc.Size,
			ModTime: loserDoc.ModTime,
			Hash:    loserDoc.LocalHash,
		}); err != nil {
			return fmt.Errorf("create conflict copy on local: %w", err)
		}
	} else {
		// Loser content is on device — must pull via GetFileContent
		loserContent, err := uc.deviceRepo.GetFileContent(ctx, loserDoc.ToDevicePath())
		if err != nil {
			return fmt.Errorf("get loser content from device: %w", err)
		}
		defer loserContent.Close()

		if err := uc.localRepo.PutFileContent(ctx, document.File{
			Path:    conflictPath,
			Size:    loserDoc.Size,
			ModTime: loserDoc.ModTime,
			Hash:    loserDoc.LocalHash,
		}, loserContent); err != nil {
			return fmt.Errorf("create conflict copy on local: %w", err)
		}

		// Now local has the conflict content — push to device as Document
		conflictContent, err := uc.localRepo.GetFileContent(ctx, conflictPath)
		if err != nil {
			return fmt.Errorf("get conflict content from local: %w", err)
		}
		defer conflictContent.Close()
		conflictDoc := document.Document{
			ID:          uuid.New(),
			LocalPath:   conflictPath,
			Type:        loserDoc.Type,
			VisibleName: loserDoc.VisibleName,
			Size:        loserDoc.Size,
			ModTime:     loserDoc.ModTime,
		}
		if err := uc.deviceRepo.PutDocument(ctx, conflictDoc, conflictContent); err != nil {
			return fmt.Errorf("create conflict copy on device: %w", err)
		}
	}

	// --- Push winner to both sides ---
	if winnerIsLocal {
		// Winner content is in local filesystem
		winnerContent, err := uc.localRepo.GetFileContent(ctx, action.Path)
		if err != nil {
			return fmt.Errorf("get winner content from local: %w", err)
		}
		defer winnerContent.Close()
		if err := uc.deviceRepo.PutDocument(ctx, winnerDoc, winnerContent); err != nil {
			return fmt.Errorf("push winner to device: %w", err)
		}

		if err := uc.localRepo.PutFile(ctx, document.File{
			Path:    winnerDoc.LocalPath,
			Hash:    winnerDoc.LocalHash,
			ModTime: winnerDoc.ModTime,
			Size:    winnerDoc.Size,
		}); err != nil {
			return fmt.Errorf("push winner to local: %w", err)
		}
	} else {
		// Winner content is on device — must pull via GetFileContent
		winnerContent, err := uc.deviceRepo.GetFileContent(ctx, winnerDoc.ToDevicePath())
		if err != nil {
			return fmt.Errorf("get winner content from device: %w", err)
		}
		defer winnerContent.Close()

		if err := uc.localRepo.PutFileContent(ctx, document.File{
			Path:    winnerDoc.LocalPath,
			Hash:    winnerDoc.LocalHash,
			ModTime: winnerDoc.ModTime,
			Size:    winnerDoc.Size,
		}, winnerContent); err != nil {
			return fmt.Errorf("pull winner to local: %w", err)
		}

		// Now local has the winner content — push to device as fresh Document
		winnerContent, err = uc.localRepo.GetFileContent(ctx, action.Path)
		if err != nil {
			return fmt.Errorf("get winner content from local: %w", err)
		}
		defer winnerContent.Close()
		if err := uc.deviceRepo.PutDocument(ctx, winnerDoc, winnerContent); err != nil {
			return fmt.Errorf("push winner to device: %w", err)
		}
	}

	// Update manifest with winner
	manifest.Set(action.Path, document.ManifestEntry{
		DeviceUUID:  winnerDoc.ID.String(),
		DeviceType:  winnerDoc.Type,
		LocalHash:   winnerDoc.LocalHash,
		DeviceHash:  winnerDoc.LocalHash,
		VisibleName: winnerDoc.VisibleName,
		Size:        winnerDoc.Size,
		SyncedAt:    time.Now(),
	})

	return nil
}



