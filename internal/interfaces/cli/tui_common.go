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

func (m Model) startLoading(message string) tea.Cmd {
	m.Loading.Active = true
	m.Loading.Message = message
	m.Loading.Spinner = 0
	return tickSpinner()
}

func (m *Model) stopLoading() {
	m.Loading.Active = false
	m.Loading.Message = ""
}
