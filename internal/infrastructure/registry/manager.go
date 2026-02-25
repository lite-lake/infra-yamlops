package registry

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/interfaces"
)

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type LoginResult struct {
	Name    string
	Success bool
	Message string
	Error   error
}

type Manager struct {
	mu         sync.RWMutex
	client     interfaces.SSHRunner
	registries map[string]*entity.Registry
	secrets    map[string]string
	loggedIn   map[string]bool
}

func NewManager(client interfaces.SSHRunner, registries []*entity.Registry, secrets map[string]string) *Manager {
	m := &Manager{
		client:     client,
		registries: make(map[string]*entity.Registry),
		secrets:    secrets,
		loggedIn:   make(map[string]bool),
	}
	for _, r := range registries {
		m.registries[r.Name] = r
	}
	return m
}

func (m *Manager) EnsureLoggedIn(registryName string) (*LoginResult, error) {
	m.mu.RLock()
	if m.loggedIn[registryName] {
		m.mu.RUnlock()
		return &LoginResult{
			Name:    registryName,
			Success: true,
			Message: "already logged in",
		}, nil
	}
	m.mu.RUnlock()

	registry, ok := m.registries[registryName]
	if !ok {
		return &LoginResult{
			Name:    registryName,
			Success: false,
			Message: "registry not found in config",
		}, fmt.Errorf("%w: %s", domainerr.ErrRegistryNotFound, registryName)
	}

	if m.isLoggedIn(registry) {
		m.mu.Lock()
		m.loggedIn[registryName] = true
		m.mu.Unlock()
		return &LoginResult{
			Name:    registryName,
			Success: true,
			Message: "already logged in",
		}, nil
	}

	return m.login(registry)
}

func (m *Manager) LoginAll() []LoginResult {
	var results []LoginResult
	for name := range m.registries {
		result, _ := m.EnsureLoggedIn(name)
		results = append(results, *result)
	}
	return results
}

func (m *Manager) isLoggedIn(r *entity.Registry) bool {
	dockerInfo, _, _ := m.client.Run("docker info 2>/dev/null | grep -i username || true")
	configJSON, _, _ := m.client.Run("cat ~/.docker/config.json 2>/dev/null || true")

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

func (m *Manager) login(r *entity.Registry) (*LoginResult, error) {
	username, err := r.Credentials.Username.Resolve(m.secrets)
	if err != nil {
		return &LoginResult{
			Name:    r.Name,
			Success: false,
			Message: "failed to resolve username",
			Error:   err,
		}, err
	}

	password, err := r.Credentials.Password.Resolve(m.secrets)
	if err != nil {
		return &LoginResult{
			Name:    r.Name,
			Success: false,
			Message: "failed to resolve password",
			Error:   err,
		}, err
	}

	cmd := fmt.Sprintf("docker login -u %s --password-stdin %s",
		shellEscape(username), shellEscape(r.URL))
	_, stderr, err := m.client.RunWithStdin(password+"\n", cmd)

	if err != nil {
		return &LoginResult{
			Name:    r.Name,
			Success: false,
			Message: "docker login failed",
			Error:   fmt.Errorf("%w: %s", domainerr.ErrRegistryLoginFailed, strings.TrimSpace(stderr)),
		}, err
	}

	m.mu.Lock()
	m.loggedIn[r.Name] = true
	m.mu.Unlock()
	return &LoginResult{
		Name:    r.Name,
		Success: true,
		Message: fmt.Sprintf("logged in to %s", r.URL),
	}, nil
}

func (m *Manager) GetRegistryURL(registryName string) (string, error) {
	registry, ok := m.registries[registryName]
	if !ok {
		return "", fmt.Errorf("%w: %s", domainerr.ErrRegistryNotFound, registryName)
	}
	return registry.URL, nil
}
