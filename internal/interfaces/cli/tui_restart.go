package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func (m *Model) fetchRestartServiceStatusAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return restartStatusFetchedMsg{statusMap: make(map[string]NodeStatus)}
		}
		secrets := m.Config.GetSecretsMap()
		servers := m.buildServerWithSSHList()
		infraServices := m.buildInfraServicesList()
		bizServices := m.buildBizServicesList()
		statusMap := fetchServiceStatus(servers, infraServices, bizServices, secrets, string(m.Environment))
		return restartStatusFetchedMsg{statusMap: statusMap}
	}
}

func (m *Model) applyRestartServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Restart.ServiceStatusMap, string(m.Environment))
}

func (m *Model) executeServiceRestartAsync() tea.Cmd {
	return func() tea.Msg {
		results := []RestartResult{}
		secrets := m.Config.GetSecretsMap()

		servicesToProcess := getSelectedServicesInfo(m.Tree.TreeNodes)
		if len(servicesToProcess) == 0 {
			return serviceRestartCompleteMsg{results: results}
		}

		serverServices := make(map[string][]serviceInfo)
		for _, svc := range servicesToProcess {
			if svc.Server != "" {
				serverServices[svc.Server] = append(serverServices[svc.Server], svc)
			}
		}

		for _, srv := range m.Server.ServerList {
			services, ok := serverServices[srv.Name]
			if !ok || len(services) == 0 {
				continue
			}

			result := RestartResult{ServerName: srv.Name}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				for _, svcInfo := range services {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: false,
						Error:   fmt.Sprintf("Cannot resolve password: %v", err),
					})
				}
				results = append(results, result)
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				for _, svcInfo := range services {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: false,
						Error:   fmt.Sprintf("Connection failed: %v", err),
					})
				}
				results = append(results, result)
				continue
			}

			for _, svcInfo := range services {
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, svcInfo.Name))

				workDir, err := os.Getwd()
				if err != nil {
					workDir = "."
				}
				composeFile := filepath.Join(workDir, "deployments", srv.Name, svcInfo.Name+".compose.yaml")

				if syncErr := m.syncComposeFile(client, composeFile, remoteDir); syncErr != nil {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: false,
						Error:   fmt.Sprintf("sync compose file failed: %v", syncErr),
					})
					continue
				}

				if syncErr := m.syncEnvFile(client, composeFile, remoteDir); syncErr != nil {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: false,
						Error:   fmt.Sprintf("sync env file failed: %v", syncErr),
					})
					continue
				}

				if svcInfo.Type == NodeTypeInfra {
					if syncErr := m.syncInfraFiles(client, svcInfo.Name, srv.Name, remoteDir, workDir); syncErr != nil {
						result.Services = append(result.Services, RestartServiceResult{
							Name:    svcInfo.Name,
							Success: false,
							Error:   fmt.Sprintf("sync infra files failed: %v", syncErr),
						})
						continue
					}
				}

				cmd := restartServiceCommand(remoteDir)
				_, stderr, err := client.Run(cmd)
				if err != nil {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: false,
						Error:   stderr,
					})
				} else {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcInfo.Name,
						Success: true,
					})
				}
			}

			client.Close()
			results = append(results, result)
		}

		return serviceRestartCompleteMsg{results: results}
	}
}

func (m *Model) syncComposeFile(client *ssh.Client, composeFile, remoteDir string) error {
	if composeFile == "" {
		return nil
	}

	if _, err := os.Stat(composeFile); err != nil {
		return nil
	}

	content, err := os.ReadFile(composeFile)
	if err != nil {
		return fmt.Errorf("read compose file: %w", err)
	}

	return m.syncContent(client, string(content), remoteDir+"/docker-compose.yml")
}

func (m *Model) syncEnvFile(client *ssh.Client, composeFile, remoteDir string) error {
	if composeFile == "" {
		return nil
	}

	envFile := composeFile[:len(composeFile)-len(".compose.yaml")] + ".env"
	if _, err := os.Stat(envFile); err != nil {
		return nil
	}

	content, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	envFileName := filepath.Base(envFile)
	return m.syncContent(client, string(content), remoteDir+"/"+envFileName)
}

func (m *Model) syncInfraFiles(client *ssh.Client, serviceName, serverName, remoteDir, workDir string) error {
	gatewayFile := filepath.Join(workDir, "deployments", serverName, serviceName+".gate.yaml")
	if _, err := os.Stat(gatewayFile); err == nil {
		content, err := os.ReadFile(gatewayFile)
		if err != nil {
			return fmt.Errorf("read gateway file: %w", err)
		}
		if err := m.syncContent(client, string(content), remoteDir+"/gateway.yml"); err != nil {
			return err
		}
	}

	sslConfigFile := filepath.Join(workDir, "userdata", string(m.Environment), "volumes", "ssl", "config.yml")
	if _, err := os.Stat(sslConfigFile); err == nil {
		if err := client.MkdirAllSudoWithPerm(remoteDir+"/ssl-config", "755"); err != nil {
			return fmt.Errorf("create ssl-config directory: %w", err)
		}
		content, err := os.ReadFile(sslConfigFile)
		if err != nil {
			return fmt.Errorf("read ssl config file: %w", err)
		}
		if err := m.syncContent(client, string(content), remoteDir+"/ssl-config/config.yml"); err != nil {
			return err
		}
	}

	return nil
}

func (m *Model) syncContent(client *ssh.Client, content, remotePath string) error {
	tmpFile, err := os.CreateTemp("", constants.TempFilePattern)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	return client.UploadFileSudo(tmpFile.Name(), remotePath)
}

func (m Model) hasSelectedRestartServices() bool {
	return m.hasSelectedServices()
}

func (m Model) renderServiceRestart() string {
	return m.renderServiceSelector("Service Restart", "Space select  Enter expand  a current  n cancel  A all  N none  p confirm restart  Esc back  q quit")
}

func (m Model) countSelectedForRestart() int {
	return m.countSelectedServices()
}

func (m Model) countTotalForRestart() int {
	return m.countTotalServices()
}

func (m Model) renderServiceRestartConfirm() string {
	selectedCount := m.countSelectedForRestart()
	var description string
	if selectedCount == 0 {
		description = "  No services selected."
	} else {
		description = fmt.Sprintf("  You are about to restart %d service(s).\n  This will restart containers without removing them.", selectedCount)
	}
	return m.renderConfirmDialog("Confirm Restart Services", description)
}

func (m Model) renderServiceRestartComplete() string {
	results := convertToServiceOpResultsFromRestart(m.Restart.RestartResults)
	return m.renderOperationComplete("Restart Complete", results, "restarted")
}

func convertToServiceOpResultsFromRestart(restartResults []RestartResult) []ServiceOpResult {
	results := make([]ServiceOpResult, len(restartResults))
	for i, rr := range restartResults {
		results[i] = ServiceOpResult{
			ServerName: rr.ServerName,
			Services:   make([]ServiceOpDetail, len(rr.Services)),
		}
		for j, s := range rr.Services {
			results[i].Services[j] = ServiceOpDetail{
				Name:    s.Name,
				Success: s.Success,
				Error:   s.Error,
			}
		}
	}
	return results
}
