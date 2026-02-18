package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/ssh"
)

type Syncer struct {
	client     *ssh.Client
	server     *entity.Server
	env        string
	registries []*entity.Registry
	secrets    map[string]string
}

func NewSyncer(client *ssh.Client, server *entity.Server, env string, registries []*entity.Registry, secrets map[string]string) *Syncer {
	return &Syncer{
		client:     client,
		server:     server,
		env:        env,
		registries: registries,
		secrets:    secrets,
	}
}

func (s *Syncer) SyncAll() []SyncResult {
	var results []SyncResult

	results = append(results, s.SyncAPTSource())

	results = append(results, s.SyncRegistries()...)

	results = append(results, s.SyncDockerNetwork())

	return results
}

func (s *Syncer) SyncAPTSource() SyncResult {
	aptSource := s.server.Environment.APTSource
	if aptSource == "" || aptSource == "official" {
		return SyncResult{
			Name:    "apt_source",
			Success: true,
			Message: "skipped (using official or no change)",
		}
	}

	content, err := GetAPTTemplate(aptSource)
	if err != nil {
		return SyncResult{
			Name:    "apt_source",
			Success: false,
			Message: fmt.Sprintf("template not found: %s", aptSource),
			Error:   err,
		}
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("sources.list.%d", os.Getpid()))
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return SyncResult{
			Name:    "apt_source",
			Success: false,
			Message: "failed to write temp file",
			Error:   err,
		}
	}
	defer os.Remove(tmpFile)

	backupCmd := "sudo cp /etc/apt/sources.list /etc/apt/sources.list.bak.$(date +%Y%m%d%H%M%S)"
	if _, stderr, err := s.client.Run(backupCmd); err != nil {
		return SyncResult{
			Name:    "apt_source",
			Success: false,
			Message: "failed to backup sources.list",
			Error:   fmt.Errorf("%w: %s", err, stderr),
		}
	}

	if err := s.client.UploadFileSudo(tmpFile, "/etc/apt/sources.list"); err != nil {
		rollbackCmd := "sudo cp /etc/apt/sources.list.bak.* /etc/apt/sources.list 2>/dev/null || true"
		s.client.Run(rollbackCmd)
		return SyncResult{
			Name:    "apt_source",
			Success: false,
			Message: "failed to upload sources.list",
			Error:   err,
		}
	}

	if _, stderr, err := s.client.Run("sudo apt-get update"); err != nil {
		return SyncResult{
			Name:    "apt_source",
			Success: false,
			Message: "apt-get update failed",
			Error:   fmt.Errorf("%w: %s", err, stderr),
		}
	}

	return SyncResult{
		Name:    "apt_source",
		Success: true,
		Message: fmt.Sprintf("configured apt source: %s", aptSource),
	}
}

func (s *Syncer) SyncRegistries() []SyncResult {
	var results []SyncResult

	registryNames := s.server.Environment.Registries
	if len(registryNames) == 0 {
		return results
	}

	registryMap := make(map[string]*entity.Registry)
	for _, r := range s.registries {
		registryMap[r.Name] = r
	}

	for _, name := range registryNames {
		registry, ok := registryMap[name]
		if !ok {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", name),
				Success: false,
				Message: "registry not found in config",
				Error:   fmt.Errorf("registry %s not found", name),
			})
			continue
		}

		username, err := registry.Credentials.Username.Resolve(s.secrets)
		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", name),
				Success: false,
				Message: "failed to resolve username",
				Error:   err,
			})
			continue
		}

		password, err := registry.Credentials.Password.Resolve(s.secrets)
		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", name),
				Success: false,
				Message: "failed to resolve password",
				Error:   err,
			})
			continue
		}

		cmd := fmt.Sprintf("echo '%s' | docker login -u '%s' --password-stdin %s",
			password, username, registry.URL)
		_, stderr, err := s.client.Run(cmd)

		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", name),
				Success: false,
				Message: "docker login failed",
				Error:   fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr)),
			})
			continue
		}

		results = append(results, SyncResult{
			Name:    fmt.Sprintf("registry:%s", name),
			Success: true,
			Message: fmt.Sprintf("logged in to %s", registry.URL),
		})
	}

	return results
}

func (s *Syncer) SyncDockerNetwork() SyncResult {
	networkName := fmt.Sprintf("yamlops-%s", s.env)
	cmd := fmt.Sprintf("sudo docker network create %s 2>/dev/null || true", networkName)

	_, stderr, err := s.client.Run(cmd)
	if err != nil {
		return SyncResult{
			Name:    "docker_network",
			Success: false,
			Message: "failed to create docker network",
			Error:   fmt.Errorf("%w: %s", err, stderr),
		}
	}

	return SyncResult{
		Name:    "docker_network",
		Success: true,
		Message: fmt.Sprintf("network %s ready", networkName),
	}
}
