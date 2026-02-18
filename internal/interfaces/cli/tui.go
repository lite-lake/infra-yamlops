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
