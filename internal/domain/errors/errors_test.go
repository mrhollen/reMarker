package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestConflictError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ConflictError
		wantMsg string
	}{
		{
			name:    "basic conflict error message includes path",
			err:     NewConflictError("abc-123.metadata", nil),
			wantMsg: "conflict on abc-123.metadata",
		},
		{
			name:    "conflict error with different path",
			err:     NewConflictError("def-456.content", nil),
			wantMsg: "conflict on def-456.content",
		},
		{
			name:    "conflict error with empty path",
			err:     NewConflictError("", nil),
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
	err := NewConflictError("test-path.metadata", nil)
	if err.Path != "test-path.metadata" {
		t.Errorf("Path = %q, want %q", err.Path, "test-path.metadata")
	}
}

func TestConflictError_ImplementsError(t *testing.T) {
	var _ error = (*ConflictError)(nil)
}

func TestSyncError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *SyncError
		wantMsg string
	}{
		{
			name: "sync error with reason",
			err: NewSyncError("abc.metadata", errors.New("connection refused")),
			wantMsg: "sync error on abc.metadata: connection refused",
		},
		{
			name: "sync error with nil reason",
			err: NewSyncError("abc.metadata", nil),
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
	err := NewSyncError("test-path.content", nil)
	if err.Path != "test-path.content" {
		t.Errorf("Path = %q, want %q", err.Path, "test-path.content")
	}
}

func TestSyncError_ImplementsError(t *testing.T) {
	var _ error = (*SyncError)(nil)
}

func TestManifestError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ManifestError
		wantMsg string
	}{
		{
			name: "manifest error with reason",
			err: NewManifestError(errors.New("file not found")),
			wantMsg: "manifest error: file not found",
		},
		{
			name: "manifest error with nil reason",
			err: NewManifestError(nil),
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
	var _ error = (*ManifestError)(nil)
}

func TestManifestError_Reason(t *testing.T) {
	cause := errors.New("corrupted manifest")
	err := NewManifestError(cause)
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false, want true")
	}
}

// --- Unwrap / errors.Is / errors.As tests ---

type sentinelError struct {
	msg string
}

func (s sentinelError) Error() string { return s.msg }

func TestConflictError_Unwrap(t *testing.T) {
	tests := []struct {
		name      string
		cause     error
		wantIs    bool
		wantAs    bool
	}{
		{
			name:      "errors.Is finds wrapped cause",
			cause:     errors.New("disk full"),
			wantIs:    true,
			wantAs:    false,
		},
		{
			name:      "errors.Is with sentinel error",
			cause:     sentinelError{msg: "quota exceeded"},
			wantIs:    true,
			wantAs:    true,
		},
		{
			name:      "nil cause does not panic",
			cause:     nil,
			wantIs:    false,
			wantAs:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewConflictError("test.metadata", tc.cause)

			// Unwrap should not panic even with nil cause
			unwrapped := err.Unwrap()
			if unwrapped != tc.cause {
				t.Errorf("Unwrap() = %v, want %v", unwrapped, tc.cause)
			}

			if tc.cause != nil {
				got := errors.Is(err, tc.cause)
				if got != tc.wantIs {
					t.Errorf("errors.Is(err, cause) = %v, want %v", got, tc.wantIs)
				}

				if tc.wantAs {
					var target sentinelError
					if !errors.As(err, &target) {
						t.Error("errors.As(err, &sentinelError) = false, want true")
					}
				}
			} else {
				// With nil cause, errors.Is against any non-nil error should be false
				other := errors.New("other")
				if errors.Is(err, other) {
					t.Error("errors.Is(err, other) = true with nil cause, want false")
				}
			}
		})
	}
}

func TestSyncError_Unwrap(t *testing.T) {
	tests := []struct {
		name      string
		cause     error
		wantIs    bool
		wantAs    bool
	}{
		{
			name:      "errors.Is finds wrapped cause",
			cause:     errors.New("connection refused"),
			wantIs:    true,
			wantAs:    false,
		},
		{
			name:      "errors.Is with sentinel error",
			cause:     sentinelError{msg: "ssh auth failed"},
			wantIs:    true,
			wantAs:    true,
		},
		{
			name:      "nil cause does not panic",
			cause:     nil,
			wantIs:    false,
			wantAs:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewSyncError("test.content", tc.cause)

			// Unwrap should not panic even with nil cause
			unwrapped := err.Unwrap()
			if unwrapped != tc.cause {
				t.Errorf("Unwrap() = %v, want %v", unwrapped, tc.cause)
			}

			if tc.cause != nil {
				got := errors.Is(err, tc.cause)
				if got != tc.wantIs {
					t.Errorf("errors.Is(err, cause) = %v, want %v", got, tc.wantIs)
				}

				if tc.wantAs {
					var target sentinelError
					if !errors.As(err, &target) {
						t.Error("errors.As(err, &sentinelError) = false, want true")
					}
				}
			}
		})
	}
}

func TestManifestError_Unwrap(t *testing.T) {
	tests := []struct {
		name      string
		cause     error
		wantIs    bool
		wantAs    bool
	}{
		{
			name:      "errors.Is finds wrapped cause",
			cause:     errors.New("file not found"),
			wantIs:    true,
			wantAs:    false,
		},
		{
			name:      "errors.Is with sentinel error",
			cause:     sentinelError{msg: "json parse error"},
			wantIs:    true,
			wantAs:    true,
		},
		{
			name:      "nil cause does not panic",
			cause:     nil,
			wantIs:    false,
			wantAs:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewManifestError(tc.cause)

			// Unwrap should not panic even with nil cause
			unwrapped := err.Unwrap()
			if unwrapped != tc.cause {
				t.Errorf("Unwrap() = %v, want %v", unwrapped, tc.cause)
			}

			if tc.cause != nil {
				got := errors.Is(err, tc.cause)
				if got != tc.wantIs {
					t.Errorf("errors.Is(err, cause) = %v, want %v", got, tc.wantIs)
				}

				if tc.wantAs {
					var target sentinelError
					if !errors.As(err, &target) {
						t.Error("errors.As(err, &sentinelError) = false, want true")
					}
				}
			}
		})
	}
}

func TestConflictError_As(t *testing.T) {
	inner := sentinelError{msg: "inner conflict"}
	err := NewConflictError("test.metadata", inner)

	var target sentinelError
	if !errors.As(err, &target) {
		t.Error("errors.As(err, &sentinelError) = false, want true")
	}
	if target.msg != "inner conflict" {
		t.Errorf("target.msg = %q, want %q", target.msg, "inner conflict")
	}
}

func TestConflictError_As_NonMatching(t *testing.T) {
	err := NewConflictError("test.metadata", errors.New("plain error"))

	var target sentinelError
	if errors.As(err, &target) {
		t.Error("errors.As(err, &sentinelError) = true, want false")
	}
}

func TestSyncError_As(t *testing.T) {
	inner := sentinelError{msg: "inner sync"}
	err := NewSyncError("test.content", inner)

	var target sentinelError
	if !errors.As(err, &target) {
		t.Error("errors.As(err, &sentinelError) = false, want true")
	}
	if target.msg != "inner sync" {
		t.Errorf("target.msg = %q, want %q", target.msg, "inner sync")
	}
}

func TestManifestError_As(t *testing.T) {
	inner := sentinelError{msg: "inner manifest"}
	err := NewManifestError(inner)

	var target sentinelError
	if !errors.As(err, &target) {
		t.Error("errors.As(err, &sentinelError) = false, want true")
	}
	if target.msg != "inner manifest" {
		t.Errorf("target.msg = %q, want %q", target.msg, "inner manifest")
	}
}

func TestError_ChainedWithWrap(t *testing.T) {
	// Verify that %w wrapping still works through our error types
	cause := sentinelError{msg: "root cause"}
	err := NewSyncError("test.content", cause)
	wrapped := fmt.Errorf("operation failed: %w", err)

	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is(wrapped, cause) = false, want true")
	}

	var target sentinelError
	if !errors.As(wrapped, &target) {
		t.Error("errors.As(wrapped, &sentinelError) = false, want true")
	}
}
