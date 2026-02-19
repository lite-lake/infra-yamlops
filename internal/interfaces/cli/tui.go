package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
)

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
			return m.handleRefresh(), nil
		}
	}
	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateTree:
		m.ViewState = ViewStateServiceManagement
		m.ErrorMessage = ""
	case ViewStateServiceManagement:
		m.ViewState = ViewStateMainMenu
	case ViewStateServerSetup, ViewStateServerCheck:
		m.ViewState = ViewStateServiceManagement
		m.ErrorMessage = ""
	case ViewStateDNSManagement:
		m.ViewState = ViewStateMainMenu
	case ViewStateDNSPullDomains, ViewStateDNSPullRecords:
		m.ViewState = ViewStateDNSManagement
	case ViewStateDNSPullDiff:
		m.DNSPullDiffs = nil
		m.DNSRecordDiffs = nil
		m.DNSPullSelected = nil
		m.ViewState = ViewStateDNSManagement
	case ViewStateServiceCleanup:
		m.CleanupResults = nil
		m.CleanupSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStateServiceCleanupConfirm:
		m.ViewState = ViewStateServiceCleanup
	case ViewStateServiceCleanupComplete:
		m.CleanupResults = nil
		m.CleanupSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStateServiceStop:
		m.ViewState = ViewStateServiceManagement
		m.StopSelected = nil
	case ViewStateServiceStopConfirm:
		m.ViewState = ViewStateServiceStop
	case ViewStateServiceStopComplete:
		m.StopResults = nil
		m.StopSelected = nil
		m.ViewState = ViewStateServiceManagement
	case ViewStatePlan:
		m.ViewState = ViewStateTree
	case ViewStateApplyConfirm:
		m.ViewState = ViewStatePlan
	default:
		m.ViewState = ViewStateMainMenu
		m.ErrorMessage = ""
	}
	return m, nil
}

func (m Model) handleCancel() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateApplyConfirm:
		m.ViewState = ViewStatePlan
	case ViewStateDNSPullDiff:
		m.DNSPullDiffs = nil
		m.DNSRecordDiffs = nil
		m.DNSPullSelected = nil
		m.ViewState = ViewStateDNSManagement
	default:
		m.ViewState = ViewStateMainMenu
		m.ErrorMessage = ""
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
