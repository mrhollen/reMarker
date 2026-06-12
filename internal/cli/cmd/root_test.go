package cmd

import (
	"testing"
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

func TestSubcommandsNotImplemented(t *testing.T) {
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
				t.Fatalf("subcommand %q not found: %v", tc.name, err)
			}
			// RunE should return an error since it's a placeholder
			err = sub.RunE(sub, nil)
			if err == nil {
				t.Errorf("expected subcommand %q to return an error (not yet implemented)", tc.name)
			}
		})
	}
}
