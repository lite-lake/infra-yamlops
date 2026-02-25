package cli

import (
	"github.com/charmbracelet/bubbletea"
)

func (m Model) handleUp() Model {
	ctrl := GetCursorController(m.ViewState, &m)
	if ctrl == nil {
		return m
	}
	if ctrl.GetCursor() > 0 {
		ctrl.SetCursor(ctrl.GetCursor() - 1)
	}
	return m
}

func (m Model) handleDown() Model {
	ctrl := GetCursorController(m.ViewState, &m)
	if ctrl == nil {
		return m
	}
	maxVal := ctrl.MaxValue()
	if ctrl.GetCursor() < maxVal {
		ctrl.SetCursor(ctrl.GetCursor() + 1)
	}
	return m
}

func (m Model) handleSpace() Model {
	if m.ViewState == ViewStateDNSPullDiff {
		if len(m.DNS.DNSPullDiffs) > 0 || len(m.DNS.DNSRecordDiffs) > 0 {
			m.DNS.DNSPullSelected[m.DNS.DNSPullCursor] = !m.DNS.DNSPullSelected[m.DNS.DNSPullCursor]
		}
		return m
	}
	if m.ViewState == ViewStateServiceCleanup {
		if m.Cleanup.CleanupSelected != nil {
			m.Cleanup.CleanupSelected[m.Cleanup.CleanupCursor] = !m.Cleanup.CleanupSelected[m.Cleanup.CleanupCursor]
		}
		return m
	}
	if m.ViewState == ViewStateServerSetup {
		node := m.getServerEnvNodeAtIndex(m.ServerEnv.CursorIndex)
		if node != nil {
			node.Selected = !node.Selected
		}
		return m
	}
	if m.ViewState == ViewStateServiceStop {
		node := m.getNodeAtIndex(m.Tree.CursorIndex)
		if node == nil || len(node.Children) > 0 {
			return m
		}
		node.Selected = !node.Selected
		node.UpdateParentSelection()
		return m
	}
	if m.ViewState == ViewStateServiceRestart {
		node := m.getNodeAtIndex(m.Tree.CursorIndex)
		if node == nil || len(node.Children) > 0 {
			return m
		}
		node.Selected = !node.Selected
		node.UpdateParentSelection()
		return m
	}
	if m.ViewState != ViewStateTree {
		return m
	}
	node := m.getNodeAtIndex(m.Tree.CursorIndex)
	if node == nil || len(node.Children) > 0 {
		return m
	}
	node.Selected = !node.Selected
	node.UpdateParentSelection()
	return m
}

func (m Model) handleDNSPullSelectAll(selected bool) Model {
	if m.ViewState != ViewStateDNSPullDiff {
		return m
	}
	maxIdx := len(m.DNS.DNSPullDiffs)
	if len(m.DNS.DNSRecordDiffs) > 0 {
		maxIdx = len(m.DNS.DNSRecordDiffs)
	}
	for i := 0; i < maxIdx; i++ {
		m.DNS.DNSPullSelected[i] = selected
	}
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateMainMenu:
		switch m.UI.MainMenuIndex {
		case 0:
			m.ViewState = ViewStateServiceManagement
			m.Server.ServiceMenuIndex = 0
			return m, nil
		case 1:
			m.ViewState = ViewStateDNSManagement
			m.DNS.DNSMenuIndex = 0
			return m, nil
		case 2:
			return m, tea.Quit
		}
	case ViewStateServiceManagement:
		switch m.Server.ServiceMenuIndex {
		case 0:
			m.ViewState = ViewStateTree
			m.TreeSource = ViewStateServiceManagement
			m.ViewMode = ViewModeApp
			return m, nil
		case 1:
			m.Tree.CursorIndex = 0
			m.Loading.Active = true
			m.Loading.Message = "Fetching service status..."
			return m, tea.Batch(tickSpinner(), m.fetchServiceStatusAsync())
		case 2:
			m.Tree.CursorIndex = 0
			m.Loading.Active = true
			m.Loading.Message = "Fetching service status..."
			return m, tea.Batch(tickSpinner(), m.fetchRestartServiceStatusAsync())
		case 3:
			m.Loading.Active = true
			m.Loading.Message = "Scanning orphan services..."
			return m, tea.Batch(tickSpinner(), m.scanOrphanServicesAsync())
		case 4:
			m.ViewState = ViewStateServerSetup
			m.initServerEnvNodes()
			return m, nil
		case 5:
			m.ViewState = ViewStateMainMenu
			return m, nil
		}
	case ViewStateDNSManagement:
		switch m.DNS.DNSMenuIndex {
		case 0:
			m.ViewState = ViewStateDNSPullDomains
			m.DNS.DNSISPIndex = 0
			return m, nil
		case 1:
			m.ViewState = ViewStateDNSPullRecords
			m.DNS.DNSDomainIndex = 0
			return m, nil
		case 2:
			m.ViewState = ViewStateTree
			m.TreeSource = ViewStateDNSManagement
			m.ViewMode = ViewModeDNS
			return m, nil
		case 3:
			m.ViewState = ViewStateMainMenu
			return m, nil
		}
	case ViewStateDNSPullDomains:
		isps := m.getDNSISPs()
		if len(isps) > 0 && m.DNS.DNSISPIndex < len(isps) {
			ispName := isps[m.DNS.DNSISPIndex]
			m.Loading.Active = true
			m.Loading.Message = "Fetching domains from " + ispName + "..."
			return m, tea.Batch(tickSpinner(), m.fetchDomainDiffsAsync(ispName))
		}
		return m, nil
	case ViewStateDNSPullRecords:
		domains := m.getDNSDomains()
		if len(domains) > 0 && m.DNS.DNSDomainIndex < len(domains) {
			domainName := domains[m.DNS.DNSDomainIndex]
			m.Loading.Active = true
			m.Loading.Message = "Fetching records for " + domainName + "..."
			return m, tea.Batch(tickSpinner(), m.fetchRecordDiffsAsync(domainName))
		}
		return m, nil
	case ViewStateDNSPullDiff:
		if len(m.DNS.DNSPullDiffs) > 0 || len(m.DNS.DNSRecordDiffs) > 0 {
			m.saveSelectedDiffs()
		}
		m.ViewState = ViewStateDNSManagement
		m.DNS.DNSPullDiffs = nil
		m.DNS.DNSRecordDiffs = nil
		m.DNS.DNSPullSelected = nil
		return m, nil
	case ViewStateServerSetup:
		if m.ServerEnv.CountSelected() == 0 {
			return m, nil
		}
		switch m.ServerEnv.OperationIndex {
		case 0:
			m.Loading.Active = true
			m.Loading.Message = "Checking server environments..."
			return m, tea.Batch(tickSpinner(), m.executeServerEnvCheckAsync())
		case 1:
			m.Loading.Active = true
			m.Loading.Message = "Syncing server environments..."
			return m, tea.Batch(tickSpinner(), m.executeServerEnvSyncAsync())
		case 2:
			m.Loading.Active = true
			m.Loading.Message = "Running full setup..."
			return m, tea.Batch(tickSpinner(), m.executeServerEnvFullSetupAsync())
		}
		return m, nil
	case ViewStateServerCheck:
		m.ViewState = ViewStateServerSetup
		return m, nil
	case ViewStateTree:
		node := m.getNodeAtIndex(m.Tree.CursorIndex)
		if node == nil {
			return m, nil
		}
		node.Expanded = !node.Expanded
		return m, nil
	case ViewStateApplyConfirm:
		if m.Action.ConfirmSelected == 0 {
			m.ViewState = ViewStateApplyProgress
			m.Action.ApplyProgress = 0
			m.Action.ApplyComplete = false
			m.Action.ApplyResults = nil
			m.Action.ApplyInProgress = true
			return m, tickApply()
		}
		m.ViewState = ViewStatePlan
		return m, nil
	case ViewStatePlan:
		m.ViewState = ViewStateApplyConfirm
		m.Action.ConfirmSelected = 0
		return m, nil
	case ViewStateApplyComplete:
		m.ViewState = ViewStateTree
		return m, nil
	case ViewStateServiceCleanup:
		if m.hasSelectedCleanupItems() {
			m.ViewState = ViewStateServiceCleanupConfirm
			m.Action.ConfirmSelected = 0
		}
		return m, nil
	case ViewStateServiceCleanupConfirm:
		if m.Action.ConfirmSelected == 0 {
			m.Loading.Active = true
			m.Loading.Message = "Cleaning up services..."
			return m, tea.Batch(tickSpinner(), m.executeServiceCleanupAsync())
		}
		m.ViewState = ViewStateServiceCleanup
		return m, nil
	case ViewStateServiceCleanupComplete:
		m.ViewState = ViewStateServiceManagement
		m.Cleanup.CleanupResults = nil
		m.Cleanup.CleanupSelected = nil
		return m, nil
	case ViewStateServiceStop:
		node := m.getNodeAtIndex(m.Tree.CursorIndex)
		if node == nil {
			return m, nil
		}
		node.Expanded = !node.Expanded
		return m, nil
	case ViewStateServiceStopConfirm:
		if m.Action.ConfirmSelected == 0 {
			m.Loading.Active = true
			m.Loading.Message = "Stopping services..."
			return m, tea.Batch(tickSpinner(), m.executeServiceStopAsync())
		}
		m.ViewState = ViewStateServiceStop
		return m, nil
	case ViewStateServiceStopComplete:
		m.ViewState = ViewStateServiceManagement
		m.Stop.StopResults = nil
		m.Stop.StopSelected = nil
		return m, nil
	case ViewStateServiceRestart:
		node := m.getNodeAtIndex(m.Tree.CursorIndex)
		if node == nil {
			return m, nil
		}
		node.Expanded = !node.Expanded
		return m, nil
	case ViewStateServiceRestartConfirm:
		if m.Action.ConfirmSelected == 0 {
			m.Loading.Active = true
			m.Loading.Message = "Restarting services..."
			return m, tea.Batch(tickSpinner(), m.executeServiceRestartAsync())
		}
		m.ViewState = ViewStateServiceRestart
		return m, nil
	case ViewStateServiceRestartComplete:
		m.ViewState = ViewStateServiceManagement
		m.Restart.RestartResults = nil
		m.Restart.RestartSelected = nil
		return m, nil
	}
	return m, nil
}

func (m Model) handleTab() Model {
	switch m.ViewState {
	case ViewStateServerSetup:
		m.ServerEnv.OperationIndex = (m.ServerEnv.OperationIndex + 1) % len(serverEnvOperations)
	case ViewStateTree:
		if m.ViewMode == ViewModeApp {
			m.ViewMode = ViewModeDNS
		} else {
			m.ViewMode = ViewModeApp
		}
		m.Tree.CursorIndex = 0
	}
	return m
}

func (m Model) handleSelectCurrent(selected bool) Model {
	if m.ViewState == ViewStateServerSetup {
		node := m.getServerEnvNodeAtIndex(m.ServerEnv.CursorIndex)
		if node != nil {
			node.Selected = selected
		}
		return m
	}
	if m.ViewState != ViewStateTree && m.ViewState != ViewStateServiceStop && m.ViewState != ViewStateServiceRestart {
		return m
	}
	node := m.getNodeAtIndex(m.Tree.CursorIndex)
	if node == nil {
		return m
	}
	node.SelectRecursive(selected)
	node.UpdateParentSelection()
	return m
}

func (m Model) handleSelectAll(selected bool) Model {
	if m.ViewState == ViewStateServerSetup {
		m.ServerEnv.SelectAll(selected)
		return m
	}
	if m.ViewState != ViewStateTree && m.ViewState != ViewStateServiceStop && m.ViewState != ViewStateServiceRestart {
		return m
	}
	nodes := m.getCurrentTree()
	for _, node := range nodes {
		node.SelectRecursive(selected)
	}
	return m
}

func (m Model) handlePlan() (tea.Model, tea.Cmd) {
	if m.ViewState == ViewStateTree {
		m.Loading.Active = true
		m.Loading.Message = "Generating plan..."
		return m, tea.Batch(tickSpinner(), m.generatePlanAsync())
	}
	if m.ViewState == ViewStateServiceStop {
		if m.hasSelectedStopServices() {
			m.ViewState = ViewStateServiceStopConfirm
			m.Action.ConfirmSelected = 0
		}
		return m, nil
	}
	if m.ViewState == ViewStateServiceRestart {
		if m.hasSelectedRestartServices() {
			m.ViewState = ViewStateServiceRestartConfirm
			m.Action.ConfirmSelected = 0
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	m.Config = nil
	m.Loading.Active = true
	m.Loading.Message = "Reloading config..."
	return m, tea.Batch(tickSpinner(), m.loadConfigAsync())
}
