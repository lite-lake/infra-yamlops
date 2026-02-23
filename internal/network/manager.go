package network

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
)

type SSHClient interface {
	Run(cmd string) (stdout, stderr string, err error)
}

type NetworkInfo struct {
	Name   string
	Driver string
	Scope  string
}

type Manager struct {
	client SSHClient
}

func NewManager(client SSHClient) *Manager {
	return &Manager{client: client}
}

func (m *Manager) List() ([]NetworkInfo, error) {
	cmd := "sudo docker network ls --format '{{.Name}}|{{.Driver}}|{{.Scope}}'"
	stdout, stderr, err := m.client.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w, stderr: %s", err, stderr)
	}

	var networks []NetworkInfo
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			networks = append(networks, NetworkInfo{
				Name:   strings.TrimSpace(parts[0]),
				Driver: strings.TrimSpace(parts[1]),
				Scope:  strings.TrimSpace(parts[2]),
			})
		}
	}
	return networks, nil
}

func (m *Manager) Exists(name string) (bool, error) {
	networks, err := m.List()
	if err != nil {
		return false, err
	}
	for _, n := range networks {
		if n.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) Inspect(name string) (*NetworkInfo, error) {
	cmd := fmt.Sprintf("sudo docker network inspect %s --format '{{json .}}'", name)
	stdout, stderr, err := m.client.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect network %s: %w, stderr: %s", name, err, stderr)
	}

	var raw struct {
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse network inspect result: %w", err)
	}

	return &NetworkInfo{
		Name:   raw.Name,
		Driver: raw.Driver,
		Scope:  raw.Scope,
	}, nil
}

func (m *Manager) Create(spec *entity.ServerNetwork) error {
	driver := spec.GetDriver()
	cmd := fmt.Sprintf("sudo docker network create --driver %s %s", driver, spec.Name)
	_, stderr, err := m.client.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w, stderr: %s", spec.Name, err, stderr)
	}
	return nil
}

func (m *Manager) Ensure(spec *entity.ServerNetwork) error {
	exists, err := m.Exists(spec.Name)
	if err != nil {
		return fmt.Errorf("failed to check network existence: %w", err)
	}
	if exists {
		return nil
	}
	return m.Create(spec)
}

func (m *Manager) EnsureAll(networks []entity.ServerNetwork) []EnsureResult {
	var results []EnsureResult
	for _, net := range networks {
		err := m.Ensure(&net)
		results = append(results, EnsureResult{
			Name:    net.Name,
			Success: err == nil,
			Error:   err,
		})
	}
	return results
}

type EnsureResult struct {
	Name    string
	Success bool
	Error   error
}
