package errors

import (
	"errors"
	"testing"
)

func TestConflictError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     ConflictError
		wantMsg string
	}{
		{
			name:    "basic conflict error message includes path",
			err:     ConflictError{Path: "abc-123.metadata"},
			wantMsg: "conflict on abc-123.metadata",
		},
		{
			name:    "conflict error with different path",
			err:     ConflictError{Path: "def-456.content"},
			wantMsg: "conflict on def-456.content",
		},
		{
			name:    "conflict error with empty path",
			err:     ConflictError{Path: ""},
			wantMsg: "conflict on ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg == "" {
				t.Fatal("Error() returned empty string")
			}
			if msg != tc.wantMsg {
				t.Errorf("Error() = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

func TestConflictError_Path(t *testing.T) {
	err := ConflictError{Path: "test-path.metadata"}
	if err.Path != "test-path.metadata" {
		t.Errorf("Path = %q, want %q", err.Path, "test-path.metadata")
	}
}

func TestConflictError_ImplementsError(t *testing.T) {
	var _ error = ConflictError{}
}

func TestSyncError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     SyncError
		wantMsg string
	}{
		{
			name: "sync error with reason",
			err: SyncError{
				Path:   "abc.metadata",
				Reason: errors.New("connection refused"),
			},
			wantMsg: "sync error on abc.metadata: connection refused",
		},
		{
			name: "sync error with nil reason",
			err: SyncError{
				Path:   "abc.metadata",
				Reason: nil,
			},
			wantMsg: "sync error on abc.metadata",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg == "" {
				t.Fatal("Error() returned empty string")
			}
			if msg != tc.wantMsg {
				t.Errorf("Error() = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

func TestSyncError_Path(t *testing.T) {
	err := SyncError{Path: "test-path.content"}
	if err.Path != "test-path.content" {
		t.Errorf("Path = %q, want %q", err.Path, "test-path.content")
	}
}

func TestSyncError_Reason(t *testing.T) {
	cause := errors.New("timeout")
	err := SyncError{Path: "x.metadata", Reason: cause}
	if err.Reason != cause {
		t.Errorf("Reason = %v, want %v", err.Reason, cause)
	}
}

func TestSyncError_ImplementsError(t *testing.T) {
	var _ error = SyncError{}
}

func TestManifestError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     ManifestError
		wantMsg string
	}{
		{
			name: "manifest error with reason",
			err: ManifestError{
				Reason: errors.New("file not found"),
			},
			wantMsg: "manifest error: file not found",
		},
		{
			name: "manifest error with nil reason",
			err: ManifestError{
				Reason: nil,
			},
			wantMsg: "manifest error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg == "" {
				t.Fatal("Error() returned empty string")
			}
			if msg != tc.wantMsg {
				t.Errorf("Error() = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

func TestManifestError_ImplementsError(t *testing.T) {
	var _ error = ManifestError{}
}

func TestManifestError_Reason(t *testing.T) {
	cause := errors.New("corrupted manifest")
	err := ManifestError{Reason: cause}
	if err.Reason != cause {
		t.Errorf("Reason = %v, want %v", err.Reason, cause)
	}
}
