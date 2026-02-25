package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	serverpkg "github.com/litelake/yamlops/internal/environment"
)

func tickSpinner() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{t}
	})
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
	case serverCheckCompleteMsg:
		return m.handleServerCheckCompleteMsg(msg)
	case serverSyncCompleteMsg:
		return m.handleServerSyncCompleteMsg(msg)
	case serverEnvCheckAllMsg:
		return m.handleServerEnvCheckAllMsg(msg)
	case serverEnvSyncAllMsg:
		return m.handleServerEnvSyncAllMsg(msg)
	case serviceCleanupCompleteMsg:
		return m.handleServiceCleanupCompleteMsg(msg)
	case serviceStopCompleteMsg:
		return m.handleServiceStopCompleteMsg(msg)
	case serviceRestartCompleteMsg:
		return m.handleServiceRestartCompleteMsg(msg)
	case applyProgressMsg:
		return m.handleApplyProgressMsg(msg, &cmds)
	case applyCompleteAsyncMsg:
		return m.handleApplyCompleteAsyncMsg(msg)
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
		m.UI.ErrorMessage = fmt.Sprintf("Failed to load config: %v", msg.err)
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
		m.UI.ErrorMessage = fmt.Sprintf("Failed to generate plan: %v", msg.err)
		return m, nil
	}
	m.Action.PlanResult = msg.plan
	m.Action.ApplyTotal = len(msg.plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
	m.ViewState = ViewStatePlan
	return m, nil
}

func (m Model) handleServiceStatusFetchedMsg(msg serviceStatusFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to fetch service status: %v", msg.err)
		return m, nil
	}
	m.Stop.ServiceStatusMap = msg.statusMap
	m.Tree.TreeNodes = m.buildAppTree()
	m.Stop.StopSelected = make(map[int]bool)
	for _, node := range m.Tree.TreeNodes {
		node.SelectRecursive(false)
	}
	m.applyServiceStatusToTree()
	m.ViewState = ViewStateServiceStop
	return m, nil
}

func (m Model) handleRestartStatusFetchedMsg(msg restartStatusFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to fetch service status: %v", msg.err)
		return m, nil
	}
	m.Restart.ServiceStatusMap = msg.statusMap
	m.Tree.TreeNodes = m.buildAppTree()
	m.Restart.RestartSelected = make(map[int]bool)
	for _, node := range m.Tree.TreeNodes {
		node.SelectRecursive(false)
	}
	m.applyRestartServiceStatusToTree()
	m.ViewState = ViewStateServiceRestart
	return m, nil
}

func (m Model) handleDNSDomainsFetchedMsg(msg dnsDomainsFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to fetch domains: %v", msg.err)
		m.ViewState = ViewStateDNSManagement
		return m, nil
	}
	m.DNS.DNSPullDiffs = msg.diffs
	if len(m.DNS.DNSPullDiffs) > 0 {
		m.ViewState = ViewStateDNSPullDiff
		m.DNS.DNSPullCursor = 0
		m.DNS.DNSPullSelected = make(map[int]bool)
		for i, diff := range m.DNS.DNSPullDiffs {
			if diff.ChangeType == valueobject.ChangeTypeCreate {
				m.DNS.DNSPullSelected[i] = true
			}
		}
	} else {
		m.ViewState = ViewStateDNSPullDiff
	}
	return m, nil
}

func (m Model) handleDNSRecordsFetchedMsg(msg dnsRecordsFetchedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Failed to fetch records: %v", msg.err)
		m.ViewState = ViewStateDNSManagement
		return m, nil
	}
	m.DNS.DNSRecordDiffs = msg.diffs
	if len(m.DNS.DNSRecordDiffs) > 0 {
		m.ViewState = ViewStateDNSPullDiff
		m.DNS.DNSPullCursor = 0
		m.DNS.DNSPullSelected = make(map[int]bool)
		for i, diff := range m.DNS.DNSRecordDiffs {
			if diff.ChangeType == valueobject.ChangeTypeCreate || diff.ChangeType == valueobject.ChangeTypeUpdate {
				m.DNS.DNSPullSelected[i] = true
			}
		}
	} else {
		m.ViewState = ViewStateDNSPullDiff
	}
	return m, nil
}

func (m Model) handleOrphanServicesScannedMsg(msg orphanServicesScannedMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = msg.err.Error()
		return m, nil
	}
	m.Cleanup.CleanupResults = msg.results
	if m.UI.ErrorMessage == "" {
		m.ViewState = ViewStateServiceCleanup
		m.Cleanup.CleanupCursor = 0
		m.buildCleanupSelected()
	}
	return m, nil
}

func (m Model) handleServerCheckCompleteMsg(msg serverCheckCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Server check failed: %v", msg.err)
		return m, nil
	}
	m.Server.ServerCheckResults = msg.results
	m.Server.ServerSyncResults = nil
	m.ViewState = ViewStateServerCheck
	return m, nil
}

func (m Model) handleServerSyncCompleteMsg(msg serverSyncCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Server sync failed: %v", msg.err)
		return m, nil
	}
	m.Server.ServerSyncResults = msg.results
	m.Server.ServerCheckResults = nil
	m.ViewState = ViewStateServerCheck
	return m, nil
}

func (m Model) handleServerEnvCheckAllMsg(msg serverEnvCheckAllMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Server check failed: %v", msg.err)
		return m, nil
	}
	m.ServerEnv.Results = msg.results
	m.ServerEnv.SyncResults = msg.syncResults
	m.ServerEnv.ResultsScrollY = 0
	m.ViewState = ViewStateServerCheck
	return m, nil
}

func (m Model) handleServerEnvSyncAllMsg(msg serverEnvSyncAllMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Server sync failed: %v", msg.err)
		return m, nil
	}
	if m.ServerEnv.Results == nil {
		m.ServerEnv.Results = make(map[string][]serverpkg.CheckResult)
	}
	m.ServerEnv.SyncResults = msg.results
	m.ServerEnv.ResultsScrollY = 0
	m.ViewState = ViewStateServerCheck
	return m, nil
}

func (m Model) handleServiceCleanupCompleteMsg(msg serviceCleanupCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Cleanup failed: %v", msg.err)
		return m, nil
	}
	m.Cleanup.CleanupResults = msg.results
	m.ViewState = ViewStateServiceCleanupComplete
	return m, nil
}

func (m Model) handleServiceStopCompleteMsg(msg serviceStopCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Stop failed: %v", msg.err)
		return m, nil
	}
	m.Stop.StopResults = msg.results
	m.ViewState = ViewStateServiceStopComplete
	return m, nil
}

func (m Model) handleServiceRestartCompleteMsg(msg serviceRestartCompleteMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Restart failed: %v", msg.err)
		return m, nil
	}
	m.Restart.RestartResults = msg.results
	m.ViewState = ViewStateServiceRestartComplete
	return m, nil
}

func (m Model) handleApplyProgressMsg(msg applyProgressMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	if m.ViewState == ViewStateApplyProgress && !m.Action.ApplyComplete {
		if m.Action.ApplyInProgress {
			m.Action.ApplyProgress++
			if m.Action.ApplyProgress >= m.Action.ApplyTotal {
				*cmds = append(*cmds, m.executeApplyAsync())
				return m, tea.Batch(*cmds...)
			}
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
		m.UI.ErrorMessage = fmt.Sprintf("Apply failed: %v", msg.err)
	}
	m.ViewState = ViewStateApplyComplete
	return m, nil
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.ShowHelp {
		m.ShowHelp = false
		return m, nil
	}
	if m.Loading.Active {
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "esc":
		return m.handleEscape()
	case "x":
		return m.handleCancel()
	case "up", "k":
		return m.handleUp(), nil
	case "down", "j":
		return m.handleDown(), nil
	case " ":
		return m.handleSpace(), nil
	case "enter":
		return m.handleEnter()
	case "tab":
		return m.handleTab(), nil
	case "a":
		if m.ViewState == ViewStateDNSPullDiff {
			return m.handleDNSPullSelectAll(true), nil
		}
		return m.handleSelectCurrent(true), nil
	case "n":
		if m.ViewState == ViewStateDNSPullDiff {
			return m.handleDNSPullSelectAll(false), nil
		}
		return m.handleSelectCurrent(false), nil
	case "A":
		return m.handleSelectAll(true), nil
	case "N":
		return m.handleSelectAll(false), nil
	case "p":
		return m.handlePlan()
	case "r":
		if m.ViewState == ViewStateServerCheck {
			if m.ServerEnv.CountSelected() > 0 {
				m.Loading.Active = true
				m.Loading.Message = "Checking server environments..."
				return m, tea.Batch(tickSpinner(), m.executeServerEnvCheckAsync())
			}
			return m, nil
		}
		return m.handleRefresh()
	case "s":
		if m.ViewState == ViewStateServerCheck {
			if m.ServerEnv.CountSelected() > 0 {
				m.Loading.Active = true
				m.Loading.Message = "Syncing server environments..."
				return m, tea.Batch(tickSpinner(), m.executeServerEnvSyncAsync())
			}
			return m, nil
		}
		return m, nil
	case "?":
		m.ShowHelp = true
		return m, nil
	}
	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateTree:
		if m.TreeSource == ViewStateDNSManagement {
			m.ViewState = ViewStateDNSManagement
		} else {
			m.ViewState = ViewStateServiceManagement
		}
		m.UI.ErrorMessage = ""
	case ViewStateServiceManagement:
		m.ViewState = ViewStateMainMenu
	case ViewStateServerSetup:
		m.ViewState = ViewStateServiceManagement
		m.UI.ErrorMessage = ""
	case ViewStateServerCheck:
		m.ViewState = ViewStateServerSetup
		m.UI.ErrorMessage = ""
	case ViewStateDNSManagement:
		m.ViewState = ViewStateMainMenu
	case ViewStateDNSPullDomains, ViewStateDNSPullRecords:
		m.ViewState = ViewStateDNSManagement
	case ViewStateDNSPullDiff:
		m.DNS.DNSPullDiffs = nil
		m.DNS.DNSRecordDiffs = nil
		m.DNS.DNSPullSelected = nil
		m.ViewState = ViewStateDNSManagement
	case ViewStateServiceCleanup:
		m.Cleanup.CleanupResults = nil
		m.Cleanup.CleanupSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStateServiceCleanupConfirm:
		m.ViewState = ViewStateServiceCleanup
	case ViewStateServiceCleanupComplete:
		m.Cleanup.CleanupResults = nil
		m.Cleanup.CleanupSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStateServiceStop:
		m.ViewState = ViewStateServiceManagement
		m.Stop.StopSelected = nil
	case ViewStateServiceStopConfirm:
		m.ViewState = ViewStateServiceStop
	case ViewStateServiceStopComplete:
		m.Stop.StopResults = nil
		m.Stop.StopSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStateServiceRestart:
		m.ViewState = ViewStateServiceManagement
		m.Restart.RestartSelected = nil
	case ViewStateServiceRestartConfirm:
		m.ViewState = ViewStateServiceRestart
	case ViewStateServiceRestartComplete:
		m.Restart.RestartResults = nil
		m.Restart.RestartSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStatePlan:
		m.ViewState = ViewStateTree
	case ViewStateApplyConfirm:
		m.ViewState = ViewStatePlan
	default:
		m.ViewState = ViewStateMainMenu
		m.UI.ErrorMessage = ""
	}
	return m, nil
}

func (m Model) handleCancel() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateApplyConfirm:
		m.ViewState = ViewStatePlan
	case ViewStateDNSPullDiff:
		m.DNS.DNSPullDiffs = nil
		m.DNS.DNSRecordDiffs = nil
		m.DNS.DNSPullSelected = nil
		m.ViewState = ViewStateDNSManagement
	default:
		m.ViewState = ViewStateMainMenu
		m.UI.ErrorMessage = ""
	}
	return m, nil
}

func Run(env string, configDir string) error {
	p := tea.NewProgram(NewModel(env, configDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runTUI(ctx *Context) {
	if err := Run(ctx.Env, ctx.ConfigDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
