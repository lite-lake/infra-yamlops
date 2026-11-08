package ssh

import (
	"os"
	"testing"
)

func TestNewClient_MissingKnownHosts(t *testing.T) {
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	tempDir, err := os.MkdirTemp("", "yamlops-ssh-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("HOME", tempDir)

	_, err = NewClient("localhost", 22, "testuser", "testpass")
	if err == nil {
		t.Error("expected error when known_hosts file does not exist, got nil")
	}
}

func TestNewClient_InvalidKnownHosts(t *testing.T) {
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	tempDir, err := os.MkdirTemp("", "yamlops-ssh-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sshDir := tempDir + "/.ssh"
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("failed to create .ssh dir: %v", err)
	}

	knownHostsPath := sshDir + "/known_hosts"
	if err := os.WriteFile(knownHostsPath, []byte("invalid content"), 0600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	os.Setenv("HOME", tempDir)

	_, err = NewClient("localhost", 22, "testuser", "testpass")
	if err == nil {
		t.Error("expected error when known_hosts file is invalid, got nil")
	}
}
