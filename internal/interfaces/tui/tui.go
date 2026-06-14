package tui

import (
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
)

func tickSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{t}
	})
}

// formatErrorMessage formats an error message in three-part format:
// Error: {short description}
// Details: {detailed information}
// Suggestion: {fix suggestion}
func formatErrorMessage(shortDesc string, err error) string {
	details := err.Error()
	suggestion := generateErrorSuggestion(err)
	return fmt.Sprintf("ERROR: %s\n\nDetails: %s\nSuggestion: %s", shortDesc, details, suggestion)
}

// generateErrorSuggestion generates a suggestion based on the error type.
func generateErrorSuggestion(err error) string {
	switch {
	case errors.Is(err, domainerr.ErrConfigNotLoaded) || errors.Is(err, domainerr.ErrConfigReadFailed):
		return "Check if the configuration file exists and has correct permissions"
	case errors.Is(err, domainerr.ErrConfigParseFailed) || errors.Is(err, domainerr.ErrConfigValidateFail):
		return "Run 'validate' to see detailed configuration errors"
	case errors.Is(err, domainerr.ErrConfigNotFound):
		return "Ensure the environment configuration directory exists"
	case errors.Is(err, domainerr.ErrSSHConnectFailed):
		return "Check server SSH configuration and network connectivity"
	case errors.Is(err, domainerr.ErrSSHAuthFailed):
		return "Verify SSH credentials (user, key/password)"
	case errors.Is(err, domainerr.ErrSSHHostKeyMismatch):
		return "Update known_hosts or verify server identity"
	case errors.Is(err, domainerr.ErrSSHCommandFailed):
		return "Check command permissions and Docker daemon status on the server"
	case errors.Is(err, domainerr.ErrSSHSessionFailed):
		return "Check SSH service availability on the server"
	case errors.Is(err, domainerr.ErrSSHFileTransfer):
		return "Check disk space and file permissions on the server"
	case errors.Is(err, domainerr.ErrSSHClientNotAvailable):
		return "Retry the operation; if persistent, check server environment setup"
	case errors.Is(err, domainerr.ErrDockerComposeFailed):
		return "Check Docker daemon status and container logs"
	case errors.Is(err, domainerr.ErrNetworkTimeout):
		return "Check network connectivity and retry"
	case errors.Is(err, domainerr.ErrNetworkUnreachable):
		return "Verify server network configuration"
	case errors.Is(err, domainerr.ErrConnectionRefused):
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
	case errors.Is(err, domainerr.ErrISPNotFound):
		return "Run 'dns pull domains' to list available ISPs"
	case errors.Is(err, domainerr.ErrISPNoDNSService):
		return "Check ISP configuration in isps.yaml"
	default:
		return "Check logs for details"
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case spinnerTickMsg:
		return m.handleSpinnerTickMsg(msg)
	case configLoadedMsg:
		return m.handleConfigLoadedMsg(msg)
	case planGeneratedMsg:
		return m.handlePlanGeneratedMsg(msg)
	case serviceStatusFetchedMsg:
		return m.handleServiceStatusFetchedMsg(msg)
	case restartStatusFetchedMsg:
		return m.handleRestartStatusFetchedMsg(msg)
	case dnsDomainsFetchedMsg:
		return m.handleDNSDomainsFetchedMsg(msg)
	case dnsRecordsFetchedMsg:
		return m.handleDNSRecordsFetchedMsg(msg)
	case orphanServicesScannedMsg:
		return m.handleOrphanServicesScannedMsg(msg)
	case serviceCleanupCompleteMsg:
		return m.handleServiceCleanupCompleteMsg(msg)
	case applyProgressMsg:
		return m.handleApplyProgressMsg(msg, &cmds)
	case applyCompleteAsyncMsg:
		return m.handleApplyCompleteAsyncMsg(msg)
	case validateCompleteMsg:
		return m.handleValidateCompleteMsg(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}
	return m, nil
}

func (m Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.UI.Width = msg.Width
	m.UI.Height = msg.Height
	return m, nil
}

func (m Model) handleSpinnerTickMsg(msg spinnerTickMsg) (tea.Model, tea.Cmd) {
	if m.Loading.Active {
		m.Loading.Spinner = (m.Loading.Spinner + 1) % len(SpinnerFrames)
		return m, tickSpinner()
	}
	return m, nil
}

func (m Model) handleConfigLoadedMsg(msg configLoadedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Failed to load configuration", msg.err)
		return m, nil
	}
	m.Config = msg.config
	for i := range m.Config.Servers {
		m.Server.ServerList = append(m.Server.ServerList, &m.Config.Servers[i])
	}
	m.buildTrees()
	return m, nil
}

func (m Model) handlePlanGeneratedMsg(msg planGeneratedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Failed to generate plan", msg.err)
		return m, nil
	}
	// Restore DNS diffs carried from force regeneration (async closure can't mutate model)
	if msg.isDNSPullForce {
		if msg.forceDomainDiffs != nil {
			m.DNS.DNSPullDiffs = msg.forceDomainDiffs
			m.DNS.DNSRecordDiffs = nil
		} else if msg.forceRecordDiffs != nil {
			m.DNS.DNSRecordDiffs = msg.forceRecordDiffs
			m.DNS.DNSPullDiffs = nil
		}
	}
	m.Action.PlanResult = msg.plan
	m.Action.ApplyTotal = len(msg.plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
	if msg.isDNSPullForce {
		m.initDNSPullPlanComponent()
	} else {
		m.initPlanComponent()
	}
	m.ViewState = ViewStatePlan
	return m, nil
}

func (m Model) handleServiceStatusFetchedMsg(msg serviceStatusFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Failed to fetch service status", msg.err)
		return m, nil
	}
	m.Stop.ServiceStatusMap = msg.statusMap
	m.Tree.TreeNodes = m.buildAppTree()
	m.Stop.StopSelected = make(map[int]bool)
	for _, node := range m.Tree.TreeNodes {
		node.SelectRecursive(false)
	}
	m.applyServiceStatusToTree()
	m.buildFilterView()
	m.ViewState = ViewStateFilter
	return m, nil
}

func (m Model) handleRestartStatusFetchedMsg(msg restartStatusFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Failed to fetch service status", msg.err)
		return m, nil
	}
	m.Restart.ServiceStatusMap = msg.statusMap
	m.Tree.TreeNodes = m.buildAppTree()
	m.Restart.RestartSelected = make(map[int]bool)
	for _, node := range m.Tree.TreeNodes {
		node.SelectRecursive(false)
	}
	m.applyRestartServiceStatusToTree()
	m.buildFilterView()
	m.ViewState = ViewStateFilter
	return m, nil
}

func (m Model) handleDNSDomainsFetchedMsg(msg dnsDomainsFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Loading.Active = false
		m.UI.ErrorMessage = formatErrorMessage("Failed to fetch domains", msg.err)
		m.ViewState = ViewStateDNSMenu
		return m, nil
	}

	// Accumulate diffs from this ISP
	m.DNS.AggregatedDomainDiffs = append(m.DNS.AggregatedDomainDiffs, msg.diffs...)

	// Remove the first pending ISP (just fetched)
	if len(m.DNS.PendingISPs) > 0 {
		m.DNS.PendingISPs = m.DNS.PendingISPs[1:]
	}

	// If more ISPs remain, fetch next
	if len(m.DNS.PendingISPs) > 0 {
		nextISP := m.DNS.PendingISPs[0]
		fetched := m.DNS.PendingISPsTotal - len(m.DNS.PendingISPs) + 1
		m.Loading.Message = fmt.Sprintf("Fetching domains from %s... (%d/%d)", nextISP, fetched, m.DNS.PendingISPsTotal)
		return m, tea.Batch(tickSpinner(), m.fetchDomainDiffsAsync(nextISP))
	}

	// All ISPs processed - deduplicate and finalize
	m.Loading.Active = false
	m.DNS.DNSPullDiffs = deduplicateDomainDiffs(m.DNS.AggregatedDomainDiffs)
	m.DNS.DNSRecordDiffs = nil
	m.DNS.DNSPullSelected = nil
	m.DNS.DNSPullCursor = 0
	m.DNS.AggregatedDomainDiffs = nil
	m.DNS.PendingISPs = nil
	m.Action.OperationType = "dns_pull_domains"
	m.initDNSPullPlanComponent()
	m.ViewState = ViewStatePlan
	return m, nil
}

func (m Model) handleDNSRecordsFetchedMsg(msg dnsRecordsFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Loading.Active = false
		m.UI.ErrorMessage = formatErrorMessage("Failed to fetch DNS records", msg.err)
		m.ViewState = ViewStateDNSMenu
		return m, nil
	}

	// Accumulate diffs from this domain
	m.DNS.AggregatedRecordDiffs = append(m.DNS.AggregatedRecordDiffs, msg.diffs...)

	// Remove the first pending domain (just fetched)
	if len(m.DNS.PendingDomains) > 0 {
		m.DNS.PendingDomains = m.DNS.PendingDomains[1:]
	}

	// If more domains remain, fetch next
	if len(m.DNS.PendingDomains) > 0 {
		nextDomain := m.DNS.PendingDomains[0]
		fetched := m.DNS.PendingDomainsTotal - len(m.DNS.PendingDomains) + 1
		m.Loading.Message = fmt.Sprintf("Fetching records for %s... (%d/%d)", nextDomain, fetched, m.DNS.PendingDomainsTotal)
		return m, tea.Batch(tickSpinner(), m.fetchRecordDiffsAsync(nextDomain))
	}

	// All domains processed - finalize
	m.Loading.Active = false
	m.DNS.DNSRecordDiffs = m.DNS.AggregatedRecordDiffs
	m.DNS.DNSPullDiffs = nil
	m.DNS.DNSPullSelected = nil
	m.DNS.DNSPullCursor = 0
	m.DNS.AggregatedRecordDiffs = nil
	m.DNS.PendingDomains = nil
	m.Action.OperationType = "dns_pull_records"
	m.initDNSPullPlanComponent()
	m.ViewState = ViewStatePlan
	return m, nil
}

func (m Model) handleOrphanServicesScannedMsg(msg orphanServicesScannedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Failed to scan orphan services", msg.err)
		return m, nil
	}
	m.Cleanup.CleanupResults = msg.results
	if m.UI.ErrorMessage == "" {
		m.Cleanup.CleanupCursor = 0
		m.buildCleanupSelected()
		m.buildFilterView()
		m.ViewState = ViewStateFilter
	}
	return m, nil
}

func (m Model) handleServiceCleanupCompleteMsg(msg serviceCleanupCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Cleanup failed", msg.err)
		return m, nil
	}
	m.Cleanup.CleanupResults = msg.results
	m.ViewState = ViewStateComplete
	return m, nil
}

func (m Model) handleApplyProgressMsg(msg applyProgressMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	if m.ViewState == ViewStateProgress && !m.Action.ApplyComplete {
		if m.Action.ApplyInProgress {
			m.syncProgressView()
			return m, tickApply()
		}
	}
	return m, nil
}

func (m Model) handleApplyCompleteAsyncMsg(msg applyCompleteAsyncMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	m.Action.ApplyResults = msg.results
	m.Action.ApplyComplete = true
	m.Action.ApplyInProgress = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Apply failed", msg.err)
	}
	// Sync any remaining progress updates from the tracker
	m.syncProgressView()
	if m.Action.ProgressView != nil {
		m.Action.ProgressView.MarkRemainingSkipped()
	}
	if !m.Action.Interrupted {
		m.ViewState = ViewStateComplete
	}
	return m, nil
}

func (m Model) handleValidateCompleteMsg(msg validateCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Validation failed", msg.err)
		return m, nil
	}
	// 存储验证结果
	m.UI.Validate = &ValidateState{
		Passed:   msg.passed,
		Failed:   msg.failed,
		Warnings: msg.warnings,
		Errors:   msg.errors,
		Module:   msg.module,
	}
	m.ViewState = ViewStateValidate
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ShowHelp {
		switch msg.String() {
		case "?", "esc":
			m.ShowHelp = false
		}
		return m, nil
	}
	if m.Loading.Active {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// 处理搜索模式下的输入
	if m.Search.Active {
		return m.handleSearchInput(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		if m.ViewState == ViewStateProgress {
			if m.Action.CancelFunc != nil {
				m.Action.CancelFunc()
			}
			if m.Action.ProgressView != nil {
				m.Action.ProgressView.MarkRemainingSkipped()
				m.Action.ProgressView.Interrupted = true
			}
			m.Action.ApplyInProgress = false
			m.Action.ApplyComplete = true
			m.Action.Interrupted = true
			m.Action.ProgressEndTime = time.Now()
			return m, nil
		}
		if m.ViewState == ViewStateMainMenu {
			return m, tea.Quit
		}
		return m, nil
	case "q":
		if m.ViewState == ViewStateMainMenu {
			return m, tea.Quit
		}
		return m, nil
	case "esc":
		return m.handleEscape()
	case "x":
		return m.handleCancel()
	case "up", "k":
		return m.handleUp(), nil
	case "down", "j":
		return m.handleDown(), nil
	case "left", "h":
		if m.ViewState == ViewStateFilter && m.Action.FilterView != nil {
			m.Action.FilterView.PrevGroup()
			return m, nil
		}
		if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS {
			node := m.getNodeAtIndex(m.Tree.CursorIndex)
			if node != nil && len(node.Children) > 0 && node.Expanded {
				node.Expanded = false
			}
			return m, nil
		}
		return m, nil
	case "right", "l":
		if m.ViewState == ViewStateFilter && m.Action.FilterView != nil {
			m.Action.FilterView.NextGroup()
			return m, nil
		}
		if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS {
			node := m.getNodeAtIndex(m.Tree.CursorIndex)
			if node != nil && len(node.Children) > 0 && !node.Expanded {
				node.Expanded = true
			}
			return m, nil
		}
		return m, nil
	case " ":
		return m.handleSpace(), nil
	case "enter":
		return m.handleEnter()
	case "tab":
		return m.handleTab(), nil
	case "a", "A":
		return m.handleSelectAll(true), nil
	case "n", "N":
		return m.handleSelectAll(false), nil
	case "p":
		return m.handlePlan()
	case "r":
		if m.ViewState == ViewStateComplete || (m.ViewState == ViewStateProgress && m.Action.Interrupted) {
			m.Action.ApplyResults = nil
			m.Action.ApplyComplete = false
			m.Action.ApplyProgress = 0
			m.Action.PlanResult = nil
			m.Action.Interrupted = false

			switch m.Action.OperationType {
			case "stop", "restart", "cleanup":
				m.generatePlanFromFilter()
				m.initPlanComponent()
				m.ViewState = ViewStatePlan
				m.Action.ConfirmSelected = 0
			default:
				m.Action.Forced = false
				m.Loading.Active = true
				m.Loading.Message = "Generating plan..."
				return m, tea.Batch(tickSpinner(), m.generatePlanAsync())
			}
			return m, nil
		}
		return m.handleRefresh()
	case "s":
		return m, nil
	case KeyDetail:
		if m.ViewState == ViewStatePlan {
			if m.Action.PlanComponent != nil {
				m.Action.PlanComponent.ToggleDetail()
			}
			return m, nil
		}
		if m.ViewState == ViewStateInfoList {
			m.ViewState = ViewStateInfoDetail
			return m, nil
		}
		if m.ViewState == ViewStateInfoDetail {
			m.ViewState = ViewStateInfoList
			return m, nil
		}
		return m, nil
	case "f":
		if m.ViewState == ViewStatePlan {
			if m.Action.PlanComponent != nil {
				wasForced := m.Action.PlanComponent.Forced
				m.Action.PlanComponent.ToggleForce()
				m.Action.Forced = m.Action.PlanComponent.Forced
				// When toggling from unfocused to forced with no items, regenerate plan
				if !wasForced && m.Action.Forced && m.Action.PlanComponent.NoChanges {
					m.Loading.Active = true
					m.Loading.Message = "Regenerating plan (forced)..."
					return m, tea.Batch(tickSpinner(), m.generateForcePlanAsync())
				}
			}
			return m, nil
		}
		return m, nil
	case "/":
		if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS ||
			m.ViewState == ViewStateInfoList || m.ViewState == ViewStateInfoDetail {
			return m.handleSearch(), nil
		}
		return m, nil
	case "?":
		m.ShowHelp = true
		return m, nil
	}
	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	// 清除搜索过滤（如果存在）
	if m.Tree.OriginalNodes != nil {
		m.clearSearchFilter()
	}

	// 清除信息展示视图的搜索
	if m.Search.Active {
		m.Search.Active = false
		m.Search.Query = ""
		m.Search.SearchFilter.Deactivate()
	}
	m.UI.InfoListFilteredRows = nil
	m.UI.InfoDetailFilteredEntities = nil

	switch m.ViewState {
	case ViewStateTreeService:
		m.ViewState = ViewStateServiceMenu
		m.UI.ErrorMessage = ""
	case ViewStateTreeDNS:
		m.ViewState = ViewStateDNSMenu
		m.UI.ErrorMessage = ""
	case ViewStateServiceMenu:
		m.ViewState = ViewStateMainMenu
	case ViewStateServerMenu:
		m.ViewState = ViewStateMainMenu
	case ViewStateDNSMenu:
		m.ViewState = ViewStateMainMenu
	case ViewStateConfigMenu:
		m.ViewState = ViewStateMainMenu
	case ViewStateFilter:
		if m.Action.OperationType == "cleanup" {
			m.Cleanup.CleanupResults = nil
			m.Cleanup.CleanupSelected = nil
		} else if m.Action.OperationType == "stop" {
			m.Stop.StopSelected = nil
		} else if m.Action.OperationType == "restart" {
			m.Restart.RestartSelected = nil
		}
		m.Action.FilterView = nil
		if m.Action.OperationType == "dns_pull_domains" || m.Action.OperationType == "dns_pull_records" {
			m.ViewState = ViewStateDNSMenu
		} else {
			m.ViewState = ViewStateServiceMenu
		}
	case ViewStatePlan:
		// 根据操作类型和来源返回正确的界面
		if m.Action.OperationType == "cleanup" || m.Action.OperationType == "stop" || m.Action.OperationType == "restart" {
			m.ViewState = ViewStateFilter
		} else if m.Action.OperationType == "dns_deploy" {
			m.ViewState = ViewStateTreeDNS
		} else if m.Action.OperationType == "dns_pull_domains" || m.Action.OperationType == "dns_pull_records" {
			m.DNS.DNSPullDiffs = nil
			m.DNS.DNSRecordDiffs = nil
			m.DNS.AggregatedRecordDiffs = nil
			m.DNS.AggregatedDomainDiffs = nil
			m.DNS.PendingDomains = nil
			m.DNS.PendingISPs = nil
			m.ViewState = ViewStateDNSMenu
		} else if m.Action.OperationType == "server_setup" {
			m.ViewState = ViewStateServerMenu
		} else {
			m.ViewState = ViewStateTreeService
		}
	case ViewStateProgress:
		if m.Action.Interrupted {
			m.ViewState = ViewStateMainMenu
			m.UI.ErrorMessage = ""
		} else {
			if m.Action.CancelFunc != nil {
				m.Action.CancelFunc()
			}
			if m.Action.ProgressView != nil {
				m.Action.ProgressView.MarkRemainingSkipped()
				m.Action.ProgressView.Interrupted = true
			}
			m.Action.ApplyInProgress = false
			m.Action.ApplyComplete = true
			m.Action.Interrupted = true
			m.Action.ProgressEndTime = time.Now()
		}
	case ViewStateComplete:
		m.ViewState = ViewStateMainMenu
		m.UI.ErrorMessage = ""
	case ViewStateInfoList, ViewStateInfoDetail, ViewStateValidate:
		// 根据来源菜单返回
		m.ViewState = m.getReturnMenu()
		m.UI.ErrorMessage = ""
	default:
		m.ViewState = ViewStateMainMenu
		m.UI.ErrorMessage = ""
	}
	return m, nil
}

// getReturnMenu 根据 SourceMenu 返回正确的上级菜单
func (m Model) getReturnMenu() ViewState {
	switch m.SourceMenu {
	case ViewStateServiceMenu, ViewStateServerMenu, ViewStateDNSMenu, ViewStateConfigMenu:
		return m.SourceMenu
	default:
		return ViewStateMainMenu
	}
}

func (m Model) handleCancel() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStatePlan:
		if m.Action.OperationType == "deploy" {
			m.ViewState = ViewStateTreeService
		} else if m.Action.OperationType == "dns_deploy" {
			m.ViewState = ViewStateTreeDNS
		} else if m.Action.OperationType == "dns_pull_domains" || m.Action.OperationType == "dns_pull_records" {
			m.DNS.DNSPullDiffs = nil
			m.DNS.DNSRecordDiffs = nil
			m.DNS.AggregatedRecordDiffs = nil
			m.DNS.AggregatedDomainDiffs = nil
			m.DNS.PendingDomains = nil
			m.DNS.PendingISPs = nil
			m.ViewState = ViewStateDNSMenu
		} else if m.Action.OperationType == "server_setup" {
			m.ViewState = ViewStateServerMenu
		} else {
			m.ViewState = ViewStateFilter
		}
	default:
		m.ViewState = ViewStateMainMenu
		m.UI.ErrorMessage = ""
	}
	return m, nil
}

func Run(env string, configDir string, concurrency int) error {
	p := tea.NewProgram(NewModel(env, configDir, concurrency), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// deduplicateDomainDiffs removes duplicate domain diffs when multiple ISPs are fetched.
// When the same domain appears from multiple ISPs, keep the one that matches the local config's DNSISP
// (if exists locally), or keep the first occurrence.
func deduplicateDomainDiffs(diffs []DomainDiff) []DomainDiff {
	seen := make(map[string]bool)
	var result []DomainDiff
	for _, diff := range diffs {
		if !seen[diff.Name] {
			seen[diff.Name] = true
			result = append(result, diff)
		}
	}
	return result
}
