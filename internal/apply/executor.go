package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/entities"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/litelake/yamlops/internal/providers/dns"
	"github.com/litelake/yamlops/internal/ssh"
)

type Executor struct {
	plan       *plan.Plan
	sshClients map[string]*ssh.Client
	secrets    map[string]string
	servers    map[string]*serverInfo
	env        string
	domains    map[string]*entities.Domain
	isps       map[string]*entities.ISP
	dnsCache   map[string]dns.Provider
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
		domains:    make(map[string]*entities.Domain),
		isps:       make(map[string]*entities.ISP),
		dnsCache:   make(map[string]dns.Provider),
		env:        env,
	}
}

func (e *Executor) SetSecrets(secrets map[string]string) {
	e.secrets = secrets
}

func (e *Executor) SetDomains(domains map[string]*entities.Domain) {
	e.domains = domains
}

func (e *Executor) SetISPs(isps map[string]*entities.ISP) {
	e.isps = isps
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

	switch ch.Entity {
	case "dns_record":
		return e.applyDNSRecord(ch)
	case "isp", "zone", "domain", "certificate", "registry":
		result.Success = true
		result.Output = "skipped (not a deployable entity)"
		return result
	case "server":
		result.Success = true
		result.Output = "server registered"
		return result
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
	if err := client.MkdirAllSudo(remoteDir); err != nil {
		result.Error = fmt.Errorf("failed to create remote directory: %w", err)
		return result
	}

	composeFile := e.getComposeFilePath(ch)
	hasCompose := false
	if composeFile != "" {
		if _, err := os.Stat(composeFile); err == nil {
			content, err := os.ReadFile(composeFile)
			if err != nil {
				result.Error = fmt.Errorf("failed to read compose file: %w", err)
				return result
			}
			if err := e.syncContent(client, string(content), remoteDir+"/docker-compose.yml"); err != nil {
				result.Error = fmt.Errorf("failed to sync compose file: %w", err)
				return result
			}
			hasCompose = true
		}
	}

	gatewayFile := e.getGatewayFilePath(ch)
	if gatewayFile != "" {
		if _, err := os.Stat(gatewayFile); err == nil {
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
	}

	configSource := e.getConfigSource(ch)
	if configSource != "" {
		if err := e.syncConfigDir(client, configSource, remoteDir); err != nil {
			result.Error = fmt.Errorf("failed to sync config directory: %w", err)
			return result
		}
	}

	if hasCompose {
		networkCmd := fmt.Sprintf("sudo docker network create yamlops-%s 2>/dev/null || true", e.env)
		_, _, _ = client.Run(networkCmd)

		cmd := fmt.Sprintf("cd %s && sudo docker compose up -d", remoteDir)
		stdout, stderr, err := client.Run(cmd)
		if err != nil {
			result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
			result.Output = stdout + "\n" + stderr
			return result
		}
		result.Output = stdout
	}

	result.Success = true
	return result
}

func (e *Executor) applyUpdate(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	switch ch.Entity {
	case "dns_record":
		return e.applyDNSRecord(ch)
	case "isp", "zone", "domain", "certificate", "registry":
		result.Success = true
		result.Output = "skipped (not a deployable entity)"
		return result
	case "server":
		result.Success = true
		result.Output = "server updated"
		return result
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
	hasCompose := false
	if composeFile != "" {
		if _, err := os.Stat(composeFile); err == nil {
			content, err := os.ReadFile(composeFile)
			if err != nil {
				result.Error = fmt.Errorf("failed to read compose file: %w", err)
				return result
			}
			if err := e.syncContent(client, string(content), remoteDir+"/docker-compose.yml"); err != nil {
				result.Error = fmt.Errorf("failed to sync compose file: %w", err)
				return result
			}
			hasCompose = true
		}
	}

	gatewayFile := e.getGatewayFilePath(ch)
	if gatewayFile != "" {
		if _, err := os.Stat(gatewayFile); err == nil {
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
	}

	configSource := e.getConfigSource(ch)
	if configSource != "" {
		if err := e.syncConfigDir(client, configSource, remoteDir); err != nil {
			result.Error = fmt.Errorf("failed to sync config directory: %w", err)
			return result
		}
	}

	if hasCompose {
		networkCmd := fmt.Sprintf("sudo docker network create yamlops-%s 2>/dev/null || true", e.env)
		_, _, _ = client.Run(networkCmd)

		cmd := fmt.Sprintf("cd %s && sudo docker compose up -d", remoteDir)
		stdout, stderr, err := client.Run(cmd)
		if err != nil {
			result.Error = fmt.Errorf("failed to run docker compose: %w, stderr: %s", err, stderr)
			result.Output = stdout + "\n" + stderr
			return result
		}
		result.Output = stdout
	}

	result.Success = true
	return result
}

func (e *Executor) applyDelete(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	switch ch.Entity {
	case "dns_record":
		return e.deleteDNSRecord(ch)
	case "isp", "zone", "domain", "certificate", "registry":
		result.Success = true
		result.Output = "skipped (not a deployable entity)"
		return result
	case "server":
		result.Success = true
		result.Output = "server removed"
		return result
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

	cmd := fmt.Sprintf("cd %s && sudo docker compose down -v 2>/dev/null || true", remoteDir)
	stdout, stderr, _ := client.Run(cmd)

	rmCmd := fmt.Sprintf("sudo rm -rf %s", remoteDir)
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
	serverName := e.extractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join("deployments", serverName, ch.Name+".compose.yaml")
}

func (e *Executor) getGatewayFilePath(ch *plan.Change) string {
	if ch.Entity == "gateway" {
		serverName := e.extractServerFromChange(ch)
		if serverName == "" {
			return ""
		}
		return filepath.Join("deployments", serverName, ch.Name+".gate.yaml")
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

	return client.UploadFileSudo(tmpFile.Name(), remotePath)
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
			return client.MkdirAllSudo(remotePath)
		}

		return client.UploadFileSudo(path, remotePath)
	})
}

func (e *Executor) getDNSProvider(ispName string) (dns.Provider, error) {
	if provider, ok := e.dnsCache[ispName]; ok {
		return provider, nil
	}

	isp, ok := e.isps[ispName]
	if !ok {
		return nil, fmt.Errorf("ISP %s not found", ispName)
	}

	if !isp.HasService(entities.ISPServiceDNS) {
		return nil, fmt.Errorf("ISP %s does not provide DNS service", ispName)
	}

	cred := isp.Credentials["access_key_id"]
	accessKeyID, err := cred.Resolve(e.secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve access_key_id: %w", err)
	}
	credSecret := isp.Credentials["access_key_secret"]
	accessKeySecret, err := credSecret.Resolve(e.secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve access_key_secret: %w", err)
	}

	provider := dns.NewAliyunProvider(accessKeyID, accessKeySecret)
	e.dnsCache[ispName] = provider
	return provider, nil
}

func (e *Executor) extractDNSRecordFromChange(ch *plan.Change) (*entities.DNSRecord, error) {
	if ch.NewState != nil {
		if record, ok := ch.NewState.(*entities.DNSRecord); ok {
			return record, nil
		}
	}
	if ch.OldState != nil {
		if record, ok := ch.OldState.(*entities.DNSRecord); ok {
			return record, nil
		}
	}
	return nil, fmt.Errorf("cannot extract DNS record from change")
}

func (e *Executor) applyDNSRecord(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	record, err := e.extractDNSRecordFromChange(ch)
	if err != nil {
		result.Error = err
		return result
	}

	domain, ok := e.domains[record.Domain]
	if !ok {
		result.Error = fmt.Errorf("domain %s not found", record.Domain)
		return result
	}

	provider, err := e.getDNSProvider(domain.ISP)
	if err != nil {
		result.Error = fmt.Errorf("failed to get DNS provider: %w", err)
		return result
	}

	dnsRecord := &dns.DNSRecord{
		Name:  record.Name,
		Type:  string(record.Type),
		Value: record.Value,
		TTL:   record.TTL,
	}

	if ch.Type == plan.ChangeTypeUpdate {
		existingRecords, err := provider.ListRecords(record.Domain)
		if err != nil {
			result.Error = fmt.Errorf("failed to list existing records: %w", err)
			return result
		}
		for _, r := range existingRecords {
			if r.Name == record.Name && r.Type == string(record.Type) {
				if err := provider.UpdateRecord(record.Domain, r.ID, dnsRecord); err != nil {
					result.Error = fmt.Errorf("failed to update record: %w", err)
					return result
				}
				result.Success = true
				result.Output = fmt.Sprintf("updated DNS record %s.%s", record.Name, record.Domain)
				return result
			}
		}
	}

	if err := provider.CreateRecord(record.Domain, dnsRecord); err != nil {
		result.Error = fmt.Errorf("failed to create record: %w", err)
		return result
	}

	result.Success = true
	result.Output = fmt.Sprintf("created DNS record %s.%s", record.Name, record.Domain)
	return result
}

func (e *Executor) deleteDNSRecord(ch *plan.Change) *Result {
	result := &Result{
		Change:  ch,
		Success: false,
	}

	record, err := e.extractDNSRecordFromChange(ch)
	if err != nil {
		result.Error = err
		return result
	}

	domain, ok := e.domains[record.Domain]
	if !ok {
		result.Error = fmt.Errorf("domain %s not found", record.Domain)
		return result
	}

	provider, err := e.getDNSProvider(domain.ISP)
	if err != nil {
		result.Error = fmt.Errorf("failed to get DNS provider: %w", err)
		return result
	}

	existingRecords, err := provider.ListRecords(record.Domain)
	if err != nil {
		result.Error = fmt.Errorf("failed to list existing records: %w", err)
		return result
	}

	for _, r := range existingRecords {
		if r.Name == record.Name && strings.EqualFold(r.Type, string(record.Type)) {
			if err := provider.DeleteRecord(record.Domain, r.ID); err != nil {
				result.Error = fmt.Errorf("failed to delete record: %w", err)
				return result
			}
			result.Success = true
			result.Output = fmt.Sprintf("deleted DNS record %s.%s", record.Name, record.Domain)
			return result
		}
	}

	result.Success = true
	result.Output = fmt.Sprintf("DNS record %s.%s not found, skipping", record.Name, record.Domain)
	return result
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
