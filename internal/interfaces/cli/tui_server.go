package cli

import (
	"fmt"
	"strings"

	serverpkg "github.com/litelake/yamlops/internal/environment"
	"github.com/litelake/yamlops/internal/ssh"
)

func (m Model) renderServerSetup() string {
	actions := []string{"Check Environment", "Sync Environment", "Full Setup (Check + Sync)", "Back to Menu"}

	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	serverListHeight := availableHeight - len(actions) - 4
	if serverListHeight < 3 {
		serverListHeight = 3
	}

	totalServers := len(m.Server.ServerList)
	serverViewport := NewViewport(0, totalServers, serverListHeight)
	serverViewport.CursorIndex = m.Server.ServerIndex
	serverViewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render("  Server Setup")
	sb.WriteString(title + "\n\n")

	sb.WriteString("  Select Server:\n")
	for i := serverViewport.VisibleStart(); i < serverViewport.VisibleEnd() && i < totalServers; i++ {
		srv := m.Server.ServerList[i]
		prefix := "  "
		focusPrefix := ""
		if m.Server.ServerFocusPanel == 0 && i == m.Server.ServerIndex {
			focusPrefix = "> "
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s (%s)\n", focusPrefix, prefix, srv.Name, srv.Zone))
	}

	if serverViewport.TotalRows > serverViewport.VisibleRows {
		sb.WriteString("  " + serverViewport.RenderSimpleScrollIndicator() + "\n")
	}

	sb.WriteString("\n  Actions:\n")
	for i, action := range actions {
		prefix := "  "
		focusPrefix := ""
		if m.Server.ServerFocusPanel == 1 && i == m.Server.ServerAction {
			focusPrefix = "> "
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", focusPrefix, prefix, action))
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Tab switch  Enter select  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServerCheck() string {
	var lines []string

	if len(m.Server.ServerCheckResults) > 0 && m.Server.ServerIndex < len(m.Server.ServerList) {
		name := m.Server.ServerList[m.Server.ServerIndex].Name
		lines = append(lines, serverpkg.FormatResults(name, m.Server.ServerCheckResults))
	}

	if len(m.Server.ServerSyncResults) > 0 {
		lines = append(lines, "")
		for _, r := range m.Server.ServerSyncResults {
			icon := "✅"
			if !r.Success {
				icon = "❌"
			}
			lines = append(lines, fmt.Sprintf("  %s %s: %s", icon, r.Name, r.Message))
		}
	}

	lines = append(lines, "")
	lines = append(lines, HelpStyle.Render("  Esc back  q quit"))

	availableHeight := m.UI.Height - 4
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	viewport := NewViewport(0, totalLines, availableHeight)
	viewport.Offset = m.UI.ScrollOffset
	maxOffset := max(0, totalLines-viewport.VisibleRows)
	if viewport.Offset > maxOffset {
		viewport.Offset = maxOffset
	}
	m.UI.ScrollOffset = viewport.Offset

	var sb strings.Builder
	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(lines); i++ {
		sb.WriteString(lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m *Model) executeServerCheck() {
	if len(m.Server.ServerList) == 0 || m.Server.ServerIndex >= len(m.Server.ServerList) {
		return
	}

	srv := m.Server.ServerList[m.Server.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	checker := serverpkg.NewChecker(client, srv, convertRegistries(m.Config.Registries), secrets)
	m.Server.ServerCheckResults = checker.CheckAll()
	m.Server.ServerSyncResults = nil
	m.ViewState = ViewStateServerCheck
}

func (m *Model) executeServerSync() {
	if len(m.Server.ServerList) == 0 || m.Server.ServerIndex >= len(m.Server.ServerList) {
		return
	}

	srv := m.Server.ServerList[m.Server.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), convertRegistries(m.Config.Registries), secrets)
	m.Server.ServerSyncResults = syncer.SyncAll()
	m.Server.ServerCheckResults = nil
	m.ViewState = ViewStateServerCheck
}

func (m *Model) executeServerFullSetup() {
	if len(m.Server.ServerList) == 0 || m.Server.ServerIndex >= len(m.Server.ServerList) {
		return
	}

	srv := m.Server.ServerList[m.Server.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	checker := serverpkg.NewChecker(client, srv, convertRegistries(m.Config.Registries), secrets)
	m.Server.ServerCheckResults = checker.CheckAll()

	syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), convertRegistries(m.Config.Registries), secrets)
	m.Server.ServerSyncResults = syncer.SyncAll()

	m.ViewState = ViewStateServerCheck
}
