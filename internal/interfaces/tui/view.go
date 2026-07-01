package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// renderTitleBar renders the top title bar: YAMLOps | Environment: {env} | {nav path}
func (m Model) renderTitleBar() string {
	envStyle := styles.EnvStyle(string(m.Environment))
	nav := m.navigationPath()

	var b strings.Builder
	b.WriteString(styles.TitleBarStyle.Render(styles.TitleBrand))
	b.WriteString(styles.SepVertical)
	b.WriteString(envStyle.Render(fmt.Sprintf("%s%s", styles.TitleEnvPrefix, string(m.Environment))))
	if nav != "" {
		b.WriteString(styles.SepVertical)
		b.WriteString(styles.NavigationStyle.Render(nav))
	}
	return b.String()
}

// renderTitleSep renders the horizontal separator after the title bar.
func renderTitleSep(width int) string {
	if width < 1 {
		width = 80
	}
	sep := styles.SepHorizontal
	for len(sep) < width-4 {
		sep += styles.SepHorizontal
	}
	return styles.SeparatorStyle.Render(styles.SepCornerBL + sep[:width-4])
}

// renderStatusBarSep renders the horizontal separator before the status bar.
func renderStatusBarSep(width int) string {
	return renderTitleSep(width)
}

// renderHelpBar renders the bottom help bar with current keyboard shortcuts.
func (m Model) renderHelpBar() string {
	if m.ShowHelp {
		return styles.HelpBarStyle.Render("[?] Hide help")
	}
	var items []string
	switch m.ViewState {
	case ViewStateMainMenu:
		items = []string{"[↑↓] Navigate", "[Space] Expand/Collapse", "[Enter] Select", "[?] Help", "[q] Quit"}
	case ViewStateServiceMenu, ViewStateServerMenu, ViewStateDNSMenu, ViewStateConfigMenu:
		items = []string{"[↑↓] Navigate", "[Enter] Select", "[?] Help", "[q] Quit"}
	case ViewStateTreeService, ViewStateTreeDNS:
		items = []string{"[Space] Toggle", "[a] Select all", "[n] Select none", "[Enter] Confirm", "[/] Search", "[?] Help", "[Esc] Back"}
	case ViewStateFilter:
		items = []string{"[Space] Toggle", "[←→] Switch group", "[a] Select all", "[n] Select none", "[Enter] Confirm", "[?] Help", "[Esc] Back"}
	case ViewStatePlan:
		items = []string{"[Space] Toggle", "[a] Select all", "[n] Select none", "[d] Toggle detail", "[f] Force mode", "[Enter] Execute", "[Esc] Back"}
	case ViewStateProgress:
		if m.Action.Interrupted {
			items = []string{"[Esc] Back to main menu", "[?] Help"}
		} else {
			items = []string{"[Ctrl+C] Interrupt (no rollback)", "[?] Help"}
		}
	case ViewStateComplete:
		items = []string{"[r] Re-plan", "[Esc] Back to main menu", "[?] Help"}
	case ViewStateInfoList, ViewStateInfoDetail:
		items = []string{"[d] Toggle detail", "[/] Search", "[↑↓] Scroll", "[?] Help", "[Esc] Back"}
	case ViewStateValidate:
		items = []string{"[↑↓] Scroll", "[Esc] Back"}
	default:
		items = []string{"[?] Help", "[Esc] Back"}
	}
	return styles.HelpBarStyle.Render(strings.Join(items, styles.HelpKeySep))
}

// renderStatusBar renders the bottom status bar with status info and statistics.
func (m Model) renderStatusBar() string {
	var parts []string

	if m.ShowHelp {
		return styles.StatusBarStyle.Render("Status: Help")
	}

	switch m.ViewState {
	case ViewStateMainMenu:
		parts = append(parts, "Status: Ready")
	case ViewStateTreeService, ViewStateTreeDNS:
		if m.Search.Active {
			parts = append(parts, "Status: Searching")
			parts = append(parts, fmt.Sprintf("Query: %s", m.Search.Query))
		} else if m.Tree.OriginalNodes != nil {
			parts = append(parts, "Status: Filtered")
			if m.Search.Query != "" {
				parts = append(parts, fmt.Sprintf("Query: %s", m.Search.Query))
			}
		} else {
			selected := m.countSelected()
			total := m.countTotal()
			parts = append(parts, fmt.Sprintf("Status: Selecting"))
			parts = append(parts, fmt.Sprintf("%d/%d selected", selected, total))
		}
	case ViewStateFilter:
		parts = append(parts, "Status: Filter")
		if m.Action.FilterView != nil {
			count := m.Action.FilterView.SelectedCount()
			switch m.Action.OperationType {
			case "dns_pull_domains":
				parts = append(parts, fmt.Sprintf("%d ISP(s) matched", count))
			case "dns_pull_records":
				parts = append(parts, fmt.Sprintf("%d domain(s) matched", count))
			default:
				parts = append(parts, fmt.Sprintf("%d services matched", count))
			}
		}
	case ViewStatePlan:
		if m.Action.PlanComponent != nil && m.Action.PlanComponent.DryRun {
			parts = append(parts, "Status: Plan ready (dry-run)")
		} else {
			parts = append(parts, "Status: Plan ready")
		}
		if m.Action.PlanComponent != nil {
			selected := m.Action.PlanComponent.SelectedCount()
			total := m.Action.PlanComponent.TotalCount()
			parts = append(parts, fmt.Sprintf("%d selected", selected))
			parts = append(parts, fmt.Sprintf("%d changes pending", total))
		} else if m.Action.PlanResult != nil {
			changes := len(m.Action.PlanResult.Changes())
			parts = append(parts, fmt.Sprintf("%d changes pending", changes))
		}
	case ViewStateProgress:
		if m.Action.Interrupted && m.Action.ProgressView != nil {
			pv := m.Action.ProgressView
			success, failed := progressSuccessFailedCount(pv)
			skipped := pv.Total - success - failed
			parts = append(parts, "Status: Interrupted")
			parts = append(parts, fmt.Sprintf("%d succeeded, %d failed, %d skipped", success, failed, skipped))
		} else if m.Action.ProgressView != nil && m.Action.ProgressView.Total > 0 {
			pv := m.Action.ProgressView
			pct := int(float64(pv.Progress) / float64(pv.Total) * 100)
			parts = append(parts, fmt.Sprintf("Status: Executing"))
			parts = append(parts, fmt.Sprintf("%d/%d (%d%%)", pv.Progress, pv.Total, pct))
		} else if m.Action.ApplyTotal > 0 {
			pct := int(float64(m.Action.ApplyProgress) / float64(m.Action.ApplyTotal) * 100)
			parts = append(parts, fmt.Sprintf("Status: Executing"))
			parts = append(parts, fmt.Sprintf("%d/%d (%d%%)", m.Action.ApplyProgress, m.Action.ApplyTotal, pct))
		}
	case ViewStateComplete:
		successCount := 0
		failCount := 0
		skippedCount := 0
		for _, r := range m.Action.ApplyResults {
			if r.Success {
				successCount++
			} else {
				failCount++
			}
		}
		if m.Action.ProgressView != nil {
			for _, g := range m.Action.ProgressView.Groups {
				for _, item := range g.Items {
					if item.Status == components.ProgressStatusSkipped {
						skippedCount++
					}
				}
			}
		}
		if skippedCount > 0 {
			parts = append(parts, "Status: Interrupted")
			parts = append(parts, fmt.Sprintf("%d succeeded, %d failed, %d skipped", successCount, failCount, skippedCount))
		} else {
			parts = append(parts, "Status: Completed")
			parts = append(parts, fmt.Sprintf("%d succeeded, %d failed", successCount, failCount))
		}
		if !m.Action.ProgressStartTime.IsZero() {
			elapsed := time.Since(m.Action.ProgressStartTime)
			parts = append(parts, fmt.Sprintf("Duration: %s", formatElapsed(elapsed)))
		}
		if failCount > 0 {
			parts = append(parts, "Exit code: 3")
		}
	case ViewStateInfoList, ViewStateInfoDetail:
		if m.Search.Active {
			parts = append(parts, "Status: Searching")
			parts = append(parts, fmt.Sprintf("Query: %s", m.Search.Query))
		} else if m.Search.Query != "" && (m.UI.InfoListFilteredRows != nil || m.UI.InfoDetailFilteredEntities != nil) {
			parts = append(parts, "Status: Filtered")
			parts = append(parts, fmt.Sprintf("Query: %s", m.Search.Query))
		} else {
			parts = append(parts, "Status: Loaded")
			// 根据 OperationType 添加对应的计数信息
			if m.Config != nil {
				switch m.Action.OperationType {
				case "show":
					showBiz, showInfra := m.getSelectedServiceTypes()
					total := 0
					if showBiz {
						total += len(m.Config.Services)
					}
					if showInfra {
						total += len(m.Config.InfraServices)
					}
					parts = append(parts, fmt.Sprintf("%d services", total))
				case "server_show":
					parts = append(parts, fmt.Sprintf("%d servers", len(m.Config.Servers)))
				case "dns_show":
					parts = append(parts, fmt.Sprintf("%d domains", len(m.Config.Domains)))
				case "config_show_isps":
					parts = append(parts, fmt.Sprintf("%d ISPs", len(m.Config.ISPs)))
				case "config_show_registries":
					parts = append(parts, fmt.Sprintf("%d registries", len(m.Config.Registries)))
				case "config_show_secrets":
					parts = append(parts, fmt.Sprintf("%d secrets", len(m.Config.Secrets)))
				}
			}
		}
	case ViewStateValidate:
		if m.UI.Validate != nil {
			// 根据验证结果显示 PASSED 或 FAILED
			if m.UI.Validate.Failed == 0 {
				parts = append(parts, "Status: Validation passed")
			} else {
				parts = append(parts, "Status: Validation failed")
				// 显示错误和警告数量
				var details []string
				if m.UI.Validate.Failed > 0 {
					details = append(details, fmt.Sprintf("%d errors", m.UI.Validate.Failed))
				}
				if m.UI.Validate.Warnings > 0 {
					details = append(details, fmt.Sprintf("%d warnings", m.UI.Validate.Warnings))
				}
				if len(details) > 0 {
					parts = append(parts, strings.Join(details, ", "))
				}
			}
		} else {
			parts = append(parts, "Status: Validation")
		}
	default:
		parts = append(parts, "Status: Ready")
	}

	return styles.StatusBarStyle.Render(strings.Join(parts, " | "))
}

// navigationPath returns the breadcrumb navigation path for the title bar.
func (m Model) navigationPath() string {
	switch m.ViewState {
	case ViewStateMainMenu:
		return "Main Menu"
	case ViewStateServiceMenu:
		return "Service Management"
	case ViewStateServerMenu:
		return "Server Management"
	case ViewStateDNSMenu:
		return "DNS Management"
	case ViewStateConfigMenu:
		return "Configuration"
	case ViewStateTreeService:
		return "Service > Deploy"
	case ViewStateTreeDNS:
		return "DNS > Deploy"
	case ViewStateFilter:
		if m.Action.OperationType == "dns_pull_domains" {
			return "DNS > Pull Domains"
		} else if m.Action.OperationType == "dns_pull_records" {
			return "DNS > Pull Records"
		}
		return fmt.Sprintf("Service > %s > Filter", capitalizeFirst(m.Action.OperationType))
	case ViewStatePlan:
		switch m.Action.OperationType {
		case "dns_pull_domains":
			return "DNS > Pull Domains > Plan"
		case "dns_pull_records":
			return "DNS > Pull Records > Plan"
		}
		return m.operationNavigationPath("Plan")
	case ViewStateProgress:
		return m.operationNavigationPath("Executing")
	case ViewStateComplete:
		return m.operationNavigationPath("Complete")
	case ViewStateInfoList:
		return m.infoNavigationPath("Show")
	case ViewStateInfoDetail:
		return m.infoNavigationPath("Show > Detail")
	case ViewStateValidate:
		return m.infoNavigationPath("Validate")
	default:
		return ""
	}
}

// operationNavigationPath returns the navigation path for change operations (Plan/Executing/Complete)
// with module prefix based on OperationType.
func (m Model) operationNavigationPath(action string) string {
	switch m.Action.OperationType {
	case "deploy":
		return "Service > Deploy > " + action
	case "stop":
		return "Service > Stop > " + action
	case "restart":
		return "Service > Restart > " + action
	case "cleanup":
		return "Service > Cleanup > " + action
	case "server_setup":
		return "Server > Setup > " + action
	case "docker_prune":
		return "Server > Prune > " + action
	case "dns_deploy":
		return "DNS > Deploy > " + action
	case "dns_pull_domains":
		return "DNS > Pull Domains > " + action
	case "dns_pull_records":
		return "DNS > Pull Records > " + action
	default:
		return fmt.Sprintf("%s > %s", capitalizeFirst(m.Action.OperationType), action)
	}
}

// infoNavigationPath returns the navigation path for InfoList/InfoDetail/Validate views
// with module context based on SourceMenu and OperationType.
func (m Model) infoNavigationPath(action string) string {
	// Determine module prefix based on SourceMenu or OperationType
	module := ""
	switch {
	case m.SourceMenu == ViewStateServiceMenu || m.Action.OperationType == "show" || m.Action.OperationType == "validate":
		module = "Service"
	case m.SourceMenu == ViewStateServerMenu || m.Action.OperationType == "server_show" || m.Action.OperationType == "server_validate":
		module = "Server"
	case m.SourceMenu == ViewStateDNSMenu || m.Action.OperationType == "dns_show" || m.Action.OperationType == "dns_validate":
		module = "DNS"
	case m.SourceMenu == ViewStateConfigMenu || strings.HasPrefix(m.Action.OperationType, "config_"):
		module = "Config"
	}

	// For Config > Show views, use specific labels based on OperationType
	if module == "Config" && strings.HasPrefix(action, "Show") {
		suffix := ""
		if action == "Show > Detail" {
			suffix = " > Detail"
		}
		switch m.Action.OperationType {
		case "config_show_isps":
			return "Config > ISPs" + suffix
		case "config_show_registries":
			return "Config > Registries" + suffix
		case "config_show_secrets":
			return "Config > Secrets" + suffix
		}
	}

	if module != "" {
		return module + " > " + action
	}
	return action
}

// renderWithLayout wraps content in the four-region layout:
// title bar | content | help bar | status bar
func (m Model) renderWithLayout(content string) string {
	var b strings.Builder
	b.WriteString(m.renderTitleBar())
	b.WriteString("\n")
	b.WriteString(renderTitleSep(m.UI.Width))
	b.WriteString("\n")
	b.WriteString(content)
	b.WriteString("\n")
	b.WriteString(renderStatusBarSep(m.UI.Width))
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())
	b.WriteString("\n")
	b.WriteString(renderStatusBarSep(m.UI.Width))
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	return styles.BasePaddingStyle.Render(b.String())
}

func (m Model) View() string {
	var baseView string
	if m.Loading.Active {
		baseView = m.renderLoadingView()
	} else if m.UI.Width < styles.MinTerminalWidth || m.UI.Height < styles.MinTerminalHeight {
		baseView = m.renderTerminalSizeWarning()
	} else {
		switch m.ViewState {
		case ViewStateMainMenu:
			baseView = m.renderWithLayout(m.renderMainMenuContent())
		case ViewStateServiceMenu:
			baseView = m.renderWithLayout(m.renderServiceMenuContent())
		case ViewStateServerMenu:
			baseView = m.renderWithLayout(m.renderServerMenuContent())
		case ViewStateDNSMenu:
			baseView = m.renderWithLayout(m.renderDNSMenuContent())
		case ViewStateConfigMenu:
			baseView = m.renderWithLayout(m.renderConfigMenuContent())
		case ViewStateFilter:
			baseView = m.renderWithLayout(m.renderFilterContent())
		default:
			var content strings.Builder
			switch m.ViewState {
			case ViewStateTreeService, ViewStateTreeDNS:
				content.WriteString(m.renderTreeContent())
			case ViewStatePlan:
				content.WriteString(m.renderPlanContent())
			case ViewStateProgress:
				content.WriteString(m.renderProgressContent())
			case ViewStateComplete:
				content.WriteString(m.renderCompleteContent())
			case ViewStateInfoList:
				content.WriteString(m.renderInfoListContent())
			case ViewStateInfoDetail:
				content.WriteString(m.renderInfoDetailContent())
			case ViewStateValidate:
				content.WriteString(m.renderValidateContent())
			default:
				content.WriteString("Unknown view state")
			}
			baseView = m.renderWithLayout(content.String())
		}
	}

	if m.ShowHelp {
		return m.renderHelpOverlay(baseView)
	}
	return baseView
}

func (m Model) renderLoadingView() string {
	var content strings.Builder
	content.WriteString(styles.BrandStyle.Render(styles.TitleBrand))
	content.WriteString(" ")
	envStyle := styles.EnvStyle(string(m.Environment))
	content.WriteString(envStyle.Render(fmt.Sprintf("[%s]", string(m.Environment))))
	content.WriteString("\n\n")

	spinner := styles.SpinnerFrames[m.Loading.Spinner]
	loadingText := fmt.Sprintf("  %s %s", spinner, m.Loading.Message)
	content.WriteString(styles.LoadingOverlayStyle.Render(loadingText))
	content.WriteString("\n\n")
	content.WriteString(styles.MutedStyle.Render("  Ctrl+C to cancel  ? help"))
	return styles.BasePaddingStyle.Render(content.String())
}

// renderTerminalSizeWarning renders a warning when the terminal is too small.
func (m Model) renderTerminalSizeWarning() string {
	var content strings.Builder
	content.WriteString(styles.WarningStyle.Render("⚠ Terminal Too Small"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("  Current size: %dx%d\n", m.UI.Width, m.UI.Height))
	content.WriteString(fmt.Sprintf("  Minimum required: %dx%d\n", styles.MinTerminalWidth, styles.MinTerminalHeight))
	content.WriteString(fmt.Sprintf("  Recommended: %dx%d\n", styles.RecTerminalWidth, styles.RecTerminalHeight))
	content.WriteString("\n")
	content.WriteString(styles.MutedStyle.Render("  Please resize your terminal window."))
	return styles.BasePaddingStyle.Render(content.String())
}

// renderErrorMessage renders an error message with per-line styling.
// Lines are styled according to their prefix:
//   - "ERROR:" or "Error:" → ErrorStyle (bold red)
//   - "Details:" → WarningStyle (yellow)
//   - "Suggestion:" → MutedStyle (gray)
func renderErrorMessage(msg string) string {
	lines := strings.Split(msg, "\n")
	var styled []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "ERROR:") || strings.HasPrefix(trimmed, "Error:"):
			styled = append(styled, styles.ErrorStyle.Render(line))
		case strings.HasPrefix(trimmed, "Details:"):
			styled = append(styled, styles.WarningStyle.Render(line))
		case strings.HasPrefix(trimmed, "Suggestion:"):
			styled = append(styled, styles.MutedStyle.Render(line))
		default:
			styled = append(styled, line)
		}
	}
	return strings.Join(styled, "\n")
}

func (m Model) renderTreeContent() string {
	lines := renderTreeNodes(m.getCurrentTree(), m.Tree.CursorIndex, true)

	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	if m.Search.Active {
		availableHeight -= 2 // 为搜索框留出空间
	}
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	if m.UI.ErrorMessage != "" {
		availableHeight -= strings.Count(m.UI.ErrorMessage, "\n") + 2
		if availableHeight < 3 {
			availableHeight = 3
		}
	}

	totalNodes := len(lines)
	viewport := NewViewport(m.Tree.CursorIndex, totalNodes, availableHeight)
	viewport.EnsureCursorVisible()
	m.UI.ScrollOffset = viewport.Offset

	var content strings.Builder

	// 根据 ViewMode 显示提示文字
	switch m.ViewState {
	case ViewStateTreeService:
		content.WriteString(styles.MutedStyle.Render("Select services to deploy:"))
	case ViewStateTreeDNS:
		content.WriteString(styles.MutedStyle.Render("Select domains and records to deploy:"))
	}
	content.WriteString("\n")
	content.WriteString("\n")

	// 显示搜索框
	if m.Search.Active {
		searchLine := m.Search.SearchFilter.Render()
		content.WriteString(searchLine)
		content.WriteString("\n")
		content.WriteString("\n")
	}

	if m.UI.ErrorMessage != "" {
		content.WriteString(renderErrorMessage(m.UI.ErrorMessage))
		content.WriteString("\n\n")
	}

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		content.WriteString(lines[i])
		content.WriteString("\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		content.WriteString("\n")
		content.WriteString(viewport.RenderScrollIndicator())
	}

	if !m.Search.Active && m.countTotal() > 50 {
		content.WriteString("\n")
		content.WriteString(styles.MutedStyle.Render("  Many items: use / to search"))
	}

	return content.String()
}

// renderPlanContent renders the plan preview with checkbox selection using the PlanView component.
func (m Model) renderPlanContent() string {
	if m.UI.ErrorMessage != "" {
		return renderErrorMessage(m.UI.ErrorMessage)
	}

	if m.Action.PlanComponent != nil {
		availableHeight := m.UI.Height - styles.LayoutOverhead - 2
		return m.Action.PlanComponent.Render(availableHeight)
	}

	return styles.MutedStyle.Render("  No plan available.")
}

func (m Model) renderProgressContent() string {
	pv := m.Action.ProgressView
	if pv == nil {
		pv = components.NewProgressView("EXECUTING...", string(m.Environment))
		if m.Action.ApplyTotal > 0 {
			pv.Total = m.Action.ApplyTotal
			pv.Progress = m.Action.ApplyProgress
		}
	} else {
		if m.Action.ProgressStartTime.IsZero() == false {
			if m.Action.Interrupted {
				pv.Elapsed = formatElapsed(m.Action.ProgressEndTime.Sub(m.Action.ProgressStartTime))
			} else {
				elapsed := time.Since(m.Action.ProgressStartTime)
				pv.Elapsed = formatElapsed(elapsed)
			}
		}
		pv.Interrupted = m.Action.Interrupted
	}
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return pv.Render(availableHeight)
}

func formatElapsed(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	secs = secs % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}

func progressSuccessFailedCount(pv *components.ProgressView) (success, failed int) {
	for _, g := range pv.Groups {
		for _, item := range g.Items {
			switch item.Status {
			case components.ProgressStatusSuccess:
				success++
			case components.ProgressStatusFailed:
				failed++
			}
		}
	}
	return
}

func generateSuggestion(err error, entity, server string) string {
	switch {
	case errors.Is(err, domainerr.ErrSSHConnectFailed):
		if server != "" {
			return fmt.Sprintf("Check SSH configuration and network connectivity to %s", server)
		}
		return "Check SSH configuration and network connectivity"
	case errors.Is(err, domainerr.ErrSSHAuthFailed):
		return "Verify SSH credentials (user, key/password)"
	case errors.Is(err, domainerr.ErrSSHHostKeyMismatch):
		return "Update known_hosts or verify server identity"
	case errors.Is(err, domainerr.ErrSSHCommandFailed):
		if server != "" {
			return fmt.Sprintf("Check command permissions and Docker daemon status on %s", server)
		}
		return "Check command permissions and Docker daemon status"
	case errors.Is(err, domainerr.ErrSSHSessionFailed):
		if server != "" {
			return fmt.Sprintf("Check SSH service availability on %s", server)
		}
		return "Check SSH service availability"
	case errors.Is(err, domainerr.ErrSSHFileTransfer):
		if server != "" {
			return fmt.Sprintf("Check disk space and file permissions on %s", server)
		}
		return "Check disk space and file permissions"
	case errors.Is(err, domainerr.ErrSSHClientNotAvailable):
		return "Retry the operation; if persistent, check server environment setup"
	case errors.Is(err, domainerr.ErrDockerComposeFailed):
		if server != "" {
			return fmt.Sprintf("Check Docker daemon status and container logs on %s", server)
		}
		return "Check Docker daemon status and container logs"
	case errors.Is(err, domainerr.ErrNetworkTimeout):
		return "Check network connectivity and retry"
	case errors.Is(err, domainerr.ErrNetworkUnreachable):
		return "Verify server network configuration"
	case errors.Is(err, domainerr.ErrConnectionRefused):
		if server != "" {
			return fmt.Sprintf("Ensure the target service is running on %s", server)
		}
		return "Ensure the target service is running"
	case errors.Is(err, domainerr.ErrRegistryLoginFailed):
		return "Check registry credentials in configuration"
	case errors.Is(err, domainerr.ErrRegistryNotFound):
		return "Verify registry name in service configuration"
	case errors.Is(err, domainerr.ErrDNSError):
		return "Check DNS provider credentials and network connectivity"
	case errors.Is(err, domainerr.ErrDNSRecordNotFound):
		return "Verify the DNS record exists in the ISP console"
	case errors.Is(err, domainerr.ErrDNSDomainNotFound):
		return "Verify the domain is registered with the ISP"
	default:
		if server != "" {
			return fmt.Sprintf("Check logs on %s for details", server)
		}
		return "Check logs for details"
	}
}

func (m Model) renderCompleteContent() string {
	cv := components.NewCompleteView("RESULT", string(m.Environment))

	if !m.Action.ProgressStartTime.IsZero() {
		elapsed := time.Since(m.Action.ProgressStartTime)
		cv.Duration = formatElapsed(elapsed)
	}

	// Build a map of changes from ApplyResults for quick lookup
	type changeKey struct {
		action string
		name   string
		server string
	}
	appliedChanges := make(map[changeKey]bool)

	if m.Action.ApplyResults != nil {
		var items []components.CompleteItem
		for _, result := range m.Action.ApplyResults {
			server := usecase.ExtractServerFromChange(result.Change)
			item := components.CompleteItem{
				Action:  strings.ToLower(result.Change.Type().String()),
				Name:    result.Change.Name(),
				Server:  server,
				Success: result.Success,
			}
			if !result.Success && result.Error != nil {
				item.Error = result.Error.Error()
				item.Suggestion = generateSuggestion(result.Error, result.Change.Entity(), item.Server)
			}
			items = append(items, item)
			appliedChanges[changeKey{item.Action, item.Name, item.Server}] = true
		}
		cv.SetItems(items)
	}

	// Map skipped items from ProgressView that were not in ApplyResults
	if m.Action.ProgressView != nil {
		for _, group := range m.Action.ProgressView.Groups {
			for _, item := range group.Items {
				if item.Status == components.ProgressStatusSkipped {
					key := changeKey{item.Action, item.Name, item.Server}
					if !appliedChanges[key] {
						cv.AddItem(components.CompleteItem{
							Action:  item.Action,
							Name:    item.Name,
							Server:  item.Server,
							Skipped: true,
						})
					}
				}
			}
		}
	}

	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return cv.Render(availableHeight)
}

func (m Model) renderTabs() string {
	var tabs strings.Builder
	if m.ViewMode == ViewModeApp {
		tabs.WriteString(styles.TabActiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(styles.TabInactiveStyle.Render("DNS"))
	} else {
		tabs.WriteString(styles.TabInactiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(styles.TabActiveStyle.Render("DNS"))
	}
	return tabs.String()
}

// renderHelpOverlay renders a bordered help overlay on top of the base view.
func (m Model) renderHelpOverlay(baseView string) string {
	helpContent := m.renderHelpContent()

	helpBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(styles.ColorBrand)).
		Padding(1, 2).
		Width(56).
		Render(helpContent)

	return lipgloss.Place(
		m.UI.Width,
		m.UI.Height,
		lipgloss.Center,
		lipgloss.Center,
		helpBox,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// renderHelpContent builds context-aware help content based on the current ViewState.
func (m Model) renderHelpContent() string {
	var b strings.Builder

	b.WriteString(styles.BrandStyle.Render("HELP"))
	b.WriteString("\n\n")

	type helpSection struct {
		Title string
		Items [][2]string
	}
	var sections []helpSection

	switch m.ViewState {
	case ViewStateTreeService, ViewStateTreeDNS:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"←/h", "Collapse node"},
					{"→/l", "Expand node"},
				},
			},
			{
				Title: "Selection",
				Items: [][2]string{
					{"Space", "Toggle select (leaf) / Expand (parent)"},
					{"a", "Select all (including collapsed)"},
					{"n", "Select none"},
					{"/", "Search / Filter"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"Enter", "Confirm"},
					{"Esc", "Back"},
					{"d", "Toggle detail view"},
					{"r", "Re-plan"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStatePlan:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
				},
			},
			{
				Title: "Selection",
				Items: [][2]string{
					{"Space", "Toggle select"},
					{"a", "Select all"},
					{"n", "Deselect all"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"d", "Toggle detail"},
					{"f", "Toggle force"},
					{"Enter", "Execute"},
					{"Esc", "Back"},
					{"r", "Re-plan"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateFilter:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"←/h", "Prev group"},
					{"→/l", "Next group"},
				},
			},
			{
				Title: "Selection",
				Items: [][2]string{
					{"Space", "Toggle select"},
					{"a", "Select all"},
					{"n", "Deselect all"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"Enter", "Confirm"},
					{"Esc", "Back"},
					{"d", "Toggle detail view"},
					{"r", "Re-plan"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateInfoList, ViewStateInfoDetail:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"d", "Toggle detail/list"},
					{"/", "Search"},
					{"Esc", "Back"},
					{"r", "Re-plan"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateProgress:
		sections = []helpSection{
			{
				Title: "Actions",
				Items: [][2]string{
					{"Ctrl+C", "Interrupt (no rollback)"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateComplete:
		sections = []helpSection{
			{
				Title: "Actions",
				Items: [][2]string{
					{"r", "Re-plan"},
					{"Esc", "Back to menu"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateMainMenu:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"Space", "Expand/collapse"},
					{"Enter", "Select"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	case ViewStateValidate:
		sections = []helpSection{
			{
				Title: "Navigation",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
				},
			},
			{
				Title: "Actions",
				Items: [][2]string{
					{"Esc", "Back"},
					{"d", "Toggle detail view"},
					{"r", "Re-plan"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	default:
		sections = []helpSection{
			{
				Title: "General",
				Items: [][2]string{
					{"↑/k", "Move up"},
					{"↓/j", "Move down"},
					{"Enter", "Select"},
					{"Esc", "Back"},
					{"q", "Quit"},
					{"?", "Show this help"},
				},
			},
		}
	}

	for _, sec := range sections {
		b.WriteString(styles.BrandStyle.Render(sec.Title))
		b.WriteString("\n")
		for _, item := range sec.Items {
			b.WriteString(fmt.Sprintf("  %-10s %s\n", item[0], item[1]))
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.MutedStyle.Render("[?] Hide help"))

	return b.String()
}

func (m Model) countSelected() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotal() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += node.CountTotal()
	}
	return count
}

// renderFilterContent renders the filter/selection view for stop/restart/cleanup using the SelectionView component.
func (m Model) renderFilterContent() string {
	if m.Action.FilterView == nil {
		return styles.MutedStyle.Render("  No filter available.")
	}
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return m.Action.FilterView.Render(availableHeight)
}

// renderInfoListContent renders the information list view using the InfoListView component.
func (m Model) renderInfoListContent() string {
	title := "Information"
	var columns []components.InfoColumn
	var rows []components.InfoRow
	var statLine string

	switch m.Action.OperationType {
	case "server_show":
		title = "Servers"
		columns = []components.InfoColumn{
			{Header: "ZONE", Width: styles.ColZoneWidth, Align: "left"},
			{Header: "SERVER", Width: styles.ColServerWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			for _, srv := range m.Config.Servers {
				rows = append(rows, components.InfoRow{Cells: []string{srv.Zone, srv.Name}})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Cells[0] != rows[j].Cells[0] {
					return rows[i].Cells[0] < rows[j].Cells[0]
				}
				return rows[i].Cells[1] < rows[j].Cells[1]
			})
		}
		zoneSet := make(map[string]bool)
		for _, srv := range m.Config.Servers {
			zoneSet[srv.Zone] = true
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d servers", len(m.UI.InfoListFilteredRows), len(m.Config.Servers))
		} else {
			statLine = fmt.Sprintf("Total: %d servers in %d zones", len(m.Config.Servers), len(zoneSet))
		}
	case "dns_show":
		title = "DNS Records"
		columns = []components.InfoColumn{
			{Header: "DOMAIN", Width: styles.ColDomainWidth, Align: "left"},
			{Header: "ISP", Width: styles.ColISPWidth, Align: "left"},
			{Header: "RECORDS", Width: styles.ColRecordsWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			for _, domain := range m.Config.Domains {
				rows = append(rows, components.InfoRow{Cells: []string{domain.Name, domain.DNSISP, fmt.Sprintf("%d records", len(domain.Records))}})
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Cells[0] < rows[j].Cells[0]
			})
		}
		totalRecords := 0
		for _, d := range m.Config.Domains {
			totalRecords += len(d.Records)
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d domains", len(m.UI.InfoListFilteredRows), len(m.Config.Domains))
		} else {
			statLine = fmt.Sprintf("Total: %d domains, %d records", len(m.Config.Domains), totalRecords)
		}
	case "show":
		title = "Services"
		columns = []components.InfoColumn{
			{Header: "ZONE", Width: styles.ColZoneWidth, Align: "left"},
			{Header: "SERVER", Width: styles.ColServerWidth, Align: "left"},
			{Header: "SERVICE", Width: styles.ColServiceWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			serverZoneMap := make(map[string]string)
			for _, srv := range m.Config.Servers {
				serverZoneMap[srv.Name] = srv.Zone
			}
			showBiz, showInfra := m.getSelectedServiceTypes()
			if showBiz {
				for _, svc := range m.Config.Services {
					zone := serverZoneMap[svc.Server]
					rows = append(rows, components.InfoRow{Cells: []string{zone, svc.Server, svc.Name}})
				}
			}
			if showInfra {
				for _, svc := range m.Config.InfraServices {
					zone := serverZoneMap[svc.Server]
					rows = append(rows, components.InfoRow{Cells: []string{zone, svc.Server, svc.Name}})
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Cells[0] != rows[j].Cells[0] {
					return rows[i].Cells[0] < rows[j].Cells[0]
				}
				if rows[i].Cells[1] != rows[j].Cells[1] {
					return rows[i].Cells[1] < rows[j].Cells[1]
				}
				return rows[i].Cells[2] < rows[j].Cells[2]
			})
		}
		serverSet := make(map[string]bool)
		zoneSet := make(map[string]bool)
		serverZoneMap := make(map[string]string)
		for _, srv := range m.Config.Servers {
			serverZoneMap[srv.Name] = srv.Zone
		}
		showBiz, showInfra := m.getSelectedServiceTypes()
		total := 0
		if showBiz {
			for _, svc := range m.Config.Services {
				serverSet[svc.Server] = true
				zoneSet[serverZoneMap[svc.Server]] = true
			}
			total += len(m.Config.Services)
		}
		if showInfra {
			for _, svc := range m.Config.InfraServices {
				serverSet[svc.Server] = true
				zoneSet[serverZoneMap[svc.Server]] = true
			}
			total += len(m.Config.InfraServices)
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d services", len(m.UI.InfoListFilteredRows), total)
		} else {
			statLine = fmt.Sprintf("Total: %d services across %d servers in %d zones", total, len(serverSet), len(zoneSet))
		}
	case "config_show_isps":
		title = "ISPs"
		columns = []components.InfoColumn{
			{Header: "ISP", Width: styles.ColISPWidth, Align: "left"},
			{Header: "TYPE", Width: styles.ColTypeWidth, Align: "left"},
			{Header: "SERVICES", Width: styles.ColServicesWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			for _, isp := range m.Config.ISPs {
				var services []string
				for _, svc := range isp.Services {
					services = append(services, string(svc))
				}
				rows = append(rows, components.InfoRow{Cells: []string{isp.Name, string(isp.Type), strings.Join(services, ", ")}})
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Cells[0] < rows[j].Cells[0]
			})
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d ISPs", len(m.UI.InfoListFilteredRows), len(m.Config.ISPs))
		} else {
			statLine = fmt.Sprintf("Total: %d ISPs", len(m.Config.ISPs))
		}
	case "config_show_registries":
		title = "Registries"
		columns = []components.InfoColumn{
			{Header: "REGISTRY", Width: styles.ColRegistryWidth, Align: "left"},
			{Header: "URL", Width: styles.ColURLWidth, Align: "left"},
			{Header: "NAMESPACE", Width: styles.ColNSWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			for _, reg := range m.Config.Registries {
				rows = append(rows, components.InfoRow{
					Cells: []string{reg.Name, reg.URL, extractRegistryNamespace(reg.URL)},
				})
			}
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d registries", len(m.UI.InfoListFilteredRows), len(m.Config.Registries))
		} else {
			statLine = fmt.Sprintf("Total: %d registries", len(m.Config.Registries))
		}
	case "config_show_secrets":
		title = "Secrets"
		columns = []components.InfoColumn{
			{Header: "KEY", Width: styles.ColKeyWidth, Align: "left"},
		}
		if m.UI.InfoListFilteredRows != nil {
			rows = m.UI.InfoListFilteredRows
		} else {
			secrets := m.Config.GetSecretsMap()
			keys := make([]string, 0, len(secrets))
			for key := range secrets {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				rows = append(rows, components.InfoRow{Cells: []string{key}})
			}
		}
		if m.UI.InfoListFilteredRows != nil {
			statLine = fmt.Sprintf("Filtered: %d of %d secrets", len(m.UI.InfoListFilteredRows), len(m.Config.Secrets))
		} else {
			statLine = fmt.Sprintf("Total: %d secrets", len(m.Config.Secrets))
		}
	default:
		statLine = styles.MutedStyle.Render("No data loaded")
	}

	var content strings.Builder

	// Show search bar when active or when filter is applied
	if m.Search.Active {
		searchLine := m.Search.SearchFilter.Render()
		content.WriteString(searchLine)
		content.WriteString("\n")
		content.WriteString("\n")
	}

	ilv := components.NewInfoListView(title)
	ilv.SetColumns(columns)
	ilv.SetRows(rows)
	ilv.SetStatLine(statLine)
	if m.UI.InfoListIndex < len(rows) {
		ilv.Cursor = m.UI.InfoListIndex
	}
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	if m.Search.Active {
		availableHeight -= 2 // space for search bar
	}
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}
	content.WriteString(ilv.Render(availableHeight))
	return content.String()
}

// renderInfoDetailContent renders the information detail view using the InfoDetailView component.
func (m Model) renderInfoDetailContent() string {
	if m.Config == nil {
		idv := components.NewInfoDetailView("Detail")
		idv.SetStatLine(styles.MutedStyle.Render("No data loaded"))
		availableHeight := m.UI.Height - styles.LayoutOverhead - 2
		return idv.Render(availableHeight)
	}

	// Show search bar when active or when filter is applied
	var searchPrefix string
	if m.Search.Active {
		var sb strings.Builder
		sb.WriteString(m.Search.SearchFilter.Render())
		sb.WriteString("\n")
		sb.WriteString("\n")
		searchPrefix = sb.String()
	}

	// If filtered entities exist, render them directly
	if m.UI.InfoDetailFilteredEntities != nil {
		return m.renderFilteredInfoDetail(searchPrefix)
	}

	var content string
	switch m.Action.OperationType {
	case "show":
		content = m.renderServiceDetail()
	case "server_show":
		content = m.renderServerDetail()
	case "dns_show":
		content = m.renderDNSDetail()
	case "config_show_isps":
		content = m.renderISPDetail()
	case "config_show_registries":
		content = m.renderRegistryDetail()
	case "config_show_secrets":
		content = m.renderSecretDetail()
	default:
		idv := components.NewInfoDetailView("Detail")
		idv.SetStatLine(styles.MutedStyle.Render("No data loaded"))
		availableHeight := m.UI.Height - styles.LayoutOverhead - 2
		return idv.Render(availableHeight)
	}

	if searchPrefix != "" {
		return searchPrefix + content
	}
	return content
}

// renderFilteredInfoDetail renders the detail view with filtered entities.
func (m Model) renderFilteredInfoDetail(searchPrefix string) string {
	idv := components.NewInfoDetailView("Detail")

	for _, ent := range m.UI.InfoDetailFilteredEntities {
		idv.AddEntity(ent.Title, ent.Fields)
		for _, line := range ent.Lines {
			// Lines are stored in InfoEntityFiltered but InfoEntity doesn't have Lines via AddEntity.
			// We need to set them directly.
			if len(idv.Entities) > 0 {
				idv.Entities[len(idv.Entities)-1].Lines = append(idv.Entities[len(idv.Entities)-1].Lines, line)
			}
		}
	}

	totalEntities := len(m.buildInfoDetailEntities())
	statLine := fmt.Sprintf("Filtered: %d of %d entities", len(m.UI.InfoDetailFilteredEntities), totalEntities)
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	if m.Search.Active {
		availableHeight -= 2
	}
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	if searchPrefix != "" {
		return searchPrefix + idv.Render(availableHeight)
	}
	return idv.Render(availableHeight)
}

func (m Model) renderServiceDetail() string {
	idv := components.NewInfoDetailView("Services")

	showBiz, showInfra := m.getSelectedServiceTypes()

	// Build summary table
	serverZoneMap := make(map[string]string)
	for _, srv := range m.Config.Servers {
		serverZoneMap[srv.Name] = srv.Zone
	}

	summaryColumns := []components.InfoColumn{
		{Header: "ZONE", Width: styles.ColZoneWidth, Align: "left"},
		{Header: "SERVER", Width: styles.ColServerWidth, Align: "left"},
		{Header: "SERVICE", Width: styles.ColServiceWidth, Align: "left"},
		{Header: "IMAGE", Width: styles.ColImageWidth, Align: "left"},
	}

	var summaryRows []components.InfoRow
	if showBiz {
		for _, svc := range m.Config.Services {
			zone := serverZoneMap[svc.Server]
			image := svc.Image
			if len(svc.ExternalBackends) > 0 {
				image = strings.Join(svc.ExternalBackends, ", ")
			}
			summaryRows = append(summaryRows, components.InfoRow{Cells: []string{zone, svc.Server, svc.Name, image}})
		}
	}
	if showInfra {
		for _, infra := range m.Config.InfraServices {
			zone := serverZoneMap[infra.Server]
			summaryRows = append(summaryRows, components.InfoRow{Cells: []string{zone, infra.Server, infra.Name, infra.Image}})
		}
	}
	if len(summaryRows) > 0 {
		idv.SetSummaryTable(summaryColumns, summaryRows)
	}

	if showBiz {
		for _, svc := range m.Config.Services {
			var fields []components.InfoField

			if len(svc.ExternalBackends) > 0 {
				fields = append(fields, components.InfoField{Label: "Backends", Value: strings.Join(svc.ExternalBackends, ", ")})
			} else {
				fields = append(fields, components.InfoField{Label: "Image", Value: svc.Image})
			}

			if len(svc.Ports) > 0 {
				var portStrs []string
				for _, p := range svc.Ports {
					proto := p.Protocol
					if proto == "" {
						proto = "tcp"
					}
					portStrs = append(portStrs, fmt.Sprintf("%d:%d/%s", p.Host, p.Container, proto))
				}
				fields = append(fields, components.InfoField{Label: "Ports", Value: strings.Join(portStrs, ", ")})
			}

			if len(svc.Gateways) > 0 {
				var endpoints []string
				var gateways []string
				for _, gw := range svc.Gateways {
					var protos []string
					if gw.HTTP {
						protos = append(protos, "http")
					}
					if gw.HTTPS {
						protos = append(protos, "https")
					}
					if len(protos) > 0 {
						for _, p := range protos {
							path := gw.Path
							if path == "/" {
								path = ""
							}
							endpoints = append(endpoints, fmt.Sprintf("%s://%s%s", p, gw.Hostname, path))
						}
						gateways = append(gateways, fmt.Sprintf("%s (%s)", gw.Hostname, strings.Join(protos, "+")))
					}
				}
				if len(endpoints) > 0 {
					fields = append(fields, components.InfoField{Label: "Endpoints", Value: endpoints[0]})
					for _, ep := range endpoints[1:] {
						fields = append(fields, components.InfoField{Label: "", Value: ep, Level: 1})
					}
				}
				if len(gateways) > 0 {
					fields = append(fields, components.InfoField{Label: "Gateways", Value: strings.Join(gateways, ", ")})
				}
			}

			if svc.Healthcheck != nil {
				val := svc.Healthcheck.Path
				if svc.Healthcheck.Interval != "" {
					val += fmt.Sprintf(" (interval: %s)", svc.Healthcheck.Interval)
				}
				fields = append(fields, components.InfoField{Label: "Health", Value: val})
			}

			idv.AddEntity(fmt.Sprintf("SERVICE: %s", svc.Name), fields)
		}
	}

	if showInfra {
		for _, infra := range m.Config.InfraServices {
			var fields []components.InfoField

			fields = append(fields, components.InfoField{Label: "Type", Value: string(infra.Type)})
			fields = append(fields, components.InfoField{Label: "Image", Value: infra.Image})
			fields = append(fields, components.InfoField{Label: "Server", Value: infra.Server})

			if infra.GatewayPorts != nil {
				var portStrs []string
				if infra.GatewayPorts.HTTP > 0 {
					portStrs = append(portStrs, fmt.Sprintf("http:%d", infra.GatewayPorts.HTTP))
				}
				if infra.GatewayPorts.HTTPS > 0 {
					portStrs = append(portStrs, fmt.Sprintf("https:%d", infra.GatewayPorts.HTTPS))
				}
				if len(portStrs) > 0 {
					fields = append(fields, components.InfoField{Label: "Ports", Value: strings.Join(portStrs, ", ")})
				}
			}

			if infra.GatewaySSL != nil {
				sslVal := infra.GatewaySSL.Mode
				if infra.GatewaySSL.Endpoint != "" {
					sslVal += fmt.Sprintf(" (endpoint: %s)", infra.GatewaySSL.Endpoint)
				}
				fields = append(fields, components.InfoField{Label: "SSL", Value: sslVal})
			}

			if infra.GatewayWAF != nil && infra.GatewayWAF.Enabled {
				wafVal := "enabled"
				if len(infra.GatewayWAF.Whitelist) > 0 {
					wafVal += fmt.Sprintf(" (whitelist: %s)", strings.Join(infra.GatewayWAF.Whitelist, ", "))
				}
				fields = append(fields, components.InfoField{Label: "WAF", Value: wafVal})
			}

			idv.AddEntity(fmt.Sprintf("INFRA: %s", infra.Name), fields)
		}
	}

	bizCount := 0
	infraCount := 0
	serverSet := make(map[string]bool)
	zoneSet := make(map[string]bool)
	if showBiz {
		bizCount = len(m.Config.Services)
		for _, svc := range m.Config.Services {
			serverSet[svc.Server] = true
			zoneSet[serverZoneMap[svc.Server]] = true
		}
	}
	if showInfra {
		infraCount = len(m.Config.InfraServices)
		for _, svc := range m.Config.InfraServices {
			serverSet[svc.Server] = true
			zoneSet[serverZoneMap[svc.Server]] = true
		}
	}
	total := bizCount + infraCount
	statLine := fmt.Sprintf("Total: %d services across %d servers in %d zones", total, len(serverSet), len(zoneSet))
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

func (m Model) renderServerDetail() string {
	idv := components.NewInfoDetailView("Servers")

	summaryColumns := []components.InfoColumn{
		{Header: "ZONE", Width: styles.ColZoneWidth, Align: "left"},
		{Header: "SERVER", Width: styles.ColServerWidth, Align: "left"},
	}
	var summaryRows []components.InfoRow
	for _, srv := range m.Config.Servers {
		summaryRows = append(summaryRows, components.InfoRow{Cells: []string{srv.Zone, srv.Name}})
	}
	if len(summaryRows) > 0 {
		idv.SetSummaryTable(summaryColumns, summaryRows)
	}

	for _, srv := range m.Config.Servers {
		var fields []components.InfoField

		if srv.IP.Public != "" {
			fields = append(fields, components.InfoField{Label: "Public IP", Value: srv.IP.Public})
		}
		if srv.IP.Private != "" {
			fields = append(fields, components.InfoField{Label: "Private IP", Value: srv.IP.Private})
		}

		sshHost := srv.SSH.Host
		if sshHost == "" && srv.IP.Public != "" {
			sshHost = srv.IP.Public
		} else if sshHost == "" && srv.IP.Private != "" {
			sshHost = srv.IP.Private
		}
		fields = append(fields, components.InfoField{
			Label: "SSH",
			Value: fmt.Sprintf("%s@%s:%d", srv.SSH.User, sshHost, srv.SSH.Port),
		})

		if srv.ISP != "" {
			fields = append(fields, components.InfoField{Label: "Provider", Value: srv.ISP})
		}

		if len(srv.Networks) > 0 {
			var netStrs []string
			for _, n := range srv.Networks {
				netStrs = append(netStrs, fmt.Sprintf("%s (%s)", n.Name, n.GetType()))
			}
			fields = append(fields, components.InfoField{Label: "Networks", Value: strings.Join(netStrs, ", ")})
		}

		idv.AddEntity(fmt.Sprintf("SERVER: %s", srv.Name), fields)
	}
	zoneSet := make(map[string]struct{})
	for _, srv := range m.Config.Servers {
		if srv.Zone != "" {
			zoneSet[srv.Zone] = struct{}{}
		}
	}
	statLine := fmt.Sprintf("Total: %d servers in %d zones", len(m.Config.Servers), len(zoneSet))
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

func (m Model) renderDNSDetail() string {
	idv := components.NewInfoDetailView("DNS Records")

	summaryColumns := []components.InfoColumn{
		{Header: "ISP", Width: styles.ColISPWidth, Align: "left"},
		{Header: "RECORDS", Width: styles.ColRecordsWidth, Align: "left"},
	}
	var summaryRows []components.InfoRow
	for _, domain := range m.Config.Domains {
		summaryRows = append(summaryRows, components.InfoRow{Cells: []string{domain.DNSISP, fmt.Sprintf("%d records", len(domain.Records))}})
	}
	if len(summaryRows) > 0 {
		idv.SetSummaryTable(summaryColumns, summaryRows)
	}

	for _, domain := range m.Config.Domains {
		fields := []components.InfoField{
			{Label: "ISP", Value: domain.DNSISP},
		}

		var lines []string
		if len(domain.Records) > 0 {
			lines = append(lines, fmt.Sprintf("%-16s %s", "Records:", ""))
			lines = append(lines, fmt.Sprintf("  %-10s %-16s %-30s %s", "TYPE", "NAME", "VALUE", "TTL"))
			for _, rec := range domain.Records {
				lines = append(lines, fmt.Sprintf("  %-10s %-16s %-30s %d", string(rec.Type), rec.Name, rec.Value, rec.TTL))
			}
		}

		entity := components.InfoEntity{
			Title:  fmt.Sprintf("DOMAIN: %s", domain.Name),
			Fields: fields,
			Lines:  lines,
		}
		idv.Entities = append(idv.Entities, entity)
	}

	totalRecords := 0
	for _, d := range m.Config.Domains {
		totalRecords += len(d.Records)
	}
	statLine := fmt.Sprintf("Total: %d domains, %d records", len(m.Config.Domains), totalRecords)
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

// getISPEndpoint returns the DNS endpoint for an ISP, checking credentials first, then using defaults.
func getISPEndpoint(isp entity.ISP) string {
	if endpoint, ok := isp.Credentials["endpoint"]; ok {
		if endpoint.Plain() != "" {
			return endpoint.Plain()
		}
	}
	// Default endpoints based on type
	switch isp.Type {
	case "aliyun":
		return "dns.aliyuncs.com"
	case "cloudflare":
		return "api.cloudflare.com"
	case "tencent":
		return "dnspod.tencentcloudapi.com"
	default:
		return ""
	}
}

func (m Model) renderISPDetail() string {
	idv := components.NewInfoDetailView("ISPs")

	summaryColumns := []components.InfoColumn{
		{Header: "ISP", Width: styles.ColISPWidth, Align: "left"},
		{Header: "TYPE", Width: styles.ColTypeWidth, Align: "left"},
		{Header: "SERVICES", Width: styles.ColServicesWidth, Align: "left"},
	}
	var summaryRows []components.InfoRow
	for _, isp := range m.Config.ISPs {
		var services []string
		for _, svc := range isp.Services {
			services = append(services, string(svc))
		}
		summaryRows = append(summaryRows, components.InfoRow{Cells: []string{isp.Name, string(isp.Type), strings.Join(services, ", ")}})
	}
	if len(summaryRows) > 0 {
		idv.SetSummaryTable(summaryColumns, summaryRows)
	}

	for _, isp := range m.Config.ISPs {
		var fields []components.InfoField

		fields = append(fields, components.InfoField{Label: "Type", Value: string(isp.Type)})

		var svcNames []string
		for _, svc := range isp.Services {
			svcNames = append(svcNames, string(svc))
		}
		fields = append(fields, components.InfoField{Label: "Services", Value: strings.Join(svcNames, ", ")})

		var regionZones []string
		for _, z := range m.Config.Zones {
			if z.ISP == isp.Name {
				regionZones = append(regionZones, z.Region)
			}
		}
		if len(regionZones) > 0 {
			fields = append(fields, components.InfoField{Label: "Regions", Value: strings.Join(regionZones, ", ")})
		}

		if isp.HasService(entity.ISPServiceDNS) {
			endpoint := getISPEndpoint(isp)
			if endpoint != "" {
				fields = append(fields, components.InfoField{Label: "Endpoint", Value: endpoint})
			}
		}

		idv.AddEntity(fmt.Sprintf("ISP: %s", isp.Name), fields)
	}
	statLine := fmt.Sprintf("Total: %d ISPs", len(m.Config.ISPs))
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

func (m Model) renderRegistryDetail() string {
	idv := components.NewInfoDetailView("Registries")

	summaryColumns := []components.InfoColumn{
		{Header: "REGISTRY", Width: styles.ColRegistryWidth, Align: "left"},
		{Header: "URL", Width: styles.ColURLWidth, Align: "left"},
		{Header: "NAMESPACE", Width: styles.ColNSWidth, Align: "left"},
	}
	var summaryRows []components.InfoRow
	for _, reg := range m.Config.Registries {
		summaryRows = append(summaryRows, components.InfoRow{Cells: []string{reg.Name, reg.URL, extractRegistryNamespace(reg.URL)}})
	}
	if len(summaryRows) > 0 {
		idv.SetSummaryTable(summaryColumns, summaryRows)
	}

	for _, reg := range m.Config.Registries {
		var fields []components.InfoField

		fields = append(fields, components.InfoField{Label: "URL", Value: reg.URL})
		fields = append(fields, components.InfoField{Label: "Namespace", Value: extractRegistryNamespace(reg.URL)})

		authStatus := "not configured"
		if reg.Credentials.Username.Plain() != "" || reg.Credentials.Username.Secret() != "" ||
			reg.Credentials.Password.Plain() != "" || reg.Credentials.Password.Secret() != "" {
			authStatus = "configured"
		}
		fields = append(fields, components.InfoField{Label: "Auth", Value: authStatus})

		idv.AddEntity(fmt.Sprintf("REGISTRY: %s", reg.Name), fields)
	}
	statLine := fmt.Sprintf("Total: %d registries", len(m.Config.Registries))
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

func extractRegistryNamespace(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func (m Model) renderSecretDetail() string {
	idv := components.NewInfoDetailView("Secrets")
	columns := []components.InfoColumn{
		{Header: "KEY", Width: styles.ColKeyWidth, Align: "left"},
	}
	var rows []components.InfoRow
	for _, s := range m.Config.Secrets {
		rows = append(rows, components.InfoRow{Cells: []string{s.Name}})
	}
	idv.SetSummaryTable(columns, rows)

	for _, s := range m.Config.Secrets {
		fields := []components.InfoField{
			{Label: "Source", Value: s.Source},
			{Label: "Description", Value: ""},
		}
		idv.AddEntity("SECRET: "+s.Name, fields)
	}

	statLine := fmt.Sprintf("Total: %d secrets", len(m.Config.Secrets))
	idv.SetStatLine(statLine)
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

// getSelectedServiceTypes returns which service types should be shown based on the filter selection.
// If no filter is set, both are shown (default behavior).
func (m Model) getSelectedServiceTypes() (showBiz, showInfra bool) {
	if m.Action.FilterView == nil {
		return true, true
	}
	for _, label := range m.Action.FilterView.GetSelectedLabels(0) {
		switch label {
		case "biz":
			showBiz = true
		case "infra":
			showInfra = true
		}
	}
	return
}

func (m Model) infoListMaxIndex() int {
	if m.Config == nil {
		return 0
	}
	var count int
	switch m.Action.OperationType {
	case "show":
		showBiz, showInfra := m.getSelectedServiceTypes()
		if showBiz {
			count += len(m.Config.Services)
		}
		if showInfra {
			count += len(m.Config.InfraServices)
		}
	case "server_show":
		count = len(m.Config.Servers)
	case "dns_show":
		count = len(m.Config.Domains)
	case "config_show_isps":
		count = len(m.Config.ISPs)
	case "config_show_registries":
		count = len(m.Config.Registries)
	case "config_show_secrets":
		count = len(m.Config.Secrets)
	}
	if count > 0 {
		return count - 1
	}
	return 0
}

func (m Model) infoDetailMaxIndex() int {
	if m.Config == nil {
		return 0
	}
	count := 0
	switch m.Action.OperationType {
	case "show":
		showBiz, showInfra := m.getSelectedServiceTypes()
		// Summary table: header + rows
		bizCount := 0
		infraCount := 0
		if showBiz {
			bizCount = len(m.Config.Services)
		}
		if showInfra {
			infraCount = len(m.Config.InfraServices)
		}
		totalServices := bizCount + infraCount
		if totalServices > 0 {
			count++                // summary header
			count += totalServices // summary rows
		}
		if showBiz {
			for _, svc := range m.Config.Services {
				count += 2 // blank + title
				if len(svc.ExternalBackends) > 0 {
					count++ // Backends
				} else {
					count++ // Image
				}
				if len(svc.Ports) > 0 {
					count++ // Ports
				}
				if len(svc.Gateways) > 0 {
					count += len(svc.Gateways) + 1 // Endpoints + Gateways
				}
				if svc.Healthcheck != nil {
					count++ // Health
				}
			}
		}
		if showInfra {
			for _, infra := range m.Config.InfraServices {
				count += 2 // blank + title
				count += 3 // Type + Image + Server
				if infra.GatewayPorts != nil {
					var portCount int
					if infra.GatewayPorts.HTTP > 0 {
						portCount++
					}
					if infra.GatewayPorts.HTTPS > 0 {
						portCount++
					}
					if portCount > 0 {
						count++ // Ports
					}
				}
				if infra.GatewaySSL != nil {
					count++ // SSL
				}
				if infra.GatewayWAF != nil && infra.GatewayWAF.Enabled {
					count++ // WAF
				}
			}
		}
	case "server_show":
		// Summary table
		if len(m.Config.Servers) > 0 {
			count++                        // summary header
			count += len(m.Config.Servers) // summary rows
		}
		for _, srv := range m.Config.Servers {
			count += 2 // blank + title
			if srv.IP.Public != "" {
				count++
			}
			if srv.IP.Private != "" {
				count++
			}
			count++ // SSH
			if srv.ISP != "" {
				count++
			}
			if len(srv.Networks) > 0 {
				count++
			}
		}
	case "dns_show":
		// Summary table
		if len(m.Config.Domains) > 0 {
			count++                        // summary header
			count += len(m.Config.Domains) // summary rows
		}
		for _, domain := range m.Config.Domains {
			count += 2 // blank + title
			count++    // ISP field
			if len(domain.Records) > 0 {
				count++                      // Records label
				count += 1                   // table header (TYPE NAME VALUE TTL)
				count += len(domain.Records) // record rows
			}
		}
	case "config_show_isps":
		// Summary table
		if len(m.Config.ISPs) > 0 {
			count++                     // summary header
			count += len(m.Config.ISPs) // summary rows
		}
		for _, isp := range m.Config.ISPs {
			count += 2 // blank + title
			count += 2 // Type + Services
			var regionZones []string
			for _, z := range m.Config.Zones {
				if z.ISP == isp.Name {
					regionZones = append(regionZones, z.Region)
				}
			}
			if len(regionZones) > 0 {
				count++ // Regions
			}
			if isp.HasService(entity.ISPServiceDNS) {
				endpoint := getISPEndpoint(isp)
				if endpoint != "" {
					count++ // Endpoint
				}
			}
		}
	case "config_show_registries":
		// Summary table
		if len(m.Config.Registries) > 0 {
			count++                           // summary header
			count += len(m.Config.Registries) // summary rows
		}
		for range m.Config.Registries {
			count += 2 // blank + title
			count += 3 // URL + Namespace + Auth
		}
	case "config_show_secrets":
		// Summary table (secrets uses InfoListView-style table, not summary)
		if len(m.Config.Secrets) > 0 {
			count++ // table header
		}
		count += len(m.Config.Secrets) // table rows
		// 每个 Secret 的详情
		for range m.Config.Secrets {
			count += 2 // blank + title
			count += 2 // Source + Description fields
		}
	}
	count += 2 // blank + stat line
	if count > 0 {
		return count - 1
	}
	return 0
}

func (m Model) validateMaxIndex() int {
	if m.UI.Validate == nil {
		return 0
	}

	// Count only scrollable content lines (errors/warnings section),
	// matching the viewport logic in ValidateView.Render().
	contentLines := 0
	if m.UI.Validate.Failed > 0 || m.UI.Validate.Warnings > 0 {
		errorIdx := 0
		warnIdx := 0
		for _, item := range m.UI.Validate.Errors {
			if item.Level == "error" {
				if errorIdx == 0 {
					contentLines++ // section header "ERRORS:"
				}
				contentLines++ // message line
				if item.Suggestion != "" {
					contentLines++ // suggestion line
				}
				contentLines++ // blank line
				errorIdx++
			}
		}
		for _, item := range m.UI.Validate.Errors {
			if item.Level == "warning" {
				if warnIdx == 0 {
					contentLines++ // section header "WARNINGS:"
				}
				contentLines++ // message line
				if item.Suggestion != "" {
					contentLines++ // suggestion line
				}
				contentLines++ // blank line
				warnIdx++
			}
		}
	}

	if contentLines > 0 {
		return contentLines - 1
	}
	return 0
}

// renderValidateContent renders the validation result view using the ValidateView component.
func (m Model) renderValidateContent() string {
	if m.UI.Validate == nil {
		// 无验证结果，显示空状态
		module := "unknown"
		switch m.Action.OperationType {
		case "validate":
			module = "service"
		case "server_validate":
			module = "server"
		case "dns_validate":
			module = "dns"
		case "config_validate":
			module = "config"
		}
		vv := components.NewValidateView(module, string(m.Environment))
		availableHeight := m.UI.Height - styles.LayoutOverhead - 2
		return vv.Render(availableHeight)
	}

	vv := components.NewValidateView(m.UI.Validate.Module, string(m.Environment))
	// 设置验证结果
	var results []components.ValidateResult
	for _, item := range m.UI.Validate.Errors {
		results = append(results, components.ValidateResult{
			Level:      item.Level,
			Message:    item.Message,
			Suggestion: item.Suggestion,
		})
	}
	vv.SetResults(m.UI.Validate.Passed, results)
	vv.Cursor = m.UI.ValidateCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return vv.Render(availableHeight)
}

// renderServiceMenuContent renders the Service Management submenu content.
func (m Model) renderServiceMenuContent() string {
	items := []string{
		"Show services",
		"Validate services",
		"Deploy services",
		"Stop services",
		"Restart services",
		"Cleanup orphan resources",
		"Back to Main Menu",
	}
	var content strings.Builder
	for i, item := range items {
		if i == m.Server.ServiceMenuIndex {
			content.WriteString(styles.MenuSelectedStyle.Render("▶ " + item))
		} else {
			content.WriteString(styles.MenuItemStyle.Render("  " + item))
		}
		content.WriteString("\n")
	}
	return content.String()
}

// renderServerMenuContent renders the Server Management submenu content.
func (m Model) renderServerMenuContent() string {
	items := []string{
		"Show servers",
		"Validate servers",
		"Setup server environment",
		"Docker prune",
		"Back to Main Menu",
	}
	var content strings.Builder
	for i, item := range items {
		if i == m.Server.ServiceMenuIndex {
			content.WriteString(styles.MenuSelectedStyle.Render("▶ " + item))
		} else {
			content.WriteString(styles.MenuItemStyle.Render("  " + item))
		}
		content.WriteString("\n")
	}
	return content.String()
}

// renderDNSMenuContent renders the DNS Management submenu content.
func (m Model) renderDNSMenuContent() string {
	items := []string{
		"Show DNS records",
		"Validate DNS configuration",
		"Deploy DNS records",
		"Pull domains from ISP",
		"Pull records from ISP",
		"Back to Main Menu",
	}
	var content strings.Builder
	for i, item := range items {
		if i == m.DNS.DNSMenuIndex {
			content.WriteString(styles.MenuSelectedStyle.Render("▶ " + item))
		} else {
			content.WriteString(styles.MenuItemStyle.Render("  " + item))
		}
		content.WriteString("\n")
	}
	return content.String()
}

// renderConfigMenuContent renders the Configuration submenu content.
func (m Model) renderConfigMenuContent() string {
	items := []string{
		"Show ISPs",
		"Show Registries",
		"Show Secrets",
		"Validate Config",
		"Back to Main Menu",
	}
	var content strings.Builder
	for i, item := range items {
		if i == m.UI.ConfigMenuIndex {
			content.WriteString(styles.MenuSelectedStyle.Render("▶ " + item))
		} else {
			content.WriteString(styles.MenuItemStyle.Render("  " + item))
		}
		content.WriteString("\n")
	}
	return content.String()
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
