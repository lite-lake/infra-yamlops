package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	client *ssh.Client
}

func NewClient(host string, port int, user, password string) (*Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return &Client{client: client}, nil
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Client) Run(cmd string) (stdout, stderr string, err error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(cmd)
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (c *Client) UploadFile(localPath, remotePath string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (c *Client) MkdirAll(path string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	return sftpClient.MkdirAll(path)
}

func (c *Client) MkdirAllSudo(path string) error {
	_, stderr, err := c.Run(fmt.Sprintf("sudo mkdir -p %s", path))
	if err != nil {
		return fmt.Errorf("sudo mkdir failed: %w, stderr: %s", err, stderr)
	}
	return nil
}

func (c *Client) MkdirAllSudoWithPerm(path, perm string) error {
	_, stderr, err := c.Run(fmt.Sprintf("sudo mkdir -p %s && sudo chmod %s %s", path, perm, path))
	if err != nil {
		return fmt.Errorf("sudo mkdir failed: %w, stderr: %s", err, stderr)
	}
	return nil
}

func (c *Client) UploadFileSudo(localPath, remotePath string) error {
	return c.UploadFileSudoWithPerm(localPath, remotePath, "640")
}

func (c *Client) UploadFileSudoWithPerm(localPath, remotePath, perm string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	tmpPath := fmt.Sprintf("/tmp/yamlops-%d", os.Getpid())
	tmpFile, err := sftpClient.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	_, stderr, err := c.Run(fmt.Sprintf("sudo mv %s %s && sudo chmod %s %s", tmpPath, remotePath, perm, remotePath))
	if err != nil {
		return fmt.Errorf("sudo mv failed: %w, stderr: %s", err, stderr)
	}
	return nil
}

func (c *Client) FileExists(path string) (bool, error) {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return false, err
	}
	defer sftpClient.Close()

	_, err = sftpClient.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) StreamRun(cmd string, stdoutChan, stderrChan chan string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	err = session.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go streamReader(stdoutPipe, stdoutChan)
	go streamReader(stderrPipe, stderrChan)

	return session.Wait()
}

func streamReader(reader io.Reader, ch chan string) {
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 && ch != nil {
			ch <- string(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				ch <- err.Error()
			}
			close(ch)
			return
		}
	}
}

func (c *Client) newSFTPClient() (sftpClient, error) {
	return newSFTP(c.client)
}

type sftpClient interface {
	Create(path string) (sftpFile, error)
	MkdirAll(path string) error
	Stat(path string) (os.FileInfo, error)
	Close() error
}

type sftpFile interface {
	io.Writer
	io.Closer
}
