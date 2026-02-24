package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/domain/entity"
	serverpkg "github.com/litelake/yamlops/internal/environment"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

var serverEnvOperations = []string{"Check", "Sync", "Full Setup"}

func (m *Model) initServerEnvNodes() {
	m.ServerEnv.Nodes = nil
	for _, srv := range m.Server.ServerList {
		m.ServerEnv.Nodes = append(m.ServerEnv.Nodes, &ServerEnvNode{
			Name:     srv.Name,
			Zone:     srv.Zone,
			Selected: false,
			Expanded: false,
			Server:   srv,
		})
	}
	m.ServerEnv.CursorIndex = 0
	m.ServerEnv.OperationIndex = 0
	m.ServerEnv.Results = nil
	m.ServerEnv.SyncResults = nil
}

func (m Model) renderServerEnvSetup() string {
	availableHeight := m.UI.Height - 12
	if availableHeight < 5 {
		availableHeight = 5
	}

	treeHeight := availableHeight - 2
	if treeHeight < 3 {
		treeHeight = 3
	}

	var lines []string
	for _, node := range m.ServerEnv.Nodes {
		m.renderServerEnvNode(node, &lines)
	}

	totalLines := len(lines)
	viewport := NewViewport(m.ServerEnv.CursorIndex, totalLines, treeHeight)
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Server Environment Setup") + "\n\n")

	sb.WriteString(TabActiveStyle.Render("  ▸ Select Servers:") + "\n")

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		sb.WriteString("  " + lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("  " + viewport.RenderSimpleScrollIndicator() + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + strings.Repeat("─", 40) + "\n")

	sb.WriteString("\n" + TabActiveStyle.Render("  ▸ Operation:") + "  ")

	for i, op := range serverEnvOperations {
		if i == m.ServerEnv.OperationIndex {
			sb.WriteString(SelectedStyle.Render("["+op+"]") + "  ")
		} else {
			sb.WriteString(MenuItemStyle.Render(op) + "  ")
		}
	}
	sb.WriteString("\n")

	selectedCount := m.ServerEnv.CountSelected()
	sb.WriteString(fmt.Sprintf("\n  Selected: %d server(s)\n", selectedCount))

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Space select  Enter expand/execute  Tab operation  a all  n none  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServerEnvNode(node *ServerEnvNode, lines *[]string) {
	cursor := " "
	lineIdx := len(*lines)
	if lineIdx == m.ServerEnv.CursorIndex {
		cursor = ">"
	}

	selectIcon := "○"
	if node.Selected {
		selectIcon = "◉"
	}

	expandIcon := "▸"
	if node.Expanded {
		expandIcon = "▾"
	}

	line := fmt.Sprintf("%s %s %s %s (%s)", cursor, selectIcon, expandIcon, node.Name, node.Zone)
	if lineIdx == m.ServerEnv.CursorIndex {
		line = SelectedStyle.Render(line)
	}
	*lines = append(*lines, line)

	if node.Expanded && m.ServerEnv.Results != nil {
		if results, ok := m.ServerEnv.Results[node.Name]; ok {
			for _, r := range results {
				icon := "✓"
				style := SuccessStyle
				if r.Status != serverpkg.CheckStatusOK {
					icon = "✗"
					style = ChangeDeleteStyle
				}
				detailLine := fmt.Sprintf("      %s %s: %s", icon, r.Name, r.Message)
				*lines = append(*lines, style.Render(detailLine))
			}
		}
	}
}

func (m Model) renderServerEnvResults() string {
	var lines []string

	checkResults := m.ServerEnv.Results
	syncResults := m.ServerEnv.SyncResults

	if len(checkResults) > 0 {
		lines = append(lines, "")
		for _, node := range m.ServerEnv.Nodes {
			results, ok := checkResults[node.Name]
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("  [%s]", node.Name))
			for _, r := range results {
				icon := "✓"
				style := SuccessStyle
				if r.Status != serverpkg.CheckStatusOK {
					icon = "✗"
					style = ChangeDeleteStyle
				}
				lines = append(lines, style.Render(fmt.Sprintf("    %s %s: %s", icon, r.Name, r.Message)))
			}
			lines = append(lines, "")
		}
	}

	if len(syncResults) > 0 {
		lines = append(lines, "  --- Sync Results ---")
		for _, node := range m.ServerEnv.Nodes {
			results, ok := syncResults[node.Name]
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("  [%s]", node.Name))
			for _, r := range results {
				icon := "✓"
				style := SuccessStyle
				if !r.Success {
					icon = "✗"
					style = ChangeDeleteStyle
				}
				lines = append(lines, style.Render(fmt.Sprintf("    %s %s: %s", icon, r.Name, r.Message)))
			}
			lines = append(lines, "")
		}
	}

	totalChecks := 0
	passedChecks := 0
	for _, results := range checkResults {
		for _, r := range results {
			totalChecks++
			if r.Status == serverpkg.CheckStatusOK {
				passedChecks++
			}
		}
	}
	totalSyncs := 0
	passedSyncs := 0
	for _, results := range syncResults {
		for _, r := range results {
			totalSyncs++
			if r.Success {
				passedSyncs++
			}
		}
	}

	summaryParts := []string{}
	if totalChecks > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("Check: %d/%d passed", passedChecks, totalChecks))
	}
	if totalSyncs > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("Sync: %d/%d passed", passedSyncs, totalSyncs))
	}
	if len(summaryParts) > 0 {
		lines = append(lines, fmt.Sprintf("  Summary: %s", strings.Join(summaryParts, ", ")))
	}

	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	viewport := NewViewport(0, totalLines, availableHeight)
	viewport.Offset = m.ServerEnv.ResultsScrollY
	maxOffset := max(0, totalLines-viewport.VisibleRows)
	if viewport.Offset > maxOffset {
		viewport.Offset = maxOffset
		m.ServerEnv.ResultsScrollY = viewport.Offset
	}
	if viewport.Offset < 0 {
		viewport.Offset = 0
		m.ServerEnv.ResultsScrollY = 0
	}

	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Environment Results") + "\n")

	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(lines); i++ {
		sb.WriteString(lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ scroll  Enter back  r re-check  s sync selected  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) countServerEnvLines() int {
	count := 0
	for _, node := range m.ServerEnv.Nodes {
		count++
		if node.Expanded && m.ServerEnv.Results != nil {
			if results, ok := m.ServerEnv.Results[node.Name]; ok {
				count += len(results)
			}
		}
	}
	return count
}

func (m Model) countServerEnvResultLines() int {
	count := 0

	checkResults := m.ServerEnv.Results
	syncResults := m.ServerEnv.SyncResults

	if len(checkResults) > 0 {
		count++
		for _, node := range m.ServerEnv.Nodes {
			results, ok := checkResults[node.Name]
			if !ok {
				continue
			}
			count++
			count += len(results)
			count++
		}
	}

	if len(syncResults) > 0 {
		count++
		for _, node := range m.ServerEnv.Nodes {
			results, ok := syncResults[node.Name]
			if !ok {
				continue
			}
			count++
			count += len(results)
			count++
		}
	}

	count++
	return count
}

func (m *Model) executeServerEnvCheckAsync() tea.Cmd {
	return func() tea.Msg {
		servers := m.ServerEnv.GetSelectedServers()
		if len(servers) == 0 {
			return serverEnvCheckAllMsg{}
		}

		results := make(map[string][]serverpkg.CheckResult)
		secrets := m.Config.GetSecretsMap()

		registries := make([]entity.Registry, 0, len(m.Config.Registries))
		for i := range m.Config.Registries {
			registries = append(registries, m.Config.Registries[i])
		}

		for _, srv := range servers {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				results[srv.Name] = []serverpkg.CheckResult{{
					Name:    "Connection",
					Status:  serverpkg.CheckStatusError,
					Message: fmt.Sprintf("Cannot resolve password: %v", err),
				}}
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				results[srv.Name] = []serverpkg.CheckResult{{
					Name:    "Connection",
					Status:  serverpkg.CheckStatusError,
					Message: fmt.Sprintf("Connection failed: %v", err),
				}}
				continue
			}

			checker := serverpkg.NewChecker(client, srv, registries, secrets)
			results[srv.Name] = checker.CheckAll()
			client.Close()
		}

		return serverEnvCheckAllMsg{results: results}
	}
}

func (m *Model) executeServerEnvSyncAsync() tea.Cmd {
	return func() tea.Msg {
		servers := m.ServerEnv.GetSelectedServers()
		if len(servers) == 0 {
			return serverEnvSyncAllMsg{}
		}

		results := make(map[string][]serverpkg.SyncResult)
		secrets := m.Config.GetSecretsMap()

		registries := make([]entity.Registry, 0, len(m.Config.Registries))
		for i := range m.Config.Registries {
			registries = append(registries, m.Config.Registries[i])
		}

		for _, srv := range servers {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				results[srv.Name] = []serverpkg.SyncResult{{
					Name:    "Connection",
					Success: false,
					Message: fmt.Sprintf("Cannot resolve password: %v", err),
				}}
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				results[srv.Name] = []serverpkg.SyncResult{{
					Name:    "Connection",
					Success: false,
					Message: fmt.Sprintf("Connection failed: %v", err),
				}}
				continue
			}

			syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), secrets, registries)
			results[srv.Name] = syncer.SyncAll()
			client.Close()
		}

		return serverEnvSyncAllMsg{results: results}
	}
}

func (m *Model) executeServerEnvFullSetupAsync() tea.Cmd {
	return func() tea.Msg {
		servers := m.ServerEnv.GetSelectedServers()
		if len(servers) == 0 {
			return serverEnvCheckAllMsg{}
		}

		checkResults := make(map[string][]serverpkg.CheckResult)
		syncResults := make(map[string][]serverpkg.SyncResult)
		secrets := m.Config.GetSecretsMap()

		registries := make([]entity.Registry, 0, len(m.Config.Registries))
		for i := range m.Config.Registries {
			registries = append(registries, m.Config.Registries[i])
		}

		for _, srv := range servers {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				checkResults[srv.Name] = []serverpkg.CheckResult{{
					Name:    "Connection",
					Status:  serverpkg.CheckStatusError,
					Message: fmt.Sprintf("Cannot resolve password: %v", err),
				}}
				continue
			}

			client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
			if err != nil {
				checkResults[srv.Name] = []serverpkg.CheckResult{{
					Name:    "Connection",
					Status:  serverpkg.CheckStatusError,
					Message: fmt.Sprintf("Connection failed: %v", err),
				}}
				continue
			}

			checker := serverpkg.NewChecker(client, srv, registries, secrets)
			checkResults[srv.Name] = checker.CheckAll()

			syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), secrets, registries)
			syncResults[srv.Name] = syncer.SyncAll()

			client.Close()
		}

		return serverEnvCheckAllMsg{results: checkResults, syncResults: syncResults}
	}
}

func (m *Model) getServerEnvNodeAtIndex(idx int) *ServerEnvNode {
	currentIdx := 0
	for _, node := range m.ServerEnv.Nodes {
		if currentIdx == idx {
			return node
		}
		currentIdx++
		if node.Expanded && m.ServerEnv.Results != nil {
			if results, ok := m.ServerEnv.Results[node.Name]; ok {
				if currentIdx+len(results) > idx {
					return node
				}
				currentIdx += len(results)
			}
		}
	}
	return nil
}

func (m *Model) countServerEnvNodes() int {
	return m.countServerEnvLines()
}
