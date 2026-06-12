// Package watch provides the use case for running a long-running daemon that
// monitors the local filesystem for changes and periodically syncs with the
// reMarkable device.
package watch

import (
	"context"
	"fmt"
	"time"

	appsync "github.com/hollen/remarker/internal/application/sync"
	"github.com/hollen/remarker/internal/domain/document"
)

const (
	// defaultSyncInterval is the default interval between periodic syncs.
	defaultSyncInterval = 5 * time.Minute

	// debounceDelay is the time to wait after the last filesystem event
	// before triggering a sync. This prevents sync flooding from rapid
	// successive events.
	debounceDelay = 500 * time.Millisecond
)

// Watcher monitors a directory for file system changes. The application
// layer depends on this interface; the infrastructure layer provides the
// concrete implementation.
type Watcher interface {
	Events() <-chan string
	Start(ctx context.Context) error
	Stop() error
}

// WatchUseCase orchestrates the long-running watch-and-sync loop.
// It monitors the local filesystem for changes and periodically syncs
// with the reMarkable device.
type WatchUseCase struct {
	deviceRepo   document.DeviceRepository
	localRepo    document.LocalRepository
	manifestRepo document.ManifestRepository
	watcher      Watcher
	syncInterval time.Duration
}

// NewWatchUseCase creates a new WatchUseCase with the given repositories
// and watcher. If syncInterval is zero or negative, it defaults to 5 minutes.
func NewWatchUseCase(
	deviceRepo document.DeviceRepository,
	localRepo document.LocalRepository,
	manifestRepo document.ManifestRepository,
	watcher Watcher,
	syncInterval time.Duration,
) *WatchUseCase {
	if syncInterval <= 0 {
		syncInterval = defaultSyncInterval
	}
	return &WatchUseCase{
		deviceRepo:   deviceRepo,
		localRepo:    localRepo,
		manifestRepo: manifestRepo,
		watcher:      watcher,
		syncInterval: syncInterval,
	}
}

// Run starts the watch-and-sync loop. It blocks until ctx is cancelled.
//
// The loop monitors two triggers:
//  1. A periodic ticker (syncInterval) — syncs on a fixed schedule.
//  2. Filesystem events from the watcher — syncs on change, debounced.
//
// Rapid filesystem events are debounced: if multiple events arrive within
// debounceDelay (500ms), only one sync is performed after the burst ends.
// Each sync creates a fresh SyncUseCase instance with new repository
// connections.
//
// Returns nil on clean shutdown (context cancelled), or an error if the
// watcher fails to start.
func (uc *WatchUseCase) Run(ctx context.Context) error {
	if err := uc.watcher.Start(ctx); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}
	defer uc.watcher.Stop()

	ticker := time.NewTicker(uc.syncInterval)
	defer ticker.Stop()

	debounceTimer := time.NewTimer(debounceDelay)
	defer debounceTimer.Stop()
	debounceActive := false

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			uc.doSync(ctx)

		case _, ok := <-uc.watcher.Events():
			if !ok {
				// Channel closed — watcher stopped unexpectedly.
				return nil
			}
			// Debounce: reset the timer on each event.
			if !debounceActive {
				debounceTimer.Reset(debounceDelay)
				debounceActive = true
			}

		case <-debounceTimer.C:
			debounceActive = false
			uc.doSync(ctx)
		}
	}
}

// doSync performs a single sync cycle using a fresh SyncUseCase.
func (uc *WatchUseCase) doSync(ctx context.Context) {
	syncUC := appsync.NewSyncUseCase(
		uc.deviceRepo,
		uc.localRepo,
		uc.manifestRepo,
	)
	_, _ = syncUC.Execute(ctx) // Errors are logged inside Execute.
}

// Compile-time satisfaction.
var _ interface {
	Run(context.Context) error
} = (*WatchUseCase)(nil)
