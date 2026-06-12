// Package ssh provides SSH connection management for communicating with
// reMarkable devices. It handles password authentication and exposes the
// underlying ssh.Client for SFTP and shell operations.
package ssh

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client wraps an ssh.Client and provides a clean interface for SSH
// operations against a reMarkable device.
type Client struct {
	conn *ssh.Client
}

// Dial creates a new SSH connection to the specified host using password
// authentication. It uses InsecureIgnoreHostKey for host key verification
// (TOFU-style) since reMarkable devices are on a local USB Ethernet bridge.
func Dial(host string, port int, user, password string) (*Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

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
// a PTY and pipes stdin/stdout/stderr from the local terminal. It blocks
// until the remote shell exits.
func (c *Client) Shell() error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("shell: no connection")
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("shell: new session: %w", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 80, 40, modes); err != nil {
		return fmt.Errorf("shell: request pty: %w", err)
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Shell()
}
