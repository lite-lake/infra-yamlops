package cli

import (
	"time"

	"github.com/charmbracelet/bubbletea"
)

type applyProgressMsg struct{}

func tickApply() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return applyProgressMsg{}
	})
}

func (m Model) handleUp() Model {
	switch m.ViewState {
	case ViewStateMainMenu:
		if m.MainMenuIndex > 0 {
			m.MainMenuIndex--
		}
	case ViewStateServerSetup:
		if m.ServerFocusPanel == 0 {
			if m.ServerIndex > 0 {
				m.ServerIndex--
			}
		} else {
			if m.ServerAction > 0 {
				m.ServerAction--
			}
		}
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
	case ViewStateMainMenu:
		if m.MainMenuIndex < 2 {
			m.MainMenuIndex++
		}
	case ViewStateServerSetup:
		if m.ServerFocusPanel == 0 {
			if m.ServerIndex < len(m.ServerList)-1 {
				m.ServerIndex++
			}
		} else {
			if m.ServerAction < 3 {
				m.ServerAction++
			}
		}
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
	case ViewStateMainMenu:
		switch m.MainMenuIndex {
		case 0:
			m.ViewState = ViewStateTree
			return m, nil
		case 1:
			m.ViewState = ViewStateServerSetup
			m.ServerIndex = 0
			m.ServerAction = 0
			m.ServerFocusPanel = 0
			return m, nil
		case 2:
			return m, tea.Quit
		}
	case ViewStateServerSetup:
		switch m.ServerAction {
		case 0:
			m.executeServerCheck()
		case 1:
			m.executeServerSync()
		case 2:
			m.executeServerFullSetup()
		case 3:
			m.ViewState = ViewStateMainMenu
		}
		return m, nil
	case ViewStateServerCheck:
		m.ViewState = ViewStateServerSetup
		return m, nil
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
	switch m.ViewState {
	case ViewStateServerSetup:
		if m.ServerFocusPanel == 0 {
			m.ServerFocusPanel = 1
		} else {
			m.ServerFocusPanel = 0
		}
	case ViewStateTree:
		if m.ViewMode == ViewModeApp {
			m.ViewMode = ViewModeDNS
		} else {
			m.ViewMode = ViewModeApp
		}
		m.CursorIndex = 0
	}
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
