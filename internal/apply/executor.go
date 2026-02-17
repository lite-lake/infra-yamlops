package apply

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/plan"
	"github.com/litelake/yamlops/internal/ssh"
)

type Executor struct {
	plan       *plan.Plan
	sshClients map[string]*ssh.Client
	secrets    map[string]string
	servers    map[string]*serverInfo
	env        string
}

type serverInfo struct {
	host     string
	port     int
	user     string
	password string
}

type Result struct {
	Change  *plan.Change
	Success bool
	Error   error
	Output  string
}

func NewExecutor(pl *plan.Plan, env string) *Executor {
	if env == "" {
		env = "dev"
	}
	return &Executor{
		plan:       pl,
		sshClients: make(map[string]*ssh.Client),
		secrets:    make(map[string]string),
		servers:    make(map[string]*serverInfo),
		env:        env,
	}
}

func (e *Executor) SetSecrets(secrets map[string]string) {
	e.secrets = secrets
}

func (e *Executor) RegisterServer(name string, host string, port int, user, password string) {
	e.servers[name] = &serverInfo{
		host:     host,
		port:     port,
		user:     user,
		password: password,
	}
}

func (e *Executor) Apply() []*Result {
	results := make([]*Result, 0, len(e.plan.Changes))

	for _, ch := range e.plan.Changes {
		var result *Result

		switch ch.Type {
		case plan.ChangeTypeCreate:
			result = e.applyCreate(ch)
		case plan.ChangeTypeUpdate:
			result = e.applyUpdate(ch)
		case plan.ChangeTypeDelete:
			result = e.applyDelete(ch)
		default:
			result = &Result{
				Change:  ch,
				Success: true,
				Error:   nil,
			}
		}

		results = append(results, result)
	}

	e.closeClients()
	return results
}

func (e *Executor) applyCreate(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	serverName := e.extractServerFromChange(ch)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", ch.Name)
		return result
	}

	client, err := e.getClient(serverName)
	if err != nil {
		result.Error = fmt.Errorf("failed to get SSH client: %w", err)
		return result
	}

	remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", e.env, ch.Name)
	if err := client.MkdirAll(remoteDir); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result
	}

	composeFile := e.getComposeFilePath(ch)
	if composeFile != "" {
		content, err := os.ReadFile(composeFile)
		if err != nil {
			result.Error = fmt.Errorf("failed to read compose file: %w", err)
			return result
		}
		if err := e.syncContent(client, string(content), remoteDir+"/docker-compose.yml"); err != nil {
			result.Error = fmt.Errorf("failed to sync compose file: %w", err)
			return result
		}
	}

	gatewayFile := e.getGatewayFilePath(ch)
	if gatewayFile != "" {
		content, err := os.ReadFile(gatewayFile)
		if err != nil {
			result.Error = fmt.Errorf("failed to read gateway file: %w", err)
			return result
		}
		if err := e.syncContent(client, string(content), remoteDir+"/gateway.yml"); err != nil {
			result.Error = fmt.Errorf("failed to sync gateway file: %w", err)
			return result
		}
	}

	configSource := e.getConfigSource(ch)
	if configSource != "" {
		if err := e.syncConfigDir(client, configSource, remoteDir); err != nil {
			result.Error = fmt.Errorf("failed to sync config directory: %w", err)
			return result
		}
	}

	cmd := fmt.Sprintf("cd %s && docker compose up -d", remoteDir)
	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
		result.Output = stdout + "\n" + stderr
		return result
	}

	result.Success = true
	result.Output = stdout
	return result
}

func (e *Executor) applyUpdate(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	serverName := e.extractServerFromChange(ch)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", ch.Name)
		return result
	}

	client, err := e.getClient(serverName)
	if err != nil {
		result.Error = fmt.Errorf("failed to get SSH client: %w", err)
		return result
	}

	remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", e.env, ch.Name)

	composeFile := e.getComposeFilePath(ch)
	if composeFile != "" {
		content, err := os.ReadFile(composeFile)
		if err != nil {
			result.Error = fmt.Errorf("failed to read compose file: %w", err)
			return result
		}
		if err := e.syncContent(client, string(content), remoteDir+"/docker-compose.yml"); err != nil {
			result.Error = fmt.Errorf("failed to sync compose file: %w", err)
			return result
		}
	}

	gatewayFile := e.getGatewayFilePath(ch)
	if gatewayFile != "" {
		content, err := os.ReadFile(gatewayFile)
		if err != nil {
			result.Error = fmt.Errorf("failed to read gateway file: %w", err)
			return result
		}
		if err := e.syncContent(client, string(content), remoteDir+"/gateway.yml"); err != nil {
			result.Error = fmt.Errorf("failed to sync gateway file: %w", err)
			return result
		}
	}

	configSource := e.getConfigSource(ch)
	if configSource != "" {
		if err := e.syncConfigDir(client, configSource, remoteDir); err != nil {
			result.Error = fmt.Errorf("failed to sync config directory: %w", err)
			return result
		}
	}

	cmd := fmt.Sprintf("cd %s && docker compose up -d", remoteDir)
	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
		result.Output = stdout + "\n" + stderr
		return result
	}

	result.Success = true
	result.Output = stdout
	return result
}

func (e *Executor) applyDelete(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	serverName := e.extractServerFromChange(ch)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", ch.Name)
		return result
	}

	client, err := e.getClient(serverName)
	if err != nil {
		result.Error = fmt.Errorf("failed to get SSH client: %w", err)
		return result
	}

	remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", e.env, ch.Name)

	cmd := fmt.Sprintf("cd %s && docker compose down -v 2>/dev/null || true", remoteDir)
	stdout, stderr, _ := client.Run(cmd)

	rmCmd := fmt.Sprintf("rm -rf %s", remoteDir)
	stdout2, stderr2, err := client.Run(rmCmd)
	if err != nil {
		result.Error = fmt.Errorf("failed to remove directory: %w, stderr: %s", err, stderr2)
		result.Output = stdout + "\n" + stderr + "\n" + stdout2 + "\n" + stderr2
		return result
	}

	result.Success = true
	result.Output = stdout + "\n" + stdout2
	return result
}

func (e *Executor) extractServerFromChange(ch *plan.Change) string {
	if ch.OldState != nil {
		if svc, ok := ch.OldState.(map[string]interface{}); ok {
			if server, ok := svc["server"].(string); ok {
				return server
			}
		}
		switch v := ch.OldState.(type) {
		case interface{ GetServer() string }:
			return v.GetServer()
		}
	}
	if ch.NewState != nil {
		if svc, ok := ch.NewState.(map[string]interface{}); ok {
			if server, ok := svc["server"].(string); ok {
				return server
			}
		}
		switch v := ch.NewState.(type) {
		case interface{ GetServer() string }:
			return v.GetServer()
		}
	}

	return ""
}

func (e *Executor) getComposeFilePath(ch *plan.Change) string {
	return filepath.Join("deployments", ch.Name, ch.Name+".compose.yaml")
}

func (e *Executor) getGatewayFilePath(ch *plan.Change) string {
	if ch.Entity == "gateway" {
		return filepath.Join("deployments", ch.Name, ch.Name+".gate.yaml")
	}
	return ""
}

func (e *Executor) getConfigSource(ch *plan.Change) string {
	return ""
}

func (e *Executor) getClient(serverName string) (*ssh.Client, error) {
	if client, ok := e.sshClients[serverName]; ok {
		return client, nil
	}

	info, ok := e.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not registered", serverName)
	}

	client, err := ssh.NewClient(info.host, info.port, info.user, info.password)
	if err != nil {
		return nil, err
	}

	e.sshClients[serverName] = client
	return client, nil
}

func (e *Executor) syncContent(client *ssh.Client, content, remotePath string) error {
	tmpFile, err := os.CreateTemp("", "yamlops-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	return client.UploadFile(tmpFile.Name(), remotePath)
}

func (e *Executor) syncConfigDir(client *ssh.Client, localDir, remoteDir string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		remotePath := filepath.Join(remoteDir, relPath)

		if info.IsDir() {
			return client.MkdirAll(remotePath)
		}

		return client.UploadFile(path, remotePath)
	})
}

func (e *Executor) closeClients() {
	for _, client := range e.sshClients {
		client.Close()
	}
	e.sshClients = make(map[string]*ssh.Client)
}

func (e *Executor) FilterPlanByServer(serverName string) *plan.Plan {
	filtered := plan.NewPlan()
	for _, ch := range e.plan.Changes {
		s := e.extractServerFromChange(ch)
		if s == serverName {
			filtered.AddChange(ch)
		}
	}
	return filtered
}
