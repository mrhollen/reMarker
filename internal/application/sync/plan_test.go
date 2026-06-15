package sync

import (
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func doc(localPath, localHash, deviceHash string) document.Document {
	return document.Document{
		LocalPath:  localPath,
		LocalHash:  localHash,
		DeviceHash: deviceHash,
	}
}

func testManifest(entries map[string]document.ManifestEntry) *document.Manifest {
	return &document.Manifest{
		Version: 2,
		Entries: entries,
	}
}

func manifestEntry(localHash, deviceHash string) document.ManifestEntry {
	return document.ManifestEntry{
		LocalHash:  localHash,
		DeviceHash: deviceHash,
	}
}

// ---------------------------------------------------------------------------
// TestPlanSync — table-driven tests for planSync using Document entities
// ---------------------------------------------------------------------------

func TestPlanSync(t *testing.T) {
	tests := []struct {
		name       string
		localDocs  []document.Document
		deviceDocs []document.Document
		manifest   *document.Manifest
		want       []document.SyncAction
	}{
		{
			name:       "empty_everything",
			localDocs:  nil,
			deviceDocs: nil,
			manifest:   testManifest(nil),
			want:       nil,
		},
		{
			name: "new_local_only_push",
			localDocs: []document.Document{
				doc("report.pdf", "local-abc", ""),
			},
			deviceDocs: nil,
			manifest:   testManifest(nil),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPush,
					Path:       "report.pdf",
					Source:     doc("report.pdf", "local-abc", ""),
				},
			},
		},
		{
			name: "new_device_only_pull",
			localDocs: nil,
			deviceDocs: []document.Document{
				doc("notes.pdf", "", "device-xyz"),
			},
			manifest: testManifest(nil),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPull,
					Path:       "notes.pdf",
					Source:     doc("notes.pdf", "device-xyz", "device-xyz"),
				},
			},
		},
		{
			name: "new_both_same_hash_push",
			localDocs: []document.Document{
				doc("shared.pdf", "same-hash", ""),
			},
			deviceDocs: []document.Document{
				doc("shared.pdf", "", "same-hash"),
			},
			manifest: testManifest(nil),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPush,
					Path:       "shared.pdf",
					Source:     doc("shared.pdf", "same-hash", ""),
				},
			},
		},
		{
			name: "new_both_different_hash_conflict",
			localDocs: []document.Document{
				doc("conflict.pdf", "local-hash", ""),
			},
			deviceDocs: []document.Document{
				doc("conflict.pdf", "", "device-hash"),
			},
			manifest: testManifest(nil),
			want: []document.SyncAction{
				{
					ActionType: document.ActionConflict,
					Path:       "conflict.pdf",
					Source:     doc("conflict.pdf", "local-hash", ""),
					Dest:       doc("conflict.pdf", "device-hash", "device-hash"),
				},
			},
		},
		{
			name: "in_sync_none",
			localDocs: []document.Document{
				doc("synced.pdf", "synced-hash", ""),
			},
			deviceDocs: []document.Document{
				doc("synced.pdf", "", "synced-hash"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"synced.pdf": manifestEntry("synced-hash", "synced-hash"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionNone,
					Path:       "synced.pdf",
				},
			},
		},
		{
			name: "missing_local_delete_local",
			localDocs: nil,
			deviceDocs: []document.Document{
				doc("orphan.pdf", "", "device-hash"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"orphan.pdf": manifestEntry("local-hash", "device-hash"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionDeleteLocal,
					Path:       "orphan.pdf",
				},
			},
		},
		{
			name: "missing_device_delete_device",
			localDocs: []document.Document{
				doc("orphan.pdf", "local-hash", ""),
			},
			deviceDocs: nil,
			manifest: testManifest(map[string]document.ManifestEntry{
				"orphan.pdf": manifestEntry("local-hash", "device-hash"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionDeleteDevice,
					Path:       "orphan.pdf",
				},
			},
		},
		{
			name: "local_changed_push",
			localDocs: []document.Document{
				doc("changed.pdf", "new-local", ""),
			},
			deviceDocs: []document.Document{
				doc("changed.pdf", "", "original"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"changed.pdf": manifestEntry("original", "original"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPush,
					Path:       "changed.pdf",
					Source:     doc("changed.pdf", "new-local", ""),
				},
			},
		},
		{
			name: "device_changed_pull",
			localDocs: []document.Document{
				doc("changed.pdf", "original", ""),
			},
			deviceDocs: []document.Document{
				doc("changed.pdf", "", "new-device"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"changed.pdf": manifestEntry("original", "original"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPull,
					Path:       "changed.pdf",
					Source:     doc("changed.pdf", "new-device", "new-device"),
				},
			},
		},
		{
			name: "both_changed_conflict",
			localDocs: []document.Document{
				doc("conflict.pdf", "new-local", ""),
			},
			deviceDocs: []document.Document{
				doc("conflict.pdf", "", "new-device"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"conflict.pdf": manifestEntry("original", "original"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionConflict,
					Path:       "conflict.pdf",
					Source:     doc("conflict.pdf", "new-local", ""),
					Dest:       doc("conflict.pdf", "new-device", "new-device"),
				},
			},
		},
		{
			name: "multiple_files_mixed",
			localDocs: []document.Document{
				doc("push.pdf", "local-only", ""),
				doc("synced.pdf", "synced", ""),
				doc("conflict.pdf", "new-local", ""),
			},
			deviceDocs: []document.Document{
				doc("pull.pdf", "", "device-only"),
				doc("synced.pdf", "", "synced"),
				doc("conflict.pdf", "", "new-device"),
			},
			manifest: testManifest(map[string]document.ManifestEntry{
				"synced.pdf":   manifestEntry("synced", "synced"),
				"conflict.pdf": manifestEntry("original", "original"),
			}),
			want: []document.SyncAction{
				{
					ActionType: document.ActionPush,
					Path:       "push.pdf",
					Source:     doc("push.pdf", "local-only", ""),
				},
				{
					ActionType: document.ActionPull,
					Path:       "pull.pdf",
					Source:     doc("pull.pdf", "device-only", "device-only"),
				},
				{
					ActionType: document.ActionNone,
					Path:       "synced.pdf",
				},
				{
					ActionType: document.ActionConflict,
					Path:       "conflict.pdf",
					Source:     doc("conflict.pdf", "new-local", ""),
					Dest:       doc("conflict.pdf", "new-device", "new-device"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planSync(tt.deviceDocs, tt.localDocs, tt.manifest)

			// Sort got by Path for deterministic comparison (map iteration is unordered)
			sortByPath(got)
			sortByPath(tt.want)

			if len(got) != len(tt.want) {
				t.Fatalf("planSync() returned %d actions, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].ActionType != tt.want[i].ActionType {
					t.Errorf("actions[%d].ActionType = %q, want %q", i, got[i].ActionType, tt.want[i].ActionType)
				}
				if got[i].Path != tt.want[i].Path {
					t.Errorf("actions[%d].Path = %q, want %q", i, got[i].Path, tt.want[i].Path)
				}

				// Compare Source Document
				if got[i].Source.LocalPath != tt.want[i].Source.LocalPath {
					t.Errorf("actions[%d].Source.LocalPath = %q, want %q", i, got[i].Source.LocalPath, tt.want[i].Source.LocalPath)
				}
				if got[i].Source.LocalHash != tt.want[i].Source.LocalHash {
					t.Errorf("actions[%d].Source.LocalHash = %q, want %q", i, got[i].Source.LocalHash, tt.want[i].Source.LocalHash)
				}
				if got[i].Source.DeviceHash != tt.want[i].Source.DeviceHash {
					t.Errorf("actions[%d].Source.DeviceHash = %q, want %q", i, got[i].Source.DeviceHash, tt.want[i].Source.DeviceHash)
				}

				// Compare Dest Document
				if got[i].Dest.LocalPath != tt.want[i].Dest.LocalPath {
					t.Errorf("actions[%d].Dest.LocalPath = %q, want %q", i, got[i].Dest.LocalPath, tt.want[i].Dest.LocalPath)
				}
				if got[i].Dest.LocalHash != tt.want[i].Dest.LocalHash {
					t.Errorf("actions[%d].Dest.LocalHash = %q, want %q", i, got[i].Dest.LocalHash, tt.want[i].Dest.LocalHash)
				}
				if got[i].Dest.DeviceHash != tt.want[i].Dest.DeviceHash {
					t.Errorf("actions[%d].Dest.DeviceHash = %q, want %q", i, got[i].Dest.DeviceHash, tt.want[i].Dest.DeviceHash)
				}
			}
		})
	}
}

// sortByPath sorts actions by Path in-place for deterministic comparison.
func sortByPath(actions []document.SyncAction) {
	for i := 0; i < len(actions); i++ {
		for j := i + 1; j < len(actions); j++ {
			if actions[j].Path < actions[i].Path {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestPlanSync_EdgeCases
// ---------------------------------------------------------------------------

func TestPlanSync_EdgeCases(t *testing.T) {
	t.Run("nil_manifest", func(t *testing.T) {
		actions := planSync(nil, []document.Document{
			doc("new.pdf", "hash", ""),
		}, nil)

		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].ActionType != document.ActionPush {
			t.Errorf("expected ActionPush, got %q", actions[0].ActionType)
		}
	})

	t.Run("manifest_entry_only_no_files", func(t *testing.T) {
		// Entry in manifest but file missing from both sides — should produce
		// a delete action (the file was deleted from both sides).
		actions := planSync(nil, nil, testManifest(map[string]document.ManifestEntry{
			"gone.pdf": manifestEntry("local-hash", "device-hash"),
		}))

		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		// Missing from both: clean up stale manifest entry via delete_local
		if actions[0].ActionType != document.ActionDeleteLocal {
			t.Errorf("expected ActionDeleteLocal, got %q", actions[0].ActionType)
		}
	})

	t.Run("device_doc_in_manifest_local_not", func(t *testing.T) {
		// Device doc hash matches manifest LocalHash, local doc is new (not in manifest).
		// This exercises the planNew path.
		actions := planSync(
			[]document.Document{doc("file.pdf", "", "synced-hash")},
			[]document.Document{doc("file.pdf", "synced-hash", "")},
			testManifest(nil),
		)

		if len(actions) != 1 {
			t.Fatalf("expected 1 action, got %d", len(actions))
		}
		if actions[0].ActionType != document.ActionPush {
			t.Errorf("expected ActionPush (hashes match), got %q", actions[0].ActionType)
		}
	})
}

// ---------------------------------------------------------------------------
// TestPlanSync_DocumentFields
// ---------------------------------------------------------------------------

func TestPlanSync_PreservesDocumentFields(t *testing.T) {
	now := time.Now()

	localDoc := document.Document{
		LocalPath:  "report.pdf",
		LocalHash:  "abc123",
		DeviceHash: "",
		Size:       1024,
		ModTime:    now,
	}

	deviceDoc := document.Document{
		LocalPath:  "report.pdf",
		LocalHash:  "",
		DeviceHash: "def456",
		Size:       2048,
		ModTime:    now.Add(-time.Hour),
	}

	actions := planSync([]document.Document{deviceDoc}, []document.Document{localDoc}, testManifest(nil))

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	// Source should be the local document (push action)
	src := actions[0].Source
	if src.LocalHash != localDoc.LocalHash {
		t.Errorf("Source.LocalHash = %q, want %q", src.LocalHash, localDoc.LocalHash)
	}
	if src.Size != localDoc.Size {
		t.Errorf("Source.Size = %d, want %d", src.Size, localDoc.Size)
	}
	if !src.ModTime.Equal(localDoc.ModTime) {
		t.Errorf("Source.ModTime = %v, want %v", src.ModTime, localDoc.ModTime)
	}

	// Test conflict preserves both documents
	actions = planSync(
		[]document.Document{deviceDoc},
		[]document.Document{localDoc},
		testManifest(map[string]document.ManifestEntry{
			"report.pdf": manifestEntry("original", "original"),
		}),
	)

	if len(actions) != 1 || actions[0].ActionType != document.ActionConflict {
		t.Fatalf("expected 1 conflict action, got %d actions with type %q", len(actions), actions[0].ActionType)
	}

	if actions[0].Source.LocalHash != localDoc.LocalHash {
		t.Errorf("conflict Source.LocalHash = %q, want %q", actions[0].Source.LocalHash, localDoc.LocalHash)
	}
	if actions[0].Dest.DeviceHash != deviceDoc.DeviceHash {
		t.Errorf("conflict Dest.DeviceHash = %q, want %q", actions[0].Dest.DeviceHash, deviceDoc.DeviceHash)
	}
}
