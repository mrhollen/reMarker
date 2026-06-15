package sync

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// ---------------------------------------------------------------------------
// pullFile content transfer tests
// ---------------------------------------------------------------------------

func TestPullFile_TransfersContent(t *testing.T) {
	// pullFile should download file content from device and write to local
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	fileContent := "this is the actual file content from the device"

	var receivedContent string
	localFiles := make(map[string]document.File)
	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", int64(len(fileContent)), now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			receivedContent = string(data)
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Verify pull action happened
	found := false
	for _, a := range result.Actions {
		if a.Path == filePath && a.ActionType == document.ActionPull {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected pull action for device-only file")
	}

	// Verify content was actually transferred
	if receivedContent != fileContent {
		t.Errorf("content mismatch: got %q, want %q", receivedContent, fileContent)
	}

	// Verify file metadata stored on local
	localFile, ok := localRepo.files[filePath]
	if !ok {
		t.Fatal("file not stored on local after pull")
	}
	if localFile.Hash != "devicehash" {
		t.Errorf("local file hash = %q, want %q", localFile.Hash, "devicehash")
	}

	// Verify manifest was updated with correct fields
	if len(manifestRepo.savedManifest.Entries) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(manifestRepo.savedManifest.Entries))
	}
	entry := manifestRepo.savedManifest.Entries[filePath]
	if entry.LocalHash != "devicehash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "devicehash")
	}
	if entry.DeviceHash != "devicehash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "devicehash")
	}
	if entry.Size != int64(len(fileContent)) {
		t.Errorf("manifest size = %d, want %d", entry.Size, int64(len(fileContent)))
	}
}

func TestPullFile_DeviceGetContentFails(t *testing.T) {
	// When device GetFileContent fails, pull should report error and not abort sync
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	getContentErr := errors.New("SFTP read failed")

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", 200, now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return nil, getContentErr
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	// Should have a non-fatal error for the file
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed pull, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, filePath) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for %s, got errors: %v", filePath, result.Errors)
	}
}

func TestPullFile_LocalPutContentFails(t *testing.T) {
	// When local PutFileContent fails, pull should report error
	now := time.Now()
	ctx := context.Background()
	filePath := "note.content"
	fileContent := "file content"
	putContentErr := errors.New("disk full")

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "devicehash", int64(len(fileContent)), now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			return putContentErr
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed local write, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, filePath) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for %s, got errors: %v", filePath, result.Errors)
	}
}

func TestPullFile_UpdatesManifestWithSourceFields(t *testing.T) {
	// Manifest entry should use action.Source fields (device file info)
	now := time.Now()
	ctx := context.Background()
	filePath := "doc.content"
	fileContent := "content"

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			filePath: file(filePath, "dev-hash", 7, now),
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fileContent)), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	_, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	entry, ok := manifestRepo.savedManifest.Entries[filePath]
	if !ok {
		t.Fatal("manifest entry not created for pulled file")
	}
	if entry.LocalHash != "dev-hash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "dev-hash")
	}
	if entry.DeviceHash != "dev-hash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "dev-hash")
	}
	if entry.Size != 7 {
		t.Errorf("manifest size = %d, want 7", entry.Size)
	}
}

// ---------------------------------------------------------------------------
// resolveConflict content transfer tests
// ---------------------------------------------------------------------------

func TestResolveConflict_LocalWinner_LocalLoser(t *testing.T) {
	// Both sides modified, local is newer. Loser is device.
	// Expected: conflict copy of device file pulled to local,
	// winner (local) pushed to device.
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local winner content"
	deviceContent := "device loser content"

	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now)
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now.Add(-time.Hour))

	var conflictContentReceived string
	var winnerPushedToDevice bool

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			if f.Path == "doc.metadata.conflict" {
				// Conflict copy pushed to device
			} else if f.Path == "doc.metadata" {
				winnerPushedToDevice = true
			}
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			if f.Path == "doc.metadata.conflict" {
				data, err := io.ReadAll(r)
				if err != nil {
					return err
				}
				conflictContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (local) should be pushed to device
	if !winnerPushedToDevice {
		t.Error("winner (local file) should be pushed to device")
	}

	// Conflict copy of device loser should be pulled to local with content
	if conflictContentReceived != deviceContent {
		t.Errorf("conflict content = %q, want %q", conflictContentReceived, deviceContent)
	}

	// Conflict copy should exist on device
	if _, ok := deviceRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on device")
	}

	// Manifest should have winner's hash
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "localhash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "localhash")
	}
	if entry.DeviceHash != "localhash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "localhash")
	}
}

func TestResolveConflict_DeviceWinner_LocalLoser(t *testing.T) {
	// Both sides modified, device is newer. Loser is local.
	// Expected: conflict copy of local file pushed to device,
	// winner (device) pulled to local with content, then pushed to device.
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local loser content"
	deviceContent := "device winner content"

	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now.Add(-time.Hour))
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now)

	var winnerContentReceived string
	var conflictPushedToDevice bool

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			if f.Path == "doc.metadata.conflict" {
				conflictPushedToDevice = true
			} else if f.Path == "doc.metadata" {
				// Winner pushed to device
			}
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			if f.Path == "doc.metadata" {
				data, err := io.ReadAll(r)
				if err != nil {
					return err
				}
				winnerContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (device) content should be pulled to local
	if winnerContentReceived != deviceContent {
		t.Errorf("winner content = %q, want %q", winnerContentReceived, deviceContent)
	}

	// Conflict copy of local loser should be pushed to device
	if !conflictPushedToDevice {
		t.Error("conflict copy (local loser) should be pushed to device")
	}

	// Conflict copy should exist on local
	if _, ok := localRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on local")
	}

	// Manifest should have winner's hash (device)
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "devicehash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "devicehash")
	}
	if entry.DeviceHash != "devicehash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "devicehash")
	}
}

func TestResolveConflict_DeviceWinner_DeviceLoser(t *testing.T) {
	// Both sides modified, local is newer. Loser is device.
	// Expected: conflict copy of device loser pulled to local with content,
	// winner (local) pushed to device via PutFile (reads local content).
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)

	localContent := "local winner content"
	deviceContent := "device loser content"

	// Source=local (newer), Dest=device (older). Local wins, device loses.
	localFile := file("doc.metadata", "localhash", int64(len(localContent)), now)
	deviceFile := file("doc.metadata", "devicehash", int64(len(deviceContent)), now.Add(-time.Hour))

	var conflictContentReceived string
	getContentCalls := 0

	deviceFiles := map[string]document.File{
		"doc.metadata": deviceFile,
	}
	localFiles := map[string]document.File{
		"doc.metadata": localFile,
	}

	deviceRepo := &mockDeviceRepository{
		files: deviceFiles,
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			getContentCalls++
			return io.NopCloser(strings.NewReader(deviceContent)), nil
		},
		putFileCall: func(_ context.Context, f document.File) error {
			deviceFiles[f.Path] = f
			return nil
		},
	}
	localRepo := &mockLocalRepository{
		files: localFiles,
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			data, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			if f.Path == "doc.metadata.conflict" {
				conflictContentReceived = string(data)
			}
			localFiles[f.Path] = f
			return nil
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !result.HasConflicts() {
		t.Fatal("expected conflict, got none")
	}

	// Winner (local) content should be pushed to device via PutFile (reads from local)
	// No PutFileContent call for winner since it's local-originated

	// Conflict copy content should be device loser content
	if conflictContentReceived != deviceContent {
		t.Errorf("conflict content = %q, want %q", conflictContentReceived, deviceContent)
	}

	// GetFileContent should be called (for device loser)
	if getContentCalls == 0 {
		t.Error("expected GetFileContent to be called for device loser")
	}

	// Conflict copy should exist on device
	if _, ok := deviceRepo.files["doc.metadata.conflict"]; !ok {
		t.Error("conflict copy should exist on device")
	}

	// Manifest should have winner's hash (local)
	entry, ok := manifestRepo.savedManifest.Entries["doc.metadata"]
	if !ok {
		t.Fatal("manifest entry not created for resolved conflict")
	}
	if entry.LocalHash != "localhash" {
		t.Errorf("manifest localHash = %q, want %q", entry.LocalHash, "localhash")
	}
	if entry.DeviceHash != "localhash" {
		t.Errorf("manifest deviceHash = %q, want %q", entry.DeviceHash, "localhash")
	}
}

func TestResolveConflict_LocalWinner_DeviceLoser_DeviceGetContentFails(t *testing.T) {
	// Device loser content pull fails => error reported
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)
	getContentErr := errors.New("SFTP read failed")

	localFile := file("doc.metadata", "localhash", 100, now)
	deviceFile := file("doc.metadata", "devicehash", 100, now.Add(-time.Hour))

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			"doc.metadata": deviceFile,
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return nil, getContentErr
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"doc.metadata": localFile,
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	// Should have a non-fatal error
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed conflict resolution, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "doc.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for doc.metadata, got errors: %v", result.Errors)
	}
}

func TestResolveConflict_DeviceWinner_LocalPutContentFails(t *testing.T) {
	// Pulling device winner to local fails => error reported
	now := time.Now()
	ctx := context.Background()
	manifestHash := "originalhash"
	manifestTime := now.Add(-2 * time.Hour)
	putContentErr := errors.New("disk full")

	localFile := file("doc.metadata", "localhash", 100, now.Add(-time.Hour))
	deviceFile := file("doc.metadata", "devicehash", 100, now)

	deviceRepo := &mockDeviceRepository{
		files: map[string]document.File{
			"doc.metadata": deviceFile,
		},
		getContent: func(_ context.Context, path string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("device content")), nil
		},
	}
	localRepo := &mockLocalRepository{
		files: map[string]document.File{
			"doc.metadata": localFile,
		},
		putFileContentCall: func(_ context.Context, f document.File, r io.Reader) error {
			return putContentErr
		},
	}
	manifestRepo := &mockManifestRepository{
		manifest: manifest(1, map[string]document.ManifestEntry{
			"doc.metadata": entry("doc.metadata", manifestHash, 100, manifestTime, manifestTime),
		}),
	}

	uc := NewSyncUseCase(deviceRepo, localRepo, manifestRepo, nil)
	result, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected fatal error: %v", err)
	}

	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors for failed local write, got none")
	}

	found := false
	for _, e := range result.Errors {
		if domainErrors.HasPath(e, "doc.metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for doc.metadata, got errors: %v", result.Errors)
	}
}
