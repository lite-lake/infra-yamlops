package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type applyProgressMsg struct{}

func tickApply() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return applyProgressMsg{}
	})
}

func (m Model) startLoading(message string) tea.Cmd {
	m.Loading.Active = true
	m.Loading.Message = message
	m.Loading.Spinner = 0
	return tickSpinner()
}

func (m *Model) stopLoading() {
	m.Loading.Active = false
	m.Loading.Message = ""
}

func (m Model) handleUp() Model {
	switch m.ViewState {
	case ViewStateMainMenu:
		if m.UI.MainMenuIndex > 0 {
			m.UI.MainMenuIndex--
		}
	case ViewStateServiceManagement:
		if m.Server.ServiceMenuIndex > 0 {
			m.Server.ServiceMenuIndex--
		}
	case ViewStateServerSetup:
		if m.ServerEnv.CursorIndex > 0 {
			m.ServerEnv.CursorIndex--
		}
	case ViewStateServerCheck:
		if m.ServerEnv.ResultsScrollY > 0 {
			m.ServerEnv.ResultsScrollY--
		}
	case ViewStateDNSManagement:
		if m.DNS.DNSMenuIndex > 0 {
			m.DNS.DNSMenuIndex--
		}
	case ViewStateDNSPullDomains:
		if m.DNS.DNSISPIndex > 0 {
			m.DNS.DNSISPIndex--
		}
	case ViewStateDNSPullRecords:
		if m.DNS.DNSDomainIndex > 0 {
			m.DNS.DNSDomainIndex--
		}
	case ViewStateDNSPullDiff:
		if m.DNS.DNSPullCursor > 0 {
			m.DNS.DNSPullCursor--
		}
	case ViewStateTree:
		if m.Tree.CursorIndex > 0 {
			m.Tree.CursorIndex--
		}
	case ViewStateApplyConfirm:
		if m.Action.ConfirmSelected > 0 {
			m.Action.ConfirmSelected--
		}
	case ViewStateServiceCleanup:
		if m.Cleanup.CleanupCursor > 0 {
			m.Cleanup.CleanupCursor--
		}
	case ViewStateServiceCleanupConfirm:
		if m.Action.ConfirmSelected > 0 {
			m.Action.ConfirmSelected--
		}
	case ViewStateServiceStop:
		if m.Tree.CursorIndex > 0 {
			m.Tree.CursorIndex--
		}
	case ViewStateServiceStopConfirm:
		if m.Action.ConfirmSelected > 0 {
			m.Action.ConfirmSelected--
		}
	case ViewStateServiceRestart:
		if m.Tree.CursorIndex > 0 {
			m.Tree.CursorIndex--
		}
	case ViewStateServiceRestartConfirm:
		if m.Action.ConfirmSelected > 0 {
			m.Action.ConfirmSelected--
		}
	}
	return m
}

func (m Model) handleDown() Model {
	switch m.ViewState {
	case ViewStateMainMenu:
		if m.UI.MainMenuIndex < 2 {
			m.UI.MainMenuIndex++
		}
	case ViewStateServiceManagement:
		if m.Server.ServiceMenuIndex < 5 {
			m.Server.ServiceMenuIndex++
		}
	case ViewStateServerSetup:
		totalNodes := m.countServerEnvNodes()
		if m.ServerEnv.CursorIndex < totalNodes-1 {
			m.ServerEnv.CursorIndex++
		}
	case ViewStateServerCheck:
		totalLines := m.countServerEnvResultLines()
		availableHeight := m.UI.Height - 8
		if availableHeight < 5 {
			availableHeight = 5
		}
		maxScroll := totalLines - availableHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.ServerEnv.ResultsScrollY < maxScroll {
			m.ServerEnv.ResultsScrollY++
		}
	case ViewStateDNSManagement:
		if m.DNS.DNSMenuIndex < 3 {
			m.DNS.DNSMenuIndex++
		}
	case ViewStateDNSPullDomains:
		isps := m.getDNSISPs()
		if m.DNS.DNSISPIndex < len(isps)-1 {
			m.DNS.DNSISPIndex++
		}
	case ViewStateDNSPullRecords:
		domains := m.getDNSDomains()
		if m.DNS.DNSDomainIndex < len(domains)-1 {
			m.DNS.DNSDomainIndex++
		}
	case ViewStateDNSPullDiff:
		maxIdx := len(m.DNS.DNSPullDiffs) - 1
		if len(m.DNS.DNSRecordDiffs) > 0 {
			maxIdx = len(m.DNS.DNSRecordDiffs) - 1
		}
		if m.DNS.DNSPullCursor < maxIdx {
			m.DNS.DNSPullCursor++
		}
	case ViewStateTree:
		totalNodes := m.countVisibleNodes()
		if m.Tree.CursorIndex < totalNodes-1 {
			m.Tree.CursorIndex++
		}
	case ViewStateApplyConfirm:
		if m.Action.ConfirmSelected < 1 {
			m.Action.ConfirmSelected++
		}
	case ViewStateServiceCleanup:
		totalItems := m.countCleanupItems()
		if m.Cleanup.CleanupCursor < totalItems-1 {
			m.Cleanup.CleanupCursor++
		}
	case ViewStateServiceCleanupConfirm:
		if m.Action.ConfirmSelected < 1 {
			m.Action.ConfirmSelected++
		}
	case ViewStateServiceStop:
		totalNodes := m.countVisibleNodes()
		if m.Tree.CursorIndex < totalNodes-1 {
			m.Tree.CursorIndex++
		}
	case ViewStateServiceStopConfirm:
		if m.Action.ConfirmSelected < 1 {
			m.Action.ConfirmSelected++
		}
	case ViewStateServiceRestart:
		totalNodes := m.countVisibleNodes()
		if m.Tree.CursorIndex < totalNodes-1 {
			m.Tree.CursorIndex++
		}
	case ViewStateServiceRestartConfirm:
		if m.Action.ConfirmSelected < 1 {
			m.Action.ConfirmSelected++
		}
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
			m.Loading.Active = true
			m.Loading.Message = "Fetching service status..."
			return m, tea.Batch(tickSpinner(), m.fetchServiceStatusAsync())
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

func (m *Model) fetchDomainDiffs(ispName string) {
	m.DNS.DNSPullDiffs = nil
	m.UI.ErrorMessage = ""

	isp := m.Config.GetISPMap()[ispName]
	if isp == nil {
		m.UI.ErrorMessage = fmt.Sprintf("ISP '%s' not found", ispName)
		return
	}

	provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error creating DNS provider: %v", err)
		return
	}

	remoteDomains, err := provider.ListDomains()
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error listing domains: %v", err)
		return
	}

	localDomainMap := make(map[string]*entity.Domain)
	for i := range m.Config.Domains {
		localDomainMap[m.Config.Domains[i].Name] = &m.Config.Domains[i]
	}

	for _, domainName := range remoteDomains {
		if _, exists := localDomainMap[domainName]; !exists {
			m.DNS.DNSPullDiffs = append(m.DNS.DNSPullDiffs, DomainDiff{
				Name:       domainName,
				DNSISP:     ispName,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		} else {
			delete(localDomainMap, domainName)
		}
	}

	for _, localDomain := range localDomainMap {
		if localDomain.DNSISP == ispName {
			m.DNS.DNSPullDiffs = append(m.DNS.DNSPullDiffs, DomainDiff{
				Name:       localDomain.Name,
				DNSISP:     localDomain.DNSISP,
				ISP:        localDomain.ISP,
				Parent:     localDomain.Parent,
				ChangeType: valueobject.ChangeTypeDelete,
			})
		}
	}

	sort.Slice(m.DNS.DNSPullDiffs, func(i, j int) bool {
		return m.DNS.DNSPullDiffs[i].Name < m.DNS.DNSPullDiffs[j].Name
	})
}

func (m *Model) fetchDomainDiffsAsync(ispName string) tea.Cmd {
	return func() tea.Msg {
		isp := m.Config.GetISPMap()[ispName]
		if isp == nil {
			return dnsDomainsFetchedMsg{err: fmt.Errorf("ISP '%s' not found", ispName)}
		}

		provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
		if err != nil {
			return dnsDomainsFetchedMsg{err: err}
		}

		remoteDomains, err := provider.ListDomains()
		if err != nil {
			return dnsDomainsFetchedMsg{err: err}
		}

		localDomainMap := make(map[string]*entity.Domain)
		for i := range m.Config.Domains {
			localDomainMap[m.Config.Domains[i].Name] = &m.Config.Domains[i]
		}

		var diffs []DomainDiff
		for _, domainName := range remoteDomains {
			if _, exists := localDomainMap[domainName]; !exists {
				diffs = append(diffs, DomainDiff{
					Name:       domainName,
					DNSISP:     ispName,
					ChangeType: valueobject.ChangeTypeCreate,
				})
			} else {
				delete(localDomainMap, domainName)
			}
		}

		for _, localDomain := range localDomainMap {
			if localDomain.DNSISP == ispName {
				diffs = append(diffs, DomainDiff{
					Name:       localDomain.Name,
					DNSISP:     localDomain.DNSISP,
					ISP:        localDomain.ISP,
					Parent:     localDomain.Parent,
					ChangeType: valueobject.ChangeTypeDelete,
				})
			}
		}

		sort.Slice(diffs, func(i, j int) bool {
			return diffs[i].Name < diffs[j].Name
		})

		return dnsDomainsFetchedMsg{diffs: diffs}
	}
}

func (m *Model) fetchRecordDiffs(domainName string) {
	m.DNS.DNSRecordDiffs = nil
	m.UI.ErrorMessage = ""

	domain := m.Config.GetDomainMap()[domainName]
	if domain == nil {
		m.UI.ErrorMessage = fmt.Sprintf("Domain '%s' not found", domainName)
		return
	}

	isp := m.Config.GetISPMap()[domain.DNSISP]
	if isp == nil {
		m.UI.ErrorMessage = fmt.Sprintf("DNS ISP '%s' not found", domain.DNSISP)
		return
	}

	provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error creating DNS provider: %v", err)
		return
	}

	remoteRecords, err := provider.ListRecords(domainName)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error listing records: %v", err)
		return
	}

	localRecordMap := make(map[string]*entity.DNSRecord)
	for i := range domain.Records {
		key := fmt.Sprintf("%s:%s:%s", domain.Records[i].Type, domain.Records[i].Name, domain.Records[i].Value)
		localRecordMap[key] = &domain.Records[i]
		localRecordMap[key].Domain = domain.Name
	}

	for _, remote := range remoteRecords {
		recordName := remote.Name
		if recordName == domainName || recordName == "" {
			recordName = "@"
		} else if strings.HasSuffix(remote.Name, "."+domainName) {
			recordName = strings.TrimSuffix(remote.Name, "."+domainName)
		}

		key := fmt.Sprintf("%s:%s:%s", remote.Type, recordName, remote.Value)
		if local, exists := localRecordMap[key]; exists {
			if local.TTL != remote.TTL {
				m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
					Domain:     domainName,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeUpdate,
				})
			}
			delete(localRecordMap, key)
		} else {
			m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
				Domain:     domainName,
				Type:       entity.DNSRecordType(remote.Type),
				Name:       recordName,
				Value:      remote.Value,
				TTL:        remote.TTL,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		}
	}

	for _, local := range localRecordMap {
		m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
			Domain:     local.Domain,
			Type:       local.Type,
			Name:       local.Name,
			Value:      local.Value,
			TTL:        local.TTL,
			ChangeType: valueobject.ChangeTypeDelete,
		})
	}

	sort.Slice(m.DNS.DNSRecordDiffs, func(i, j int) bool {
		if m.DNS.DNSRecordDiffs[i].Name != m.DNS.DNSRecordDiffs[j].Name {
			return m.DNS.DNSRecordDiffs[i].Name < m.DNS.DNSRecordDiffs[j].Name
		}
		return m.DNS.DNSRecordDiffs[i].Type < m.DNS.DNSRecordDiffs[j].Type
	})
}

func (m *Model) fetchRecordDiffsAsync(domainName string) tea.Cmd {
	return func() tea.Msg {
		domain := m.Config.GetDomainMap()[domainName]
		if domain == nil {
			return dnsRecordsFetchedMsg{err: fmt.Errorf("domain '%s' not found", domainName)}
		}

		isp := m.Config.GetISPMap()[domain.DNSISP]
		if isp == nil {
			return dnsRecordsFetchedMsg{err: fmt.Errorf("DNS ISP '%s' not found", domain.DNSISP)}
		}

		provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
		if err != nil {
			return dnsRecordsFetchedMsg{err: err}
		}

		remoteRecords, err := provider.ListRecords(domainName)
		if err != nil {
			return dnsRecordsFetchedMsg{err: err}
		}

		localRecordMap := make(map[string]*entity.DNSRecord)
		for i := range domain.Records {
			key := fmt.Sprintf("%s:%s:%s", domain.Records[i].Type, domain.Records[i].Name, domain.Records[i].Value)
			localRecordMap[key] = &domain.Records[i]
			localRecordMap[key].Domain = domain.Name
		}

		var diffs []RecordDiff
		for _, remote := range remoteRecords {
			recordName := remote.Name
			if recordName == domainName || recordName == "" {
				recordName = "@"
			} else if strings.HasSuffix(remote.Name, "."+domainName) {
				recordName = strings.TrimSuffix(remote.Name, "."+domainName)
			}

			key := fmt.Sprintf("%s:%s:%s", remote.Type, recordName, remote.Value)
			if local, exists := localRecordMap[key]; exists {
				if local.TTL != remote.TTL {
					diffs = append(diffs, RecordDiff{
						Domain:     domainName,
						Type:       entity.DNSRecordType(remote.Type),
						Name:       recordName,
						Value:      remote.Value,
						TTL:        remote.TTL,
						ChangeType: valueobject.ChangeTypeUpdate,
					})
				}
				delete(localRecordMap, key)
			} else {
				diffs = append(diffs, RecordDiff{
					Domain:     domainName,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeCreate,
				})
			}
		}

		for _, local := range localRecordMap {
			diffs = append(diffs, RecordDiff{
				Domain:     local.Domain,
				Type:       local.Type,
				Name:       local.Name,
				Value:      local.Value,
				TTL:        local.TTL,
				ChangeType: valueobject.ChangeTypeDelete,
			})
		}

		sort.Slice(diffs, func(i, j int) bool {
			if diffs[i].Name != diffs[j].Name {
				return diffs[i].Name < diffs[j].Name
			}
			return diffs[i].Type < diffs[j].Type
		})

		return dnsRecordsFetchedMsg{diffs: diffs}
	}
}

func (m *Model) saveSelectedDiffs() {
	if len(m.DNS.DNSPullDiffs) > 0 {
		selectedDiffs := make([]DomainDiff, 0)
		for i, diff := range m.DNS.DNSPullDiffs {
			if m.DNS.DNSPullSelected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}
		if len(selectedDiffs) > 0 {
			m.saveDomainDiffsToConfig(selectedDiffs)
		}
	}
	if len(m.DNS.DNSRecordDiffs) > 0 {
		selectedDiffs := make([]RecordDiff, 0)
		for i, diff := range m.DNS.DNSRecordDiffs {
			if m.DNS.DNSPullSelected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}
		if len(selectedDiffs) > 0 {
			m.saveRecordDiffsToConfig(selectedDiffs)
		}
	}
}

func (m *Model) saveDomainDiffsToConfig(diffs []DomainDiff) {
	configDir := filepath.Join(m.ConfigDir, "userdata", string(m.Environment))
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		if diff.ChangeType == valueobject.ChangeTypeCreate {
			newDomains = append(newDomains, entity.Domain{
				Name:   diff.Name,
				DNSISP: diff.DNSISP,
			})
			domainSet[diff.Name] = true
		}
	}

	for _, d := range m.Config.Domains {
		if !domainSet[d.Name] {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Name == d.Name && diff.ChangeType == valueobject.ChangeTypeDelete {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomains = append(newDomains, d)
			}
		}
	}

	saveYAMLConfig(dnsPath, "domains", newDomains)
	m.Config = nil
	m.loadConfig()
	m.buildTrees()
}

func (m *Model) saveRecordDiffsToConfig(diffs []RecordDiff) {
	configDir := filepath.Join(m.ConfigDir, "userdata", string(m.Environment))
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		domainSet[diff.Domain] = true
	}

	for _, d := range m.Config.Domains {
		newDomain := entity.Domain{
			Name:   d.Name,
			ISP:    d.ISP,
			DNSISP: d.DNSISP,
			Parent: d.Parent,
		}
		for _, r := range d.Records {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Domain == d.Name && string(diff.Type) == string(r.Type) && diff.Name == r.Name &&
					(diff.ChangeType == valueobject.ChangeTypeDelete || diff.ChangeType == valueobject.ChangeTypeUpdate) {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomain.Records = append(newDomain.Records, r)
			}
		}
		if domainSet[d.Name] {
			for _, diff := range diffs {
				if diff.Domain == d.Name && (diff.ChangeType == valueobject.ChangeTypeCreate || diff.ChangeType == valueobject.ChangeTypeUpdate) {
					newDomain.Records = append(newDomain.Records, entity.DNSRecord{
						Type:  diff.Type,
						Name:  diff.Name,
						Value: diff.Value,
						TTL:   diff.TTL,
					})
				}
			}
			delete(domainSet, d.Name)
		}
		newDomains = append(newDomains, newDomain)
	}

	saveYAMLConfig(dnsPath, "domains", newDomains)
	m.Config = nil
	m.loadConfig()
	m.buildTrees()
}

func saveYAMLConfig(path, key string, data interface{}) {
	yamlData := map[string]interface{}{key: data}
	content, err := yaml.Marshal(yamlData)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, content, 0644)
}

func createDNSProviderFromConfig(isp *entity.ISP, secrets map[string]string) (dns.Provider, error) {
	switch isp.Type {
	case entity.ISPTypeAliyun:
		accessKeyIDRef := isp.Credentials["access_key_id"]
		accessKeyID, err := (&accessKeyIDRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve access_key_id: %w", err)
		}
		accessKeySecretRef := isp.Credentials["access_key_secret"]
		accessKeySecret, err := (&accessKeySecretRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve access_key_secret: %w", err)
		}
		return dns.NewAliyunProvider(accessKeyID, accessKeySecret)
	case entity.ISPTypeCloudflare:
		apiTokenRef := isp.Credentials["api_token"]
		apiToken, err := (&apiTokenRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve api_token: %w", err)
		}
		accountID := ""
		if accountIDRef, ok := isp.Credentials["account_id"]; ok {
			accountID, err = (&accountIDRef).Resolve(secrets)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve account_id: %w", err)
			}
		}
		return dns.NewCloudflareProvider(apiToken, accountID), nil
	case entity.ISPTypeTencent:
		secretIDRef := isp.Credentials["secret_id"]
		secretID, err := (&secretIDRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret_id: %w", err)
		}
		secretKeyRef := isp.Credentials["secret_key"]
		secretKey, err := (&secretKeyRef).Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret_key: %w", err)
		}
		return dns.NewTencentProvider(secretID, secretKey)
	default:
		return nil, fmt.Errorf("unsupported DNS provider type: %s (ISP name: %s)", isp.Type, isp.Name)
	}
}
