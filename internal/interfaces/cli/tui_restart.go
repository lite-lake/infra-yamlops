package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
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
	config := ServiceOperationConfig{
		OpType: ServiceOpRestart,
		ExecuteFunc: func(client *ssh.Client, svcName, remoteDir string) (string, error) {
			cmd := restartServiceCommand(remoteDir)
			_, stderr, err := client.Run(cmd)
			return stderr, err
		},
		SuccessVerb:  "restarted",
		LoadingTitle: "Restarting Services...",
	}
	return m.executeServiceOperationAsync(config)
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
