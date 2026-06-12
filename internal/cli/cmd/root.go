package cmd

import (
	"context"
	"fmt"

	initpkg "github.com/hollen/remarker/internal/application/init"
	syncpkg "github.com/hollen/remarker/internal/application/sync"
	"github.com/hollen/remarker/internal/infrastructure/config"
	"github.com/hollen/remarker/internal/infrastructure/localfs"
	"github.com/hollen/remarker/internal/infrastructure/manifeststore"
	"github.com/hollen/remarker/internal/infrastructure/sftp"
	"github.com/hollen/remarker/internal/infrastructure/ssh"
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
		Short: "Show current sync status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
}

func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch for file changes and sync automatically",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
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
