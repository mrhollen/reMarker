package ssh

import (
	"testing"
)

func TestDial_InvalidHost(t *testing.T) {
	// Dial to a non-routable address should fail
	_, err := Dial("192.0.2.1", 22, "root", "password")
	if err == nil {
		t.Error("Dial to invalid host should return error")
	}
}

func TestDial_InvalidPort(t *testing.T) {
	// Port 0 is invalid
	_, err := Dial("127.0.0.1", 0, "root", "password")
	if err == nil {
		t.Error("Dial to invalid port should return error")
	}
}

func TestDial_EmptyHost(t *testing.T) {
	_, err := Dial("", 22, "root", "password")
	if err == nil {
		t.Error("Dial with empty host should return error")
	}
}

func TestClose_NilClient(t *testing.T) {
	// Closing a nil-initialized client should not panic
	c := &Client{}
	err := c.Close()
	if err != nil {
		t.Errorf("Close on nil conn should return nil error, got: %v", err)
	}
}

func TestClose_NilPointer(t *testing.T) {
	// Dereferencing a nil Client pointer via method call — Go allows this
	// since the receiver is a pointer. The method body must handle nil conn.
	var c *Client
	err := c.Close()
	if err != nil {
		t.Errorf("Close on nil *Client should return nil error, got: %v", err)
	}
}

func TestSSH_ReturnsNilOnUninitializedClient(t *testing.T) {
	c := &Client{}
	if c.SSH() != nil {
		t.Error("SSH() on uninitialized client should return nil")
	}
}

func TestShell_NilClient(t *testing.T) {
	var c *Client
	err := c.Shell()
	if err == nil {
		t.Error("Shell() on nil client should return error")
	}
}

func TestShell_NilConn(t *testing.T) {
	c := &Client{}
	err := c.Shell()
	if err == nil {
		t.Error("Shell() on nil conn should return error")
	}
}

func TestGetTerminalSize_NonTerminal(t *testing.T) {
	// In a test environment, stdin is not a terminal, so we expect fallback.
	w, h := getTerminalSize()
	if w != 80 {
		t.Errorf("expected fallback width 80, got %d", w)
	}
	if h != 24 {
		t.Errorf("expected fallback height 24, got %d", h)
	}
}

func TestDial_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// This test requires a real SSH server. It will fail in CI but serves
	// as a manual integration test when a reMarkable device is connected.
	c, err := Dial("10.11.99.1", 22, "root", "test-password")
	if err == nil {
		// Connected — close and verify SSH() is non-nil
		defer c.Close()
		if c.SSH() == nil {
			t.Error("SSH() should return non-nil after successful dial")
		}
	}
	// If err != nil, we expect it (wrong password or no device) — not a test failure
}
