package init

import (
	"context"
	"errors"
	"testing"

	"github.com/hollen/remarker/internal/domain/document"
)

// mockManifestRepository implements document.ManifestRepository for testing.
type mockManifestRepository struct {
	exists    bool
	existsErr error
	saveErr   error
	saved     *document.Manifest
}

func (m *mockManifestRepository) Load(_ context.Context) (*document.Manifest, error) {
	return nil, nil
}

func (m *mockManifestRepository) Save(_ context.Context, manifest *document.Manifest) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = manifest
	return nil
}

func (m *mockManifestRepository) Exists(_ context.Context) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	return m.exists, nil
}

// Compile-time check: mockManifestRepository implements document.ManifestRepository.
var _ document.ManifestRepository = (*mockManifestRepository)(nil)

func TestNewInitUseCase(t *testing.T) {
	t.Run("returns use case with repository set", func(t *testing.T) {
		repo := &mockManifestRepository{}
		uc := NewInitUseCase(repo)
		if uc == nil {
			t.Fatal("NewInitUseCase() returned nil")
		}
	})
}

func TestExecute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		repo        *mockManifestRepository
		wantErr     bool
		wantErrIs   error
		validateFn  func(t *testing.T, repo *mockManifestRepository)
	}{
		{
			name: "happy path initializes manifest when not exists",
			repo: &mockManifestRepository{
				exists: false,
			},
			wantErr: false,
			validateFn: func(t *testing.T, repo *mockManifestRepository) {
				if repo.saved == nil {
					t.Fatal("manifest was not saved")
				}
				if repo.saved.Version != 2 {
					t.Errorf("saved manifest Version = %d, want 2", repo.saved.Version)
				}
				if repo.saved.Entries == nil {
					t.Error("saved manifest Entries is nil, want empty map")
				} else if len(repo.saved.Entries) != 0 {
					t.Errorf("saved manifest Entries has %d entries, want 0", len(repo.saved.Entries))
				}
			},
		},
		{
			name: "returns ErrAlreadyInitialized when manifest exists",
			repo: &mockManifestRepository{
				exists: true,
			},
			wantErr:   true,
			wantErrIs: ErrAlreadyInitialized,
		},
		{
			name: "returns error when Exists fails",
			repo: &mockManifestRepository{
				existsErr: errors.New("disk read error"),
			},
			wantErr: true,
		},
		{
			name: "returns error when Save fails",
			repo: &mockManifestRepository{
				exists:  false,
				saveErr: errors.New("write permission denied"),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewInitUseCase(tc.repo)
			err := uc.Execute(ctx)

			if tc.wantErr {
				if err == nil {
					t.Fatal("Execute() expected error, got nil")
				}
				if tc.wantErrIs != nil {
					if !errors.Is(err, tc.wantErrIs) {
						t.Errorf("Execute() error = %v, want wrapped error %v", err, tc.wantErrIs)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Execute() unexpected error: %v", err)
				}
			}

			if tc.validateFn != nil {
				tc.validateFn(t, tc.repo)
			}
		})
	}
}

func TestErrAlreadyInitialized(t *testing.T) {
	t.Run("is a non-nil error", func(t *testing.T) {
		if ErrAlreadyInitialized == nil {
			t.Fatal("ErrAlreadyInitialized is nil")
		}
	})

	t.Run("has meaningful message", func(t *testing.T) {
		msg := ErrAlreadyInitialized.Error()
		if msg == "" {
			t.Error("ErrAlreadyInitialized has empty message")
		}
	})
}
