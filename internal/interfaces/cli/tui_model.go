package cli

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	serverpkg "github.com/litelake/yamlops/internal/server"
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
	ViewStateMainMenu
	ViewStateServerSetup
	ViewStateServerCheck
	ViewStateDNSManagement
	ViewStateDNSPullDomains
	ViewStateDNSPullRecords
	ViewStateDNSPullDiff
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
	ViewState          ViewState
	ViewMode           ViewMode
	Environment        Environment
	ConfigDir          string
	Config             *entity.Config
	TreeNodes          []*TreeNode
	DNSTreeNodes       []*TreeNode
	CursorIndex        int
	Width              int
	Height             int
	ErrorMessage       string
	PlanResult         *valueobject.Plan
	ApplyProgress      int
	ApplyTotal         int
	ApplyComplete      bool
	ApplyResults       []*handler.Result
	ApplyInProgress    bool
	ConfirmSelected    int
	PlanScope          *valueobject.Scope
	ConfirmView        bool
	MainMenuIndex      int
	ServerList         []*entity.Server
	ServerIndex        int
	ServerCheckResults []serverpkg.CheckResult
	ServerSyncResults  []serverpkg.SyncResult
	ServerAction       int
	ServerFocusPanel   int
	DNSMenuIndex       int
	DNSISPIndex        int
	DNSDomainIndex     int
	DNSPullDiffs       []DomainDiff
	DNSRecordDiffs     []RecordDiff
	DNSPullSelected    map[int]bool
	DNSPullCursor      int
	DNSPullDone        bool
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
		ViewState:        ViewStateMainMenu,
		ViewMode:         ViewModeApp,
		Environment:      environment,
		ConfigDir:        configDir,
		PlanScope:        &valueobject.Scope{},
		Width:            80,
		Height:           24,
		ServerFocusPanel: 0,
	}
	m.loadConfig()
	if m.Config != nil {
		for i := range m.Config.Servers {
			m.ServerList = append(m.ServerList, &m.Config.Servers[i])
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}
