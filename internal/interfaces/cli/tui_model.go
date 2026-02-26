package cli

import (
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/application/handler"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	serverpkg "github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
)

type LoadingState struct {
	Active      bool
	Message     string
	Spinner     int
	OperationID string
}

type spinnerTickMsg struct {
	time.Time
}

type configLoadedMsg struct {
	config *entity.Config
	err    error
}

type planGeneratedMsg struct {
	plan *valueobject.Plan
	err  error
}

type applyCompleteMsg struct {
	results []*handler.Result
	err     error
}

type serviceStatusFetchedMsg struct {
	statusMap map[string]NodeStatus
	err       error
}

type restartStatusFetchedMsg struct {
	statusMap map[string]NodeStatus
	err       error
}

type dnsDomainsFetchedMsg struct {
	diffs []DomainDiff
	err   error
}

type dnsRecordsFetchedMsg struct {
	diffs []RecordDiff
	err   error
}

type orphanServicesScannedMsg struct {
	results []CleanupResult
	err     error
}

type serverCheckCompleteMsg struct {
	results []serverpkg.CheckResult
	err     error
}

type serverSyncCompleteMsg struct {
	results []serverpkg.SyncResult
	err     error
}

type serverEnvCheckAllMsg struct {
	results     map[string][]serverpkg.CheckResult
	syncResults map[string][]serverpkg.SyncResult
	err         error
}

type serverEnvSyncAllMsg struct {
	results map[string][]serverpkg.SyncResult
	err     error
}

type serviceCleanupCompleteMsg struct {
	results []CleanupResult
	err     error
}

type serviceStopCompleteMsg struct {
	results []StopResult
	err     error
}

type serviceRestartCompleteMsg struct {
	results []RestartResult
	err     error
}

type dnsProviderCreatedMsg struct {
	provider dns.Provider
	ispName  string
	err      error
}

type applyCompleteAsyncMsg struct {
	results []*handler.Result
	err     error
}

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
	ViewStateServiceManagement
	ViewStateServerSetup
	ViewStateServerCheck
	ViewStateDNSManagement
	ViewStateDNSPullDomains
	ViewStateDNSPullRecords
	ViewStateDNSPullDiff
	ViewStateServiceCleanup
	ViewStateServiceCleanupConfirm
	ViewStateServiceCleanupProgress
	ViewStateServiceCleanupComplete
	ViewStateServiceStop
	ViewStateServiceStopConfirm
	ViewStateServiceStopProgress
	ViewStateServiceStopComplete
	ViewStateServiceRestart
	ViewStateServiceRestartConfirm
	ViewStateServiceRestartProgress
	ViewStateServiceRestartComplete
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

type OrphanItem struct {
	Type        string
	Name        string
	ServerIndex int
}

type CleanupResult struct {
	ServerName        string
	OrphanContainers  []string
	OrphanDirs        []string
	RemovedContainers []string
	RemovedDirs       []string
	FailedContainers  []string
	FailedDirs        []string
}

type StopResult struct {
	ServerName string
	Services   []StopServiceResult
}

type StopServiceResult struct {
	Name    string
	Success bool
	Error   string
}

type RestartResult struct {
	ServerName string
	Services   []RestartServiceResult
}

type RestartServiceResult struct {
	Name    string
	Success bool
	Error   string
}

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

type UIState struct {
	Width         int
	Height        int
	ScrollOffset  int
	ErrorMessage  string
	MainMenuIndex int
}

type TreeState struct {
	TreeNodes    []*TreeNode
	DNSTreeNodes []*TreeNode
	CursorIndex  int
}

type ServerState struct {
	ServerList         []*entity.Server
	ServerIndex        int
	ServerCheckResults []serverpkg.CheckResult
	ServerSyncResults  []serverpkg.SyncResult
	ServerAction       int
	ServerFocusPanel   int
	ServiceMenuIndex   int
}

type ServerEnvNode struct {
	Name     string
	Zone     string
	Selected bool
	Expanded bool
	Server   *entity.Server
}

type ServerEnvState struct {
	Nodes           []*ServerEnvNode
	CursorIndex     int
	OperationIndex  int
	Results         map[string][]serverpkg.CheckResult
	SyncResults     map[string][]serverpkg.SyncResult
	ResultsScrollY  int
	ProgressCurrent int
	ProgressTotal   int
}

func (s *ServerEnvState) CountSelected() int {
	count := 0
	for _, node := range s.Nodes {
		if node.Selected {
			count++
		}
	}
	return count
}

func (s *ServerEnvState) GetSelectedServers() []*entity.Server {
	var servers []*entity.Server
	for _, node := range s.Nodes {
		if node.Selected && node.Server != nil {
			servers = append(servers, node.Server)
		}
	}
	return servers
}

func (s *ServerEnvState) SelectAll(selected bool) {
	for _, node := range s.Nodes {
		node.Selected = selected
	}
}

type DNSState struct {
	DNSMenuIndex    int
	DNSISPIndex     int
	DNSDomainIndex  int
	DNSPullDiffs    []DomainDiff
	DNSRecordDiffs  []RecordDiff
	DNSPullSelected map[int]bool
	DNSPullCursor   int
}

type CleanupState struct {
	CleanupResults  []CleanupResult
	CleanupSelected map[int]bool
	CleanupCursor   int
}

type StopState struct {
	StopResults      []StopResult
	StopSelected     map[int]bool
	StopCursor       int
	ServiceStatusMap map[string]NodeStatus
}

type RestartState struct {
	RestartResults   []RestartResult
	RestartSelected  map[int]bool
	RestartCursor    int
	ServiceStatusMap map[string]NodeStatus
}

type ActionState struct {
	PlanResult      *valueobject.Plan
	ApplyProgress   int
	ApplyTotal      int
	ApplyComplete   bool
	ApplyResults    []*handler.Result
	ApplyInProgress bool
	ConfirmSelected int
	PlanScope       *valueobject.Scope
}

type Model struct {
	ViewState   ViewState
	ViewMode    ViewMode
	TreeSource  ViewState
	Environment Environment
	ConfigDir   string
	Config      *entity.Config

	UI        *UIState
	Tree      *TreeState
	Server    *ServerState
	ServerEnv *ServerEnvState
	DNS       *DNSState
	Cleanup   *CleanupState
	Stop      *StopState
	Restart   *RestartState
	Action    *ActionState
	Loading   *LoadingState
	ShowHelp  bool
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
		ViewState:   ViewStateMainMenu,
		ViewMode:    ViewModeApp,
		Environment: environment,
		ConfigDir:   configDir,
		UI: &UIState{
			Width:         80,
			Height:        24,
			MainMenuIndex: 0,
		},
		Tree:      &TreeState{},
		Server:    &ServerState{ServerFocusPanel: 0},
		ServerEnv: &ServerEnvState{},
		DNS:       &DNSState{},
		Cleanup:   &CleanupState{},
		Stop:      &StopState{},
		Restart:   &RestartState{},
		Action: &ActionState{
			PlanScope: &valueobject.Scope{},
		},
		Loading:  &LoadingState{},
		ShowHelp: false,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return m.loadConfigAsync()
}
