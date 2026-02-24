package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func (m *Model) buildStopTree() {
	m.loadConfig()
	if m.Config == nil {
		return
	}
	m.Tree.TreeNodes = m.buildAppTree()
	m.Stop.StopSelected = make(map[int]bool)
	for _, node := range m.Tree.TreeNodes {
		node.SelectRecursive(false)
	}
	m.fetchServiceStatus()
	m.applyServiceStatusToTree()
}

func (m *Model) fetchServiceStatus() {
	m.Stop.ServiceStatusMap = make(map[string]NodeStatus)
	if m.Config == nil {
		return
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
		client.Close()

		if err != nil || stdout == "" {
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
			m.Stop.ServiceStatusMap[proj.Name] = StatusRunning
		}

		for _, infra := range m.Config.InfraServices {
			if infra.Server != srv.Name {
				continue
			}
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, infra.Name))
			key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, infra.Name)
			if _, exists := m.Stop.ServiceStatusMap[key]; !exists {
				stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if strings.TrimSpace(stdout) == "exists" {
					m.Stop.ServiceStatusMap[key] = StatusStopped
				}
			}
		}

		for _, svc := range m.Config.Services {
			if svc.Server != srv.Name {
				continue
			}
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, m.Environment, svc.Name))
			key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, svc.Name)
			if _, exists := m.Stop.ServiceStatusMap[key]; !exists {
				stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if strings.TrimSpace(stdout) == "exists" {
					m.Stop.ServiceStatusMap[key] = StatusStopped
				}
			}
		}
	}
}

func (m *Model) fetchServiceStatusAsync() tea.Cmd {
	return func() tea.Msg {
		statusMap := make(map[string]NodeStatus)
		if m.Config == nil {
			return serviceStatusFetchedMsg{statusMap: statusMap}
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
					stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
					if strings.TrimSpace(stdout) == "exists" {
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
					stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
					if strings.TrimSpace(stdout) == "exists" {
						statusMap[key] = StatusStopped
					}
				}
			}

			client.Close()
		}

		return serviceStatusFetchedMsg{statusMap: statusMap}
	}
}

func (m *Model) applyServiceStatusToTree() {
	for _, node := range m.Tree.TreeNodes {
		m.applyStatusToNode(node)
	}
}

func (m *Model) applyStatusToNode(node *TreeNode) {
	if node.Type == NodeTypeInfra || node.Type == NodeTypeBiz {
		key := fmt.Sprintf(constants.ServicePrefixFormat, m.Environment, node.Name)
		if status, exists := m.Stop.ServiceStatusMap[key]; exists {
			node.Status = status
		}
	}
	for _, child := range node.Children {
		m.applyStatusToNode(child)
	}
}

func (m *Model) executeServiceStop() {
	m.Stop.StopResults = nil
	secrets := m.Config.GetSecretsMap()

	servicesToStop := m.getSelectedServicesForStop()
	if len(servicesToStop) == 0 {
		return
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
			m.Stop.StopResults = append(m.Stop.StopResults, result)
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
			m.Stop.StopResults = append(m.Stop.StopResults, result)
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
		m.Stop.StopResults = append(m.Stop.StopResults, result)
	}
}

func (m *Model) executeServiceStopAsync() tea.Cmd {
	return func() tea.Msg {
		var results []StopResult
		secrets := m.Config.GetSecretsMap()

		servicesToStop := m.getSelectedServicesForStop()
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

type serviceInfo struct {
	Name   string
	Server string
}

func (m *Model) getSelectedServicesForStop() []serviceInfo {
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

func (m Model) hasSelectedStopServices() bool {
	for _, node := range m.Tree.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func (m Model) renderServiceStop() string {
	var lines []string
	idx := 0
	for _, node := range m.Tree.TreeNodes {
		m.renderNodeToLinesForStop(node, 0, &idx, &lines)
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

func (m Model) renderNodeToLinesForStop(node *TreeNode, depth int, idx *int, lines *[]string) {
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
				m.renderNodeLastChildToLinesForStop(child, depth+1, idx, lines)
			} else {
				m.renderNodeToLinesForStop(child, depth+1, idx, lines)
			}
		}
	}
}

func (m Model) renderNodeLastChildToLinesForStop(node *TreeNode, depth int, idx *int, lines *[]string) {
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
				m.renderNodeLastChildToLinesForStop(child, depth+1, idx, lines)
			} else {
				m.renderNodeToLinesForStop(child, depth+1, idx, lines)
			}
		}
	}
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
