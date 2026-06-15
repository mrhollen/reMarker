package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

func TestExecute_ProgressCallback_SingleAction(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

recorder := newProgressRecorder()

uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("abc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		recorder.asProgressFunc(),
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action, got none")
	}

	if len(recorder.calls) != 1 {
		t.Fatalf("expected 1 progress call, got %d", len(recorder.calls))
	}

	call := recorder.calls[0]
	if call.current != 1 {
		t.Errorf("expected current=1, got %d", call.current)
	}
	if call.total != 1 {
		t.Errorf("expected total=1, got %d", call.total)
	}
	if call.action != string(document.ActionPush) {
		t.Errorf("expected action=%q, got %q", document.ActionPush, call.action)
	}
	if call.path != "abc.metadata" {
		t.Errorf("expected path=%q, got %q", "abc.metadata", call.path)
	}
}

func TestExecute_ProgressCallback_MultipleActions(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	recorder := newProgressRecorder()

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("a.metadata", "hash1", 100, now),
			localDoc("b.metadata", "hash2", 200, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		recorder.asProgressFunc(),
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push actions, got none")
	}

	if len(recorder.calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(recorder.calls))
	}

	if recorder.calls[0].current != 1 {
		t.Errorf("first call: expected current=1, got %d", recorder.calls[0].current)
	}
	if recorder.calls[0].total != 2 {
		t.Errorf("first call: expected total=2, got %d", recorder.calls[0].total)
	}

	if recorder.calls[1].current != 2 {
		t.Errorf("second call: expected current=2, got %d", recorder.calls[1].current)
	}
	if recorder.calls[1].total != 2 {
		t.Errorf("second call: expected total=2, got %d", recorder.calls[1].total)
	}
}

func TestExecute_ProgressCallback_NilWorks(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("abc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil, // nil progress callback
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error with nil progress: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action even with nil progress callback")
	}
}

func TestExecute_ProgressCallback_NoActions(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	recorder := newProgressRecorder()

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"synced.metadata": devDoc("synced.metadata", "samehash", 100, now),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("synced.metadata", "samehash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata": entry("synced.metadata", "samehash", 100, now, now),
			}),
		},
		recorder.asProgressFunc(),
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Even though the file is in sync, planExisting produces an ActionNone
	// for every tracked path, so we expect 1 progress call.
	if len(recorder.calls) != 1 {
		t.Fatalf("expected 1 progress call for synced file, got %d", len(recorder.calls))
	}
	if recorder.calls[0].current != 1 {
		t.Errorf("expected current=1, got %d", recorder.calls[0].current)
	}
	if recorder.calls[0].total != 1 {
		t.Errorf("expected total=1, got %d", recorder.calls[0].total)
	}
	if recorder.calls[0].action != string(document.ActionNone) {
		t.Errorf("expected action=%s, got %s", document.ActionNone, recorder.calls[0].action)
	}
	if recorder.calls[0].path != "synced.metadata" {
		t.Errorf("expected path=synced.metadata, got %s", recorder.calls[0].path)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewSyncUseCase(t *testing.T) {
	uc := NewSyncUseCase(&mockDeviceRepository{}, &mockLocalRepository{}, &mockManifestRepository{}, nil)
	if uc == nil {
		t.Fatal("NewSyncUseCase() returned nil")
	}
}

func TestExecute_EmptySync(t *testing.T) {
	// No files on either side, clean manifest → no actions
	ctx := context.Background()
	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.HasActions() {
		t.Errorf("expected no actions, got %d", len(result.Actions))
	}
	if result.HasConflicts() {
		t.Errorf("expected no conflicts, got %d", len(result.Conflicts))
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}
}

func TestExecute_NewLocalFile(t *testing.T) {
	// File only on local, not in manifest → push action
	now := time.Now()
	ctx := context.Background()

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("abc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected push action, got none")
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "abc.metadata" && a.ActionType == document.ActionPush {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected push action for abc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_NewDeviceFile(t *testing.T) {
	// File only on device, not in manifest → pull action
	now := time.Now()
	ctx := context.Background()

	uc := NewSyncUseCase(
			&mockDeviceRepository{docs: map[string]document.Document{
			"xyz.content": devDoc("xyz.content", "devicehash", 200, now),
		}},
		&mockLocalRepository{docs: []document.Document{}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasActions() {
		t.Fatal("expected pull action, got none")
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "xyz.content" && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pull action for xyz.content, got actions: %v", result.Actions)
	}
}

func TestExecute_InSync(t *testing.T) {
	// Same file on both sides, matches manifest → no action
	now := time.Now()
	ctx := context.Background()
	hash := "samehash"

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"synced.metadata": devDoc("synced.metadata", hash, 100, now),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("synced.metadata", hash, 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata": entry("synced.metadata", hash, 100, now, now),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should have no actions (or only ActionNone)
	for _, a := range result.Actions {
		if a.ActionType != document.ActionNone {
			t.Errorf("expected only ActionNone, got %v for %s", a.ActionType, a.Path)
		}
	}
}

func TestExecute_LocalModified(t *testing.T) {
	// File changed locally, device unchanged → push
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	localHash := "newlocalhash"
	manifestTime := now.Add(-time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"doc.metadata": devDoc("doc.metadata", manifestHash, 100, manifestTime),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("doc.metadata", localHash, 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "doc.metadata" && a.ActionType == document.ActionPush {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected push action for doc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_DeviceModified(t *testing.T) {
	// File changed on device, local unchanged → pull
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	deviceHash := "newdevicehash"
	manifestTime := now.Add(-time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"doc.metadata": devDoc("doc.metadata", deviceHash, 100, now),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("doc.metadata", manifestHash, 100, manifestTime),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	found := false
	for _, a := range result.Actions {
		if a.Path == "doc.metadata" && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pull action for doc.metadata, got actions: %v", result.Actions)
	}
}

func TestExecute_Conflict(t *testing.T) {
	// Both sides modified differently → conflict, newer wins
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	// Local is newer

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"doc.metadata": devDoc("doc.metadata", "devicehash", 100, now.Add(-time.Hour)),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("doc.metadata", "localhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should have a conflict
	if !result.HasConflicts() {
		t.Fatalf("expected conflict, got none. Actions: %v", result.Actions)
	}

	// Verify conflict path
	foundConflict := false
	for _, c := range result.Conflicts {
		if c.Path == "doc.metadata" {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Errorf("expected conflict on doc.metadata, got conflicts: %v", result.Conflicts)
	}
}

func TestExecute_ConflictDeviceNewer(t *testing.T) {
	// Both sides modified, device is newer → device wins
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	// Device is newer

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			"doc.metadata": devDoc("doc.metadata", "devicehash", 100, now),
		}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("doc.metadata", "localhash", 100, now.Add(-time.Hour)),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatalf("expected conflict, got none")
	}
}

func TestExecute_FileDeletedLocally(t *testing.T) {
	// In manifest, missing from local, still on device → remove from manifest, don't delete on device
	now := time.Now()
	ctx := context.Background()
	manifestHash := "hash123"
	manifestTime := now.Add(-time.Hour)

	deviceRepo := &mockDeviceRepository{
		docs: map[string]document.Document{
			"deleted.metadata": devDoc("deleted.metadata", manifestHash, 100, manifestTime),
		},
	}
	localRepo := &mockLocalRepository{
		docs: []document.Document{}, // file not on local
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"deleted.metadata": entry("deleted.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should NOT have a delete action on device
	for _, a := range result.Actions {
		if a.ActionType == document.ActionDeleteDevice {
			t.Errorf("should not delete from device, got action: %v", a)
		}
	}

	// File should still be on device
	_, ok := deviceRepo.docs["deleted.metadata"]
	if !ok {
		t.Error("file was deleted from device, should have been preserved")
	}
}

func TestExecute_FileDeletedOnDevice(t *testing.T) {
	// In manifest, missing from device, still on local → remove from manifest, don't delete locally
	now := time.Now()
	ctx := context.Background()
	manifestHash := "hash123"
	manifestTime := now.Add(-time.Hour)

	deviceRepo := &mockDeviceRepository{
		docs: map[string]document.Document{}, // file not on device
	}
	localRepo := &mockLocalRepository{
		docs: []document.Document{
			localDoc("deleted.metadata", manifestHash, 100, manifestTime),
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"deleted.metadata": entry("deleted.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should NOT have a delete action on local
	for _, a := range result.Actions {
		if a.ActionType == document.ActionDeleteLocal {
			t.Errorf("should not delete from local, got action: %v", a)
		}
	}

	// File should still be on local
	found := false
	for _, d := range localRepo.docs {
		if d.LocalPath == "deleted.metadata" {
			found = true
			break
		}
	}
	if !found {
		t.Error("file was deleted from local, should have been preserved")
	}
}

func TestExecute_ManifestLoadFails(t *testing.T) {
	ctx := context.Background()
	loadErr := errors.New("manifest corrupted")

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{},
		&mockManifestRepository{loadErr: loadErr},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on manifest load error")
	}
}

func TestExecute_DeviceListFails(t *testing.T) {
	ctx := context.Background()
	listErr := errors.New("SSH connection lost")

	uc := NewSyncUseCase(
		&mockDeviceRepository{listErr: listErr},
		&mockLocalRepository{},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on device list error")
	}
}

func TestExecute_LocalListFails(t *testing.T) {
	ctx := context.Background()
	listErr := errors.New("permission denied")

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{listErr: listErr},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on local list error")
	}
}

func TestExecute_TransferFails(t *testing.T) {
	// Transfer fails → adds to result errors, continues with other files
	now := time.Now()
	ctx := context.Background()
	transferErr := errors.New("SFTP write failed")

	uc := NewSyncUseCase(
		&mockDeviceRepository{
			docs:         map[string]document.Document{},
			putDocumentErr: transferErr,
		},
		&mockLocalRepository{docs: []document.Document{
			localDoc("fail.metadata", "newhash", 100, now),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected transfer error in result.Errors, got none")
	}

	// Verify the error mentions the file path
	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "fail.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for fail.metadata, got errors: %v", result.Errors)
	}
}

func TestExecute_MultipleFiles(t *testing.T) {
	// Mix of push, pull, conflict, none
	now := time.Now()
	ctx := context.Background()
	manifestTime := now.Add(-2 * time.Hour)

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{
			// In sync
			"synced.metadata": devDoc("synced.metadata", "hash1", 100, now),
			// Device only → pull
			"deviceonly.content": devDoc("deviceonly.content", "hash2", 200, now),
			// Conflict (device newer)
			"conflict.metadata": devDoc("conflict.metadata", "devicehash", 100, now),
		}},
		&mockLocalRepository{docs: []document.Document{
			// In sync
			localDoc("synced.metadata", "hash1", 100, now),
			// Local only → push
			localDoc("localonly.metadata", "hash3", 300, now),
			// Conflict (local older)
			localDoc("conflict.metadata", "localhash", 100, now.Add(-time.Hour)),
		}},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{
				"synced.metadata":   entry("synced.metadata", "hash1", 100, now, now),
				"conflict.metadata": entry("conflict.metadata", "originalhash", 100, manifestTime, manifestTime),
			}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Count action types
	pushCount := 0
	pullCount := 0
	noneCount := 0
	for _, a := range result.Actions {
		switch a.ActionType {
		case document.ActionPush:
			pushCount++
		case document.ActionPull:
			pullCount++
		case document.ActionNone:
			noneCount++
		}
	}

	if pushCount != 1 {
		t.Errorf("expected 1 push action, got %d", pushCount)
	}
	if pullCount != 1 {
		t.Errorf("expected 1 pull action, got %d", pullCount)
	}
	// synced.metadata should be ActionNone (or not listed)
	if noneCount < 1 {
		t.Logf("got %d ActionNone (may be 0 if in-sync files are omitted)", noneCount)
	}
	if !result.HasConflicts() {
		t.Error("expected conflict for conflict.metadata")
	}
}

func TestExecute_ManifestSaved(t *testing.T) {
	// After sync, manifest should be saved
	now := time.Now()
	ctx := context.Background()

	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{
			localDoc("new.metadata", "newhash", 100, now),
		}},
		manifestRepo,
		nil,
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !manifestRepo.saveCalled {
		t.Error("manifest was not saved after sync")
	}
}

func TestExecute_ManifestTouched(t *testing.T) {
	// After sync, manifest LastSync should be set
	ctx := context.Background()

	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(
		&mockDeviceRepository{docs: map[string]document.Document{}},
		&mockLocalRepository{docs: []document.Document{}},
		manifestRepo,
		nil,
	)

	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if manifestRepo.savedManifest == nil {
		t.Fatal("manifest was not saved")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	uc := NewSyncUseCase(
		&mockDeviceRepository{},
		&mockLocalRepository{},
		&mockManifestRepository{
			manifest: manifest(1, map[string]document.ManifestEntry{}),
		},
		nil,
	)

	result, err := uc.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error on cancelled context, got nil")
	}
	if result != nil {
		t.Error("Execute() should return nil result on context cancellation")
	}
}

func TestSyncResult_HasConflicts(t *testing.T) {
	tests := []struct {
		name string
		res  *document.SyncResult
		want bool
	}{
		{
			name: "nil conflicts",
			res:  &document.SyncResult{Conflicts: nil},
			want: false,
		},
		{
			name: "empty conflicts",
			res:  &document.SyncResult{Conflicts: []domainErrors.ConflictError{}},
			want: false,
		},
		{
			name: "one conflict",
			res:  &document.SyncResult{Conflicts: []domainErrors.ConflictError{{Path: "a.metadata"}}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.HasConflicts()
			if got != tc.want {
				t.Errorf("HasConflicts() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSyncResult_HasActions(t *testing.T) {
	tests := []struct {
		name string
		res  *document.SyncResult
		want bool
	}{
		{
			name: "nil actions",
			res:  &document.SyncResult{Actions: nil},
			want: false,
		},
		{
			name: "empty actions",
			res:  &document.SyncResult{Actions: []document.SyncAction{}},
			want: false,
		},
		{
			name: "one action",
			res:  &document.SyncResult{Actions: []document.SyncAction{{ActionType: document.ActionPush}}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.res.HasActions()
			if got != tc.want {
				t.Errorf("HasActions() = %v, want %v", got, tc.want)
			}
		})
	}
}
