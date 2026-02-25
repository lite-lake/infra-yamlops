package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func (m *Model) applyServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Stop.ServiceStatusMap, string(m.Environment))
}

func (m *Model) executeServiceStopAsync() tea.Cmd {
	config := ServiceOperationConfig{
		OpType: ServiceOpStop,
		ExecuteFunc: func(client *ssh.Client, svcName, remoteDir string) (string, error) {
			cmd := stopServiceCommand(remoteDir)
			_, stderr, err := client.Run(cmd)
			return stderr, err
		},
		SuccessVerb:  "stopped",
		LoadingTitle: "Stopping Services...",
	}
	return m.executeServiceOperationAsync(config)
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
