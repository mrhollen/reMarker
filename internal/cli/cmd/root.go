package cmd

import (
	"fmt"

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
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
}

func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync documents with the reMarkable device",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
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
