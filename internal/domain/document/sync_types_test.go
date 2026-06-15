package document

import (
	"testing"
	"time"

	"github.com/google/uuid"
	domainErrors "github.com/hollen/remarker/internal/domain/errors"
)

// ============================================================================
// SyncAction tests (with Document types)
// ============================================================================

func TestSyncAction_DocumentTypes(t *testing.T) {
	now := time.Now()
	sourceDoc := Document{
		ID:          uuid.New(),
		LocalPath:   "documents/test.pdf",
		Type:        DocumentTypePDF,
		VisibleName: "test.pdf",
		LocalHash:   "local_hash",
		DeviceHash:  "",
		Size:        1024,
		ModTime:     now,
	}

	destDoc := Document{
		ID:          uuid.New(),
		LocalPath:   "documents/test.pdf",
		Type:        DocumentTypePDF,
		VisibleName: "test.pdf",
		LocalHash:   "",
		DeviceHash:  "device_hash",
		Size:        1024,
		ModTime:     now.Add(-time.Hour),
	}

	tests := []struct {
		name       string
		actionType ActionType
	}{
		{
			name:       "push action with documents",
			actionType: ActionPush,
		},
		{
			name:       "pull action with documents",
			actionType: ActionPull,
		},
		{
			name:       "conflict action with documents",
			actionType: ActionConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sa := SyncAction{
				ActionType: tc.actionType,
				Path:       "documents/test.pdf",
				Source:     sourceDoc,
				Dest:       destDoc,
			}

			if sa.ActionType != tc.actionType {
				t.Errorf("ActionType = %v, want %v", sa.ActionType, tc.actionType)
			}
			if sa.Path != "documents/test.pdf" {
				t.Errorf("Path = %q, want %q", sa.Path, "documents/test.pdf")
			}
			if sa.Source.LocalHash != "local_hash" {
				t.Errorf("Source.LocalHash = %q, want %q", sa.Source.LocalHash, "local_hash")
			}
			if sa.Dest.DeviceHash != "device_hash" {
				t.Errorf("Dest.DeviceHash = %q, want %q", sa.Dest.DeviceHash, "device_hash")
			}
			if sa.Source.Type != DocumentTypePDF {
				t.Errorf("Source.Type = %q, want %q", sa.Source.Type, DocumentTypePDF)
			}
		})
	}
}

func TestSyncAction_ZeroValue(t *testing.T) {
	sa := SyncAction{}
	if sa.ActionType != "" {
		t.Errorf("zero value ActionType = %q, want empty", sa.ActionType)
	}
	if sa.Path != "" {
		t.Errorf("zero value Path = %q, want empty", sa.Path)
	}
	// Source and Dest should be zero-value Documents
	if sa.Source.ID != uuid.Nil {
		t.Error("zero value Source.ID should be nil UUID")
	}
	if sa.Dest.ID != uuid.Nil {
		t.Error("zero value Dest.ID should be nil UUID")
	}
}

// ============================================================================
// SyncResult tests
// ============================================================================

func TestSyncResult_HasConflicts(t *testing.T) {
	tests := []struct {
		name     string
		result   SyncResult
		wantTrue bool
	}{
		{
			name: "empty result has no conflicts",
			result: SyncResult{
				Actions:   []SyncAction{},
				Conflicts: []domainErrors.ConflictError{},
				Errors:    []error{},
			},
			wantTrue: false,
		},
		{
			name: "nil slices has no conflicts",
			result: SyncResult{
				Actions:   nil,
				Conflicts: nil,
				Errors:    nil,
			},
			wantTrue: false,
		},
		{
			name: "one conflict returns true",
			result: SyncResult{
				Conflicts: []domainErrors.ConflictError{{Path: "documents/test.pdf"}},
			},
			wantTrue: true,
		},
		{
			name: "multiple conflicts returns true",
			result: SyncResult{
				Conflicts: []domainErrors.ConflictError{
					{Path: "documents/a.pdf"},
					{Path: "documents/b.pdf"},
				},
			},
			wantTrue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.result.HasConflicts()
			if got != tc.wantTrue {
				t.Errorf("HasConflicts() = %v, want %v", got, tc.wantTrue)
			}
		})
	}
}

func TestSyncResult_HasActions(t *testing.T) {
	tests := []struct {
		name     string
		result   SyncResult
		wantTrue bool
	}{
		{
			name: "empty result has no actions",
			result: SyncResult{
				Actions: []SyncAction{},
			},
			wantTrue: false,
		},
		{
			name: "nil actions has no actions",
			result: SyncResult{
				Actions: nil,
			},
			wantTrue: false,
		},
		{
			name: "one action returns true",
			result: SyncResult{
				Actions: []SyncAction{{
					ActionType: ActionPush,
					Path:       "documents/test.pdf",
					Source:     Document{Type: DocumentTypePDF},
					Dest:       Document{Type: DocumentTypePDF},
				}},
			},
			wantTrue: true,
		},
		{
			name: "multiple actions returns true",
			result: SyncResult{
				Actions: []SyncAction{
					{ActionType: ActionPush, Path: "documents/a.pdf", Source: Document{Type: DocumentTypePDF}, Dest: Document{Type: DocumentTypePDF}},
					{ActionType: ActionPull, Path: "documents/b.notebook", Source: Document{Type: DocumentTypeNotebook}, Dest: Document{Type: DocumentTypeNotebook}},
				},
			},
			wantTrue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.result.HasActions()
			if got != tc.wantTrue {
				t.Errorf("HasActions() = %v, want %v", got, tc.wantTrue)
			}
		})
	}
}

func TestActionType_Values(t *testing.T) {
	// Verify all action type constants have expected string values
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionNone, "none"},
		{ActionPush, "push"},
		{ActionPull, "pull"},
		{ActionConflict, "conflict"},
		{ActionDeleteLocal, "delete_local"},
		{ActionDeleteDevice, "delete_device"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.action) != tc.want {
				t.Errorf("ActionType = %q, want %q", tc.action, tc.want)
			}
		})
	}
}
