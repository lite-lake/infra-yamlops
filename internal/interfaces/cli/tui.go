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
		case "q", "ctrl+c":
			return m.handleQuit()
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
		case "esc":
			return m.handleEscape()
		}
	}
	return m, nil
}

func (m Model) handleQuit() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateMainMenu:
		return m, tea.Quit
	case ViewStateTree:
		return m, tea.Quit
	case ViewStateServerSetup, ViewStateServerCheck:
		m.ViewState = ViewStateMainMenu
		m.ErrorMessage = ""
		return m, nil
	case ViewStateDNSManagement:
		m.ViewState = ViewStateMainMenu
		return m, nil
	case ViewStateDNSPullDomains, ViewStateDNSPullRecords:
		m.ViewState = ViewStateDNSManagement
		return m, nil
	case ViewStateDNSPullDiff:
		m.DNSPullDiffs = nil
		m.DNSRecordDiffs = nil
		m.DNSPullSelected = nil
		m.ViewState = ViewStateDNSManagement
		return m, nil
	default:
		m.ViewState = ViewStateMainMenu
		m.ErrorMessage = ""
		return m, nil
	}
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.ViewState {
	case ViewStateServerSetup, ViewStateServerCheck:
		m.ViewState = ViewStateMainMenu
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
	case ViewStateTree:
		m.ViewState = ViewStateMainMenu
		m.ErrorMessage = ""
	default:
		if m.ViewState != ViewStateTree {
			m.ViewState = ViewStateTree
			m.ErrorMessage = ""
		}
	}
	return m, nil
}

func Run(env string, configDir string) error {
	p := tea.NewProgram(NewModel(env, configDir), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runTUI() {
	if err := Run(Env, ConfigDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
