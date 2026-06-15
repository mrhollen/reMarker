// Package sync contains the use case for synchronizing files between the
// local filesystem and the reMarkable device.
package sync

import (
	"github.com/hollen/remarker/internal/domain/document"
)

// planSync compares the documents on the local filesystem, the device, and the
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
func planSync(deviceDocs, localDocs []document.Document, manifest *document.Manifest) []document.SyncAction {
	// Index documents by local path for O(1) lookups
	deviceMap := indexDocuments(deviceDocs)
	localMap := indexDocuments(localDocs)

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
func planOne(path string, deviceMap, localMap map[string]document.Document, manifest *document.Manifest) *document.SyncAction {
	deviceDoc, onDevice := deviceMap[path]
	localDoc, onLocal := localMap[path]

	var manifestEntry document.ManifestEntry
	var inManifest bool
	if manifest != nil {
		manifestEntry, inManifest = manifest.Get(path)
	}

	// --- Not in manifest: new document(s) ---
	if !inManifest {
		return planNew(path, localDoc, onLocal, deviceDoc, onDevice)
	}

	// --- In manifest: existing document ---
	return planExisting(path, localDoc, onLocal, deviceDoc, onDevice, manifestEntry)
}

// planNew handles documents that are not yet tracked in the manifest.
func planNew(path string, localDoc document.Document, onLocal bool, deviceDoc document.Document, onDevice bool) *document.SyncAction {
	switch {
	case onLocal && onDevice:
		// Document exists on both sides but not in manifest.
		// If hashes match, just push (either side is fine).
		// If hashes differ, treat as conflict.
		if localDoc.LocalHash == deviceDoc.DeviceHash {
			return &document.SyncAction{
				ActionType: document.ActionPush,
				Path:       path,
				Source:     localDoc,
			}
		}
		return &document.SyncAction{
			ActionType: document.ActionConflict,
			Path:       path,
			Source:     localDoc,
			Dest:       withLocalHash(deviceDoc),
		}
	case onLocal && !onDevice:
		return &document.SyncAction{
			ActionType: document.ActionPush,
			Path:       path,
			Source:     localDoc,
		}
	case !onLocal && onDevice:
		return &document.SyncAction{
			ActionType: document.ActionPull,
			Path:       path,
			Source:     withLocalHash(deviceDoc),
		}
	default:
		// Not on either side and not in manifest — shouldn't happen, skip
		return nil
	}
}

// planExisting handles documents that are already tracked in the manifest.
func planExisting(path string, localDoc document.Document, onLocal bool, deviceDoc document.Document, onDevice bool, manifestEntry document.ManifestEntry) *document.SyncAction {
	// Determine if each side matches the manifest
	localMatches := onLocal && localDoc.LocalHash == manifestEntry.LocalHash
	deviceMatches := onDevice && deviceDoc.DeviceHash == manifestEntry.LocalHash

	switch {
	case localMatches && deviceMatches:
		// Both match manifest — nothing to do
		return &document.SyncAction{
			ActionType: document.ActionNone,
			Path:       path,
		}
	case !onLocal && !onDevice:
		// Missing from both sides — deleted everywhere, clean up manifest
		return &document.SyncAction{
			ActionType: document.ActionDeleteDevice,
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
			Source:     localDoc,
		}
	case localMatches && !deviceMatches:
		// Device changed, local matches manifest → pull
		return &document.SyncAction{
			ActionType: document.ActionPull,
			Path:       path,
			Source:     withLocalHash(deviceDoc),
		}
	default:
		// Both sides differ from manifest → conflict
		return &document.SyncAction{
			ActionType: document.ActionConflict,
			Path:       path,
			Source:     localDoc,
			Dest:       withLocalHash(deviceDoc),
		}
	}
}

// withLocalHash ensures a document has LocalHash set for use as a SyncAction
// source. sync.go reads action.Source.LocalHash for manifest updates and
// documentToFile conversions. For device-originated documents, LocalHash may
// be empty while DeviceHash carries the actual hash.
func withLocalHash(d document.Document) document.Document {
	if d.LocalHash == "" && d.DeviceHash != "" {
		d.LocalHash = d.DeviceHash
	}
	return d
}

// indexDocuments builds a map from local path to document for O(1) lookups.
func indexDocuments(docs []document.Document) map[string]document.Document {
	result := make(map[string]document.Document, len(docs))
	for _, d := range docs {
		result[d.LocalPath] = d
	}
	return result
}

// uniquePaths collects all unique paths across the device, local, and manifest.
func uniquePaths(deviceMap, localMap map[string]document.Document, manifest *document.Manifest) []string {
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
