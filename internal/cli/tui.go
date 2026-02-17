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
	"github.com/litelake/yamlops/internal/ssh"
)

type Environment string

const (
	EnvProd    Environment = "prod"
	EnvStaging Environment = "staging"
	EnvDev     Environment = "dev"
)

type ViewState int

const (
	ViewStateMain ViewState = iota
	ViewStatePlanSubMenu
	ViewStateEnvSubMenu
	ViewStatePlanResult
	ViewStateApplyConfirm
	ViewStateApplyProgress
	ViewStateManageEntities
	ViewStateViewStatus
	ViewStateSettings
)

type MenuItem struct {
	Label string
	View  ViewState
}

var mainMenuItems = []MenuItem{
	{Label: "Plan & Apply", View: ViewStatePlanSubMenu},
	{Label: "Environment", View: ViewStateEnvSubMenu},
	{Label: "Manage Entities", View: ViewStateManageEntities},
	{Label: "View Status", View: ViewStateViewStatus},
	{Label: "Settings", View: ViewStateSettings},
}

var planSubMenuItems = []MenuItem{
	{Label: "Infrastructure", View: ViewStatePlanResult},
	{Label: "Global Resources", View: ViewStatePlanResult},
}

var envSubMenuItems = []MenuItem{
	{Label: "Check", View: ViewStateViewStatus},
	{Label: "Sync", View: ViewStateApplyProgress},
}

var baseStyle = lipgloss.NewStyle().
	Padding(1, 2)

var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7C3AED")).
	Padding(0, 1).
	MarginBottom(1)

var menuItemStyle = lipgloss.NewStyle().
	Padding(0, 2)

var selectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED")).
	Bold(true).
	Padding(0, 2)

var envStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#10B981")).
	Bold(true)

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280")).
	MarginTop(1)

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

var progressBarBgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#374151"))

var confirmStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F59E0B")).
	Bold(true)

var successStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#10B981"))

type Model struct {
	ViewState       ViewState
	MenuIndex       int
	ParentIndex     int
	Environment     Environment
	PlanResult      *valueobject.Plan
	ApplyProgress   int
	ApplyTotal      int
	ApplyComplete   bool
	ApplyResults    []*handler.Result
	Width           int
	Height          int
	ErrorMessage    string
	ConfirmSelected int
	ConfigDir       string
	PlanScope       *valueobject.Scope
	Config          *entity.Config
	StatusInfo      *StatusInfo
	EnvSyncComplete bool
	EnvSyncResults  []string
	ApplyInProgress bool
}

type StatusInfo struct {
	Servers   []ServerStatus
	Services  []ServiceStatus
	LoadError string
}

type ServerStatus struct {
	Name       string
	Running    bool
	Containers []string
	Error      string
}

type ServiceStatus struct {
	Name    string
	Server  string
	Healthy bool
	Error   string
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
	return Model{
		ViewState:     ViewStateMain,
		MenuIndex:     0,
		Environment:   environment,
		PlanResult:    valueobject.NewPlan(),
		ApplyProgress: 0,
		ApplyTotal:    100,
		Width:         80,
		Height:        24,
		ConfigDir:     configDir,
		PlanScope:     &valueobject.Scope{},
	}
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
					return m, nil
				}
				return m, tickApply()
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.ViewState == ViewStateMain {
				return m, tea.Quit
			}
			m.ViewState = ViewStateMain
			m.MenuIndex = 0
			m.ErrorMessage = ""
			return m, nil
		case "up", "k":
			return m.handleUp(), nil
		case "down", "j":
			return m.handleDown(), nil
		case "enter", " ":
			return m.handleEnter()
		case "esc":
			if m.ViewState != ViewStateMain {
				m.ViewState = ViewStateMain
				m.MenuIndex = 0
				m.ErrorMessage = ""
			}
			return m, nil
		case "left", "h":
			if m.ViewState == ViewStateEnvSubMenu {
				m.Environment = m.prevEnvironment()
			}
			return m, nil
		case "right", "l":
			if m.ViewState == ViewStateEnvSubMenu {
				m.Environment = m.nextEnvironment()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleUp() Model {
	var maxItems int
	switch m.ViewState {
	case ViewStateMain:
		maxItems = len(mainMenuItems)
	case ViewStatePlanSubMenu:
		maxItems = len(planSubMenuItems)
	case ViewStateEnvSubMenu:
		maxItems = len(envSubMenuItems)
	case ViewStateApplyConfirm:
		maxItems = 2
	default:
		return m
	}
	if m.MenuIndex > 0 {
		m.MenuIndex--
	} else {
		m.MenuIndex = maxItems - 1
	}
	return m
}

func (m Model) handleDown() Model {
	var maxItems int
	switch m.ViewState {
	case ViewStateMain:
		maxItems = len(mainMenuItems)
	case ViewStatePlanSubMenu:
		maxItems = len(planSubMenuItems)
	case ViewStateEnvSubMenu:
		maxItems = len(envSubMenuItems)
	case ViewStateApplyConfirm:
		maxItems = 2
	default:
		return m
	}
	if m.MenuIndex < maxItems-1 {
		m.MenuIndex++
	} else {
		m.MenuIndex = 0
	}
	return m
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateMain:
		item := mainMenuItems[m.MenuIndex]
		m.ParentIndex = m.MenuIndex
		m.ViewState = item.View
		m.MenuIndex = 0
		if m.ViewState == ViewStatePlanResult {
			m.generatePlan()
		} else if m.ViewState == ViewStateViewStatus {
			m.checkEnvStatus()
		} else if m.ViewState == ViewStateManageEntities {
			m.loadConfig()
		}
		return m, nil
	case ViewStatePlanSubMenu:
		item := planSubMenuItems[m.MenuIndex]
		m.ViewState = item.View
		m.MenuIndex = 0
		m.generatePlan()
		return m, nil
	case ViewStateEnvSubMenu:
		item := envSubMenuItems[m.MenuIndex]
		m.ViewState = item.View
		m.MenuIndex = 0
		if m.ViewState == ViewStateViewStatus {
			m.checkEnvStatus()
		} else if m.ViewState == ViewStateApplyProgress {
			m.EnvSyncComplete = false
			m.EnvSyncResults = []string{}
			m.syncEnv()
		}
		return m, nil
	case ViewStateApplyConfirm:
		if m.MenuIndex == 0 {
			m.ViewState = ViewStateApplyProgress
			m.ApplyProgress = 0
			m.ApplyComplete = false
			m.ApplyResults = nil
			m.ApplyInProgress = true
			return m, tickApply()
		}
		m.ViewState = ViewStatePlanResult
		m.MenuIndex = 0
		return m, nil
	case ViewStatePlanResult:
		m.ViewState = ViewStateApplyConfirm
		m.MenuIndex = 0
		return m, nil
	case ViewStateViewStatus:
		m.checkEnvStatus()
		return m, nil
	}
	return m, nil
}

func (m Model) prevEnvironment() Environment {
	switch m.Environment {
	case EnvProd:
		return EnvDev
	case EnvStaging:
		return EnvProd
	case EnvDev:
		return EnvStaging
	}
	return EnvDev
}

func (m Model) nextEnvironment() Environment {
	switch m.Environment {
	case EnvProd:
		return EnvStaging
	case EnvStaging:
		return EnvDev
	case EnvDev:
		return EnvProd
	}
	return EnvDev
}

func (m *Model) generatePlan() {
	m.PlanResult = valueobject.NewPlan()
	m.ErrorMessage = ""

	m.loadConfig()
	if m.ErrorMessage != "" {
		return
	}

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
}

func (m *Model) checkEnvStatus() {
	m.loadConfig()
	if m.Config == nil {
		return
	}

	m.StatusInfo = &StatusInfo{}
	secrets := m.Config.GetSecretsMap()

	for _, srv := range m.Config.Servers {
		status := ServerStatus{Name: srv.Name}

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			status.Error = fmt.Sprintf("Cannot resolve password: %v", err)
			m.StatusInfo.Servers = append(m.StatusInfo.Servers, status)
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			status.Error = fmt.Sprintf("Connection failed: %v", err)
			m.StatusInfo.Servers = append(m.StatusInfo.Servers, status)
			continue
		}

		stdout, _, err := client.Run("sudo docker ps --format '{{.Names}}'")
		client.Close()

		if err != nil {
			status.Error = fmt.Sprintf("Check failed: %v", err)
		} else {
			status.Running = true
			containers := strings.TrimSpace(stdout)
			if containers != "" {
				status.Containers = strings.Split(containers, "\n")
			}
		}
		m.StatusInfo.Servers = append(m.StatusInfo.Servers, status)
	}
}

func (m *Model) syncEnv() {
	m.loadConfig()
	if m.Config == nil {
		return
	}

	m.EnvSyncResults = []string{}
	m.EnvSyncComplete = false
	secrets := m.Config.GetSecretsMap()

	for _, srv := range m.Config.Servers {
		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			m.EnvSyncResults = append(m.EnvSyncResults, fmt.Sprintf("[%s] Cannot resolve password: %v", srv.Name, err))
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			m.EnvSyncResults = append(m.EnvSyncResults, fmt.Sprintf("[%s] Connection failed: %v", srv.Name, err))
			continue
		}

		_, stderr, err := client.Run("sudo docker network create yamlops-" + string(m.Environment) + " 2>/dev/null || true")
		client.Close()

		if err != nil {
			m.EnvSyncResults = append(m.EnvSyncResults, fmt.Sprintf("[%s] Sync failed: %v\n%s", srv.Name, err, stderr))
		} else {
			m.EnvSyncResults = append(m.EnvSyncResults, fmt.Sprintf("[%s] Network yamlops-%s ready", srv.Name, m.Environment))
		}
	}
	m.EnvSyncComplete = true
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

func (m Model) View() string {
	var content strings.Builder
	content.WriteString(m.renderHeader())
	switch m.ViewState {
	case ViewStateMain:
		content.WriteString(m.renderMenu(mainMenuItems))
	case ViewStatePlanSubMenu:
		content.WriteString(m.renderMenu(planSubMenuItems))
	case ViewStateEnvSubMenu:
		content.WriteString(m.renderEnvSubMenu())
	case ViewStatePlanResult:
		content.WriteString(m.renderPlanResult())
	case ViewStateApplyConfirm:
		content.WriteString(m.renderApplyConfirm())
	case ViewStateApplyProgress:
		content.WriteString(m.renderApplyProgress())
	case ViewStateManageEntities:
		content.WriteString(m.renderManageEntities())
	case ViewStateViewStatus:
		content.WriteString(m.renderViewStatus())
	case ViewStateSettings:
		content.WriteString(m.renderSettings())
	}
	content.WriteString(m.renderHelp())
	return baseStyle.Render(content.String())
}

func (m Model) renderHeader() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("YAMLops"))
	header.WriteString(" ")
	header.WriteString(envStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(m.Environment)))))
	header.WriteString("\n")
	return header.String()
}

func (m Model) renderMenu(items []MenuItem) string {
	var menu strings.Builder
	for i, item := range items {
		if i == m.MenuIndex {
			menu.WriteString(selectedStyle.Render("▸ " + item.Label))
		} else {
			menu.WriteString(menuItemStyle.Render("  " + item.Label))
		}
		menu.WriteString("\n")
	}
	return menu.String()
}

func (m Model) renderEnvSubMenu() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Environment: " + string(m.Environment)))
	content.WriteString("\n\n")
	content.WriteString(m.renderMenu(envSubMenuItems))
	return content.String()
}

func (m Model) renderPlanResult() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Plan Result"))
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
	content.WriteString(confirmStyle.Render("Press Enter to apply changes"))
	content.WriteString("\n")
	return content.String()
}

func (m Model) renderApplyConfirm() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Confirm Apply"))
	content.WriteString("\n\n")
	content.WriteString("Apply the following changes?\n\n")
	if m.PlanResult != nil {
		nonNoopCount := 0
		for _, ch := range m.PlanResult.Changes {
			if ch.Type != valueobject.ChangeTypeNoop {
				nonNoopCount++
			}
		}
		content.WriteString(fmt.Sprintf("Changes to apply: %d\n", nonNoopCount))
	}
	content.WriteString("\n")
	options := []string{"Yes, Apply", "No, Cancel"}
	for i, opt := range options {
		if i == m.MenuIndex {
			content.WriteString(selectedStyle.Render("▸ " + opt))
		} else {
			content.WriteString(menuItemStyle.Render("  " + opt))
		}
		content.WriteString("\n")
	}
	return content.String()
}

func (m Model) renderApplyProgress() string {
	var content strings.Builder

	if len(m.EnvSyncResults) > 0 || m.EnvSyncComplete {
		content.WriteString(titleStyle.Render("Environment Sync"))
	} else {
		content.WriteString(titleStyle.Render("Applying Changes"))
	}
	content.WriteString("\n\n")

	if len(m.EnvSyncResults) > 0 || m.EnvSyncComplete {
		for _, result := range m.EnvSyncResults {
			if strings.Contains(result, "failed") || strings.Contains(result, "Error") {
				content.WriteString(changeDeleteStyle.Render("✗ "+result) + "\n")
			} else {
				content.WriteString(changeCreateStyle.Render("✓ "+result) + "\n")
			}
		}
		if m.EnvSyncComplete {
			content.WriteString("\n")
			content.WriteString(successStyle.Render("Sync complete!"))
			content.WriteString("\n")
		}
	} else {
		progress := float64(m.ApplyProgress) / float64(m.ApplyTotal)
		barWidth := 30
		filled := int(progress * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		content.WriteString(progressBarStyle.Render(bar))
		content.WriteString(fmt.Sprintf(" %.0f%%\n", progress*100))
		content.WriteString("\n")

		if m.ApplyComplete && m.ApplyResults != nil {
			for _, result := range m.ApplyResults {
				if result.Success {
					content.WriteString(changeCreateStyle.Render(fmt.Sprintf("✓ %s: %s", result.Change.Entity, result.Change.Name)))
				} else {
					content.WriteString(changeDeleteStyle.Render(fmt.Sprintf("✗ %s: %s - %v", result.Change.Entity, result.Change.Name, result.Error)))
				}
				content.WriteString("\n")
			}
			content.WriteString("\n")
			content.WriteString(successStyle.Render("Apply complete!"))
			content.WriteString("\n")
		}
	}
	return content.String()
}

func (m Model) renderManageEntities() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Manage Entities"))
	content.WriteString("\n\n")

	if m.Config == nil {
		m.loadConfig()
	}

	if m.ErrorMessage != "" && m.ViewState == ViewStateManageEntities {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.ErrorMessage))
		content.WriteString("\n\n")
	} else if m.Config != nil {
		content.WriteString(fmt.Sprintf("ISPs: %d\n", len(m.Config.ISPs)))
		content.WriteString(fmt.Sprintf("Zones: %d\n", len(m.Config.Zones)))
		content.WriteString(fmt.Sprintf("Servers: %d\n", len(m.Config.Servers)))
		content.WriteString(fmt.Sprintf("Services: %d\n", len(m.Config.Services)))
		content.WriteString(fmt.Sprintf("Gateways: %d\n", len(m.Config.Gateways)))
		content.WriteString(fmt.Sprintf("Domains: %d\n", len(m.Config.Domains)))
		content.WriteString(fmt.Sprintf("DNS Records: %d\n", len(m.Config.DNSRecords)))
		content.WriteString(fmt.Sprintf("Certificates: %d\n", len(m.Config.Certificates)))
		content.WriteString(fmt.Sprintf("Registries: %d\n", len(m.Config.Registries)))
	}
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press q to go back"))
	return content.String()
}

func (m Model) renderViewStatus() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Status Overview"))
	content.WriteString("\n\n")
	content.WriteString(envStyle.Render("Environment: "))
	content.WriteString(string(m.Environment) + "\n\n")

	if m.StatusInfo == nil {
		content.WriteString(helpStyle.Render("Press Enter to check status"))
		content.WriteString("\n")
	} else if m.StatusInfo.LoadError != "" {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.StatusInfo.LoadError))
		content.WriteString("\n")
	} else {
		content.WriteString("Servers:\n")
		for _, srv := range m.StatusInfo.Servers {
			if srv.Error != "" {
				content.WriteString(fmt.Sprintf("  • %s: %s\n", srv.Name, changeDeleteStyle.Render(srv.Error)))
			} else if srv.Running {
				content.WriteString(fmt.Sprintf("  • %s: %s\n", srv.Name, changeCreateStyle.Render("running")))
				for _, c := range srv.Containers {
					content.WriteString(fmt.Sprintf("      - %s\n", c))
				}
			} else {
				content.WriteString(fmt.Sprintf("  • %s: %s\n", srv.Name, changeNoopStyle.Render("unknown")))
			}
		}
	}
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press q to go back"))
	return content.String()
}

func (m Model) renderSettings() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Settings"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Config Dir: %s\n", m.ConfigDir))
	content.WriteString(fmt.Sprintf("Environment: %s\n", m.Environment))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press q to go back"))
	return content.String()
}

func (m Model) renderHelp() string {
	return helpStyle.Render("\n↑/↓ Navigate | Enter Select | q Quit")
}

func Run(env string, configDir string) error {
	p := tea.NewProgram(NewModel(env, configDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
