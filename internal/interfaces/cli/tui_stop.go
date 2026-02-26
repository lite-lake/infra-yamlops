package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

func (m *Model) applyServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Stop.ServiceStatusMap, string(m.Environment))
}

func (m *Model) executeServiceStopAsync() tea.Cmd {
	return func() tea.Msg {
		results := []StopResult{}
		secrets := m.Config.GetSecretsMap()

		servicesToProcess := getSelectedServicesInfo(m.Tree.TreeNodes)
		if len(servicesToProcess) == 0 {
			return serviceStopCompleteMsg{results: results}
		}

		serverServices := make(map[string][]string)
		for _, svc := range servicesToProcess {
			if svc.Server != "" {
				serverServices[svc.Server] = append(serverServices[svc.Server], svc.Name)
			}
		}

		for _, srv := range m.Server.ServerList {
			services, ok := serverServices[srv.Name]
			if !ok || len(services) == 0 {
				continue
			}

			result := StopResult{ServerName: srv.Name}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				for _, svcName := range services {
					result.Services = append(result.Services, StopServiceResult{
						Name:    svcName,
						Success: false,
						Error:   fmt.Sprintf("Cannot resolve password: %v", err),
					})
				}
				results = append(results, result)
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				for _, svcName := range services {
					result.Services = append(result.Services, StopServiceResult{
						Name:    svcName,
						Success: false,
						Error:   fmt.Sprintf("Connection failed: %v", err),
					})
				}
				results = append(results, result)
				continue
			}

			for _, svcName := range services {
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, svcName))
				cmd := stopServiceCommand(remoteDir)
				_, stderr, err := client.Run(cmd)
				if err != nil {
					result.Services = append(result.Services, StopServiceResult{
						Name:    svcName,
						Success: false,
						Error:   stderr,
					})
				} else {
					result.Services = append(result.Services, StopServiceResult{
						Name:    svcName,
						Success: true,
					})
				}
			}

			client.Close()
			results = append(results, result)
		}

		return serviceStopCompleteMsg{results: results}
	}
}

func (m Model) hasSelectedStopServices() bool {
	return m.hasSelectedServices()
}

func (m Model) renderServiceStop() string {
	return m.renderServiceSelector("Service Stop", "Space select  Enter expand  a current  n cancel  A all  N none  p confirm stop  Esc back  q quit")
}

func (m Model) countSelectedForStop() int {
	return m.countSelectedServices()
}

func (m Model) countTotalForStop() int {
	return m.countTotalServices()
}

func (m Model) renderServiceStopConfirm() string {
	selectedCount := m.countSelectedForStop()
	var description string
	if selectedCount == 0 {
		description = "  No services selected."
	} else {
		description = fmt.Sprintf("  You are about to stop %d service(s).\n  This will only stop containers, data will be preserved.", selectedCount)
	}
	return m.renderConfirmDialog("Confirm Stop Services", description)
}

func (m Model) renderServiceStopComplete() string {
	results := convertToServiceOpResults(m.Stop.StopResults)
	return m.renderOperationComplete("Stop Complete", results, "stopped")
}

func convertToServiceOpResults(stopResults []StopResult) []ServiceOpResult {
	results := make([]ServiceOpResult, len(stopResults))
	for i, sr := range stopResults {
		results[i] = ServiceOpResult{
			ServerName: sr.ServerName,
			Services:   make([]ServiceOpDetail, len(sr.Services)),
		}
		for j, s := range sr.Services {
			results[i].Services[j] = ServiceOpDetail{
				Name:    s.Name,
				Success: s.Success,
				Error:   s.Error,
			}
		}
	}
	return results
}
