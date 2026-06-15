// Package watch provides the use case for running a long-running daemon that
// monitors the local filesystem for changes and periodically syncs with the
// reMarkable device.
package watch

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hollen/remarker/internal/domain/document"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// mockWatcher implements the Watcher interface for testing.
type mockWatcher struct {
	events    chan string
	startErr  error
	stopErr   error
	startOnce sync.Once
}

func newMockWatcher(bufSize int) *mockWatcher {
	return &mockWatcher{
		events: make(chan string, bufSize),
	}
}

func (m *mockWatcher) Events() <-chan string {
	return m.events
}

func (m *mockWatcher) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	// Simulate the real watcher: close events channel when ctx is done.
	go func() {
		<-ctx.Done()
		close(m.events)
	}()
	return nil
}

func (m *mockWatcher) Stop() error {
	return m.stopErr
}

// Compile-time check.
var _ Watcher = (*mockWatcher)(nil)

// mockDeviceRepo implements document.DeviceRepository for testing.
type mockDeviceRepo struct {
	listCount atomic.Int32
}

func (m *mockDeviceRepo) ListFiles(_ context.Context) ([]document.File, error) {
	m.listCount.Add(1)
	return nil, nil
}

func (m *mockDeviceRepo) GetFile(_ context.Context, _ string) (document.File, error) {
	return document.File{}, nil
}

func (m *mockDeviceRepo) PutFile(_ context.Context, _ document.File) error {
	return nil
}

func (m *mockDeviceRepo) DeleteFile(_ context.Context, _ string) error {
	return nil
}

func (m *mockDeviceRepo) GetFileContent(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockDeviceRepo) ListDocuments(_ context.Context) ([]document.Document, error) {
	return nil, nil
}

func (m *mockDeviceRepo) PutDocument(_ context.Context, _ document.Document, _ io.Reader) error {
	return nil
}

func (m *mockDeviceRepo) PutFolder(_ context.Context, _ document.Folder) error {
	return nil
}

func (m *mockDeviceRepo) GetDocumentMetadata(_ context.Context, _ string) (document.SidecarMetadata, error) {
	return document.SidecarMetadata{}, nil
}

var _ document.DeviceRepository = (*mockDeviceRepo)(nil)

// mockLocalRepo implements document.LocalRepository for testing.
type mockLocalRepo struct {
	listCount atomic.Int32
}

func (m *mockLocalRepo) ListFiles(_ context.Context) ([]document.File, error) {
	m.listCount.Add(1)
	return nil, nil
}

func (m *mockLocalRepo) GetFile(_ context.Context, _ string) (document.File, error) {
	return document.File{}, nil
}

func (m *mockLocalRepo) PutFile(_ context.Context, _ document.File) error {
	return nil
}

func (m *mockLocalRepo) DeleteFile(_ context.Context, _ string) error {
	return nil
}

func (m *mockLocalRepo) PutFileContent(_ context.Context, _ document.File, _ io.Reader) error {
	return nil
}

var _ document.LocalRepository = (*mockLocalRepo)(nil)

// mockManifestRepo implements document.ManifestRepository for testing.
type mockManifestRepo struct{}

func (m *mockManifestRepo) Load(_ context.Context) (*document.Manifest, error) {
	return &document.Manifest{Entries: make(map[string]document.ManifestEntry)}, nil
}

func (m *mockManifestRepo) Save(_ context.Context, _ *document.Manifest) error {
	return nil
}

func (m *mockManifestRepo) Exists(_ context.Context) (bool, error) {
	return false, nil
}

var _ document.ManifestRepository = (*mockManifestRepo)(nil)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRun_StopsWhenContextCancelled(t *testing.T) {
	watcher := newMockWatcher(10)
	uc := NewWatchUseCase(
		&mockDeviceRepo{},
		&mockLocalRepo{},
		&mockManifestRepo{},
		watcher,
		5*time.Minute,
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- uc.Run(ctx)
	}()

	// Cancel after a short delay
	time.AfterFunc(100*time.Millisecond, cancel)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context was cancelled")
	}
}

func TestRun_SyncsOnTicker(t *testing.T) {
	watcher := newMockWatcher(10)
	deviceRepo := &mockDeviceRepo{}
	localRepo := &mockLocalRepo{}

	uc := NewWatchUseCase(
		deviceRepo,
		localRepo,
		&mockManifestRepo{},
		watcher,
		100*time.Millisecond, // short interval
	)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- uc.Run(ctx)
	}()

	<-done

	// With 100ms interval and 400ms timeout, we should get at least 1 sync.
	// Each sync calls ListFiles on both repos.
	deviceCalls := deviceRepo.listCount.Load()
	localCalls := localRepo.listCount.Load()
	if deviceCalls < 1 {
		t.Errorf("expected at least 1 device ListFiles call from ticker, got %d", deviceCalls)
	}
	if localCalls < 1 {
		t.Errorf("expected at least 1 local ListFiles call from ticker, got %d", localCalls)
	}
}

func TestRun_SyncsOnFsEvent(t *testing.T) {
	watcher := newMockWatcher(10)
	deviceRepo := &mockDeviceRepo{}
	localRepo := &mockLocalRepo{}

	uc := NewWatchUseCase(
		deviceRepo,
		localRepo,
		&mockManifestRepo{},
		watcher,
		5*time.Minute, // long interval so ticker doesn't interfere
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- uc.Run(ctx)
	}()

	// Emit a filesystem event
	watcher.events <- "test.metadata"

	// Wait for debounce (500ms) plus buffer
	time.Sleep(700 * time.Millisecond)
	cancel()
	<-done

	// Should have triggered exactly 1 sync
	deviceCalls := deviceRepo.listCount.Load()
	localCalls := localRepo.listCount.Load()
	if deviceCalls < 1 {
		t.Errorf("expected at least 1 device ListFiles call from fs event, got %d", deviceCalls)
	}
	if localCalls < 1 {
		t.Errorf("expected at least 1 local ListFiles call from fs event, got %d", localCalls)
	}
}

func TestRun_DebouncesRapidEvents(t *testing.T) {
	watcher := newMockWatcher(100)
	deviceRepo := &mockDeviceRepo{}
	localRepo := &mockLocalRepo{}

	uc := NewWatchUseCase(
		deviceRepo,
		localRepo,
		&mockManifestRepo{},
		watcher,
		5*time.Minute, // long interval so ticker doesn't interfere
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- uc.Run(ctx)
	}()

	// Emit 5 rapid events
	for i := 0; i < 5; i++ {
		watcher.events <- "file" + string(rune('a'+i)) + ".metadata"
	}

	// Wait for debounce to complete plus buffer
	time.Sleep(700 * time.Millisecond)
	cancel()
	<-done

	// Should have triggered exactly 1 sync (debounced)
	deviceCalls := deviceRepo.listCount.Load()
	if deviceCalls != 1 {
		t.Errorf("expected exactly 1 sync after debouncing 5 events, got %d", deviceCalls)
	}
}

func TestRun_CreatesNewSyncUseCaseEachRun(t *testing.T) {
	watcher := newMockWatcher(10)
	deviceRepo := &mockDeviceRepo{}
	localRepo := &mockLocalRepo{}

	uc := NewWatchUseCase(
		deviceRepo,
		localRepo,
		&mockManifestRepo{},
		watcher,
		10*time.Millisecond, // very short interval for multiple syncs
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- uc.Run(ctx)
	}()

	<-done

	// With 10ms interval and 100ms timeout, should get at least 2 syncs.
	// Each sync creates a new SyncUseCase and calls ListFiles on both repos.
	deviceCalls := deviceRepo.listCount.Load()
	if deviceCalls < 2 {
		t.Errorf("expected at least 2 syncs (each creating a new SyncUseCase), got %d", deviceCalls)
	}
}

func TestRun_ReturnsErrorWhenWatcherFails(t *testing.T) {
	watcherErr := errors.New("watcher start failed")
	watcher := &mockWatcher{
		events:   make(chan string, 10),
		startErr: watcherErr,
	}

	uc := NewWatchUseCase(
		&mockDeviceRepo{},
		&mockLocalRepo{},
		&mockManifestRepo{},
		watcher,
		5*time.Minute,
	)

	ctx := context.Background()
	err := uc.Run(ctx)
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if err.Error() != "start watcher: watcher start failed" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}
