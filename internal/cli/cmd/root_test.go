package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	initpkg "github.com/hollen/remarker/internal/application/init"
)

func TestRootCommand(t *testing.T) {
	t.Run("name is remarker", func(t *testing.T) {
		cmd := NewRootCommand()
		if cmd.Use != "remarker" {
			t.Errorf("expected Use to be 'remarker', got %q", cmd.Use)
		}
	})

	t.Run("has short description", func(t *testing.T) {
		cmd := NewRootCommand()
		if cmd.Short == "" {
			t.Error("expected non-empty Short description")
		}
	})
}

func TestInitCommand(t *testing.T) {
	t.Run("successfully initializes in fresh directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"init"})
		if err != nil {
			t.Fatalf("subcommand init not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Verify manifest file was created
		manifestPath := filepath.Join(tmpDir, ".remarker", "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			t.Fatalf("manifest file not created at %s: %v", manifestPath, err)
		}
	})

	t.Run("returns ErrAlreadyInitialized on second run", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"init"})
		if err != nil {
			t.Fatalf("subcommand init not found: %v", err)
		}

		// First run should succeed
		err = sub.RunE(sub, nil)
		if err != nil {
			t.Fatalf("first init failed: %v", err)
		}

		// Second run should return ErrAlreadyInitialized
		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected ErrAlreadyInitialized on second init, got nil")
		}
		if !errors.Is(err, initpkg.ErrAlreadyInitialized) {
			t.Errorf("expected ErrAlreadyInitialized, got: %v", err)
		}
	})
}

func TestSyncCommand(t *testing.T) {
	t.Run("returns error when password not set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		// Ensure REMARKABLE_PASSWORD is unset
		os.Unsetenv("REMARKABLE_PASSWORD")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"sync"})
		if err != nil {
			t.Fatalf("subcommand sync not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when password not set, got nil")
		}
		if !strings.Contains(err.Error(), "invalid config") {
			t.Errorf("expected error to contain 'invalid config', got: %v", err)
		}
	})

	t.Run("returns error when cannot connect", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)
		t.Setenv("REMARKABLE_PASSWORD", "test")
		t.Setenv("REMARKABLE_HOST", "invalid.invalid")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"sync"})
		if err != nil {
			t.Fatalf("subcommand sync not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when cannot connect, got nil")
		}
		if !strings.Contains(err.Error(), "connect to device") {
			t.Errorf("expected error to contain 'connect to device', got: %v", err)
		}
	})
}

func TestSubcommandsExist(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "init"},
		{name: "sync"},
		{name: "status"},
		{name: "watch"},
		{name: "ssh"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewRootCommand()
			sub, _, err := cmd.Find([]string{tc.name})
			if err != nil {
				t.Errorf("subcommand %q not found: %v", tc.name, err)
				return
			}
			if sub.Name() != tc.name {
				t.Errorf("expected subcommand name %q, got %q", tc.name, sub.Name())
			}
		})
	}
}

func TestStatusCommand(t *testing.T) {
	t.Run("returns error when password not set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		// Ensure REMARKABLE_PASSWORD is unset
		os.Unsetenv("REMARKABLE_PASSWORD")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"status"})
		if err != nil {
			t.Fatalf("subcommand status not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when password not set, got nil")
		}
		if !strings.Contains(err.Error(), "invalid config") {
			t.Errorf("expected error to contain 'invalid config', got: %v", err)
		}
	})

	t.Run("returns error when cannot connect", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)
		t.Setenv("REMARKABLE_PASSWORD", "test")
		t.Setenv("REMARKABLE_HOST", "invalid.invalid")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"status"})
		if err != nil {
			t.Fatalf("subcommand status not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when cannot connect, got nil")
		}
		if !strings.Contains(err.Error(), "connect to device") {
			t.Errorf("expected error to contain 'connect to device', got: %v", err)
		}
	})
}

func TestSSHCommand(t *testing.T) {
	t.Run("returns error when password not set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		// Ensure REMARKABLE_PASSWORD is unset
		os.Unsetenv("REMARKABLE_PASSWORD")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"ssh"})
		if err != nil {
			t.Fatalf("subcommand ssh not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when password not set, got nil")
		}
		if !strings.Contains(err.Error(), "invalid config") {
			t.Errorf("expected error to contain 'invalid config', got: %v", err)
		}
	})

	t.Run("returns error when cannot connect", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)
		t.Setenv("REMARKABLE_PASSWORD", "test")
		t.Setenv("REMARKABLE_HOST", "invalid.invalid")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"ssh"})
		if err != nil {
			t.Fatalf("subcommand ssh not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when cannot connect, got nil")
		}
		if !strings.Contains(err.Error(), "connect to device") {
			t.Errorf("expected error to contain 'connect to device', got: %v", err)
		}
	})
}

func TestWatchCommand(t *testing.T) {
	t.Run("returns error when password not set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)

		// Ensure REMARKABLE_PASSWORD is unset
		os.Unsetenv("REMARKABLE_PASSWORD")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"watch"})
		if err != nil {
			t.Fatalf("subcommand watch not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when password not set, got nil")
		}
		if !strings.Contains(err.Error(), "invalid config") {
			t.Errorf("expected error to contain 'invalid config', got: %v", err)
		}
	})

	t.Run("returns error when cannot connect", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("REMARKER_SYNC_DIR", tmpDir)
		t.Setenv("REMARKABLE_PASSWORD", "test")
		t.Setenv("REMARKABLE_HOST", "invalid.invalid")

		cmd := NewRootCommand()
		sub, _, err := cmd.Find([]string{"watch"})
		if err != nil {
			t.Fatalf("subcommand watch not found: %v", err)
		}

		err = sub.RunE(sub, nil)
		if err == nil {
			t.Fatal("expected error when cannot connect, got nil")
		}
		if !strings.Contains(err.Error(), "connect to device") {
			t.Errorf("expected error to contain 'connect to device', got: %v", err)
		}
	})
}
