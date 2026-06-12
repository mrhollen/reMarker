// Package watcher monitors a directory for file system changes using fsnotify
// and emits relative file paths on Create, Write, Remove, and Rename events.
// It filters out .git/ and .remarker/ paths and deduplicates rapid events.
package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// defaultBufSize is the default buffer size for the events channel.
	defaultBufSize = 100

	// dedupWindow is the time window within which duplicate events for the
	// same path are suppressed.
	dedupWindow = 1 * time.Second
)

// Watcher monitors a directory for file system changes and emits relative
// file paths on Create, Write, Remove, and Rename events.
type Watcher struct {
	baseDir string
	events  chan string
	watcher *fsnotify.Watcher
	running bool
	mu      sync.Mutex
}

// New creates a new Watcher for the given base directory. The bufSize
// parameter controls the buffer size for the events channel; values <= 0
// default to 100. The watcher does not start monitoring until Start is called.
func New(baseDir string, bufSize int) *Watcher {
	if bufSize <= 0 {
		bufSize = defaultBufSize
	}
	return &Watcher{
		baseDir: baseDir,
		events:  make(chan string, bufSize),
	}
}

// Start begins monitoring the base directory for file system changes. It
// creates an fsnotify watcher, adds the base directory and all existing
// subdirectories, and starts a goroutine that processes events. The goroutine
// exits when ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := fsw.Add(w.baseDir); err != nil {
		fsw.Close()
		return err
	}

	// Recursively add all existing subdirectories
	if err := w.addExistingDirs(fsw, w.baseDir); err != nil {
		fsw.Close()
		return err
	}

	w.watcher = fsw
	w.running = true

	go w.monitor(ctx)

	return nil
}

// addExistingDirs walks the directory tree and adds all subdirectories to the
// fsnotify watcher.
func (w *Watcher) addExistingDirs(fsw *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return fsw.Add(path)
		}
		return nil
	})
}

// monitor reads events from the fsnotify watcher and sends filtered,
// deduplicated relative paths to the events channel. It exits when ctx
// is cancelled.
func (w *Watcher) monitor(ctx context.Context) {
	// recent tracks the last time we emitted an event for a given path.
	recent := make(map[string]time.Time)
	// recentMu protects the recent map.
	var recentMu sync.Mutex

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		close(w.events)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process Create, Write, Remove, Rename
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			relPath, err := filepath.Rel(w.baseDir, event.Name)
			if err != nil {
				continue
			}
			relPath = filepath.ToSlash(relPath)

			// Skip .git and .remarker paths
			if shouldSkip(relPath) {
				continue
			}

			// Handle directory creation — add to watcher recursively
			info, statErr := os.Stat(event.Name)
			if statErr == nil && info.IsDir() {
				_ = w.watcher.Add(event.Name)
				// Recursively add any subdirectories that already exist
				_ = filepath.Walk(event.Name, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if info.IsDir() && path != event.Name {
						_ = w.watcher.Add(path)
					}
					return nil
				})
				continue
			}

			// For Remove events, the file may no longer exist — that's fine,
			// we still want to emit the event.
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				// Deduplicate
				recentMu.Lock()
				lastTime, exists := recent[relPath]
				now := time.Now()
				if exists && now.Sub(lastTime) < dedupWindow {
					recentMu.Unlock()
					continue
				}
				recent[relPath] = now
				recentMu.Unlock()

				select {
				case w.events <- relPath:
				default:
				}
				continue
			}

			// Skip if we can't stat (file gone, etc.)
			if statErr != nil {
				continue
			}

			// Deduplicate: skip if we sent this path within the dedup window
			recentMu.Lock()
			lastTime, exists := recent[relPath]
			now := time.Now()
			if exists && now.Sub(lastTime) < dedupWindow {
				recentMu.Unlock()
				continue
			}
			recent[relPath] = now
			recentMu.Unlock()

			// Non-blocking send to avoid blocking if receiver is slow
			select {
			case w.events <- relPath:
			default:
				// Channel full, drop the event
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
			continue
		}
	}
}

// Stop closes the fsnotify watcher and stops monitoring. It is safe to call
// multiple times.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.running = false

	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

// Events returns a receive-only channel of relative file paths. Each value
// represents a file that was created, modified, removed, or renamed within
// the watched directory. Paths under .git/ and .remarker/ are filtered out.
func (w *Watcher) Events() <-chan string {
	return w.events
}

// shouldSkip returns true if the relative path is under a .git or .remarker
// directory and should be filtered out.
func shouldSkip(relPath string) bool {
	// Normalize separators to forward slashes for consistent matching
	normalized := filepath.ToSlash(relPath)

	// Check if path starts with .git/ or .remarker/
	if strings.HasPrefix(normalized, ".git/") || normalized == ".git" {
		return true
	}
	if strings.HasPrefix(normalized, ".remarker/") || normalized == ".remarker" {
		return true
	}

	// Check if path contains /.git/ or /.remarker/
	if strings.Contains(normalized, "/.git/") || strings.Contains(normalized, "/.git") {
		return true
	}
	if strings.Contains(normalized, "/.remarker/") || strings.Contains(normalized, "/.remarker") {
		return true
	}

	return false
}
