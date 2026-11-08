package ssh

import (
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type realSFTPClient struct {
	client *sftp.Client
}

func newSFTP(sshClient *ssh.Client) (*realSFTPClient, error) {
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	return &realSFTPClient{client: client}, nil
}

func (c *realSFTPClient) Create(path string) (sftpFile, error) {
	return c.client.Create(path)
}

func (c *realSFTPClient) MkdirAll(path string) error {
	return c.client.MkdirAll(path)
}

func (c *realSFTPClient) Stat(path string) (os.FileInfo, error) {
	return c.client.Stat(path)
}

func (c *realSFTPClient) Close() error {
	return c.client.Close()
}
