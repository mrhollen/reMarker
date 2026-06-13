package ssh

import (
	"context"
	"crypto/rand"
	"crypto/ed25519"
	"fmt"
	"net"
	"os"
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

func TestShell_WaitsForSessionCompletion(t *testing.T) {
	// This test verifies that Shell() actually waits for the remote
	// session to complete, rather than returning immediately.
	// We use a test server that properly handles shell and PTY requests.
	listener, err := startTestSSHServerWithShell(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("startTestSSHServerWithShell: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	host, port, err := splitHostPort(addr)
	if err != nil {
		t.Fatalf("splitHostPort: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Dial(ctx, host, port, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	// Shell() should block for at least the server's session duration (500ms).
	// If it returns immediately (< 100ms), the Wait() is missing or broken.
	start := time.Now()
	err = client.Shell()
	elapsed := time.Since(start)

	// Shell() should have waited for the server to close the session.
	// The server holds the session for 500ms, so we should see at least that.
	if elapsed < 400*time.Millisecond {
		t.Fatalf("Shell() returned in %v — should have waited ~500ms for session completion", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Fatal("Shell() hung — likely goroutine leak or missing session cleanup")
	}
	// Session should exit cleanly (server closes it after duration)
	if err != nil {
		t.Logf("Shell() returned error: %v", err)
	}
}

// startTestSSHServerWithShell creates a minimal SSH server that handles
// shell requests and keeps the session alive for the specified duration.
func startTestSSHServerWithShell(sessionDuration time.Duration) (net.Listener, error) {
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
				sc, newChannels, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer sc.Close()

				// Handle incoming channel and request streams
				go func() {
					for ch := range newChannels {
						// Only accept session channels
						if ch.ChannelType() != "session" {
							ch.Reject(ssh.UnknownChannelType, "unknown channel type")
							continue
						}
						channel, reqs, err := ch.Accept()
						if err != nil {
							continue
						}
						go handleShellSession(channel, reqs, sessionDuration)
					}
				}()
				go ssh.DiscardRequests(reqs)

				// Block until connection closes
				sc.Wait()
			}()
		}
	}()

	return listener, nil
}

// handleShellSession handles a single SSH shell session, accepting PTY
// requests and keeping the session alive for the specified duration.
func handleShellSession(channel ssh.Channel, reqs <-chan *ssh.Request, duration time.Duration) {
	defer channel.Close()

	// Accept the session with a timeout so we don't block forever
	done := make(chan struct{})
	go func() {
		for req := range reqs {
			switch req.Type {
			case "pty-req", "shell", "env":
				// Accept these requests
				if req.WantReply {
					req.Reply(true, nil)
				}
			case "exit-status":
				// Client is telling us the exit status, just ack
				if req.WantReply {
					req.Reply(true, nil)
				}
			default:
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}
		close(done)
	}()

	// Keep the session alive for the specified duration
	time.Sleep(duration)

	// Send exit status
	channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
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

func TestIsTerminal_NonTerminal_ReturnsFalse(t *testing.T) {
	// Create a pipe — the read end is definitely not a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	fd := int(r.Fd())
	if isTerminal(fd) {
		t.Error("expected isTerminal to return false for a pipe")
	}
}

func TestGetTerminalSize_NonTerminal_ReturnsFallback(t *testing.T) {
	// Replace stdin temporarily with a pipe fd so getTerminalSize
	// sees a non-terminal. We save and restore the real os.Stdin.
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	w.Close() // close write end so reads would block (not needed here)

	width, height := getTerminalSize()
	if width != 80 || height != 24 {
		t.Errorf("expected fallback 80x24, got %dx%d", width, height)
	}
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
