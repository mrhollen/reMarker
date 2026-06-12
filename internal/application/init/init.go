// Package init contains the use case for initializing the reMarker
// local state directory and manifest.
package init

import (
	"context"
	"errors"

	"github.com/hollen/remarker/internal/domain/document"
)

// ErrAlreadyInitialized is returned when the manifest already exists
// and the user should run sync instead of init.
var ErrAlreadyInitialized = errors.New("already initialized: run sync instead")

// InitUseCase orchestrates the initialization of a new reMarker sync state.
type InitUseCase struct {
	manifestRepo document.ManifestRepository
}

// NewInitUseCase creates a new InitUseCase with the given manifest repository.
func NewInitUseCase(repo document.ManifestRepository) *InitUseCase {
	return &InitUseCase{manifestRepo: repo}
}

// Execute performs the initialization: checks that the manifest does not
// already exist, creates a fresh empty manifest, and persists it.
func (uc *InitUseCase) Execute(ctx context.Context) error {
	exists, err := uc.manifestRepo.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyInitialized
	}

	manifest := &document.Manifest{
		Version: 1,
		Entries: make(map[string]document.ManifestEntry),
	}

	return uc.manifestRepo.Save(ctx, manifest)
}
