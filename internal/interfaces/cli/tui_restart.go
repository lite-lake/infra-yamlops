package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func (m *Model) fetchRestartServiceStatusAsync() tea.Cmd {
	return func() tea.Msg {
		statusMap := make(map[string]NodeStatus)
		if m.Config == nil {
			return restartStatusFetchedMsg{statusMap: statusMap}
		}
		secrets := m.Config.GetSecretsMap()

		for _, srv := range m.Config.Servers {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				continue
			}

			stdout, _, err := client.Run("sudo docker compose ls -a --format json 2>/dev/null || sudo docker compose ls -a --format json")
			if err != nil || stdout == "" {
				client.Close()
				continue
			}

			type composeProject struct {
				Name string `json:"Name"`
			}
			var projects []composeProject
			if err := json.Unmarshal([]byte(stdout), &projects); err != nil {
				for _, line := range strings.Split(stdout, "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					var proj composeProject
					if err := json.Unmarshal([]byte(line), &proj); err == nil && proj.Name != "" {
						projects = append(projects, proj)
					}
				}
			}

			for _, proj := range projects {
				statusMap[proj.Name] = StatusRunning
			}

			for _, infra := range m.Config.InfraServices {
				if infra.Server != srv.Name {
					continue
				}
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, infra.Name))
				key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, infra.Name)
				if _, exists := statusMap[key]; !exists {
					stdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
					if err != nil {
						statusMap[key] = StatusError
					} else if strings.TrimSpace(stdout) == "exists" {
						statusMap[key] = StatusStopped
					}
				}
			}

			for _, svc := range m.Config.Services {
				if svc.Server != srv.Name {
					continue
				}
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, svc.Name))
				key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, svc.Name)
				if _, exists := statusMap[key]; !exists {
					stdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
					if err != nil {
						statusMap[key] = StatusError
					} else if strings.TrimSpace(stdout) == "exists" {
						statusMap[key] = StatusStopped
					}
				}
			}

			client.Close()
		}

		return restartStatusFetchedMsg{statusMap: statusMap}
	}
}

func (m *Model) applyRestartServiceStatusToTree() {
	for _, node := range m.Tree.TreeNodes {
		m.applyRestartStatusToNode(node)
	}
}

func (m *Model) applyRestartStatusToNode(node *TreeNode) {
	if node.Type == NodeTypeInfra || node.Type == NodeTypeBiz {
		key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, node.Name)
		if status, exists := m.Restart.ServiceStatusMap[key]; exists {
			node.Status = status
		}
	}
	for _, child := range node.Children {
		m.applyRestartStatusToNode(child)
	}
}

func (m *Model) executeServiceRestartAsync() tea.Cmd {
	return func() tea.Msg {
		var results []RestartResult
		secrets := m.Config.GetSecretsMap()

		servicesToRestart := m.getSelectedServicesForRestart()
		if len(servicesToRestart) == 0 {
			return serviceRestartCompleteMsg{results: results}
		}

		serverServices := make(map[string][]string)
		for _, svc := range servicesToRestart {
			if svc.Server != "" {
				serverServices[svc.Server] = append(serverServices[svc.Server], svc.Name)
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
				for _, svcName := range services {
					result.Services = append(result.Services, RestartServiceResult{
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
					result.Services = append(result.Services, RestartServiceResult{
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
				cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml restart 2>&1", remoteDir)
				_, stderr, err := client.Run(cmd)
				if err != nil {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcName,
						Success: false,
						Error:   stderr,
					})
				} else {
					result.Services = append(result.Services, RestartServiceResult{
						Name:    svcName,
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

func (m *Model) getSelectedServicesForRestart() []serviceInfo {
	var services []serviceInfo
	serviceSet := make(map[string]bool)

	for _, node := range m.Tree.TreeNodes {
		leaves := node.GetSelectedLeaves()
		for _, leaf := range leaves {
			var serverName string
			if leaf.Parent != nil {
				serverName = leaf.Parent.Name
			}
			switch leaf.Type {
			case NodeTypeInfra:
				if !serviceSet[leaf.Name] {
					services = append(services, serviceInfo{Name: leaf.Name, Server: serverName})
					serviceSet[leaf.Name] = true
				}
			case NodeTypeBiz:
				if !serviceSet[leaf.Name] {
					services = append(services, serviceInfo{Name: leaf.Name, Server: serverName})
					serviceSet[leaf.Name] = true
				}
			}
		}
	}
	return services
}

func (m Model) hasSelectedRestartServices() bool {
	for _, node := range m.Tree.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func (m Model) renderServiceRestart() string {
	var lines []string
	idx := 0
	for _, node := range m.Tree.TreeNodes {
		m.renderNodeToLinesForRestart(node, 0, &idx, &lines)
	}

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
	sb.WriteString(TitleStyle.Render("  Service Restart") + "\n\n")

	selected := m.countSelectedForRestart()
	total := m.countTotalForRestart()
	sb.WriteString(fmt.Sprintf("  Selected: %d/%d\n\n", selected, total))

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		sb.WriteString("  " + lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	sb.WriteString("\n" + HelpStyle.Render("  Space select  Enter expand  a current  n cancel  A all  N none  p confirm restart  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderNodeToLinesForRestart(node *TreeNode, depth int, idx *int, lines *[]string) {
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		prefix = indent[:len(indent)-2] + "├─"
	}
	cursor := "  "
	if *idx == m.Tree.CursorIndex {
		cursor = "> "
	}
	selectIcon := "○"
	if node.Selected {
		selectIcon = "◉"
	} else if node.IsPartiallySelected() {
		selectIcon = "◐"
	}
	expandIcon := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIcon = "▾"
		} else {
			expandIcon = "▸"
		}
	}
	typePrefix := ""
	switch node.Type {
	case NodeTypeInfra:
		typePrefix = "[infra] "
	case NodeTypeBiz:
		typePrefix = "[biz] "
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, selectIcon, expandIcon, typePrefix, node.Name)
	if statusStr := formatNodeStatus(node.Status); statusStr != "" {
		line = fmt.Sprintf("%s %s", line, statusStr)
	}
	if *idx == m.Tree.CursorIndex {
		line = SelectedStyle.Render(line)
	}
	*lines = append(*lines, line)
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				m.renderNodeLastChildToLinesForRestart(child, depth+1, idx, lines)
			} else {
				m.renderNodeToLinesForRestart(child, depth+1, idx, lines)
			}
		}
	}
}

func (m Model) renderNodeLastChildToLinesForRestart(node *TreeNode, depth int, idx *int, lines *[]string) {
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		prefix = indent[:len(indent)-2] + "└─"
	}
	cursor := "  "
	if *idx == m.Tree.CursorIndex {
		cursor = "> "
	}
	selectIcon := "○"
	if node.Selected {
		selectIcon = "◉"
	} else if node.IsPartiallySelected() {
		selectIcon = "◐"
	}
	expandIcon := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIcon = "▾"
		} else {
			expandIcon = "▸"
		}
	}
	typePrefix := ""
	switch node.Type {
	case NodeTypeInfra:
		typePrefix = "[infra] "
	case NodeTypeBiz:
		typePrefix = "[biz] "
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, selectIcon, expandIcon, typePrefix, node.Name)
	if statusStr := formatNodeStatus(node.Status); statusStr != "" {
		line = fmt.Sprintf("%s %s", line, statusStr)
	}
	if *idx == m.Tree.CursorIndex {
		line = SelectedStyle.Render(line)
	}
	*lines = append(*lines, line)
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				m.renderNodeLastChildToLinesForRestart(child, depth+1, idx, lines)
			} else {
				m.renderNodeToLinesForRestart(child, depth+1, idx, lines)
			}
		}
	}
}

func (m Model) countSelectedForRestart() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotalForRestart() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountTotal()
	}
	return count
}

func (m Model) renderServiceRestartConfirm() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Confirm Restart Services") + "\n\n")

	selectedCount := m.countSelectedForRestart()
	if selectedCount == 0 {
		sb.WriteString("  No services selected.\n")
	} else {
		sb.WriteString(fmt.Sprintf("  You are about to restart %d service(s).\n", selectedCount))
	}
	sb.WriteString("  This will restart containers without removing them.\n\n")

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

func (m Model) renderServiceRestartComplete() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Restart Complete") + "\n\n")

	if len(m.Restart.RestartResults) > 0 {
		for _, result := range m.Restart.RestartResults {
			if len(result.Services) > 0 {
				sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
				for _, svc := range result.Services {
					if svc.Success {
						sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ restarted: %s", svc.Name)) + "\n")
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
