// Package sync contains the use case for synchronizing files between the
// local filesystem and the reMarkable device.
package sync

import (
	"github.com/hollen/remarker/internal/domain/document"
)

// planSync compares the files on the local filesystem, the device, and the
// manifest to produce a list of actions that need to be taken to bring all
// three into agreement.
//
// The algorithm works as follows for each unique path:
//
//   - Not in manifest, only on local → push (local → device)
//   - Not in manifest, only on device → pull (device → local)
//   - Not in manifest, on both → compare hashes; if equal push, else conflict
//   - In manifest, missing from local, present on device → delete_local (ignored at execution)
//   - In manifest, present on local, missing from device → delete_device (ignored at execution)
//   - In manifest, both present, local differs, device matches → push
//   - In manifest, both present, device differs, local matches → pull
//   - In manifest, both present, both differ → conflict
//   - In manifest, both present, both match → none
func planSync(deviceFiles, localFiles []document.File, manifest *document.Manifest) []document.SyncAction {
	// Index files by path for O(1) lookups
	deviceMap := indexFiles(deviceFiles)
	localMap := indexFiles(localFiles)

	// Collect all unique paths
	paths := uniquePaths(deviceMap, localMap, manifest)

	var actions []document.SyncAction
	for _, path := range paths {
		action := planOne(path, deviceMap, localMap, manifest)
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions
}

// planOne determines the action for a single file path.
func planOne(path string, deviceMap, localMap map[string]document.File, manifest *document.Manifest) *document.SyncAction {
	deviceFile, onDevice := deviceMap[path]
	localFile, onLocal := localMap[path]
	manifestEntry, inManifest := manifest.Get(path)

	// --- Not in manifest: new file(s) ---
	if !inManifest {
		return planNew(path, localFile, onLocal, deviceFile, onDevice)
	}

	// --- In manifest: existing file ---
	return planExisting(path, localFile, onLocal, deviceFile, onDevice, manifestEntry)
}

// planNew handles files that are not yet tracked in the manifest.
func planNew(path string, localFile document.File, onLocal bool, deviceFile document.File, onDevice bool) *document.SyncAction {
	switch {
	case onLocal && onDevice:
		// File exists on both sides but not in manifest.
		// If hashes match, just push (either side is fine).
		// If hashes differ, treat as conflict.
		if localFile.Hash == deviceFile.Hash {
			return &document.SyncAction{
				ActionType: document.ActionPush,
				Path:       path,
				Source:     localFile,
			}
		}
		return &document.SyncAction{
			ActionType: document.ActionConflict,
			Path:       path,
			Source:     localFile,
			Dest:       deviceFile,
		}
	case onLocal && !onDevice:
		return &document.SyncAction{
			ActionType: document.ActionPush,
			Path:       path,
			Source:     localFile,
		}
	case !onLocal && onDevice:
		return &document.SyncAction{
			ActionType: document.ActionPull,
			Path:       path,
			Source:     deviceFile,
		}
	default:
		// Not on either side and not in manifest — shouldn't happen, skip
		return nil
	}
}

// planExisting handles files that are already tracked in the manifest.
func planExisting(path string, localFile document.File, onLocal bool, deviceFile document.File, onDevice bool, manifestEntry document.ManifestEntry) *document.SyncAction {
	// Determine if each side matches the manifest
	localMatches := onLocal && localFile.Hash == manifestEntry.Hash
	deviceMatches := onDevice && deviceFile.Hash == manifestEntry.Hash

	switch {
	case localMatches && deviceMatches:
		// Both match manifest — nothing to do
		return &document.SyncAction{
			ActionType: document.ActionNone,
			Path:       path,
		}
	case !onLocal && onDevice:
		// Missing from local, present on device → deletion on local side
		return &document.SyncAction{
			ActionType: document.ActionDeleteLocal,
			Path:       path,
		}
	case onLocal && !onDevice:
		// Present on local, missing from device → deletion on device side
		return &document.SyncAction{
			ActionType: document.ActionDeleteDevice,
			Path:       path,
		}
	case !localMatches && deviceMatches:
		// Local changed, device matches manifest → push
		return &document.SyncAction{
			ActionType: document.ActionPush,
			Path:       path,
			Source:     localFile,
		}
	case localMatches && !deviceMatches:
		// Device changed, local matches manifest → pull
		return &document.SyncAction{
			ActionType: document.ActionPull,
			Path:       path,
			Source:     deviceFile,
		}
	default:
		// Both sides differ from manifest → conflict
		return &document.SyncAction{
			ActionType: document.ActionConflict,
			Path:       path,
			Source:     localFile,
			Dest:       deviceFile,
		}
	}
}

// indexFiles builds a map from path to file for O(1) lookups.
func indexFiles(files []document.File) map[string]document.File {
	result := make(map[string]document.File, len(files))
	for _, f := range files {
		result[f.Path] = f
	}
	return result
}

// uniquePaths collects all unique paths across the device, local, and manifest.
func uniquePaths(deviceMap, localMap map[string]document.File, manifest *document.Manifest) []string {
	seen := make(map[string]bool)
	var paths []string

	for path := range deviceMap {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for path := range localMap {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	if manifest != nil {
		for path := range manifest.Entries {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}

	return paths
}
