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
	var sb strings.Builder

	title := titleStyle.Render(fmt.Sprintf("  YAMLOps [%s]", strings.ToUpper(string(m.Environment))))
	sb.WriteString(title + "\n\n")

	items := []string{
		"Plan & Apply        基础设施部署",
		"Server Setup        服务器环境设置",
		"DNS Management      域名/DNS管理",
		"Exit                退出",
	}

	for i, item := range items {
		if i == m.MainMenuIndex {
			sb.WriteString(menuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(menuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Select  Enter Confirm  q Quit"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderServerSetup() string {
	var sb strings.Builder

	title := titleStyle.Render("  Server Setup")
	sb.WriteString(title + "\n\n")

	sb.WriteString("  Select Server:\n")
	for i, srv := range m.ServerList {
		prefix := "  "
		focusPrefix := ""
		if m.ServerFocusPanel == 0 && i == m.ServerIndex {
			focusPrefix = "> "
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s (%s)\n", focusPrefix, prefix, srv.Name, srv.Zone))
	}

	sb.WriteString("\n  Actions:\n")
	actions := []string{"Check Environment", "Sync Environment", "Full Setup (Check + Sync)", "Back to Menu"}
	for i, action := range actions {
		prefix := "  "
		focusPrefix := ""
		if m.ServerFocusPanel == 1 && i == m.ServerAction {
			focusPrefix = "> "
		}
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", focusPrefix, prefix, action))
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Select  Tab Switch Panel  Enter Confirm  q Back"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderServerCheck() string {
	var sb strings.Builder

	if len(m.ServerCheckResults) > 0 && m.ServerIndex < len(m.ServerList) {
		name := m.ServerList[m.ServerIndex].Name
		sb.WriteString(serverpkg.FormatResults(name, m.ServerCheckResults))
	}

	if len(m.ServerSyncResults) > 0 {
		sb.WriteString("\n")
		for _, r := range m.ServerSyncResults {
			icon := "✅"
			if !r.Success {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, r.Name, r.Message))
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  Enter/q Back"))

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

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Select  Enter Confirm  q Back"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDomains() string {
	var sb strings.Builder

	title := titleStyle.Render("  Pull Domains - Select ISP")
	sb.WriteString(title + "\n\n")

	isps := m.getDNSISPs()
	if len(isps) == 0 {
		sb.WriteString("No ISPs with DNS service configured.\n")
	} else {
		for i, isp := range isps {
			if i == m.DNSISPIndex {
				sb.WriteString(menuSelectedStyle.Render("> "+isp) + "\n")
			} else {
				sb.WriteString(menuItemStyle.Render("  "+isp) + "\n")
			}
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Select  Enter Confirm  q Back"))

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
	for _, d := range m.Config.Domains {
		domains = append(domains, d.Name)
	}
	return domains
}

func (m Model) renderDNSPullRecords() string {
	var sb strings.Builder

	title := titleStyle.Render("  Pull Records - Select Domain")
	sb.WriteString(title + "\n\n")

	domains := m.getDNSDomains()
	if len(domains) == 0 {
		sb.WriteString("No domains configured.\n")
	} else {
		for i, domain := range domains {
			if i == m.DNSDomainIndex {
				sb.WriteString(menuSelectedStyle.Render("> "+domain) + "\n")
			} else {
				sb.WriteString(menuItemStyle.Render("  "+domain) + "\n")
			}
		}
	}

	sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Select  Enter Confirm  q Back"))

	return baseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDiff() string {
	var sb strings.Builder

	if m.ErrorMessage != "" {
		sb.WriteString(titleStyle.Render("  Error") + "\n\n")
		sb.WriteString(changeDeleteStyle.Render("  "+m.ErrorMessage) + "\n")
		sb.WriteString("\n" + helpStyle.Render("  Enter/q Back"))
		return baseStyle.Render(sb.String())
	}

	if len(m.DNSPullDiffs) > 0 {
		title := titleStyle.Render("  Select Domains to Sync")
		sb.WriteString(title + "\n\n")

		for i, diff := range m.DNSPullDiffs {
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

		sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Move  Space Toggle  a Select All  n Deselect All  Enter Confirm  q Cancel"))
	} else if len(m.DNSRecordDiffs) > 0 {
		title := titleStyle.Render("  Select DNS Records to Sync")
		sb.WriteString(title + "\n\n")

		for i, diff := range m.DNSRecordDiffs {
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

		sb.WriteString("\n" + helpStyle.Render("  ↑/↓ Move  Space Toggle  a Select All  n Deselect All  Enter Confirm  q Cancel"))
	} else {
		sb.WriteString(titleStyle.Render("  No Differences") + "\n\n")
		sb.WriteString("All items are in sync.\n")
		sb.WriteString("\n" + helpStyle.Render("  Enter/q Back"))
	}

	return baseStyle.Render(sb.String())
}
