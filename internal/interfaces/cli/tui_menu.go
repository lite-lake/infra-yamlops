package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	serverpkg "github.com/litelake/yamlops/internal/server"
	"github.com/litelake/yamlops/internal/ssh"
)

var menuStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED")).
	Bold(true)

var menuItemStyle = lipgloss.NewStyle().
	Padding(0, 2)

var menuSelectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED")).
	Background(lipgloss.Color("#1E1B4B")).
	Padding(0, 2).
	Bold(true)

func (m Model) renderMainMenu() string {
	items := []string{
		"Plan & Apply        基础设施部署",
		"Server Setup        服务器环境设置",
		"DNS Management      域名/DNS管理",
		"Exit                退出",
	}

	availableHeight := m.Height - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(items), availableHeight)
	viewport.CursorIndex = m.MainMenuIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := titleStyle.Render(fmt.Sprintf("  YAMLOps [%s]", strings.ToUpper(string(m.Environment))))
	sb.WriteString(title + "\n\n")

	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(items); i++ {
		if i == m.MainMenuIndex {
			sb.WriteString(menuSelectedStyle.Render("> "+items[i]) + "\n")
		} else {
			sb.WriteString(menuItemStyle.Render("  "+items[i]) + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 选择  Enter 确认  q 退出"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderServerSetup() string {
	actions := []string{"Check Environment", "Sync Environment", "Full Setup (Check + Sync)", "Back to Menu"}

	availableHeight := m.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	serverListHeight := availableHeight - len(actions) - 4
	if serverListHeight < 3 {
		serverListHeight = 3
	}

	totalServers := len(m.ServerList)
	serverViewport := NewViewport(0, totalServers, serverListHeight)
	serverViewport.CursorIndex = m.ServerIndex
	serverViewport.EnsureCursorVisible()

	var sb strings.Builder
	title := titleStyle.Render("  Server Setup")
	sb.WriteString(title + "\n\n")

	sb.WriteString("  Select Server:\n")
	for i := serverViewport.VisibleStart(); i < serverViewport.VisibleEnd() && i < totalServers; i++ {
		srv := m.ServerList[i]
		prefix := "  "
		focusPrefix := ""
		if m.ServerFocusPanel == 0 && i == m.ServerIndex {
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
		if m.ServerFocusPanel == 1 && i == m.ServerAction {
			focusPrefix = "> "
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", focusPrefix, prefix, action))
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 选择  Tab 切换面板  Enter 确认  Esc 返回  q 退出"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderServerCheck() string {
	var lines []string

	if len(m.ServerCheckResults) > 0 && m.ServerIndex < len(m.ServerList) {
		name := m.ServerList[m.ServerIndex].Name
		lines = append(lines, serverpkg.FormatResults(name, m.ServerCheckResults))
	}

	if len(m.ServerSyncResults) > 0 {
		lines = append(lines, "")
		for _, r := range m.ServerSyncResults {
			icon := "✅"
			if !r.Success {
				icon = "❌"
			}
			lines = append(lines, fmt.Sprintf("  %s %s: %s", icon, r.Name, r.Message))
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("  Esc 返回  q 退出"))

	availableHeight := m.Height - 4
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	viewport := NewViewport(0, totalLines, availableHeight)
	viewport.Offset = m.ScrollOffset
	maxOffset := max(0, totalLines-viewport.VisibleRows)
	if viewport.Offset > maxOffset {
		viewport.Offset = maxOffset
	}
	m.ScrollOffset = viewport.Offset

	var sb strings.Builder
	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(lines); i++ {
		sb.WriteString(lines[i] + "\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return baseStyle.Render(sb.String())
}

func (m *Model) executeServerCheck() {
	if len(m.ServerList) == 0 || m.ServerIndex >= len(m.ServerList) {
		return
	}

	srv := m.ServerList[m.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	checker := serverpkg.NewChecker(client, srv, convertRegistries(m.Config.Registries), secrets)
	m.ServerCheckResults = checker.CheckAll()
	m.ServerSyncResults = nil
	m.ViewState = ViewStateServerCheck
}

func (m *Model) executeServerSync() {
	if len(m.ServerList) == 0 || m.ServerIndex >= len(m.ServerList) {
		return
	}

	srv := m.ServerList[m.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), convertRegistries(m.Config.Registries), secrets)
	m.ServerSyncResults = syncer.SyncAll()
	m.ServerCheckResults = nil
	m.ViewState = ViewStateServerCheck
}

func (m *Model) executeServerFullSetup() {
	if len(m.ServerList) == 0 || m.ServerIndex >= len(m.ServerList) {
		return
	}

	srv := m.ServerList[m.ServerIndex]
	secrets := m.Config.GetSecretsMap()

	password, err := srv.SSH.Password.Resolve(secrets)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Cannot resolve password: %v", err)
		return
	}

	client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Connection failed: %v", err)
		return
	}
	defer client.Close()

	checker := serverpkg.NewChecker(client, srv, convertRegistries(m.Config.Registries), secrets)
	m.ServerCheckResults = checker.CheckAll()

	syncer := serverpkg.NewSyncer(client, srv, string(m.Environment), convertRegistries(m.Config.Registries), secrets)
	m.ServerSyncResults = syncer.SyncAll()

	m.ViewState = ViewStateServerCheck
}

func (m Model) renderDNSManagement() string {
	var sb strings.Builder

	title := titleStyle.Render("  DNS Management")
	sb.WriteString(title + "\n\n")

	items := []string{
		"Pull Domains        从ISP拉取域名列表",
		"Pull Records        从域名拉取DNS记录",
		"Plan & Apply        DNS变更计划/执行",
		"Back to Menu        返回主菜单",
	}

	for i, item := range items {
		if i == m.DNSMenuIndex {
			sb.WriteString(menuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(menuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDomains() string {
	isps := m.getDNSISPs()

	availableHeight := m.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(isps), availableHeight)
	viewport.CursorIndex = m.DNSISPIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := titleStyle.Render("  Pull Domains - Select ISP")
	sb.WriteString(title + "\n\n")

	if len(isps) == 0 {
		sb.WriteString("No ISPs with DNS service configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(isps); i++ {
			if i == m.DNSISPIndex {
				sb.WriteString(menuSelectedStyle.Render("> "+isps[i]) + "\n")
			} else {
				sb.WriteString(menuItemStyle.Render("  "+isps[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return baseStyle.Render(sb.String())
}

func (m Model) getDNSISPs() []string {
	var isps []string
	for _, isp := range m.Config.ISPs {
		if isp.HasService(entity.ISPServiceDNS) {
			isps = append(isps, isp.Name)
		}
	}
	return isps
}

func (m Model) getDNSDomains() []string {
	var domains []string
	if m.Config == nil || m.Config.Domains == nil {
		return domains
	}
	for _, d := range m.Config.Domains {
		domains = append(domains, d.Name)
	}
	return domains
}

func (m Model) renderDNSPullRecords() string {
	domains := m.getDNSDomains()

	availableHeight := m.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(domains), availableHeight)
	viewport.CursorIndex = m.DNSDomainIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := titleStyle.Render("  Pull Records - Select Domain")
	sb.WriteString(title + "\n\n")

	if len(domains) == 0 {
		sb.WriteString("No domains configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(domains); i++ {
			if i == m.DNSDomainIndex {
				sb.WriteString(menuSelectedStyle.Render("> "+domains[i]) + "\n")
			} else {
				sb.WriteString(menuItemStyle.Render("  "+domains[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return baseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDiff() string {
	availableHeight := m.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	if m.ErrorMessage != "" {
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("  Error") + "\n\n")
		sb.WriteString(changeDeleteStyle.Render("  "+m.ErrorMessage) + "\n")
		sb.WriteString("\n" + helpStyle.Render("  Esc 返回  q 退出"))
		return baseStyle.Render(sb.String())
	}

	if len(m.DNSPullDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNSPullDiffs), availableHeight)
		viewport.CursorIndex = m.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := titleStyle.Render("  Select Domains to Sync")
		sb.WriteString(title + "\n\n")

		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(m.DNSPullDiffs); i++ {
			diff := m.DNSPullDiffs[i]
			cursor := " "
			if m.DNSPullCursor == i {
				cursor = ">"
			}
			checked := " "
			if m.DNSPullSelected[i] {
				checked = "x"
			}

			var prefix string
			var style = lipgloss.NewStyle()
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = changeCreateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = changeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %s", cursor, checked, prefix, diff.Name)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 移动  Space 切换  a 全选  n 全不选  Enter 确认  Esc 取消  q 退出"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return baseStyle.Render(sb.String())
	} else if len(m.DNSRecordDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNSRecordDiffs), availableHeight)
		viewport.CursorIndex = m.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := titleStyle.Render("  Select DNS Records to Sync")
		sb.WriteString(title + "\n\n")

		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(m.DNSRecordDiffs); i++ {
			diff := m.DNSRecordDiffs[i]
			cursor := " "
			if m.DNSPullCursor == i {
				cursor = ">"
			}
			checked := " "
			if m.DNSPullSelected[i] {
				checked = "x"
			}

			var prefix string
			var style = lipgloss.NewStyle()
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = changeCreateStyle
			case valueobject.ChangeTypeUpdate:
				prefix = "~"
				style = changeUpdateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = changeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %-6s %-20s -> %-30s",
				cursor, checked, prefix, diff.Type, diff.Name, diff.Value)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + helpStyle.Render("  ↑/↓ 移动  Space 切换  a 全选  n 全不选  Enter 确认  Esc 取消  q 退出"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return baseStyle.Render(sb.String())
	} else {
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("  No Differences") + "\n\n")
		sb.WriteString("All items are in sync.\n")
		sb.WriteString("\n" + helpStyle.Render("  Esc 返回  q 退出"))
		return baseStyle.Render(sb.String())
	}
}
