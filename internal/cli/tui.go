package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
)

type Environment string

const (
	EnvProd    Environment = "prod"
	EnvStaging Environment = "staging"
	EnvDev     Environment = "dev"
)

type ViewState int

const (
	ViewStateTree ViewState = iota
	ViewStatePlan
	ViewStateApplyConfirm
	ViewStateApplyProgress
	ViewStateApplyComplete
)

type ViewMode int

const (
	ViewModeApp ViewMode = iota
	ViewModeDNS
)

type NodeType string

const (
	NodeTypeZone      NodeType = "zone"
	NodeTypeServer    NodeType = "server"
	NodeTypeInfra     NodeType = "infra"
	NodeTypeBiz       NodeType = "biz"
	NodeTypeDomain    NodeType = "domain"
	NodeTypeDNSRecord NodeType = "record"
)

type NodeStatus string

const (
	StatusRunning     NodeStatus = "running"
	StatusStopped     NodeStatus = "stopped"
	StatusNeedsUpdate NodeStatus = "needs_update"
	StatusError       NodeStatus = "error"
	StatusSynced      NodeStatus = "synced"
)

type TreeNode struct {
	ID       string
	Type     NodeType
	Name     string
	Selected bool
	Expanded bool
	Children []*TreeNode
	Parent   *TreeNode
	Status   NodeStatus
	Info     string
}

func (n *TreeNode) IsPartiallySelected() bool {
	if len(n.Children) == 0 {
		return false
	}
	hasSelected := false
	hasUnselected := false
	for _, child := range n.Children {
		if child.Selected || child.IsPartiallySelected() {
			hasSelected = true
		}
		if !child.Selected {
			hasUnselected = true
		}
	}
	return hasSelected && hasUnselected
}

func (n *TreeNode) SelectRecursive(selected bool) {
	n.Selected = selected
	for _, child := range n.Children {
		child.SelectRecursive(selected)
	}
}

func (n *TreeNode) UpdateParentSelection() {
	if n.Parent == nil {
		return
	}
	allSelected := true
	for _, child := range n.Parent.Children {
		if !child.Selected {
			allSelected = false
			break
		}
	}
	n.Parent.Selected = allSelected
	n.Parent.UpdateParentSelection()
}

func (n *TreeNode) CountSelected() int {
	count := 0
	if len(n.Children) == 0 {
		if n.Selected {
			return 1
		}
		return 0
	}
	for _, child := range n.Children {
		count += child.CountSelected()
	}
	return count
}

func (n *TreeNode) CountTotal() int {
	count := 0
	if len(n.Children) == 0 {
		return 1
	}
	for _, child := range n.Children {
		count += child.CountTotal()
	}
	return count
}

func (n *TreeNode) GetVisibleNodes() []*TreeNode {
	var nodes []*TreeNode
	nodes = append(nodes, n)
	if n.Expanded {
		for _, child := range n.Children {
			nodes = append(nodes, child.GetVisibleNodes()...)
		}
	}
	return nodes
}

func (n *TreeNode) GetSelectedLeaves() []*TreeNode {
	var leaves []*TreeNode
	if len(n.Children) == 0 {
		if n.Selected {
			leaves = append(leaves, n)
		}
		return leaves
	}
	for _, child := range n.Children {
		leaves = append(leaves, child.GetSelectedLeaves()...)
	}
	return leaves
}

var baseStyle = lipgloss.NewStyle().Padding(1, 2)

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7C3AED")).
	Padding(0, 1)

var envStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#10B981")).
	Bold(true)

var selectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED")).
	Bold(true)

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280"))

var changeCreateStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#10B981"))

var changeUpdateStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F59E0B"))

var changeDeleteStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#EF4444"))

var changeNoopStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280"))

var progressBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED"))

var successStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#10B981"))

var tabActiveStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7C3AED")).
	Underline(true)

var tabInactiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280"))

type Model struct {
	ViewState       ViewState
	ViewMode        ViewMode
	Environment     Environment
	ConfigDir       string
	Config          *entity.Config
	TreeNodes       []*TreeNode
	DNSTreeNodes    []*TreeNode
	CursorIndex     int
	Width           int
	Height          int
	ErrorMessage    string
	PlanResult      *valueobject.Plan
	ApplyProgress   int
	ApplyTotal      int
	ApplyComplete   bool
	ApplyResults    []*handler.Result
	ApplyInProgress bool
	ConfirmSelected int
	PlanScope       *valueobject.Scope
	ConfirmView     bool
}

func NewModel(env string, configDir string) Model {
	environment := EnvDev
	switch env {
	case "prod":
		environment = EnvProd
	case "staging":
		environment = EnvStaging
	case "dev":
		environment = EnvDev
	default:
		environment = Environment(env)
	}
	m := Model{
		ViewState:   ViewStateTree,
		ViewMode:    ViewModeApp,
		Environment: environment,
		ConfigDir:   configDir,
		PlanScope:   &valueobject.Scope{},
		Width:       80,
		Height:      24,
	}
	m.loadConfig()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case applyProgressMsg:
		if m.ViewState == ViewStateApplyProgress && !m.ApplyComplete {
			if m.ApplyInProgress {
				m.ApplyProgress++
				if m.ApplyProgress >= m.ApplyTotal {
					m.executeApply()
					m.ApplyInProgress = false
					m.ViewState = ViewStateApplyComplete
					return m, nil
				}
				return m, tickApply()
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.ViewState == ViewStateTree {
				return m, tea.Quit
			}
			m.ViewState = ViewStateTree
			m.ErrorMessage = ""
			return m, nil
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
			return m.handleSelectCurrent(true), nil
		case "n":
			return m.handleSelectCurrent(false), nil
		case "A":
			return m.handleSelectAll(true), nil
		case "N":
			return m.handleSelectAll(false), nil
		case "p":
			return m.handlePlan()
		case "r":
			return m.handleRefresh(), nil
		case "esc":
			if m.ViewState != ViewStateTree {
				m.ViewState = ViewStateTree
				m.ErrorMessage = ""
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleUp() Model {
	switch m.ViewState {
	case ViewStateTree:
		if m.CursorIndex > 0 {
			m.CursorIndex--
		}
	case ViewStateApplyConfirm:
		if m.ConfirmSelected > 0 {
			m.ConfirmSelected--
		}
	}
	return m
}

func (m Model) handleDown() Model {
	switch m.ViewState {
	case ViewStateTree:
		totalNodes := m.countVisibleNodes()
		if m.CursorIndex < totalNodes-1 {
			m.CursorIndex++
		}
	case ViewStateApplyConfirm:
		if m.ConfirmSelected < 1 {
			m.ConfirmSelected++
		}
	}
	return m
}

func (m Model) handleSpace() Model {
	if m.ViewState != ViewStateTree {
		return m
	}
	node := m.getNodeAtIndex(m.CursorIndex)
	if node == nil || len(node.Children) > 0 {
		return m
	}
	node.Selected = !node.Selected
	node.UpdateParentSelection()
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateTree:
		node := m.getNodeAtIndex(m.CursorIndex)
		if node == nil {
			return m, nil
		}
		node.Expanded = !node.Expanded
		return m, nil
	case ViewStateApplyConfirm:
		if m.ConfirmSelected == 0 {
			m.ViewState = ViewStateApplyProgress
			m.ApplyProgress = 0
			m.ApplyComplete = false
			m.ApplyResults = nil
			m.ApplyInProgress = true
			return m, tickApply()
		}
		m.ViewState = ViewStatePlan
		return m, nil
	case ViewStatePlan:
		m.ViewState = ViewStateApplyConfirm
		m.ConfirmSelected = 0
		return m, nil
	case ViewStateApplyComplete:
		m.ViewState = ViewStateTree
		return m, nil
	}
	return m, nil
}

func (m Model) handleTab() Model {
	if m.ViewState != ViewStateTree {
		return m
	}
	if m.ViewMode == ViewModeApp {
		m.ViewMode = ViewModeDNS
	} else {
		m.ViewMode = ViewModeApp
	}
	m.CursorIndex = 0
	return m
}

func (m Model) handleSelectCurrent(selected bool) Model {
	if m.ViewState != ViewStateTree {
		return m
	}
	node := m.getNodeAtIndex(m.CursorIndex)
	if node == nil {
		return m
	}
	node.SelectRecursive(selected)
	node.UpdateParentSelection()
	return m
}

func (m Model) handleSelectAll(selected bool) Model {
	if m.ViewState != ViewStateTree {
		return m
	}
	nodes := m.getCurrentTree()
	for _, node := range nodes {
		node.SelectRecursive(selected)
	}
	return m
}

func (m Model) handlePlan() (tea.Model, tea.Cmd) {
	if m.ViewState != ViewStateTree {
		return m, nil
	}
	m.generatePlan()
	if m.ErrorMessage == "" {
		m.ViewState = ViewStatePlan
	}
	return m, nil
}

func (m Model) handleRefresh() Model {
	m.Config = nil
	m.loadConfig()
	m.buildTrees()
	return m
}

func (m *Model) loadConfig() {
	if m.Config != nil {
		return
	}
	loader := persistence.NewConfigLoader(m.ConfigDir)
	cfg, err := loader.Load(nil, string(m.Environment))
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Failed to load config: %v", err)
		return
	}
	if err := loader.Validate(cfg); err != nil {
		m.ErrorMessage = fmt.Sprintf("Validation error: %v", err)
		return
	}
	m.Config = cfg
	m.buildTrees()
}

func (m *Model) buildTrees() {
	if m.Config == nil {
		return
	}
	m.TreeNodes = m.buildAppTree()
	m.DNSTreeNodes = m.buildDNSTree()
}

func (m *Model) buildAppTree() []*TreeNode {
	if m.Config == nil {
		return nil
	}
	zoneMap := make(map[string]*TreeNode)
	serverByZone := make(map[string][]*TreeNode)
	serviceByServer := make(map[string][]*TreeNode)
	for _, z := range m.Config.Zones {
		zoneNode := &TreeNode{
			ID:       fmt.Sprintf("zone:%s", z.Name),
			Type:     NodeTypeZone,
			Name:     z.Name,
			Info:     z.Description,
			Expanded: true,
		}
		zoneMap[z.Name] = zoneNode
	}
	for _, srv := range m.Config.Servers {
		serverNode := &TreeNode{
			ID:       fmt.Sprintf("server:%s", srv.Name),
			Type:     NodeTypeServer,
			Name:     srv.Name,
			Info:     srv.IP.Public,
			Expanded: true,
		}
		if zNode, ok := zoneMap[srv.Zone]; ok {
			serverNode.Parent = zNode
			zNode.Children = append(zNode.Children, serverNode)
		}
		serverByZone[srv.Zone] = append(serverByZone[srv.Zone], serverNode)
		serviceByServer[srv.Name] = []*TreeNode{}
	}
	for _, infra := range m.Config.InfraServices {
		infraNode := &TreeNode{
			ID:   fmt.Sprintf("infra:%s", infra.Name),
			Type: NodeTypeInfra,
			Name: infra.Name,
			Info: m.getServicePortsInfo(infra.Server),
		}
		for _, sn := range serverByZone {
			for _, s := range sn {
				if s.Name == infra.Server {
					infraNode.Parent = s
					s.Children = append(s.Children, infraNode)
				}
			}
		}
	}
	for _, svc := range m.Config.Services {
		svcNode := &TreeNode{
			ID:   fmt.Sprintf("biz:%s", svc.Name),
			Type: NodeTypeBiz,
			Name: svc.Name,
			Info: m.getBizServicePortsInfo(svc),
		}
		for _, z := range m.Config.Zones {
			for _, srv := range m.Config.Servers {
				if srv.Name == svc.Server && srv.Zone == z.Name {
					if zNode, ok := zoneMap[z.Name]; ok {
						for _, sNode := range zNode.Children {
							if sNode.Name == srv.Name {
								svcNode.Parent = sNode
								sNode.Children = append(sNode.Children, svcNode)
							}
						}
					}
				}
			}
		}
	}
	var roots []*TreeNode
	for _, z := range m.Config.Zones {
		if zNode, ok := zoneMap[z.Name]; ok {
			roots = append(roots, zNode)
		}
	}
	return roots
}

func (m *Model) getServicePortsInfo(serverName string) string {
	for _, srv := range m.Config.Servers {
		if srv.Name == serverName {
			return ""
		}
	}
	return ""
}

func (m *Model) getBizServicePortsInfo(svc entity.BizService) string {
	if len(svc.Ports) == 0 {
		return ""
	}
	var ports []string
	for _, p := range svc.Ports {
		ports = append(ports, fmt.Sprintf(":%d", p.Host))
	}
	return strings.Join(ports, ",")
}

func (m *Model) buildDNSTree() []*TreeNode {
	if m.Config == nil {
		return nil
	}
	domainMap := make(map[string]*TreeNode)
	for _, d := range m.Config.Domains {
		domainNode := &TreeNode{
			ID:       fmt.Sprintf("domain:%s", d.Name),
			Type:     NodeTypeDomain,
			Name:     d.Name,
			Info:     d.DNSISP,
			Expanded: true,
		}
		domainMap[d.Name] = domainNode
	}
	for _, r := range m.Config.DNSRecords {
		recordNode := &TreeNode{
			ID:   fmt.Sprintf("record:%s:%s:%s", r.Domain, r.Type, r.Name),
			Type: NodeTypeDNSRecord,
			Name: fmt.Sprintf("%-6s %s", r.Type, r.Name),
			Info: r.Value,
		}
		if dNode, ok := domainMap[r.Domain]; ok {
			recordNode.Parent = dNode
			dNode.Children = append(dNode.Children, recordNode)
		}
	}
	var roots []*TreeNode
	for _, d := range m.Config.Domains {
		if dNode, ok := domainMap[d.Name]; ok {
			roots = append(roots, dNode)
		}
	}
	return roots
}

func (m *Model) generatePlan() {
	m.PlanResult = valueobject.NewPlan()
	m.ErrorMessage = ""
	m.loadConfig()
	if m.ErrorMessage != "" {
		return
	}
	m.buildScopeFromSelection()
	planner := plan.NewPlanner(m.Config, string(m.Environment))
	executionPlan, err := planner.Plan(m.PlanScope)
	if err != nil {
		m.ErrorMessage = fmt.Sprintf("Failed to generate plan: %v", err)
		return
	}
	m.PlanResult = executionPlan
	m.ApplyTotal = len(executionPlan.Changes)
	if m.ApplyTotal == 0 {
		m.ApplyTotal = 1
	}
}

func (m *Model) buildScopeFromSelection() {
	m.PlanScope = &valueobject.Scope{}
	currentTree := m.getCurrentTree()
	for _, node := range currentTree {
		leaves := node.GetSelectedLeaves()
		for _, leaf := range leaves {
			switch leaf.Type {
			case NodeTypeInfra, NodeTypeBiz:
				if m.PlanScope.Service == "" {
					m.PlanScope.Service = leaf.Name
				}
			case NodeTypeDNSRecord:
				if m.PlanScope.Domain == "" {
					parts := strings.Split(leaf.ID, ":")
					if len(parts) >= 2 {
						m.PlanScope.Domain = parts[1]
					}
				}
			}
		}
	}
}

func (m *Model) executeApply() {
	if m.PlanResult == nil || !m.PlanResult.HasChanges() {
		m.ApplyComplete = true
		return
	}
	m.loadConfig()
	if m.Config == nil {
		m.ApplyComplete = true
		return
	}
	planner := plan.NewPlanner(m.Config, string(m.Environment))
	if err := planner.GenerateDeployments(); err != nil {
		m.ErrorMessage = fmt.Sprintf("Failed to generate deployments: %v", err)
		m.ApplyComplete = true
		return
	}
	executor := usecase.NewExecutor(m.PlanResult, string(m.Environment))
	executor.SetSecrets(m.Config.GetSecretsMap())
	executor.SetDomains(m.Config.GetDomainMap())
	executor.SetISPs(m.Config.GetISPMap())
	executor.SetWorkDir(m.ConfigDir)
	secrets := m.Config.GetSecretsMap()
	for _, srv := range m.Config.Servers {
		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			continue
		}
		executor.RegisterServer(srv.Name, srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	}
	m.ApplyResults = executor.Apply()
	m.ApplyComplete = true
}

func tickApply() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return applyProgressMsg{}
	})
}

type applyProgressMsg struct{}

func (m Model) getCurrentTree() []*TreeNode {
	if m.ViewMode == ViewModeDNS {
		return m.DNSTreeNodes
	}
	return m.TreeNodes
}

func (m Model) countVisibleNodes() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += len(node.GetVisibleNodes())
	}
	return count
}

func (m Model) getNodeAtIndex(index int) *TreeNode {
	count := 0
	for _, node := range m.getCurrentTree() {
		visible := node.GetVisibleNodes()
		if index < count+len(visible) {
			return visible[index-count]
		}
		count += len(visible)
	}
	return nil
}

func (m Model) View() string {
	var content strings.Builder
	content.WriteString(m.renderHeader())
	switch m.ViewState {
	case ViewStateTree:
		content.WriteString(m.renderTree())
	case ViewStatePlan:
		content.WriteString(m.renderPlan())
	case ViewStateApplyConfirm:
		content.WriteString(m.renderApplyConfirm())
	case ViewStateApplyProgress:
		content.WriteString(m.renderApplyProgress())
	case ViewStateApplyComplete:
		content.WriteString(m.renderApplyComplete())
	}
	content.WriteString(m.renderHelp())
	return baseStyle.Render(content.String())
}

func (m Model) renderHeader() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("YAMLOps"))
	header.WriteString(" ")
	header.WriteString(envStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(m.Environment)))))
	selected := m.countSelected()
	total := m.countTotal()
	header.WriteString(fmt.Sprintf("    选中: %d/%d", selected, total))
	header.WriteString("\n")
	return header.String()
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

func (m Model) renderTree() string {
	var content strings.Builder
	content.WriteString(m.renderTabs())
	content.WriteString("\n\n")
	if m.ErrorMessage != "" {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.ErrorMessage))
		content.WriteString("\n\n")
	}
	idx := 0
	for _, node := range m.getCurrentTree() {
		content.WriteString(m.renderNode(node, 0, &idx))
	}
	return content.String()
}

func (m Model) renderTabs() string {
	var tabs strings.Builder
	if m.ViewMode == ViewModeApp {
		tabs.WriteString(tabActiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(tabInactiveStyle.Render("DNS"))
	} else {
		tabs.WriteString(tabInactiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(tabActiveStyle.Render("DNS"))
	}
	return tabs.String()
}

func (m Model) renderNode(node *TreeNode, depth int, idx *int) string {
	var content strings.Builder
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
	case NodeTypeDNSRecord:
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, selectIcon, expandIcon, typePrefix, node.Name)
	if node.Info != "" {
		line = fmt.Sprintf("%-50s %s", line, node.Info)
	}
	if *idx == m.CursorIndex {
		line = selectedStyle.Render(line)
	}
	content.WriteString(line)
	content.WriteString("\n")
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				content.WriteString(m.renderNodeLastChild(child, depth+1, idx))
			} else {
				content.WriteString(m.renderNode(child, depth+1, idx))
			}
		}
	}
	return content.String()
}

func (m Model) renderNodeLastChild(node *TreeNode, depth int, idx *int) string {
	var content strings.Builder
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
	if node.Info != "" {
		line = fmt.Sprintf("%-50s %s", line, node.Info)
	}
	if *idx == m.CursorIndex {
		line = selectedStyle.Render(line)
	}
	content.WriteString(line)
	content.WriteString("\n")
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				content.WriteString(m.renderNodeLastChild(child, depth+1, idx))
			} else {
				content.WriteString(m.renderNode(child, depth+1, idx))
			}
		}
	}
	return content.String()
}

func (m Model) renderPlan() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行计划"))
	content.WriteString("\n\n")
	if m.ErrorMessage != "" {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.ErrorMessage))
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render("Press q to go back"))
		return content.String()
	}
	if m.PlanResult == nil || len(m.PlanResult.Changes) == 0 {
		content.WriteString("No changes detected.\n")
	} else {
		for _, ch := range m.PlanResult.Changes {
			style := changeNoopStyle
			prefix := "~"
			switch ch.Type {
			case valueobject.ChangeTypeCreate:
				style = changeCreateStyle
				prefix = "+"
			case valueobject.ChangeTypeUpdate:
				style = changeUpdateStyle
				prefix = "~"
			case valueobject.ChangeTypeDelete:
				style = changeDeleteStyle
				prefix = "-"
			}
			line := fmt.Sprintf("%s %s: %s", prefix, ch.Entity, ch.Name)
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}
	content.WriteString("\n")
	content.WriteString(changeCreateStyle.Render("Press Enter to apply changes"))
	content.WriteString("\n")
	return content.String()
}

func (m Model) renderApplyConfirm() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("确认执行"))
	content.WriteString("\n\n")
	content.WriteString("是否执行以下变更?\n\n")
	if m.PlanResult != nil {
		nonNoopCount := 0
		for _, ch := range m.PlanResult.Changes {
			if ch.Type != valueobject.ChangeTypeNoop {
				nonNoopCount++
			}
		}
		content.WriteString(fmt.Sprintf("变更项数: %d\n", nonNoopCount))
	}
	content.WriteString("\n")
	options := []string{"确认执行", "取消"}
	for i, opt := range options {
		if i == m.ConfirmSelected {
			content.WriteString(selectedStyle.Render("▸ " + opt))
		} else {
			content.WriteString("  " + opt)
		}
		content.WriteString("\n")
	}
	return content.String()
}

func (m Model) renderApplyProgress() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行中..."))
	content.WriteString("\n\n")
	progress := float64(m.ApplyProgress) / float64(m.ApplyTotal)
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	content.WriteString(progressBarStyle.Render(bar))
	content.WriteString(fmt.Sprintf(" %.0f%%\n", progress*100))
	return content.String()
}

func (m Model) renderApplyComplete() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行完成"))
	content.WriteString("\n\n")
	if m.ApplyResults != nil {
		successCount := 0
		failCount := 0
		for _, result := range m.ApplyResults {
			if result.Success {
				successCount++
				content.WriteString(changeCreateStyle.Render(fmt.Sprintf("✓ %s: %s", result.Change.Entity, result.Change.Name)))
			} else {
				failCount++
				content.WriteString(changeDeleteStyle.Render(fmt.Sprintf("✗ %s: %s - %v", result.Change.Entity, result.Change.Name, result.Error)))
			}
			content.WriteString("\n")
		}
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("成功: %d  失败: %d\n", successCount, failCount))
	}
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press Enter to return"))
	return content.String()
}

func (m Model) renderHelp() string {
	if m.ViewState == ViewStateTree {
		return helpStyle.Render("\n[Space] 选择  [Enter] 展开/折叠  [a] 全选当前  [n] 取消当前  [p] Plan\n[A] 全部选中  [N] 全部取消  [Tab] 切换 App/DNS  [r] 刷新  [q] 退出")
	}
	if m.ViewState == ViewStateApplyProgress {
		return ""
	}
	return helpStyle.Render("\n[q] 返回  [Esc] 返回主界面")
}

func Run(env string, configDir string) error {
	p := tea.NewProgram(NewModel(env, configDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
