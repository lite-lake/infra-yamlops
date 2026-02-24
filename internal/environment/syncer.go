package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/infrastructure/network"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

type Syncer struct {
	client     *ssh.Client
	server     *entity.Server
	env        string
	secrets    map[string]string
	registries map[string]*entity.Registry
}

func NewSyncer(client *ssh.Client, server *entity.Server, env string, secrets map[string]string, registries []entity.Registry) *Syncer {
	regMap := make(map[string]*entity.Registry)
	for i := range registries {
		regMap[registries[i].Name] = &registries[i]
	}
	return &Syncer{
		client:     client,
		server:     server,
		env:        env,
		secrets:    secrets,
		registries: regMap,
	}
}

func (s *Syncer) SyncAll() []SyncResult {
	var results []SyncResult

	results = append(results, s.SyncAPTSource())

	results = append(results, s.SyncDockerNetwork())

	results = append(results, s.SyncRegistries()...)

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

func (s *Syncer) SyncDockerNetwork() SyncResult {
	return s.SyncDockerNetworks()
}

func (s *Syncer) SyncDockerNetworks() SyncResult {
	netMgr := network.NewManager(s.client)

	if len(s.server.Networks) == 0 {
		defaultNetwork := entity.ServerNetwork{
			Name: fmt.Sprintf("yamlops-%s", s.env),
			Type: entity.NetworkTypeBridge,
		}
		if err := netMgr.Ensure(&defaultNetwork); err != nil {
			return SyncResult{
				Name:    "docker_network",
				Success: false,
				Message: fmt.Sprintf("failed to create docker network %s", defaultNetwork.Name),
				Error:   err,
			}
		}
		return SyncResult{
			Name:    "docker_network",
			Success: true,
			Message: fmt.Sprintf("network %s ready", defaultNetwork.Name),
		}
	}

	var failedNetworks []string
	for _, netSpec := range s.server.Networks {
		if err := netMgr.Ensure(&netSpec); err != nil {
			failedNetworks = append(failedNetworks, netSpec.Name)
		}
	}

	if len(failedNetworks) > 0 {
		return SyncResult{
			Name:    "docker_network",
			Success: false,
			Message: fmt.Sprintf("failed to create networks: %s", strings.Join(failedNetworks, ", ")),
			Error:   fmt.Errorf("network creation failed"),
		}
	}

	return SyncResult{
		Name:    "docker_network",
		Success: true,
		Message: fmt.Sprintf("all networks ready (%d networks)", len(s.server.Networks)),
	}
}

func (s *Syncer) SyncRegistries() []SyncResult {
	var results []SyncResult

	if len(s.server.Environment.Registries) == 0 {
		return results
	}

	for _, regName := range s.server.Environment.Registries {
		registry, ok := s.registries[regName]
		if !ok {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", regName),
				Success: false,
				Message: "not found in config",
			})
			continue
		}

		if s.isRegistryLoggedIn(registry) {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", regName),
				Success: true,
				Message: fmt.Sprintf("already logged in to %s", registry.URL),
			})
			continue
		}

		username, err := registry.Credentials.Username.Resolve(s.secrets)
		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", regName),
				Success: false,
				Message: "failed to resolve username",
				Error:   err,
			})
			continue
		}

		password, err := registry.Credentials.Password.Resolve(s.secrets)
		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", regName),
				Success: false,
				Message: "failed to resolve password",
				Error:   err,
			})
			continue
		}

		cmd := fmt.Sprintf("sudo docker login -u %s --password-stdin %s 2>&1",
			ssh.ShellEscape(username), ssh.ShellEscape(registry.URL))
		stdout, _, err := s.client.RunWithStdin(password+"\n", cmd)

		if err != nil {
			results = append(results, SyncResult{
				Name:    fmt.Sprintf("registry:%s", regName),
				Success: false,
				Message: fmt.Sprintf("login failed: %s", strings.TrimSpace(stdout)),
				Error:   err,
			})
			continue
		}

		results = append(results, SyncResult{
			Name:    fmt.Sprintf("registry:%s", regName),
			Success: true,
			Message: fmt.Sprintf("logged in to %s", registry.URL),
		})
	}

	return results
}

func (s *Syncer) isRegistryLoggedIn(r *entity.Registry) bool {
	dockerInfo, _, _ := s.client.Run("sudo docker info 2>/dev/null | grep -i username || true")
	configJSON, _, _ := s.client.Run("cat ~/.docker/config.json 2>/dev/null || true")

	if strings.Contains(strings.ToLower(dockerInfo), strings.ToLower(r.URL)) {
		return true
	}

	type dockerConfig struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	var cfg dockerConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err == nil {
		for host, auth := range cfg.Auths {
			if strings.Contains(host, r.URL) && auth.Auth != "" {
				return true
			}
		}
	}

	return false
}
