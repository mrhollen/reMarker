// Package sftp provides SFTP operations for reading and writing files on a
// reMarkable device. It implements the document.DeviceRepository interface
// for device-side file operations.
package sftp

import (
	"fmt"
	"io"
	"os"

	"github.com/hollen/remarker/internal/infrastructure/ssh"
	"github.com/pkg/sftp"
)

// XochitlDir is the root directory on the reMarkable device where documents
// are stored.
const XochitlDir = "/home/root/.local/share/remarkable/xochitl"

// sftpFile abstracts an SFTP file handle for testability.
type sftpFile interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
}

// sftpClientWrapper adapts *sftp.Client to the sftpClient interface.
type sftpClientWrapper struct {
	client *sftp.Client
}

func (w *sftpClientWrapper) Create(path string) (sftpFile, error) {
	return w.client.Create(path)
}
func (w *sftpClientWrapper) Stat(path string) (os.FileInfo, error) {
	return w.client.Stat(path)
}
func (w *sftpClientWrapper) Remove(path string) error {
	return w.client.Remove(path)
}
func (w *sftpClientWrapper) Rename(oldPath, newPath string) error {
	return w.client.Rename(oldPath, newPath)
}
func (w *sftpClientWrapper) Chmod(path string, mode os.FileMode) error {
	return w.client.Chmod(path, mode)
}
func (w *sftpClientWrapper) ReadDir(path string) ([]os.FileInfo, error) {
	return w.client.ReadDir(path)
}
func (w *sftpClientWrapper) Open(path string) (sftpFile, error) {
	return w.client.Open(path)
}
func (w *sftpClientWrapper) MkdirAll(path string) error {
	return w.client.MkdirAll(path)
}
func (w *sftpClientWrapper) Close() error {
	return w.client.Close()
}

// sftpClient abstracts the SFTP client for testability.
type sftpClient interface {
	Create(path string) (sftpFile, error)
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	Rename(oldPath, newPath string) error
	Chmod(path string, mode os.FileMode) error
	ReadDir(path string) ([]os.FileInfo, error)
	Open(path string) (sftpFile, error)
	MkdirAll(path string) error
	Close() error
}

// Client wraps an sftp.Client and provides operations for the reMarkable
// device's xochitl directory. It implements document.DeviceRepository.
type Client struct {
	sftp    sftpClient
	sftpRaw *sftp.Client // kept for Close()
	baseDir string
}

// New creates a new SFTP client using the provided SSH client. The SFTP
// client operates relative to the xochitl directory on the device.
func New(sshClient *ssh.Client) (*Client, error) {
	if sshClient == nil || sshClient.SSH() == nil {
		return nil, fmt.Errorf("sftp: ssh client is nil")
	}

	rawClient, err := sftp.NewClient(sshClient.SSH())
	if err != nil {
		return nil, fmt.Errorf("sftp new client: %w", err)
	}

	return &Client{
		sftp:    &sftpClientWrapper{client: rawClient},
		sftpRaw: rawClient,
		baseDir: XochitlDir,
	}, nil
}

// Close closes the underlying SFTP connection.
func (c *Client) Close() error {
	if c == nil || c.sftpRaw == nil {
		return nil
	}
	return c.sftpRaw.Close()
}
