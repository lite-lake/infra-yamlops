package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	PlanResult      *plan.Plan
	ApplyProgress   int
	ApplyTotal      int
	ApplyComplete   bool
	Width           int
	Height          int
	ErrorMessage    string
	ConfirmSelected int
}

func NewModel() Model {
	return Model{
		ViewState:     ViewStateMain,
		MenuIndex:     0,
		Environment:   EnvDev,
		PlanResult:    plan.NewPlan(),
		ApplyProgress: 0,
		ApplyTotal:    100,
		Width:         80,
		Height:        24,
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
			m.ApplyProgress++
			if m.ApplyProgress >= m.ApplyTotal {
				m.ApplyComplete = true
				return m, nil
			}
			return m, tickApply()
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
			m.generateSamplePlan()
		}
		return m, nil
	case ViewStatePlanSubMenu:
		item := planSubMenuItems[m.MenuIndex]
		m.ViewState = item.View
		m.MenuIndex = 0
		m.generateSamplePlan()
		return m, nil
	case ViewStateEnvSubMenu:
		item := envSubMenuItems[m.MenuIndex]
		m.ViewState = item.View
		m.MenuIndex = 0
		return m, nil
	case ViewStateApplyConfirm:
		if m.MenuIndex == 0 {
			m.ViewState = ViewStateApplyProgress
			m.ApplyProgress = 0
			m.ApplyComplete = false
			return m, tickApply()
		}
		m.ViewState = ViewStatePlanResult
		m.MenuIndex = 0
		return m, nil
	case ViewStatePlanResult:
		m.ViewState = ViewStateApplyConfirm
		m.MenuIndex = 0
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

func (m *Model) generateSamplePlan() {
	m.PlanResult = plan.NewPlan()
	m.PlanResult.AddChange(&plan.Change{
		Type:   plan.ChangeTypeCreate,
		Entity: "Server",
		Name:   "web-server-01",
	})
	m.PlanResult.AddChange(&plan.Change{
		Type:   plan.ChangeTypeUpdate,
		Entity: "Service",
		Name:   "api-service",
	})
	m.PlanResult.AddChange(&plan.Change{
		Type:   plan.ChangeTypeNoop,
		Entity: "Gateway",
		Name:   "main-gateway",
	})
	m.ApplyTotal = len(m.PlanResult.Changes)
	if m.ApplyTotal == 0 {
		m.ApplyTotal = 1
	}
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
	if m.PlanResult == nil || len(m.PlanResult.Changes) == 0 {
		content.WriteString("No changes detected.\n")
	} else {
		for _, ch := range m.PlanResult.Changes {
			style := changeNoopStyle
			prefix := "~"
			switch ch.Type {
			case plan.ChangeTypeCreate:
				style = changeCreateStyle
				prefix = "+"
			case plan.ChangeTypeUpdate:
				style = changeUpdateStyle
				prefix = "~"
			case plan.ChangeTypeDelete:
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
			if ch.Type != plan.ChangeTypeNoop {
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
	content.WriteString(titleStyle.Render("Applying Changes"))
	content.WriteString("\n\n")
	progress := float64(m.ApplyProgress) / float64(m.ApplyTotal)
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	content.WriteString(progressBarStyle.Render(bar))
	content.WriteString(fmt.Sprintf(" %.0f%%\n", progress*100))
	content.WriteString("\n")
	if m.ApplyComplete {
		content.WriteString(successStyle.Render("Apply complete!"))
		content.WriteString("\n")
	}
	return content.String()
}

func (m Model) renderManageEntities() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Manage Entities"))
	content.WriteString("\n\n")
	content.WriteString("Servers: 3\n")
	content.WriteString("Services: 5\n")
	content.WriteString("Gateways: 2\n")
	content.WriteString("Domains: 4\n")
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
	content.WriteString("Servers:\n")
	content.WriteString("  • web-server-01: running\n")
	content.WriteString("  • api-server-01: running\n")
	content.WriteString("  • db-server-01: running\n\n")
	content.WriteString("Services:\n")
	content.WriteString("  • api-service: healthy\n")
	content.WriteString("  • web-service: healthy\n")
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press q to go back"))
	return content.String()
}

func (m Model) renderSettings() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Settings"))
	content.WriteString("\n\n")
	content.WriteString("Config Path: ./config\n")
	content.WriteString("SSH Timeout: 30s\n")
	content.WriteString("Max Retries: 3\n")
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press q to go back"))
	return content.String()
}

func (m Model) renderHelp() string {
	return helpStyle.Render("\n↑/↓ Navigate | Enter Select | q Quit")
}

func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
