package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
)

func (m Model) handleUp() Model {
	if m.ViewState == ViewStateFilter && m.Action.FilterView != nil {
		m.Action.FilterView.CursorUp()
		return m
	}
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
	if m.ViewState == ViewStateFilter && m.Action.FilterView != nil {
		m.Action.FilterView.CursorDown()
		return m
	}
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
	if m.ViewState == ViewStateMainMenu {
		rows := m.flattenMenuRows()
		idx := m.UI.MainMenuIndex
		if idx >= 0 && idx < len(rows) && rows[idx].isParent {
			pi := rows[idx].parent
			m.UI.MenuNodes[pi].Expanded = !m.UI.MenuNodes[pi].Expanded
		}
		return m
	}
	if m.ViewState == ViewStatePlan {
		if m.Action.PlanComponent != nil {
			m.Action.PlanComponent.ToggleCurrent()
		}
		return m
	}
	if m.ViewState == ViewStateFilter {
		if m.Action.FilterView != nil {
			m.Action.FilterView.ToggleCurrent()
			m.updateFilterMatchedLine()
		}
		return m
	}
	if m.ViewState != ViewStateTreeService && m.ViewState != ViewStateTreeDNS {
		return m
	}
	node := m.getNodeAtIndex(m.Tree.CursorIndex)
	if node == nil {
		return m
	}
	if len(node.Children) > 0 {
		node.Expanded = !node.Expanded
		return m
	}
	node.Selected = !node.Selected
	node.UpdateParentSelection()
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateMainMenu:
		rows := m.flattenMenuRows()
		idx := m.UI.MainMenuIndex
		if idx < 0 || idx >= len(rows) {
			return m, nil
		}
		row := rows[idx]
		if row.isParent {
			// Toggle expand/collapse on parent
			m.UI.MenuNodes[row.parent].Expanded = !m.UI.MenuNodes[row.parent].Expanded
			return m, nil
		}
		// Leaf item: execute the corresponding operation
		child := m.UI.MenuNodes[row.parent].Children[row.child]
		return m.executeMainMenuOperation(child.Operation)
	case ViewStateServiceMenu:
		switch m.Server.ServiceMenuIndex {
		case 0: // Show services
			m.Action.OperationType = "show"
			m.SourceMenu = ViewStateServiceMenu
			m.buildFilterView()
			m.ViewState = ViewStateFilter
			return m, nil
		case 1: // Validate services
			m.Action.OperationType = "validate"
			m.SourceMenu = ViewStateServiceMenu
			m.Loading.Active = true
			m.Loading.Message = fmt.Sprintf("Validating service configuration (%s)...", m.Environment)
			return m, tea.Batch(tickSpinner(), m.validateServicesAsync())
		case 2: // Deploy services
			m.ViewState = ViewStateTreeService
			m.TreeSource = ViewStateServiceMenu
			m.ViewMode = ViewModeApp
			m.Action.OperationType = "deploy"
			m.Tree.CursorIndex = 0
			return m, nil
		case 3: // Stop services
			m.Action.OperationType = "stop"
			m.Tree.CursorIndex = 0
			m.Loading.Active = true
			m.Loading.Message = "Fetching service status..."
			return m, tea.Batch(tickSpinner(), m.fetchServiceStatusAsync())
		case 4: // Restart services
			m.Action.OperationType = "restart"
			m.Tree.CursorIndex = 0
			m.Loading.Active = true
			m.Loading.Message = "Fetching service status..."
			return m, tea.Batch(tickSpinner(), m.fetchRestartServiceStatusAsync())
		case 5: // Cleanup orphan resources
			m.Action.OperationType = "cleanup"
			m.Loading.Active = true
			m.Loading.Message = "Scanning orphan services..."
			return m, tea.Batch(tickSpinner(), m.scanOrphanServicesAsync())
		case 6: // Back to Main Menu
			m.ViewState = ViewStateMainMenu
			return m, nil
		}
	case ViewStateServerMenu:
		switch m.Server.ServiceMenuIndex {
		case 0: // Show servers
			m.Action.OperationType = "server_show"
			m.SourceMenu = ViewStateServerMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		case 1: // Validate servers
			m.Action.OperationType = "server_validate"
			m.SourceMenu = ViewStateServerMenu
			m.Loading.Active = true
			m.Loading.Message = "Validating servers..."
			return m, tea.Batch(tickSpinner(), m.validateServersAsync())
		case 2: // Setup server environment
			m.Action.OperationType = "server_setup"
			m.generateServerSetupPlan()
			m.ViewState = ViewStatePlan
			m.Action.ConfirmSelected = 0
			return m, nil
		case 3: // Back to Main Menu
			m.ViewState = ViewStateMainMenu
			return m, nil
		}
	case ViewStateDNSMenu:
		switch m.DNS.DNSMenuIndex {
		case 0: // Show DNS records
			m.Action.OperationType = "dns_show"
			m.SourceMenu = ViewStateDNSMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		case 1: // Validate DNS configuration
			m.Action.OperationType = "dns_validate"
			m.SourceMenu = ViewStateDNSMenu
			m.Loading.Active = true
			m.Loading.Message = "Validating DNS configuration..."
			return m, tea.Batch(tickSpinner(), m.validateDNSAsync())
		case 2: // Deploy DNS records
			m.ViewState = ViewStateTreeDNS
			m.TreeSource = ViewStateDNSMenu
			m.ViewMode = ViewModeDNS
			m.Action.OperationType = "dns_deploy"
			m.Tree.CursorIndex = 0
			return m, nil
		case 3: // Pull domains from ISP
			m.Action.OperationType = "dns_pull_domains"
			m.buildDNSISPFilterView()
			m.ViewState = ViewStateFilter
			return m, nil
		case 4: // Pull records from ISP
			m.Action.OperationType = "dns_pull_records"
			m.buildDNSDomainFilterView()
			m.ViewState = ViewStateFilter
			return m, nil
		case 5: // Back to Main Menu
			m.ViewState = ViewStateMainMenu
			return m, nil
		}
	case ViewStateConfigMenu:
		switch m.UI.ConfigMenuIndex {
		case 0: // Show ISPs
			m.Action.OperationType = "config_show_isps"
			m.SourceMenu = ViewStateConfigMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		case 1: // Show Registries
			m.Action.OperationType = "config_show_registries"
			m.SourceMenu = ViewStateConfigMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		case 2: // Show Secrets
			m.Action.OperationType = "config_show_secrets"
			m.SourceMenu = ViewStateConfigMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		case 3: // Validate Config
			m.Action.OperationType = "config_validate"
			m.SourceMenu = ViewStateConfigMenu
			m.Loading.Active = true
			m.Loading.Message = "Validating configuration..."
			return m, tea.Batch(tickSpinner(), m.validateConfigAsync())
		case 4: // Back to Main Menu
			m.ViewState = ViewStateMainMenu
			return m, nil
		}

	case ViewStateTreeService, ViewStateTreeDNS:
		return m.handlePlan()
	case ViewStatePlan:
		if m.Action.PlanComponent != nil && !m.Action.PlanComponent.HasSelected() {
			m.UI.ErrorMessage = "No items selected. Press 'a' to select all or Space to toggle individual items."
			return m, nil
		}
		// DNS pull: save selected diffs directly (no Executor)
		if m.Action.OperationType == "dns_pull_domains" || m.Action.OperationType == "dns_pull_records" {
			m.saveDNSPullSelectedFromPlan()
			m.DNS.DNSPullDiffs = nil
			m.DNS.DNSRecordDiffs = nil
			m.DNS.DNSPullSelected = nil
			m.ViewState = ViewStateDNSMenu
			return m, nil
		}
		m.ViewState = ViewStateProgress
		m.Action.ApplyProgress = 0
		m.Action.ApplyComplete = false
		m.Action.ApplyResults = nil
		m.Action.ApplyInProgress = true
		m.initProgressView()
		m.Action.ProgressTracker = NewProgressTracker()
		return m, tea.Batch(tickApply(), m.executeApplyAsync())
	case ViewStateComplete:
		m.ViewState = ViewStateMainMenu
		return m, nil
	case ViewStateFilter:
		if m.Action.FilterView == nil || !m.Action.FilterView.HasSelected() {
			return m, nil
		}
		// Service Show: enter InfoList with type filter applied
		if m.Action.OperationType == "show" {
			m.SourceMenu = ViewStateMainMenu
			m.UI.InfoListIndex = 0
			m.ViewState = ViewStateInfoList
			return m, nil
		}
		// DNS pull uses filter for ISP/domain selection, not plan generation
		if m.Action.OperationType == "dns_pull_domains" {
			selectedISPs := m.Action.FilterView.GetSelectedLabels(0)
			if len(selectedISPs) == 0 {
				return m, nil
			}
			m.DNS.PendingISPs = selectedISPs
			m.DNS.PendingISPsTotal = len(selectedISPs)
			m.DNS.AggregatedDomainDiffs = nil
			ispName := selectedISPs[0]
			m.Loading.Active = true
			m.Loading.Message = fmt.Sprintf("Fetching domains from %s... (1/%d)", ispName, len(selectedISPs))
			return m, tea.Batch(tickSpinner(), m.fetchDomainDiffsAsync(ispName))
		}
		if m.Action.OperationType == "dns_pull_records" {
			selectedDomains := m.Action.FilterView.GetSelectedLabels(0)
			if len(selectedDomains) == 0 {
				return m, nil
			}
			m.DNS.PendingDomains = selectedDomains
			m.DNS.PendingDomainsTotal = len(selectedDomains)
			m.DNS.AggregatedRecordDiffs = nil
			domainName := selectedDomains[0]
			m.Loading.Active = true
			m.Loading.Message = fmt.Sprintf("Fetching records for %s... (1/%d)", domainName, len(selectedDomains))
			return m, tea.Batch(tickSpinner(), m.fetchRecordDiffsAsync(domainName))
		}
		m.generatePlanFromFilter()
		m.initPlanComponent()
		m.ViewState = ViewStatePlan
		m.Action.ConfirmSelected = 0
		return m, nil
	}
	return m, nil
}

func (m Model) handleTab() Model {
	switch m.ViewState {
	case ViewStateTreeService, ViewStateTreeDNS:
		if m.ViewMode == ViewModeApp {
			m.ViewMode = ViewModeDNS
		} else {
			m.ViewMode = ViewModeApp
		}
		m.Tree.CursorIndex = 0
	}
	return m
}

func (m Model) handleSelectAll(selected bool) Model {
	if m.ViewState == ViewStatePlan {
		if m.Action.PlanComponent != nil {
			m.Action.PlanComponent.SelectAll(selected)
		}
		return m
	}
	if m.ViewState == ViewStateFilter {
		if m.Action.FilterView != nil {
			m.Action.FilterView.SelectAll(selected)
			m.updateFilterMatchedLine()
		}
		return m
	}
	if m.ViewState != ViewStateTreeService && m.ViewState != ViewStateTreeDNS {
		return m
	}
	nodes := m.getCurrentTree()
	for _, node := range nodes {
		node.SelectRecursive(selected)
	}
	return m
}

func (m Model) handlePlan() (tea.Model, tea.Cmd) {
	if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS {
		m.Action.Forced = false
		m.Loading.Active = true
		m.Loading.Message = "Generating plan..."
		return m, tea.Batch(tickSpinner(), m.generatePlanAsync())
	}
	if m.ViewState == ViewStateFilter {
		if m.Action.FilterView == nil || !m.Action.FilterView.HasSelected() {
			return m, nil
		}
		m.generatePlanFromFilter()
		m.initPlanComponent()
		m.ViewState = ViewStatePlan
		m.Action.ConfirmSelected = 0
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

// handleSearch 激活搜索模式
func (m Model) handleSearch() Model {
	if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS ||
		m.ViewState == ViewStateInfoList || m.ViewState == ViewStateInfoDetail {
		m.Search.Active = true
		m.Search.Query = ""
		m.Search.SearchFilter.Activate()
	}
	return m
}

// handleSearchInput 处理搜索模式下的输入
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.Search.Active {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		// 取消搜索，恢复原始内容
		m.Search.Active = false
		m.Search.Query = ""
		m.Search.SearchFilter.Deactivate()
		// 清除信息展示视图的搜索过滤
		m.UI.InfoListFilteredRows = nil
		m.UI.InfoDetailFilteredEntities = nil
		// 清除树视图的搜索过滤
		if m.Tree.OriginalNodes != nil {
			m.clearSearchFilter()
		}
		return m, nil
	case "enter":
		// 执行搜索过滤
		if m.ViewState == ViewStateTreeService || m.ViewState == ViewStateTreeDNS {
			m.applySearchFilter()
		} else if m.ViewState == ViewStateInfoList || m.ViewState == ViewStateInfoDetail {
			m.applyInfoSearchFilter()
		}
		m.Search.Active = false
		m.Search.SearchFilter.Deactivate()
		return m, nil
	case "backspace":
		// 删除最后一个字符
		m.Search.SearchFilter.Backspace()
		m.Search.Query = m.Search.SearchFilter.GetQuery()
		return m, nil
	default:
		// 处理字符输入
		if len(msg.String()) == 1 {
			m.Search.SearchFilter.AppendChar(msg.String())
			m.Search.Query = m.Search.SearchFilter.GetQuery()
		}
		return m, nil
	}
}

// applySearchFilter 应用搜索过滤
func (m *Model) applySearchFilter() {
	query := m.Search.Query
	if query == "" {
		// 清除过滤，恢复原始树
		if m.Tree.OriginalNodes != nil {
			m.restoreFilteredTree()
		}
		m.Tree.CursorIndex = 0
		return
	}

	// 保存原始树引用（如果尚未保存）
	if m.Tree.OriginalNodes == nil {
		if m.ViewMode == ViewModeApp {
			m.Tree.OriginalNodes = m.Tree.TreeNodes
		} else {
			m.Tree.OriginalNodes = m.Tree.DNSTreeNodes
		}
	}

	// 在深拷贝上执行过滤，原始节点永远不会被修改
	filtered := m.filterTreeNodes(m.Tree.OriginalNodes, query)
	m.Tree.FilteredNodes = filtered

	if m.ViewMode == ViewModeApp {
		m.Tree.TreeNodes = filtered
	} else {
		m.Tree.DNSTreeNodes = filtered
	}
	m.Tree.CursorIndex = 0
}

// filterTreeNodes 过滤树节点，保持树形结构
func (m *Model) filterTreeNodes(nodes []*TreeNode, query string) []*TreeNode {
	var result []*TreeNode
	for _, node := range nodes {
		filtered := m.filterTreeNode(node, query, nil)
		if filtered != nil {
			result = append(result, filtered)
		}
	}
	return result
}

// filterTreeNode 递归过滤单个树节点，返回深拷贝的新节点，原始节点永远不会被修改
func (m *Model) filterTreeNode(node *TreeNode, query string, parent *TreeNode) *TreeNode {
	nodeMatches := m.matchesQuery(node.Name, query) || m.matchesQuery(node.Info, query)

	var filteredChildren []*TreeNode
	for _, child := range node.Children {
		filtered := m.filterTreeNode(child, query, nil)
		if filtered != nil {
			filteredChildren = append(filteredChildren, filtered)
		}
	}

	if nodeMatches || len(filteredChildren) > 0 {
		copied := copyTreeNodeShallow(node)
		copied.Children = filteredChildren
		copied.Parent = parent
		for _, child := range filteredChildren {
			child.Parent = copied
		}
		return copied
	}

	return nil
}

// copyTreeNodeShallow 创建节点的浅拷贝（不拷贝 Children/Parent），共享 Selected 等字段的引用
func copyTreeNodeShallow(node *TreeNode) *TreeNode {
	return &TreeNode{
		ID:       node.ID,
		Type:     node.Type,
		Name:     node.Name,
		Selected: node.Selected,
		Expanded: node.Expanded,
		Children: nil,
		Parent:   nil,
		Status:   node.Status,
		Info:     node.Info,
	}
}

// matchesQuery 检查字符串是否匹配搜索查询（不区分大小写）
func (m *Model) matchesQuery(s, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(query))
}

// clearSearchFilter 清除搜索过滤
func (m *Model) clearSearchFilter() {
	if m.Tree.OriginalNodes != nil {
		m.restoreFilteredTree()
	}
	m.Search.Active = false
	m.Search.Query = ""
	m.Search.SearchFilter.Deactivate()
	m.Tree.CursorIndex = 0
}

// restoreFilteredTree 恢复原始树并清理过滤状态
func (m *Model) restoreFilteredTree() {
	if m.Tree.FilteredNodes != nil {
		m.syncSelectionFromFiltered()
	}
	if m.ViewMode == ViewModeApp {
		m.Tree.TreeNodes = m.Tree.OriginalNodes
	} else {
		m.Tree.DNSTreeNodes = m.Tree.OriginalNodes
	}
	m.Tree.OriginalNodes = nil
	m.Tree.FilteredNodes = nil
}

// syncSelectionFromFiltered 将过滤视图中的 Selected 状态同步回原始树
func (m *Model) syncSelectionFromFiltered() {
	lookup := make(map[string]*TreeNode)
	m.buildSelectionLookup(m.Tree.OriginalNodes, lookup)
	m.applySelectionFromFiltered(m.Tree.FilteredNodes, lookup)
}

func (m *Model) buildSelectionLookup(nodes []*TreeNode, lookup map[string]*TreeNode) {
	for _, node := range nodes {
		lookup[node.ID] = node
		if len(node.Children) > 0 {
			m.buildSelectionLookup(node.Children, lookup)
		}
	}
}

func (m *Model) applySelectionFromFiltered(nodes []*TreeNode, lookup map[string]*TreeNode) {
	for _, node := range nodes {
		if orig, ok := lookup[node.ID]; ok {
			orig.Selected = node.Selected
		}
		if len(node.Children) > 0 {
			m.applySelectionFromFiltered(node.Children, lookup)
		}
	}
}

// executeMainMenuOperation dispatches the selected menu child operation.
func (m Model) executeMainMenuOperation(operation string) (tea.Model, tea.Cmd) {
	switch operation {
	case "show":
		m.Action.OperationType = "show"
		m.buildFilterView()
		m.ViewState = ViewStateFilter
		return m, nil
	case "validate":
		m.Action.OperationType = "validate"
		m.SourceMenu = ViewStateMainMenu
		m.Loading.Active = true
		m.Loading.Message = "Validating services..."
		return m, tea.Batch(tickSpinner(), m.validateServicesAsync())
	case "deploy":
		m.ViewState = ViewStateTreeService
		m.TreeSource = ViewStateMainMenu
		m.ViewMode = ViewModeApp
		m.Action.OperationType = "deploy"
		m.Tree.CursorIndex = 0
	case "stop":
		m.Action.OperationType = "stop"
		m.Tree.CursorIndex = 0
		m.Loading.Active = true
		m.Loading.Message = "Fetching service status..."
		return m, tea.Batch(tickSpinner(), m.fetchServiceStatusAsync())
	case "restart":
		m.Action.OperationType = "restart"
		m.Tree.CursorIndex = 0
		m.Loading.Active = true
		m.Loading.Message = "Fetching service status..."
		return m, tea.Batch(tickSpinner(), m.fetchRestartServiceStatusAsync())
	case "cleanup":
		m.Action.OperationType = "cleanup"
		m.Loading.Active = true
		m.Loading.Message = "Scanning orphan services..."
		return m, tea.Batch(tickSpinner(), m.scanOrphanServicesAsync())
	case "server_show":
		m.Action.OperationType = "server_show"
		m.SourceMenu = ViewStateMainMenu
		m.UI.InfoListIndex = 0
		m.ViewState = ViewStateInfoList
	case "server_validate":
		m.Action.OperationType = "server_validate"
		m.SourceMenu = ViewStateMainMenu
		m.Loading.Active = true
		m.Loading.Message = "Validating servers..."
		return m, tea.Batch(tickSpinner(), m.validateServersAsync())
	case "server_setup":
		m.Action.OperationType = "server_setup"
		m.generateServerSetupPlan()
		m.initPlanComponent()
		m.ViewState = ViewStatePlan
		m.Action.ConfirmSelected = 0
	case "dns_show":
		m.Action.OperationType = "dns_show"
		m.SourceMenu = ViewStateMainMenu
		m.UI.InfoListIndex = 0
		m.ViewState = ViewStateInfoList
	case "dns_validate":
		m.Action.OperationType = "dns_validate"
		m.SourceMenu = ViewStateMainMenu
		m.Loading.Active = true
		m.Loading.Message = "Validating DNS configuration..."
		return m, tea.Batch(tickSpinner(), m.validateDNSAsync())
	case "dns_deploy":
		m.ViewState = ViewStateTreeDNS
		m.TreeSource = ViewStateMainMenu
		m.ViewMode = ViewModeDNS
		m.Action.OperationType = "dns_deploy"
		m.Tree.CursorIndex = 0
	case "dns_pull_domains":
		m.Action.OperationType = "dns_pull_domains"
		m.buildDNSISPFilterView()
		m.ViewState = ViewStateFilter
	case "dns_pull_records":
		m.Action.OperationType = "dns_pull_records"
		m.buildDNSDomainFilterView()
		m.ViewState = ViewStateFilter
	case "config_show_isps":
		m.Action.OperationType = "config_show_isps"
		m.SourceMenu = ViewStateMainMenu
		m.UI.InfoListIndex = 0
		m.ViewState = ViewStateInfoList
	case "config_show_registries":
		m.Action.OperationType = "config_show_registries"
		m.SourceMenu = ViewStateMainMenu
		m.UI.InfoListIndex = 0
		m.ViewState = ViewStateInfoList
	case "config_show_secrets":
		m.Action.OperationType = "config_show_secrets"
		m.SourceMenu = ViewStateMainMenu
		m.UI.InfoListIndex = 0
		m.ViewState = ViewStateInfoList
	case "config_validate":
		m.Action.OperationType = "config_validate"
		m.SourceMenu = ViewStateMainMenu
		m.Loading.Active = true
		m.Loading.Message = "Validating configuration..."
		return m, tea.Batch(tickSpinner(), m.validateConfigAsync())
	}
	return m, nil
}

// saveDNSPullSelectedFromPlan saves only the items selected in the PlanView for DNS pull operations.
// It maps PlanView selected items back to the original diffs by index, then delegates to saveSelectedDiffs.
func (m *Model) saveDNSPullSelectedFromPlan() {
	if m.Action.PlanComponent == nil {
		return
	}
	selectedItems := m.Action.PlanComponent.GetSelectedItems()
	if len(selectedItems) == 0 {
		return
	}

	// Build a set of selected indices by matching PlanItem back to the original diffs
	selectedSet := make(map[int]bool)
	if len(m.DNS.DNSPullDiffs) > 0 {
		for _, item := range selectedItems {
			for i, diff := range m.DNS.DNSPullDiffs {
				if item.Name == diff.Name {
					selectedSet[i] = true
					break
				}
			}
		}
	} else if len(m.DNS.DNSRecordDiffs) > 0 {
		for _, item := range selectedItems {
			planLabel := item.Name // e.g. "A @" or "CNAME api"
			for i, diff := range m.DNS.DNSRecordDiffs {
				diffLabel := fmt.Sprintf("%s %s", diff.Type, diff.Name)
				if planLabel == diffLabel {
					selectedSet[i] = true
					break
				}
			}
		}
	}

	// Temporarily set DNSPullSelected for saveSelectedDiffs to use
	m.DNS.DNSPullSelected = selectedSet
	m.saveSelectedDiffs()
}

// applyInfoSearchFilter applies search filtering for InfoList and InfoDetail views.
func (m *Model) applyInfoSearchFilter() {
	query := m.Search.Query
	if query == "" {
		m.UI.InfoListFilteredRows = nil
		m.UI.InfoDetailFilteredEntities = nil
		return
	}
	if m.ViewState == ViewStateInfoList {
		m.applyInfoListSearch(query)
	} else if m.ViewState == ViewStateInfoDetail {
		m.applyInfoDetailSearch(query)
	}
	m.UI.InfoListIndex = 0
	m.UI.InfoDetailCursor = 0
}

// applyInfoListSearch filters InfoList rows by matching any cell content.
func (m *Model) applyInfoListSearch(query string) {
	rows := m.buildInfoListRows()
	lowerQuery := strings.ToLower(query)
	var filtered []components.InfoRow
	for _, row := range rows {
		for _, cell := range row.Cells {
			if strings.Contains(strings.ToLower(cell), lowerQuery) {
				filtered = append(filtered, row)
				break
			}
		}
	}
	m.UI.InfoListFilteredRows = filtered
}

// applyInfoDetailSearch filters InfoDetail entities by matching title, fields, and lines.
func (m *Model) applyInfoDetailSearch(query string) {
	entities := m.buildInfoDetailEntities()
	lowerQuery := strings.ToLower(query)
	var filtered []InfoEntityFiltered
	for _, ent := range entities {
		// Check title match
		titleMatch := strings.Contains(strings.ToLower(ent.Title), lowerQuery)
		// Check if any field matches
		fieldMatch := false
		for _, f := range ent.Fields {
			if strings.Contains(strings.ToLower(f.Label), lowerQuery) ||
				strings.Contains(strings.ToLower(f.Value), lowerQuery) {
				fieldMatch = true
				break
			}
		}
		// Check if any line matches
		lineMatch := false
		for _, l := range ent.Lines {
			if strings.Contains(strings.ToLower(l), lowerQuery) {
				lineMatch = true
				break
			}
		}
		if titleMatch || fieldMatch || lineMatch {
			filtered = append(filtered, InfoEntityFiltered{
				Title:  ent.Title,
				Fields: ent.Fields,
				Lines:  ent.Lines,
			})
		}
	}
	m.UI.InfoDetailFilteredEntities = filtered
}

// buildInfoListRows builds all InfoRow data for the current operation type.
func (m *Model) buildInfoListRows() []components.InfoRow {
	if m.Config == nil {
		return nil
	}
	var rows []components.InfoRow
	switch m.Action.OperationType {
	case "server_show":
		for _, srv := range m.Config.Servers {
			rows = append(rows, components.InfoRow{Cells: []string{srv.Zone, srv.Name}})
		}
	case "dns_show":
		for _, domain := range m.Config.Domains {
			rows = append(rows, components.InfoRow{Cells: []string{domain.Name, domain.DNSISP, fmt.Sprintf("%d records", len(domain.Records))}})
		}
	case "show":
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
	case "config_show_isps":
		for _, isp := range m.Config.ISPs {
			var services []string
			for _, svc := range isp.Services {
				services = append(services, string(svc))
			}
			rows = append(rows, components.InfoRow{Cells: []string{isp.Name, string(isp.Type), strings.Join(services, ", ")}})
		}
	case "config_show_registries":
		for _, reg := range m.Config.Registries {
			rows = append(rows, components.InfoRow{
				Cells: []string{reg.Name, reg.URL, extractRegistryNamespace(reg.URL)},
			})
		}
	case "config_show_secrets":
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
	return rows
}

// infoDetailEntity is an intermediate struct for building detail entities.
type infoDetailEntity struct {
	Title  string
	Fields []components.InfoField
	Lines  []string
}

// buildInfoDetailEntities builds all entity data for the current operation type.
func (m *Model) buildInfoDetailEntities() []infoDetailEntity {
	if m.Config == nil {
		return nil
	}
	switch m.Action.OperationType {
	case "show":
		return m.buildServiceDetailEntities()
	case "server_show":
		return m.buildServerDetailEntities()
	case "dns_show":
		return m.buildDNSDetailEntities()
	case "config_show_isps":
		return m.buildISPDetailEntities()
	case "config_show_registries":
		return m.buildRegistryDetailEntities()
	case "config_show_secrets":
		return m.buildSecretDetailEntities()
	}
	return nil
}

func (m *Model) buildServiceDetailEntities() []infoDetailEntity {
	showBiz, showInfra := m.getSelectedServiceTypes()
	var entities []infoDetailEntity
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
							if path == "" {
								path = "/"
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
			entities = append(entities, infoDetailEntity{
				Title:  fmt.Sprintf("SERVICE: %s", svc.Name),
				Fields: fields,
			})
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
			entities = append(entities, infoDetailEntity{
				Title:  fmt.Sprintf("INFRA: %s", infra.Name),
				Fields: fields,
			})
		}
	}
	return entities
}

func (m *Model) buildServerDetailEntities() []infoDetailEntity {
	var entities []infoDetailEntity
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
		entities = append(entities, infoDetailEntity{
			Title:  fmt.Sprintf("SERVER: %s", srv.Name),
			Fields: fields,
		})
	}
	return entities
}

func (m *Model) buildDNSDetailEntities() []infoDetailEntity {
	var entities []infoDetailEntity
	for _, domain := range m.Config.Domains {
		fields := []components.InfoField{
			{Label: "ISP", Value: domain.DNSISP},
		}
		var lines []string
		if len(domain.Records) > 0 {
			lines = append(lines, fmt.Sprintf("%-12s %s", "Records:", ""))
			lines = append(lines, fmt.Sprintf("  %-10s %-16s %-30s %s", "TYPE", "NAME", "VALUE", "TTL"))
			for _, rec := range domain.Records {
				lines = append(lines, fmt.Sprintf("  %-10s %-16s %-30s %d", string(rec.Type), rec.Name, rec.Value, rec.TTL))
			}
		}
		entities = append(entities, infoDetailEntity{
			Title:  fmt.Sprintf("DOMAIN: %s", domain.Name),
			Fields: fields,
			Lines:  lines,
		})
	}
	return entities
}

func (m *Model) buildISPDetailEntities() []infoDetailEntity {
	var entities []infoDetailEntity
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
		entities = append(entities, infoDetailEntity{
			Title:  fmt.Sprintf("ISP: %s", isp.Name),
			Fields: fields,
		})
	}
	return entities
}

func (m *Model) buildRegistryDetailEntities() []infoDetailEntity {
	var entities []infoDetailEntity
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
		entities = append(entities, infoDetailEntity{
			Title:  fmt.Sprintf("REGISTRY: %s", reg.Name),
			Fields: fields,
		})
	}
	return entities
}

func (m *Model) buildSecretDetailEntities() []infoDetailEntity {
	var entities []infoDetailEntity
	for _, s := range m.Config.Secrets {
		fields := []components.InfoField{
			{Label: "Source", Value: s.Source},
			{Label: "Description", Value: ""},
		}
		entities = append(entities, infoDetailEntity{
			Title:  "SECRET: " + s.Name,
			Fields: fields,
		})
	}
	return entities
}
