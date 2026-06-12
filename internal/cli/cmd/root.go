package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	initpkg "github.com/hollen/remarker/internal/application/init"
	statuspkg "github.com/hollen/remarker/internal/application/status"
	syncpkg "github.com/hollen/remarker/internal/application/sync"
	watchpkg "github.com/hollen/remarker/internal/application/watch"
	"github.com/hollen/remarker/internal/infrastructure/config"
	"github.com/hollen/remarker/internal/infrastructure/localfs"
	"github.com/hollen/remarker/internal/infrastructure/manifeststore"
	"github.com/hollen/remarker/internal/infrastructure/sftp"
	"github.com/hollen/remarker/internal/infrastructure/ssh"
	"github.com/hollen/remarker/internal/infrastructure/watcher"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root cobra command for the reMarkable syncing CLI.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remarker",
		Short: "Automatic SSH syncing for reMarkable 2",
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newWatchCmd())
	cmd.AddCommand(newSSHCmd())

	return cmd
}

// Execute runs the root command and handles errors.
func Execute() error {
	return NewRootCommand().Execute()
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a reMarkable sync configuration",
		Long:  "Create the local .remarker/ directory and initialize the sync manifest.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Load()
			manifestRepo := manifeststore.NewDefault(cfg.SyncDir)
			uc := initpkg.NewInitUseCase(manifestRepo)
			return uc.Execute(context.Background())
		},
	}
}

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync documents with the reMarkable device",
		Long:  "Perform a bidirectional sync between the local documents directory and the reMarkable device.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Load()
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			manifestRepo := manifeststore.NewDefault(cfg.SyncDir)
			localRepo := localfs.New(cfg.SyncDir)

			sshClient, err := ssh.Dial(cfg.Host, cfg.Port, cfg.User, cfg.Password)
			if err != nil {
				return fmt.Errorf("connect to device: %w", err)
			}
			defer sshClient.Close()

			sftpClient, err := sftp.New(sshClient)
			if err != nil {
				return fmt.Errorf("initialize sftp: %w", err)
			}
			defer sftpClient.Close()

			uc := syncpkg.NewSyncUseCase(sftpClient, localRepo, manifestRepo)
			result, err := uc.Execute(context.Background())
			if err != nil {
				return err
			}

			if result.HasActions() || result.HasConflicts() || len(result.Errors) > 0 {
				fmt.Printf("Sync complete: %d actions, %d conflicts, %d errors\n",
					len(result.Actions), len(result.Conflicts), len(result.Errors))
			} else {
				fmt.Println("Already in sync.")
			}

			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show pending sync changes",
		Long:  "Preview what changes would be made during a sync without applying them.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Load()
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			manifestRepo := manifeststore.NewDefault(cfg.SyncDir)
			localRepo := localfs.New(cfg.SyncDir)

			sshClient, err := ssh.Dial(cfg.Host, cfg.Port, cfg.User, cfg.Password)
			if err != nil {
				return fmt.Errorf("connect to device: %w", err)
			}
			defer sshClient.Close()

			sftpClient, err := sftp.New(sshClient)
			if err != nil {
				return fmt.Errorf("initialize sftp: %w", err)
			}
			defer sftpClient.Close()

			uc := statuspkg.NewStatusUseCase(sftpClient, localRepo, manifestRepo)
			result, err := uc.Execute(context.Background())
			if err != nil {
				return err
			}

			fmt.Print(result.Summary())
			return nil
		},
	}
}

func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch for changes and sync automatically",
		Long:  "Monitor the local documents directory for changes and automatically sync with the reMarkable device.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Load()
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			manifestRepo := manifeststore.NewDefault(cfg.SyncDir)
			localRepo := localfs.New(cfg.SyncDir)

			sshClient, err := ssh.Dial(cfg.Host, cfg.Port, cfg.User, cfg.Password)
			if err != nil {
				return fmt.Errorf("connect to device: %w", err)
			}
			defer sshClient.Close()

			sftpClient, err := sftp.New(sshClient)
			if err != nil {
				return fmt.Errorf("initialize sftp: %w", err)
			}
			defer sftpClient.Close()

			w := watcher.New(cfg.SyncDir, 0)

			uc := watchpkg.NewWatchUseCase(sftpClient, localRepo, manifestRepo, w, cfg.SyncInterval)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle SIGINT/SIGTERM for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			fmt.Println("Watching for changes... (Ctrl+C to stop)")
			return uc.Run(ctx)
		},
	}
}

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh",
		Short: "Open an SSH session to the reMarkable device",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
}
