package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func (m *Model) applyServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Stop.ServiceStatusMap, string(m.Environment))
}

func (m *Model) executeServiceStopAsync() tea.Cmd {
	return func() tea.Msg {
		var results []StopResult
		secrets := m.Config.GetSecretsMap()

		servicesToStop := getSelectedServicesInfo(m.Tree.TreeNodes)
		if len(servicesToStop) == 0 {
			return serviceStopCompleteMsg{results: results}
		}

		serverServices := make(map[string][]string)
		for _, svc := range servicesToStop {
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
				cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml down 2>/dev/null || true", remoteDir)
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
	for _, node := range m.Tree.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func (m Model) renderServiceStop() string {
	lines := renderTreeNodes(m.Tree.TreeNodes, m.Tree.CursorIndex, false)

	availableHeight := m.UI.Height - 10
	if availableHeight < 5 {
		availableHeight = 5
	}

	treeHeight := availableHeight - 2
	if treeHeight < 3 {
		treeHeight = 3
	}

	totalNodes := len(lines)
	viewport := NewViewport(m.Tree.CursorIndex, totalNodes, treeHeight)
	viewport.EnsureCursorVisible()
	m.UI.ScrollOffset = viewport.Offset

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Service Stop") + "\n\n")

	selected := m.countSelectedForStop()
	total := m.countTotalForStop()
	sb.WriteString(fmt.Sprintf("  Selected: %d/%d\n\n", selected, total))

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		sb.WriteString("  " + lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	sb.WriteString("\n" + HelpStyle.Render("  Space select  Enter expand  a current  n cancel  A all  N none  p confirm stop  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) countSelectedForStop() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotalForStop() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountTotal()
	}
	return count
}

func (m Model) renderServiceStopConfirm() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Confirm Stop Services") + "\n\n")

	selectedCount := m.countSelectedForStop()
	if selectedCount == 0 {
		sb.WriteString("  No services selected.\n")
	} else {
		sb.WriteString(fmt.Sprintf("  You are about to stop %d service(s).\n", selectedCount))
	}
	sb.WriteString("  This will only stop containers, data will be preserved.\n\n")

	options := []string{"Yes, proceed", "Cancel"}
	for i, opt := range options {
		if i == m.Action.ConfirmSelected {
			sb.WriteString(MenuSelectedStyle.Render("  > "+opt) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("    "+opt) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceStopComplete() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Stop Complete") + "\n\n")

	if len(m.Stop.StopResults) > 0 {
		for _, result := range m.Stop.StopResults {
			if len(result.Services) > 0 {
				sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
				for _, svc := range result.Services {
					if svc.Success {
						sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ stopped: %s", svc.Name)) + "\n")
					} else {
						sb.WriteString(ChangeDeleteStyle.Render(fmt.Sprintf("    ✗ failed: %s - %s", svc.Name, svc.Error)) + "\n")
					}
				}
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  Enter back  q quit"))

	return BaseStyle.Render(sb.String())
}
