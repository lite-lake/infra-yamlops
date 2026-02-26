package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

type ServiceOpType int

const (
	ServiceOpStop ServiceOpType = iota
	ServiceOpRestart
	ServiceOpCleanup
)

type ServiceOpResult struct {
	ServerName string
	Services   []ServiceOpDetail
}

type ServiceOpDetail struct {
	Name    string
	Success bool
	Error   string
}

type ServiceOperationConfig struct {
	OpType       ServiceOpType
	ExecuteFunc  func(client *ssh.Client, svcName, remoteDir string) (string, error)
	SuccessVerb  string
	LoadingTitle string
}

func (c ServiceOperationConfig) resultKey() string {
	switch c.OpType {
	case ServiceOpStop:
		return "stop"
	case ServiceOpRestart:
		return "restart"
	default:
		return "operation"
	}
}

func (m *Model) executeServiceOperationAsync(config ServiceOperationConfig) tea.Cmd {
	return func() tea.Msg {
		var results []ServiceOpResult
		secrets := m.Config.GetSecretsMap()

		servicesToProcess := getSelectedServicesInfo(m.Tree.TreeNodes)
		if len(servicesToProcess) == 0 {
			return serviceOpCompleteMsg{results: results, config: config}
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

			result := ServiceOpResult{ServerName: srv.Name}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				for _, svcName := range services {
					result.Services = append(result.Services, ServiceOpDetail{
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
					result.Services = append(result.Services, ServiceOpDetail{
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
				stderr, err := config.ExecuteFunc(client, svcName, remoteDir)
				if err != nil {
					result.Services = append(result.Services, ServiceOpDetail{
						Name:    svcName,
						Success: false,
						Error:   stderr,
					})
				} else {
					result.Services = append(result.Services, ServiceOpDetail{
						Name:    svcName,
						Success: true,
					})
				}
			}

			client.Close()
			results = append(results, result)
		}

		return serviceOpCompleteMsg{results: results, config: config}
	}
}

type serviceOpCompleteMsg struct {
	results []ServiceOpResult
	config  ServiceOperationConfig
}

func (m Model) renderServiceSelector(title, helpText string) string {
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
	sb.WriteString(TitleStyle.Render("  "+title) + "\n\n")

	selected := m.countSelectedServices()
	total := m.countTotalServices()
	sb.WriteString(fmt.Sprintf("  Selected: %d/%d\n\n", selected, total))

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		sb.WriteString("  " + lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	sb.WriteString("\n" + HelpStyle.Render("  "+helpText))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderConfirmDialog(title, description string) string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  "+title) + "\n\n")

	sb.WriteString(description + "\n\n")

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

func (m Model) renderOperationComplete(title string, results []ServiceOpResult, successVerb string) string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  "+title) + "\n\n")

	if len(results) > 0 {
		for _, result := range results {
			if len(result.Services) > 0 {
				sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
				for _, svc := range result.Services {
					if svc.Success {
						sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ %s: %s", successVerb, svc.Name)) + "\n")
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

func (m Model) countSelectedServices() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotalServices() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountTotal()
	}
	return count
}

func (m Model) hasSelectedServices() bool {
	for _, node := range m.Tree.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func stopServiceCommand(remoteDir string) string {
	return fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml stop 2>&1", remoteDir)
}

func restartServiceCommand(remoteDir string) string {
	return fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml restart 2>&1", remoteDir)
}
