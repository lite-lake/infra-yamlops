package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
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
	plan             *valueobject.Plan
	err              error
	isDNSPullForce   bool
	forceDomainDiffs []DomainDiff
	forceRecordDiffs []RecordDiff
}

type applyCompleteMsg struct {
	results []*usecase.Result
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

type serviceCleanupCompleteMsg struct {
	results []CleanupResult
	err     error
}

type dockerPruneScannedMsg struct {
	results []DockerPruneScanResult
	err     error
}

type DockerPruneScanResult struct {
	ServerName string
	DiskUsage  *environment.DockerDiskUsage
	ScanError  string
}

type dnsProviderCreatedMsg struct {
	provider dns.Provider
	ispName  string
	err      error
}

type applyCompleteAsyncMsg struct {
	results []*usecase.Result
	err     error
}

// ValidateErrorItem represents a structured validation error with level, message, and suggestion.
type ValidateErrorItem struct {
	Level      string // "error", "warning"
	Message    string
	Suggestion string
}

type validateCompleteMsg struct {
	module   string // "service", "server", "dns", "config"
	passed   int
	failed   int
	warnings int
	errors   []ValidateErrorItem
	err      error
}

type Environment string

const (
	EnvProd    Environment = "prod"
	EnvStaging Environment = "staging"
	EnvDev     Environment = "dev"
)

// ViewState represents the current view state of the TUI.
// Consolidated enum: all change operations reuse ViewStatePlan/Progress/Complete.
type ViewState int

const (
	// Navigation
	ViewStateMainMenu    ViewState = iota // Main menu (4 modules)
	ViewStateServiceMenu                  // Service Management submenu
	ViewStateServerMenu                   // Server Management submenu
	ViewStateDNSMenu                      // DNS Management submenu
	ViewStateConfigMenu                   // Configuration submenu

	// Tree / Selection
	ViewStateTreeService // Tree View for service deploy scope selection
	ViewStateTreeDNS     // Tree View for DNS deploy scope selection

	// Filter (pre-Plan selection for stop/restart/cleanup)
	ViewStateFilter

	// Unified execution flow (reused by all change operations)
	ViewStatePlan     // Plan preview + checkbox confirm
	ViewStateProgress // Execution progress
	ViewStateComplete // Execution complete

	// Read-only views
	ViewStateInfoList   // Information list view (show)
	ViewStateInfoDetail // Information detail view (show --detail)
	ViewStateValidate   // Validation result view
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

// MenuNode represents a single node in the main menu tree.
type MenuNode struct {
	Label    string
	Expanded bool
	Children []MenuChild
}

// MenuChild represents a leaf item under a menu category.
type MenuChild struct {
	Label     string
	Operation string // e.g. "show", "validate", "deploy", etc.
}

// InfoEntityFiltered represents a filtered entity for search results.
type InfoEntityFiltered struct {
	Title  string
	Fields []components.InfoField
	Lines  []string
}

type ValidateState struct {
	Passed   int
	Failed   int
	Warnings int
	Errors   []ValidateErrorItem
	Module   string // "service" / "server" / "dns" / "config"
}

type UIState struct {
	Width            int
	Height           int
	ScrollOffset     int
	ErrorMessage     string
	MainMenuIndex    int
	ConfigMenuIndex  int
	InfoListIndex    int
	InfoDetailCursor int
	ValidateCursor   int
	Validate         *ValidateState
	MenuNodes        []MenuNode

	// Info search state
	InfoListFilteredRows       []components.InfoRow
	InfoDetailFilteredEntities []InfoEntityFiltered
}

type TreeState struct {
	TreeNodes     []*TreeNode
	DNSTreeNodes  []*TreeNode
	CursorIndex   int
	FilteredNodes []*TreeNode
	OriginalNodes []*TreeNode
}

type SearchState struct {
	Active       bool
	Query        string
	SearchFilter *components.SearchFilter
}

type ServerState struct {
	ServerList       []*entity.Server
	ServerIndex      int
	ServiceMenuIndex int
}

type ServerEnvNode struct {
	Name     string
	Zone     string
	Selected bool
	Expanded bool
	Server   *entity.Server
}

type ServerEnvState struct {
	Nodes []*ServerEnvNode
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

	// Batch fetch tracking for multi-domain/ISP support
	PendingDomains        []string     // remaining domains to fetch records for
	PendingDomainsTotal   int          // total number of domains being fetched
	AggregatedRecordDiffs []RecordDiff // accumulated record diffs across all domains

	PendingISPs           []string     // remaining ISPs to fetch domains from
	PendingISPsTotal      int          // total number of ISPs being fetched
	AggregatedDomainDiffs []DomainDiff // accumulated domain diffs across all ISPs
}

type CleanupState struct {
	CleanupResults  []CleanupResult
	CleanupSelected map[int]bool
	CleanupCursor   int
}

type StopState struct {
	StopSelected     map[int]bool
	StopCursor       int
	ServiceStatusMap map[string]NodeStatus
}

type RestartState struct {
	RestartSelected  map[int]bool
	RestartCursor    int
	ServiceStatusMap map[string]NodeStatus
}

type ActionState struct {
	PlanResult        *valueobject.Plan
	PlanComponent     *components.PlanView
	FilterView        *components.SelectionView
	ApplyProgress     int
	ApplyTotal        int
	ApplyComplete     bool
	ApplyResults      []*usecase.Result
	ApplyInProgress   bool
	ConfirmSelected   int
	PlanScope         *valueobject.Scope
	OperationType     string // "deploy", "stop", "restart", "cleanup", "server_setup", "dns_deploy", "dns_pull"
	ProgressView      *components.ProgressView
	ProgressStartTime time.Time
	ProgressEndTime   time.Time
	ProgressTracker   *ProgressTracker
	CancelFunc        context.CancelFunc
	Forced            bool // persistent forced state for plan regeneration
	Interrupted       bool // true after Ctrl+C during execution
}

type Model struct {
	ViewState   ViewState
	ViewMode    ViewMode
	TreeSource  ViewState
	SourceMenu  ViewState // 记录进入 InfoList/InfoDetail/Validate 的来源菜单
	Environment Environment
	ConfigDir   string
	Config      *entity.Config
	Concurrency int

	UI        *UIState
	Tree      *TreeState
	Search    *SearchState
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

func NewModel(env string, configDir string, concurrency int) Model {
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
	if concurrency <= 0 {
		concurrency = 5
	}
	m := Model{
		ViewState:   ViewStateMainMenu,
		ViewMode:    ViewModeApp,
		Environment: environment,
		ConfigDir:   configDir,
		Concurrency: concurrency,
		UI: &UIState{
			Width:         80,
			Height:        24,
			MainMenuIndex: 0,
			MenuNodes: []MenuNode{
				{
					Label:    "Service Management",
					Expanded: true,
					Children: []MenuChild{
						{Label: "Show services", Operation: "show"},
						{Label: "Validate services", Operation: "validate"},
						{Label: "Deploy services", Operation: "deploy"},
						{Label: "Stop services", Operation: "stop"},
						{Label: "Restart services", Operation: "restart"},
						{Label: "Cleanup orphan resources", Operation: "cleanup"},
					},
				},
				{
					Label:    "Server Management",
					Expanded: true,
					Children: []MenuChild{
						{Label: "Show servers", Operation: "server_show"},
						{Label: "Validate servers", Operation: "server_validate"},
						{Label: "Setup server environment", Operation: "server_setup"},
						{Label: "Docker prune", Operation: "docker_prune"},
					},
				},
				{
					Label:    "DNS Management",
					Expanded: true,
					Children: []MenuChild{
						{Label: "Show DNS records", Operation: "dns_show"},
						{Label: "Validate DNS configuration", Operation: "dns_validate"},
						{Label: "Deploy DNS records", Operation: "dns_deploy"},
						{Label: "Pull domains from ISP", Operation: "dns_pull_domains"},
						{Label: "Pull records from ISP", Operation: "dns_pull_records"},
					},
				},
				{
					Label:    "Configuration",
					Expanded: true,
					Children: []MenuChild{
						{Label: "Show ISPs", Operation: "config_show_isps"},
						{Label: "Show Registries", Operation: "config_show_registries"},
						{Label: "Show Secrets", Operation: "config_show_secrets"},
						{Label: "Validate Config", Operation: "config_validate"},
					},
				},
			},
		},
		Tree:      &TreeState{},
		Search:    &SearchState{SearchFilter: components.NewSearchFilter()},
		Server:    &ServerState{},
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

// buildDNSISPFilterView builds a SelectionView for ISP selection (DNS Pull Domains).
func (m *Model) buildDNSISPFilterView() {
	fv := components.NewSelectionView("Select ISP to pull domains from:")
	isps := m.getDNSISPs()
	var items []components.SelectionItem
	for _, isp := range isps {
		items = append(items, components.SelectionItem{
			Label: isp,
			Meta:  "(dns)",
		})
	}
	fv.AddGroup("ISP", items)
	fv.MatchedLine = "Only ISPs with DNS service are listed."
	m.Action.FilterView = fv
}

// buildDNSDomainFilterView builds a SelectionView for domain selection (DNS Pull Records).
func (m *Model) buildDNSDomainFilterView() {
	fv := components.NewSelectionView("Select domains to pull records from:")
	domains := m.getDNSDomainObjects()
	var items []components.SelectionItem
	for _, d := range domains {
		items = append(items, components.SelectionItem{
			Label: d.Name,
			Meta:  fmt.Sprintf("(ISP: %s)", d.DNSISP),
		})
	}
	fv.AddGroup("Domain", items)
	m.Action.FilterView = fv
}

func (m *Model) initPlanComponent() {
	title := fmt.Sprintf("PLAN: %s", m.Action.OperationType)
	opType := ""
	if m.Action.OperationType == "deploy" || m.Action.OperationType == "stop" || m.Action.OperationType == "restart" || m.Action.OperationType == "cleanup" {
		opType = m.getSelectedTypeLabel()
	}
	pv := components.NewPlanView(title, string(m.Environment), opType, m.Action.OperationType, m.Action.Forced)
	pv.EnvWarning = true

	if m.Action.PlanResult != nil && len(m.Action.PlanResult.Changes()) > 0 {
		var items []components.PlanItem
		for _, ch := range m.Action.PlanResult.Changes() {
			prefix := "~"
			switch ch.Type() {
			case valueobject.ChangeTypeCreate:
				prefix = "+"
			case valueobject.ChangeTypeDelete:
				prefix = "-"
			}

			details := ""
			name := ch.Name()
			server := ""
			action := ""

			if ch.Entity() == "service" || ch.Entity() == "infra_service" {
				switch m.Action.OperationType {
				case "stop":
					action = "stop"
					details = "status: running → stopped"
				case "restart":
					action = "restart"
					details = "status: running → restarted"
				case "cleanup":
					action = "cleanup"
				default:
					switch ch.Type() {
					case valueobject.ChangeTypeCreate:
						action = "create"
					case valueobject.ChangeTypeUpdate:
						action = "update"
					case valueobject.ChangeTypeDelete:
						action = "delete"
					}
					if oldSvc, ok := ch.OldState().(*entity.BizService); ok {
						if newSvc, ok := ch.NewState().(*entity.BizService); ok {
							if oldSvc.Image != newSvc.Image {
								details = fmt.Sprintf("image: %s → %s", oldSvc.Image, newSvc.Image)
							} else {
								details = fmt.Sprintf("image: %s", newSvc.Image)
							}
						} else {
							details = fmt.Sprintf("image: %s", oldSvc.Image)
						}
					} else if newSvc, ok := ch.NewState().(*entity.BizService); ok {
						details = fmt.Sprintf("image: %s", newSvc.Image)
					} else if oldSvc, ok := ch.OldState().(*entity.InfraService); ok {
						if newSvc, ok := ch.NewState().(*entity.InfraService); ok {
							if oldSvc.Image != newSvc.Image {
								details = fmt.Sprintf("image: %s → %s", oldSvc.Image, newSvc.Image)
							} else {
								details = fmt.Sprintf("image: %s", newSvc.Image)
							}
						} else {
							details = fmt.Sprintf("image: %s", oldSvc.Image)
						}
					} else if newSvc, ok := ch.NewState().(*entity.InfraService); ok {
						details = fmt.Sprintf("image: %s", newSvc.Image)
					}
				}
				if svc, ok := ch.OldState().(interface{ GetServer() string }); ok {
					server = svc.GetServer()
				} else if svc, ok := ch.NewState().(interface{ GetServer() string }); ok {
					server = svc.GetServer()
				} else if m, ok := ch.OldState().(map[string]interface{}); ok {
					if s, ok := m["server"].(string); ok {
						server = s
					}
				} else if m, ok := ch.NewState().(map[string]interface{}); ok {
					if s, ok := m["server"].(string); ok {
						server = s
					}
				}
			} else if ch.Entity() == "container" {
				action = "cleanup"
				details = fmt.Sprintf("container: %s", ch.Name())
				if m, ok := ch.OldState().(map[string]interface{}); ok {
					if s, ok := m["server"].(string); ok {
						server = s
					}
				}
			} else if ch.Entity() == "directory" {
				action = "cleanup"
				details = fmt.Sprintf("directory: %s", ch.Name())
				if m, ok := ch.OldState().(map[string]interface{}); ok {
					if s, ok := m["server"].(string); ok {
						server = s
					}
				}
			} else if ch.Entity() == "dns_record" {
				parts := strings.SplitN(ch.Name(), ":", 3)
				if len(parts) >= 3 {
					name = parts[0]
					server = fmt.Sprintf("%s %s", parts[1], parts[2])
				}
				for _, a := range ch.Actions() {
					details = a
					break
				}
				switch ch.Type() {
				case valueobject.ChangeTypeCreate:
					action = "create"
				case valueobject.ChangeTypeUpdate:
					action = "update"
				case valueobject.ChangeTypeDelete:
					action = "delete"
				}
			} else if ch.Entity() == "domain" {
				switch ch.Type() {
				case valueobject.ChangeTypeCreate:
					action = "create"
				case valueobject.ChangeTypeUpdate:
					action = "update"
				case valueobject.ChangeTypeDelete:
					action = "delete"
				}
			}

			detailLines := buildServiceDetailLines(ch)
			items = append(items, components.PlanItem{
				Action:      action,
				Name:        name,
				Server:      server,
				Details:     details,
				ChangeType:  prefix,
				DetailLines: detailLines,
			})
		}
		pv.SetItems(items)
	} else {
		pv.NoChanges = true
		pv.ForceHint = true
	}
	m.Action.PlanComponent = pv
}

// initDNSPullPlanComponent builds a PlanView from DNS pull diffs (domain or record).
func (m *Model) initDNSPullPlanComponent() {
	title := fmt.Sprintf("PLAN: %s", m.Action.OperationType)
	pv := components.NewPlanView(title, string(m.Environment), "", m.Action.OperationType, m.Action.Forced)
	pv.EnvWarning = false

	if m.Action.OperationType == "dns_pull_domains" && len(m.DNS.DNSPullDiffs) > 0 {
		if m.DNS.PendingISPsTotal > 1 {
			pv.SubHeader = fmt.Sprintf("ISP:  %d ISPs", m.DNS.PendingISPsTotal)
		} else {
			pv.SubHeader = fmt.Sprintf("ISP:  %s", m.DNS.DNSPullDiffs[0].DNSISP)
		}
	} else if m.Action.OperationType == "dns_pull_records" && len(m.DNS.DNSRecordDiffs) > 0 {
		if m.DNS.PendingDomainsTotal > 1 {
			pv.SubHeader = fmt.Sprintf("DOMAIN: %d domains", m.DNS.PendingDomainsTotal)
		} else {
			pv.SubHeader = fmt.Sprintf("DOMAIN: %s", m.DNS.DNSRecordDiffs[0].Domain)
		}
	}

	var items []components.PlanItem
	if len(m.DNS.DNSPullDiffs) > 0 {
		for _, diff := range m.DNS.DNSPullDiffs {
			items = append(items, components.PlanItem{
				Action:     "import",
				Name:       diff.Name,
				Server:     "",
				Details:    fmt.Sprintf("ISP: %s", diff.DNSISP),
				ChangeType: diff.Prefix,
			})
		}
	} else if len(m.DNS.DNSRecordDiffs) > 0 {
		for _, diff := range m.DNS.DNSRecordDiffs {
			detail := fmt.Sprintf("%s %s TTL:%d", diff.Type, diff.Value, diff.TTL)
			if diff.ChangeType == valueobject.ChangeTypeDelete {
				detail = "(remove)"
			}
			items = append(items, components.PlanItem{
				Action:     "import",
				Name:       fmt.Sprintf("%s %s", diff.Type, diff.Name),
				Server:     diff.Domain,
				Details:    detail,
				ChangeType: diff.Prefix,
			})
		}
	}

	if len(items) > 0 {
		pv.SetItems(items)
	} else {
		pv.NoChanges = true
		pv.ForceHint = true
	}
	m.Action.PlanComponent = pv
}

// buildServiceDetailLines extracts extra detail lines from a service change for detail view.
func buildServiceDetailLines(ch *valueobject.Change) []string {
	if ch.Entity() != "service" && ch.Entity() != "infra_service" {
		return nil
	}

	svc := extractServiceFromChange(ch)
	if svc == nil {
		return nil
	}

	var lines []string

	if s, ok := svc.(*entity.BizService); ok {
		if len(s.Ports) > 0 {
			portParts := make([]string, len(s.Ports))
			for i, p := range s.Ports {
				proto := "tcp"
				if p.Protocol != "" {
					proto = p.Protocol
				}
				portParts[i] = fmt.Sprintf("%d:%d/%s", p.Host, p.Container, proto)
			}
			lines = append(lines, fmt.Sprintf("ports: %s", strings.Join(portParts, ", ")))
		}
		if len(s.Gateways) > 0 {
			gwParts := make([]string, len(s.Gateways))
			for i, gw := range s.Gateways {
				scheme := ""
				if gw.HTTP && gw.HTTPS {
					scheme = "http+https"
				} else if gw.HTTPS {
					scheme = "https"
				} else if gw.HTTP {
					scheme = "http"
				}
				path := gw.Path
				if path == "" {
					path = "/"
				}
				if scheme != "" {
					gwParts[i] = fmt.Sprintf("%s%s (%s)", gw.Hostname, path, scheme)
				} else {
					gwParts[i] = fmt.Sprintf("%s%s", gw.Hostname, path)
				}
			}
			lines = append(lines, fmt.Sprintf("gateways: %s", strings.Join(gwParts, ", ")))
		}
		if len(s.Networks) > 0 {
			lines = append(lines, fmt.Sprintf("networks: %s", strings.Join(s.Networks, ", ")))
		}
		if s.Registry != "" {
			lines = append(lines, fmt.Sprintf("registry: %s", s.Registry))
		}
	} else if s, ok := svc.(*entity.InfraService); ok {
		if s.GatewayPorts != nil {
			var portParts []string
			if s.GatewayPorts.HTTP > 0 {
				portParts = append(portParts, fmt.Sprintf("http:%d", s.GatewayPorts.HTTP))
			}
			if s.GatewayPorts.HTTPS > 0 {
				portParts = append(portParts, fmt.Sprintf("https:%d", s.GatewayPorts.HTTPS))
			}
			if len(portParts) > 0 {
				lines = append(lines, fmt.Sprintf("ports: %s", strings.Join(portParts, ", ")))
			}
		}
		if len(s.Networks) > 0 {
			lines = append(lines, fmt.Sprintf("networks: %s", strings.Join(s.Networks, ", ")))
		}
	}

	return lines
}

func extractServiceFromChange(ch *valueobject.Change) interface{} {
	if svc, ok := ch.NewState().(*entity.BizService); ok {
		return svc
	}
	if svc, ok := ch.OldState().(*entity.BizService); ok {
		return svc
	}
	if svc, ok := ch.NewState().(*entity.InfraService); ok {
		return svc
	}
	if svc, ok := ch.OldState().(*entity.InfraService); ok {
		return svc
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return m.loadConfigAsync()
}
