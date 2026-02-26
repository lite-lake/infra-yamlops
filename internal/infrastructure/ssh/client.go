package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/retry"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func closeWithLog(closer io.Closer, name string) {
	if err := closer.Close(); err != nil {
		logger.Debug("close error", "name", name, "error", err)
	}
}

var knownHostsMu sync.Mutex

type Client struct {
	client *ssh.Client
	user   string
}

type SSHConfig struct {
	StrictHostKeyChecking bool
	Timeout               time.Duration
}

func DefaultSSHConfig() *SSHConfig {
	return &SSHConfig{
		StrictHostKeyChecking: true,
		Timeout:               constants.DefaultSSHTimeout,
	}
}

func NewClient(host string, port int, user, password string) (*Client, error) {
	return NewClientWithConfig(host, port, user, password, nil)
}

func NewClientWithConfig(host string, port int, user, password string, cfg *SSHConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultSSHConfig()
	}
	logger.Debug("connecting to SSH server", "host", host, "port", port, "user", user, "strict_host_key", cfg.StrictHostKeyChecking, "timeout", cfg.Timeout)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	knownHosts := filepath.Join(homeDir, ".ssh", "known_hosts")

	hostKeyCallback, err := createHostKeyCallback(knownHosts, cfg.StrictHostKeyChecking)
	if err != nil {
		logger.Error("failed to create host key callback", "error", err)
		return nil, domainerr.WrapOp("create host key callback", domainerr.ErrSSHConnectFailed)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		logger.Error("SSH connection failed", "host", host, "port", port, "error", err)
		return nil, domainerr.WrapOp("dial", domainerr.ErrSSHConnectFailed)
	}

	logger.Info("SSH connection established", "host", host, "port", port)
	return &Client{client: client, user: user}, nil
}

func IsRetryableSSHError(err error) bool {
	if err == nil {
		return false
	}

	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no route to host",
		"i/o timeout",
		"broken pipe",
		"connection closed",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

type SSHRetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

func DefaultSSHRetryConfig() *SSHRetryConfig {
	return &SSHRetryConfig{
		MaxAttempts:  constants.DefaultSSHRetryAttempts,
		InitialDelay: constants.DefaultSSHRetryInitialDelay,
		MaxDelay:     constants.DefaultSSHRetryMaxDelay,
	}
}

func NewClientWithRetry(ctx context.Context, host string, port int, user, password string, cfg *SSHRetryConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultSSHRetryConfig()
	}

	logger.Debug("SSH connection with retry", "host", host, "port", port, "max_attempts", cfg.MaxAttempts)

	var client *Client
	err := retry.Do(ctx, func() error {
		var err error
		client, err = NewClient(host, port, user, password)
		return err
	}, retry.WithMaxAttempts(cfg.MaxAttempts), retry.WithInitialDelay(cfg.InitialDelay), retry.WithMaxDelay(cfg.MaxDelay), retry.WithIsRetryable(IsRetryableSSHError))

	if err != nil {
		logger.Error("SSH connection with retry failed", "host", host, "error", err)
	}
	return client, err
}

func createHostKeyCallback(knownHostsPath string, strict bool) (ssh.HostKeyCallback, error) {
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(knownHostsPath), constants.DirPermissionOwner); err != nil {
			return nil, fmt.Errorf("creating known_hosts directory %s: %w", filepath.Dir(knownHostsPath), err)
		}
		if err := os.WriteFile(knownHostsPath, []byte{}, constants.FilePermissionOwnerRW); err != nil {
			return nil, fmt.Errorf("creating known_hosts file %s: %w", knownHostsPath, err)
		}
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("loading known_hosts from %s: %w", knownHostsPath, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}

		keyErr, ok := err.(*knownhosts.KeyError)
		if !ok {
			return err
		}

		if len(keyErr.Want) > 0 {
			return domainerr.WrapEntity("host key", hostname, domainerr.ErrSSHHostKeyMismatch)
		}

		if strict {
			logger.Warn("unknown host key rejected", "hostname", hostname, "fingerprint", ssh.FingerprintSHA256(key))
			return fmt.Errorf("%w: unknown host %s (fingerprint: %s). Add to known_hosts manually or disable strict host key checking",
				domainerr.ErrSSHConnectFailed, hostname, ssh.FingerprintSHA256(key))
		}

		logger.Warn("auto-accepting unknown host key", "hostname", hostname, "fingerprint", ssh.FingerprintSHA256(key))
		line := knownhosts.Line([]string{hostname}, key)
		knownHostsMu.Lock()
		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, constants.FilePermissionOwnerRW)
		if err != nil {
			knownHostsMu.Unlock()
			return domainerr.WrapOp("open known_hosts", domainerr.ErrSSHConnectFailed)
		}
		defer func() {
			closeWithLog(f, "known_hosts file")
			knownHostsMu.Unlock()
		}()
		if _, err := fmt.Fprintln(f, line); err != nil {
			return domainerr.WrapOp("write known_hosts", domainerr.ErrSSHConnectFailed)
		}
		return nil
	}, nil
}

func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *Client) Run(cmd string) (stdout, stderr string, err error) {
	logger.Debug("running SSH command", "cmd", cmd)

	session, err := c.client.NewSession()
	if err != nil {
		logger.Error("failed to create SSH session", "error", err)
		return "", "", domainerr.WrapOp("create session", domainerr.ErrSSHSessionFailed)
	}
	defer closeWithLog(session, "ssh session")

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(cmd)
	if err != nil {
		logger.Debug("SSH command failed", "cmd", cmd, "error", err, "stderr", stderrBuf.String())
	}
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (c *Client) RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", "", domainerr.WrapOp("create session", domainerr.ErrSSHSessionFailed)
	}
	defer closeWithLog(session, "ssh session")

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return "", "", domainerr.WrapOp("get stdin pipe", domainerr.ErrSSHSessionFailed)
	}

	if err := session.Start(cmd); err != nil {
		return "", "", domainerr.WrapOp("start command", domainerr.ErrSSHCommandFailed)
	}

	_, err = io.WriteString(stdinPipe, stdin)
	if err != nil {
		stdinPipe.Close()
		return "", "", domainerr.WrapOp("write to stdin", domainerr.ErrSSHCommandFailed)
	}
	stdinPipe.Close()

	err = session.Wait()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func (c *Client) UploadFile(localPath, remotePath string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer closeWithLog(sftpClient, "sftp client")

	localFile, err := os.Open(localPath)
	if err != nil {
		return domainerr.WrapOp("open local file", domainerr.ErrSSHFileTransfer)
	}
	defer closeWithLog(localFile, "local file")

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return domainerr.WrapOp("create remote file", domainerr.ErrSSHFileTransfer)
	}
	defer closeWithLog(remoteFile, "remote file")

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return domainerr.WrapOp("copy file", domainerr.ErrSSHFileTransfer)
	}

	return nil
}

func (c *Client) MkdirAll(path string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer closeWithLog(sftpClient, "sftp client")

	return sftpClient.MkdirAll(path)
}

func (c *Client) MkdirAllSudo(path string) error {
	_, stderr, err := c.Run(fmt.Sprintf("sudo mkdir -p %s", ShellEscape(path)))
	if err != nil {
		return domainerr.WrapOp("sudo mkdir", fmt.Errorf("%w: stderr: %s", domainerr.ErrSSHCommandFailed, stderr))
	}
	return nil
}

func (c *Client) MkdirAllSudoWithPerm(path, perm string) error {
	// Use 777 for volumes to allow Docker containers with any user to access
	// Recursive chmod ensures existing subdirectories are also accessible
	cmd := fmt.Sprintf("sudo mkdir -p %s && sudo chmod -R 777 %s", ShellEscape(path), ShellEscape(path))
	_, stderr, err := c.Run(cmd)
	if err != nil {
		return domainerr.WrapOp("sudo mkdir", fmt.Errorf("%w: stderr: %s", domainerr.ErrSSHCommandFailed, stderr))
	}
	return nil
}

func (c *Client) UploadFileSudo(localPath, remotePath string) error {
	return c.UploadFileSudoWithPerm(localPath, remotePath, constants.DefaultRemoteFilePerm)
}

func (c *Client) UploadFileSudoWithPerm(localPath, remotePath, perm string) error {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer closeWithLog(sftpClient, "sftp client")

	localFile, err := os.Open(localPath)
	if err != nil {
		return domainerr.WrapOp("open local file", domainerr.ErrSSHFileTransfer)
	}
	defer closeWithLog(localFile, "local file")

	tmpPath := fmt.Sprintf(constants.RemoteTempFileFmt, os.Getpid())
	tmpFile, err := sftpClient.Create(tmpPath)
	if err != nil {
		return domainerr.WrapOp("create temp file", domainerr.ErrSSHFileTransfer)
	}
	defer closeWithLog(tmpFile, "temp file")

	_, err = io.Copy(tmpFile, localFile)
	if err != nil {
		return domainerr.WrapOp("copy file", domainerr.ErrSSHFileTransfer)
	}

	cmd := fmt.Sprintf("sudo mv %s %s && sudo chown %s:%s %s && sudo chmod %s %s", ShellEscape(tmpPath), ShellEscape(remotePath), ShellEscape(c.user), ShellEscape(c.user), ShellEscape(remotePath), ShellEscape(perm), ShellEscape(remotePath))
	_, stderr, err := c.Run(cmd)
	if err != nil {
		return domainerr.WrapOp("sudo mv", fmt.Errorf("%w: stderr: %s", domainerr.ErrSSHCommandFailed, stderr))
	}
	return nil
}

func (c *Client) FileExists(path string) (bool, error) {
	sftpClient, err := c.newSFTPClient()
	if err != nil {
		return false, err
	}
	defer closeWithLog(sftpClient, "sftp client")

	_, err = sftpClient.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) StreamRun(ctx context.Context, cmd string, stdoutChan, stderrChan chan string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return domainerr.WrapOp("create session", domainerr.ErrSSHSessionFailed)
	}
	defer closeWithLog(session, "ssh session")

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return domainerr.WrapOp("get stdout pipe", domainerr.ErrSSHSessionFailed)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return domainerr.WrapOp("get stderr pipe", domainerr.ErrSSHSessionFailed)
	}

	err = session.Start(cmd)
	if err != nil {
		return domainerr.WrapOp("start command", domainerr.ErrSSHCommandFailed)
	}

	done := make(chan struct{})
	go func() {
		streamReader(ctx, stdoutPipe, stdoutChan)
		streamReader(ctx, stderrPipe, stderrChan)
		close(done)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGKILL)
		session.Close()
		<-done
		return ctx.Err()
	case err := <-waitSession(session, done):
		return err
	}
}

func waitSession(session *ssh.Session, done <-chan struct{}) <-chan error {
	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		ch <- session.Wait()
		<-done
	}()
	return ch
}

func streamReader(ctx context.Context, reader io.Reader, ch chan string) {
	buf := make([]byte, constants.SSHStreamBufferSize)
	for {
		// Create a channel to receive read result
		type readResult struct {
			n   int
			err error
		}
		readDone := make(chan readResult, 1)

		go func() {
			n, err := reader.Read(buf)
			readDone <- readResult{n: n, err: err}
		}()

		// Wait for read or context cancellation
		select {
		case result := <-readDone:
			n, err := result.n, result.err
			if n > 0 && ch != nil {
				select {
				case ch <- string(buf[:n]):
				case <-ctx.Done():
					if ch != nil {
						close(ch)
					}
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) && ch != nil {
					select {
					case ch <- err.Error():
					case <-ctx.Done():
					}
				}
				if ch != nil {
					close(ch)
				}
				return
			}
		case <-ctx.Done():
			if ch != nil {
				close(ch)
			}
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
