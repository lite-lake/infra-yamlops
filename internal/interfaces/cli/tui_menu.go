package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	serverpkg "github.com/litelake/yamlops/internal/server"
	"github.com/litelake/yamlops/internal/ssh"
)

func (m Model) renderMainMenu() string {
	items := []string{
		"服务管理",
		"域名/DNS管理",
		"退出",
	}

	availableHeight := m.Height - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(items), availableHeight)
	viewport.CursorIndex = m.MainMenuIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render(fmt.Sprintf("  YAMLOps [%s]", strings.ToUpper(string(m.Environment))))
	sb.WriteString(title + "\n\n")

	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(items); i++ {
		if i == m.MainMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+items[i]) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+items[i]) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  q 退出"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceManagement() string {
	items := []string{
		"服务部署",
		"服务停止",
		"服务清理",
		"服务器环境维护",
		"返回主菜单",
	}

	var sb strings.Builder
	title := TitleStyle.Render("  服务管理")
	sb.WriteString(title + "\n\n")

	for i, item := range items {
		if i == m.ServiceMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
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
	title := TitleStyle.Render("  Server Setup")
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

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Tab 切换面板  Enter 确认  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
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
	lines = append(lines, HelpStyle.Render("  Esc 返回  q 退出"))

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

	return BaseStyle.Render(sb.String())
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

	title := TitleStyle.Render("  DNS Management")
	sb.WriteString(title + "\n\n")

	items := []string{
		"Pull Domains        从ISP拉取域名列表",
		"Pull Records        从域名拉取DNS记录",
		"Plan & Apply        DNS变更计划/执行",
		"Back to Menu        返回主菜单",
	}

	for i, item := range items {
		if i == m.DNSMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
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
	title := TitleStyle.Render("  Pull Domains - Select ISP")
	sb.WriteString(title + "\n\n")

	if len(isps) == 0 {
		sb.WriteString("No ISPs with DNS service configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(isps); i++ {
			if i == m.DNSISPIndex {
				sb.WriteString(MenuSelectedStyle.Render("> "+isps[i]) + "\n")
			} else {
				sb.WriteString(MenuItemStyle.Render("  "+isps[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
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
	title := TitleStyle.Render("  Pull Records - Select Domain")
	sb.WriteString(title + "\n\n")

	if len(domains) == 0 {
		sb.WriteString("No domains configured.\n")
	} else {
		for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(domains); i++ {
			if i == m.DNSDomainIndex {
				sb.WriteString(MenuSelectedStyle.Render("> "+domains[i]) + "\n")
			} else {
				sb.WriteString(MenuItemStyle.Render("  "+domains[i]) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m Model) renderDNSPullDiff() string {
	availableHeight := m.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	if m.ErrorMessage != "" {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  Error") + "\n\n")
		sb.WriteString(ChangeDeleteStyle.Render("  "+m.ErrorMessage) + "\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc 返回  q 退出"))
		return BaseStyle.Render(sb.String())
	}

	if len(m.DNSPullDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNSPullDiffs), availableHeight)
		viewport.CursorIndex = m.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := TitleStyle.Render("  Select Domains to Sync")
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
			var style = ChangeNoopStyle
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = ChangeCreateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = ChangeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %s", cursor, checked, prefix, diff.Name)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 移动  Space 切换  a 全选  n 全不选  Enter 确认  Esc 取消  q 退出"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return BaseStyle.Render(sb.String())
	} else if len(m.DNSRecordDiffs) > 0 {
		viewport := NewViewport(0, len(m.DNSRecordDiffs), availableHeight)
		viewport.CursorIndex = m.DNSPullCursor
		viewport.EnsureCursorVisible()

		var sb strings.Builder
		title := TitleStyle.Render("  Select DNS Records to Sync")
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
			var style = ChangeNoopStyle
			switch diff.ChangeType {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
				style = ChangeCreateStyle
			case valueobject.ChangeTypeUpdate:
				prefix = "~"
				style = ChangeUpdateStyle
			case valueobject.ChangeTypeDelete:
				prefix = "-"
				style = ChangeDeleteStyle
			}

			line := fmt.Sprintf("%s [%s] %s %-6s %-20s -> %-30s",
				cursor, checked, prefix, diff.Type, diff.Name, diff.Value)
			sb.WriteString(style.Render(line) + "\n")
		}

		sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 移动  Space 切换  a 全选  n 全不选  Enter 确认  Esc 取消  q 退出"))

		if viewport.TotalRows > viewport.VisibleRows {
			sb.WriteString("\n" + viewport.RenderScrollIndicator())
		}

		return BaseStyle.Render(sb.String())
	} else {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  No Differences") + "\n\n")
		sb.WriteString("All items are in sync.\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc 返回  q 退出"))
		return BaseStyle.Render(sb.String())
	}
}

func (m Model) renderServiceCleanup() string {
	availableHeight := m.Height - 10
	if availableHeight < 5 {
		availableHeight = 5
	}

	if m.ErrorMessage != "" {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  Error") + "\n\n")
		sb.WriteString(ChangeDeleteStyle.Render("  "+m.ErrorMessage) + "\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc 返回  q 退出"))
		return BaseStyle.Render(sb.String())
	}

	totalItems := m.countCleanupItems()
	viewport := NewViewport(0, totalItems, availableHeight)
	viewport.CursorIndex = m.CleanupCursor
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render("  Service Cleanup - Orphan Resources")
	sb.WriteString(title + "\n\n")

	if totalItems == 0 {
		sb.WriteString("  No orphan services found on any server.\n")
	} else {
		itemIndex := 0
		for _, result := range m.CleanupResults {
			sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
			for _, container := range result.OrphanContainers {
				cursor := " "
				if m.CleanupCursor == itemIndex {
					cursor = ">"
				}
				checked := " "
				if m.CleanupSelected[itemIndex] {
					checked = "x"
				}
				line := fmt.Sprintf("  %s [%s] container: %s", cursor, checked, container)
				style := ChangeDeleteStyle
				sb.WriteString(style.Render(line) + "\n")
				itemIndex++
			}
			for _, dir := range result.OrphanDirs {
				cursor := " "
				if m.CleanupCursor == itemIndex {
					cursor = ">"
				}
				checked := " "
				if m.CleanupSelected[itemIndex] {
					checked = "x"
				}
				line := fmt.Sprintf("  %s [%s] directory: %s", cursor, checked, dir)
				style := ChangeDeleteStyle
				sb.WriteString(style.Render(line) + "\n")
				itemIndex++
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 移动  Space 切换  Enter 确认  Esc 返回  q 退出"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceCleanupConfirm() string {
	var sb strings.Builder
	title := TitleStyle.Render("  Confirm Cleanup")
	sb.WriteString(title + "\n\n")

	selectedCount := 0
	for _, selected := range m.CleanupSelected {
		if selected {
			selectedCount++
		}
	}

	sb.WriteString(fmt.Sprintf("  You are about to remove %d orphan resource(s).\n", selectedCount))
	sb.WriteString("  This action cannot be undone.\n\n")

	options := []string{"Yes, proceed", "Cancel"}
	for i, opt := range options {
		if i == m.ConfirmSelected {
			sb.WriteString(MenuSelectedStyle.Render("> "+opt) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+opt) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceCleanupComplete() string {
	var sb strings.Builder
	title := TitleStyle.Render("  Cleanup Complete")
	sb.WriteString(title + "\n\n")

	for _, result := range m.CleanupResults {
		if len(result.RemovedContainers) > 0 || len(result.RemovedDirs) > 0 ||
			len(result.FailedContainers) > 0 || len(result.FailedDirs) > 0 {
			sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
			for _, c := range result.RemovedContainers {
				sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ removed container: %s", c)) + "\n")
			}
			for _, d := range result.RemovedDirs {
				sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ removed directory: %s", d)) + "\n")
			}
			for _, c := range result.FailedContainers {
				sb.WriteString(ChangeDeleteStyle.Render(fmt.Sprintf("    ✗ failed container: %s", c)) + "\n")
			}
			for _, d := range result.FailedDirs {
				sb.WriteString(ChangeDeleteStyle.Render(fmt.Sprintf("    ✗ failed directory: %s", d)) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  Enter 返回主菜单  q 退出"))

	return BaseStyle.Render(sb.String())
}

func (m Model) countCleanupItems() int {
	count := 0
	for _, result := range m.CleanupResults {
		count += len(result.OrphanContainers) + len(result.OrphanDirs)
	}
	return count
}

func (m *Model) buildCleanupSelected() {
	m.CleanupSelected = make(map[int]bool)
	itemIndex := 0
	for _, result := range m.CleanupResults {
		for range result.OrphanContainers {
			m.CleanupSelected[itemIndex] = true
			itemIndex++
		}
		for range result.OrphanDirs {
			m.CleanupSelected[itemIndex] = true
			itemIndex++
		}
	}
}

func (m Model) hasSelectedCleanupItems() bool {
	for _, selected := range m.CleanupSelected {
		if selected {
			return true
		}
	}
	return false
}

func (m *Model) scanOrphanServices() {
	m.CleanupResults = nil
	m.ErrorMessage = ""

	secrets := m.Config.GetSecretsMap()
	serviceMap := m.Config.GetServiceMap()
	infraServiceMap := m.Config.GetInfraServiceMap()

	for _, srv := range m.ServerList {
		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			m.ErrorMessage = fmt.Sprintf("[%s] Cannot resolve password: %v", srv.Name, err)
			return
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			m.ErrorMessage = fmt.Sprintf("[%s] Connection failed: %v", srv.Name, err)
			return
		}

		containerStdout, _, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			m.ErrorMessage = fmt.Sprintf("[%s] Failed to list containers: %v", srv.Name, err)
			client.Close()
			return
		}

		dirStdout, _, err := client.Run("sudo ls -1 /data/yamlops 2>/dev/null || true")
		if err != nil {
			m.ErrorMessage = fmt.Sprintf("[%s] Failed to list directories: %v", srv.Name, err)
			client.Close()
			return
		}

		client.Close()

		result := CleanupResult{ServerName: srv.Name}

		for _, line := range strings.Split(strings.TrimSpace(containerStdout), "\n") {
			if line == "" {
				continue
			}
			var container struct {
				Name string `json:"Names"`
			}
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				continue
			}

			if !strings.HasPrefix(container.Name, "yo-"+string(m.Environment)+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(container.Name, "yo-"+string(m.Environment)+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				result.OrphanContainers = append(result.OrphanContainers, container.Name)
			}
		}

		for _, line := range strings.Split(strings.TrimSpace(dirStdout), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "yo-"+string(m.Environment)+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(line, "yo-"+string(m.Environment)+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				result.OrphanDirs = append(result.OrphanDirs, line)
			}
		}

		if len(result.OrphanContainers) > 0 || len(result.OrphanDirs) > 0 {
			m.CleanupResults = append(m.CleanupResults, result)
		}
	}
}

func (m *Model) executeServiceCleanup() {
	secrets := m.Config.GetSecretsMap()

	for i, result := range m.CleanupResults {
		srv := m.findServerByName(result.ServerName)
		if srv == nil {
			continue
		}

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			for _, c := range result.OrphanContainers {
				m.CleanupResults[i].FailedContainers = append(m.CleanupResults[i].FailedContainers, c)
			}
			for _, d := range result.OrphanDirs {
				m.CleanupResults[i].FailedDirs = append(m.CleanupResults[i].FailedDirs, d)
			}
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			for _, c := range result.OrphanContainers {
				m.CleanupResults[i].FailedContainers = append(m.CleanupResults[i].FailedContainers, c)
			}
			for _, d := range result.OrphanDirs {
				m.CleanupResults[i].FailedDirs = append(m.CleanupResults[i].FailedDirs, d)
			}
			continue
		}

		itemIndex := m.getServerCleanupStartIndex(i)
		for _, container := range result.OrphanContainers {
			if m.CleanupSelected[itemIndex] {
				_, stderr, err := client.Run(fmt.Sprintf("sudo docker rm -f %s", container))
				if err != nil {
					m.CleanupResults[i].FailedContainers = append(m.CleanupResults[i].FailedContainers, container+": "+stderr)
				} else {
					m.CleanupResults[i].RemovedContainers = append(m.CleanupResults[i].RemovedContainers, container)
				}
			}
			itemIndex++
		}
		for _, dir := range result.OrphanDirs {
			if m.CleanupSelected[itemIndex] {
				remoteDir := fmt.Sprintf("/data/yamlops/%s", dir)
				_, stderr, err := client.Run(fmt.Sprintf("sudo rm -rf %s", remoteDir))
				if err != nil {
					m.CleanupResults[i].FailedDirs = append(m.CleanupResults[i].FailedDirs, dir+": "+stderr)
				} else {
					m.CleanupResults[i].RemovedDirs = append(m.CleanupResults[i].RemovedDirs, dir)
				}
			}
			itemIndex++
		}

		client.Close()
	}
}

func (m *Model) findServerByName(name string) *entity.Server {
	for _, srv := range m.ServerList {
		if srv.Name == name {
			return srv
		}
	}
	return nil
}

func (m *Model) getServerCleanupStartIndex(serverIndex int) int {
	count := 0
	for i := 0; i < serverIndex; i++ {
		count += len(m.CleanupResults[i].OrphanContainers) + len(m.CleanupResults[i].OrphanDirs)
	}
	return count
}

func (m *Model) buildStopTree() {
	m.loadConfig()
	if m.Config == nil {
		return
	}
	m.TreeNodes = m.buildAppTree()
	m.StopSelected = make(map[int]bool)
	for _, node := range m.TreeNodes {
		node.SelectRecursive(false)
	}
	m.fetchServiceStatus()
	m.applyServiceStatusToTree()
}

func (m *Model) fetchServiceStatus() {
	m.ServiceStatusMap = make(map[string]NodeStatus)
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
			m.ServiceStatusMap[proj.Name] = StatusRunning
		}

		for _, infra := range m.Config.InfraServices {
			if infra.Server != srv.Name {
				continue
			}
			remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", m.Environment, infra.Name)
			key := fmt.Sprintf("yo-%s-%s", m.Environment, infra.Name)
			if _, exists := m.ServiceStatusMap[key]; !exists {
				stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if strings.TrimSpace(stdout) == "exists" {
					m.ServiceStatusMap[key] = StatusStopped
				}
			}
		}

		for _, svc := range m.Config.Services {
			if svc.Server != srv.Name {
				continue
			}
			remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", m.Environment, svc.Name)
			key := fmt.Sprintf("yo-%s-%s", m.Environment, svc.Name)
			if _, exists := m.ServiceStatusMap[key]; !exists {
				stdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if strings.TrimSpace(stdout) == "exists" {
					m.ServiceStatusMap[key] = StatusStopped
				}
			}
		}
	}
}

func (m *Model) applyServiceStatusToTree() {
	for _, node := range m.TreeNodes {
		m.applyStatusToNode(node)
	}
}

func (m *Model) applyStatusToNode(node *TreeNode) {
	if node.Type == NodeTypeInfra || node.Type == NodeTypeBiz {
		key := fmt.Sprintf("yo-%s-%s", m.Environment, node.Name)
		if status, exists := m.ServiceStatusMap[key]; exists {
			node.Status = status
		}
	}
	for _, child := range node.Children {
		m.applyStatusToNode(child)
	}
}

func (m *Model) executeServiceStop() {
	m.StopResults = nil
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

	for _, srv := range m.ServerList {
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
			m.StopResults = append(m.StopResults, result)
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
			m.StopResults = append(m.StopResults, result)
			continue
		}

		for _, svcName := range services {
			remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", m.Environment, svcName)
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
		m.StopResults = append(m.StopResults, result)
	}
}

type serviceInfo struct {
	Name   string
	Server string
}

func (m *Model) getSelectedServicesForStop() []serviceInfo {
	var services []serviceInfo
	serviceSet := make(map[string]bool)

	for _, node := range m.TreeNodes {
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
	for _, node := range m.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func (m Model) renderServiceStop() string {
	var lines []string
	idx := 0
	for _, node := range m.TreeNodes {
		m.renderNodeToLinesForStop(node, 0, &idx, &lines)
	}

	availableHeight := m.Height - 10
	if availableHeight < 5 {
		availableHeight = 5
	}

	treeHeight := availableHeight - 2
	if treeHeight < 3 {
		treeHeight = 3
	}

	totalNodes := len(lines)
	viewport := NewViewport(m.CursorIndex, totalNodes, treeHeight)
	viewport.EnsureCursorVisible()
	m.ScrollOffset = viewport.Offset

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

	sb.WriteString("\n" + HelpStyle.Render("  Space 选择  Enter 展开  a 当前  n 取消  A 全选  N 全不选  p 确认停止  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
}

func formatNodeStatus(status NodeStatus) string {
	switch status {
	case StatusRunning:
		return SuccessStyle.Render("[运行中]")
	case StatusStopped:
		return WarningStyle.Render("[已停止]")
	case StatusNeedsUpdate:
		return ChangeUpdateStyle.Render("[需更新]")
	case StatusError:
		return ChangeDeleteStyle.Render("[错误]")
	default:
		return ""
	}
}

func (m Model) renderNodeToLinesForStop(node *TreeNode, depth int, idx *int, lines *[]string) {
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		prefix = indent[:len(indent)-2] + "├─"
	}
	cursor := "  "
	if *idx == m.CursorIndex {
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
	if *idx == m.CursorIndex {
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
	if *idx == m.CursorIndex {
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
	if *idx == m.CursorIndex {
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
	for _, node := range m.TreeNodes {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotalForStop() int {
	count := 0
	for _, node := range m.TreeNodes {
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
		if i == m.ConfirmSelected {
			sb.WriteString(MenuSelectedStyle.Render("  > "+opt) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("    "+opt) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ 选择  Enter 确认  Esc 返回  q 退出"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceStopComplete() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render("  Stop Complete") + "\n\n")

	if len(m.StopResults) > 0 {
		for _, result := range m.StopResults {
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

	sb.WriteString("\n" + HelpStyle.Render("  Enter 返回  q 退出"))

	return BaseStyle.Render(sb.String())
}
