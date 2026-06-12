package ssh

import (
	"context"
	"crypto/rand"
	"crypto/ed25519"
	"fmt"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestClose_NilClient(t *testing.T) {
	var c *Client
	err := c.Close()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestClose_NilConn(t *testing.T) {
	c := &Client{}
	err := c.Close()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestSSH_NilClient(t *testing.T) {
	var c *Client
	got := c.SSH()
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestShell_NilClient(t *testing.T) {
	var c *Client
	err := c.Shell()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestShell_NilConn(t *testing.T) {
	c := &Client{}
	err := c.Shell()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDial_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Dial(ctx, "127.0.0.1", 22, "root", "password")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDial_ConnectionRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Dial(ctx, "127.0.0.1", 59999, "root", "password")
	if err == nil {
		t.Error("expected error for refused connection, got nil")
	}
}

func TestDial_ContextDeadlinePropagation(t *testing.T) {
	// Verify that a context with a deadline results in a short timeout
	// so the call fails quickly rather than hanging for 30s.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(500*time.Millisecond))
	defer cancel()

	start := time.Now()
	_, err := Dial(ctx, "127.0.0.1", 59999, "root", "password")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error, got nil")
	}
	// Should fail well under 2 seconds, proving the deadline was used
	if elapsed > 2*time.Second {
		t.Errorf("dial took %v, expected it to use the context deadline and fail quickly", elapsed)
	}
}

func TestDial_Success(t *testing.T) {
	listener, err := startTestSSHServer()
	if err != nil {
		t.Fatalf("startTestSSHServer: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	host, port, err := splitHostPort(addr)
	if err != nil {
		t.Fatalf("splitHostPort: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, host, port, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	if client.SSH() == nil {
		t.Error("expected non-nil SSH client")
	}
}

func TestDial_WrongPassword(t *testing.T) {
	listener, err := startTestSSHServer()
	if err != nil {
		t.Fatalf("startTestSSHServer: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	host, port, err := splitHostPort(addr)
	if err != nil {
		t.Fatalf("splitHostPort: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = Dial(ctx, host, port, "testuser", "wrongpass")
	if err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

// startTestSSHServer creates a minimal SSH server listening on a random
// localhost port, using ed25519 host key and password authentication.
func startTestSSHServer() (net.Listener, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, err
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				// Drain requests so the connection doesn't hang
				go ssh.DiscardRequests(reqs)
			}()
		}
	}()

	return listener, nil
}

// splitHostPort splits "127.0.0.1:12345" into host and port.
func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}
