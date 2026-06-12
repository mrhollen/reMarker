// Package ssh provides SSH connection management for communicating with
// reMarkable devices. It handles password authentication and exposes the
// underlying ssh.Client for SFTP and shell operations.
package ssh

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

// Client wraps an ssh.Client and provides a clean interface for SSH
// operations against a reMarkable device.
type Client struct {
	conn *ssh.Client
}

// Dial creates a new SSH connection to the specified host using password
// authentication. It uses InsecureIgnoreHostKey for host key verification
// (TOFU-style) since reMarkable devices are on a local USB Ethernet bridge.
// The provided context controls the connection timeout: if the context has a
// deadline, that deadline is used as the SSH dial timeout. If the context is
// already cancelled, Dial returns immediately with an error.
func Dial(ctx context.Context, host string, port int, user, password string) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("ssh dial: context already done: %w", err)
	}

	timeout := 30 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining > 0 {
			timeout = remaining
		}
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
		// Explicitly set host key algorithms. golang.org/x/crypto/ssh
		// SetDefaults() does NOT populate HostKeyAlgorithms, leaving it
		// empty which causes handshake failure with Dropbear (reMarkable's
		// SSH server). We list ed25519 first since that's what Dropbear
		// advertises, followed by common ECDSA and RSA variants.
		HostKeyAlgorithms: []string{
			"ssh-ed25519",
			"ecdsa-sha2-nistp256",
			"ecdsa-sha2-nistp384",
			"ecdsa-sha2-nistp521",
			"ssh-rsa",
		},
	}
	config.SetDefaults()

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Close closes the underlying SSH connection. It is safe to call on a
// client with a nil connection or multiple times.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// SSH returns the underlying ssh.Client for advanced operations such as
// creating SFTP sessions or running shell commands.
func (c *Client) SSH() *ssh.Client {
	if c == nil {
		return nil
	}
	return c.conn
}

// Shell starts an interactive shell session on the remote host. It requests
// a PTY with the local terminal's dimensions and pipes stdin/stdout/stderr
// from the local terminal. It listens for SIGWINCH to forward resize events
// to the remote side. It blocks until the remote shell exits.
func (c *Client) Shell() error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("shell: no connection")
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("shell: new session: %w", err)
	}
	defer session.Close()

	// Get terminal size
	width, height := getTerminalSize()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", width, height, modes); err != nil {
		return fmt.Errorf("shell: request pty: %w", err)
	}

	// Handle resize signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go func() {
		for range sigChan {
			w, h := getTerminalSize()
			if w > 0 && h > 0 {
				session.WindowChange(w, h)
			}
		}
	}()
	defer signal.Stop(sigChan)

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Shell()
}

// getTerminalSize returns the current terminal dimensions. If the standard
// input is not a terminal (e.g., piped input or test environment), it
// returns the fallback size of 80x24.
func getTerminalSize() (int, int) {
	fd := int(os.Stdin.Fd())
	if !isTerminal(fd) {
		return 80, 24 // fallback for non-terminal
	}
	winsize, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 80, 24
	}
	return int(winsize.Col), int(winsize.Row)
}

// isTerminal checks whether the given file descriptor refers to a terminal
// by attempting a TCGETS ioctl. Returns false on any error.
func isTerminal(fd int) bool {
	var termios unix.Termios
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TCGETS), uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}
