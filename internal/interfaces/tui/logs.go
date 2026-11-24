package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// scanServiceLogsBatch returns one tea.Cmd per service; each completion emits
// its own serviceLogOneFetchedMsg, driving the "N/M done" loading progress.
func (m *Model) scanServiceLogsBatch() tea.Cmd {
	if m.Config == nil {
		return func() tea.Msg { return serviceLogsAllDoneMsg{err: fmt.Errorf("config not loaded")} }
	}
	env := string(m.Environment)
	secrets := m.Config.GetSecretsMap()

	type svcEntry struct {
		name   string
		server string
	}
	var services []svcEntry
	for _, svc := range m.Config.Services {
		if svc.Image == "" {
			continue
		}
		services = append(services, svcEntry{name: svc.Name, server: svc.Server})
	}
	for _, infra := range m.Config.InfraServices {
		services = append(services, svcEntry{name: infra.Name, server: infra.Server})
	}

	var cmds []tea.Cmd
	for _, s := range services {
		srv := m.Config.GetServerMap()[s.server]
		if srv == nil {
			name := s.name
			cmds = append(cmds, func() tea.Msg {
				return serviceLogOneFetchedMsg{view: &usecase.ServiceLogView{
					ServiceName: name, Err: fmt.Sprintf("server %q not found", s.server),
				}}
			})
			continue
		}
		entry := s
		server := srv
		cmds = append(cmds, func() tea.Msg {
			password, err := server.SSH.Password.Resolve(secrets)
			if err != nil {
				return serviceLogOneFetchedMsg{view: &usecase.ServiceLogView{
					ServiceName: entry.name, Err: fmt.Sprintf("cannot resolve password: %v", err),
				}}
			}
			strict := true
			if !server.SSH.StrictHostKeyChecking {
				strict = false
			}
			client, err := ssh.NewClientWithConfig(server.SSH.Host, server.SSH.Port, server.SSH.User, password, &ssh.SSHConfig{
				StrictHostKeyChecking: strict,
				Timeout:               constants.DefaultSSHTimeout,
			})
			if err != nil {
				return serviceLogOneFetchedMsg{view: &usecase.ServiceLogView{
					ServiceName: entry.name, ServerName: entry.server, Err: fmt.Sprintf("connection failed: %v", err),
				}}
			}
			defer client.Close()
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, env, entry.name))
			view := usecase.FetchServiceLogsForServer(client, env, entry.name, entry.server, remoteDir, usecase.LogOptions{Tail: constants.DefaultLogsTail})
			return serviceLogOneFetchedMsg{view: view}
		})
	}
	if len(cmds) == 0 {
		return func() tea.Msg { return serviceLogsAllDoneMsg{} }
	}
	return tea.Batch(cmds...)
}

// handleServiceLogOneFetchedMsg accumulates per-service log views.
func (m Model) handleServiceLogOneFetchedMsg(msg serviceLogOneFetchedMsg) (tea.Model, tea.Cmd) {
	if m.Logs == nil {
		return m, nil
	}
	if msg.view != nil {
		m.Logs.Views = append(m.Logs.Views, msg.view)
	}
	m.Logs.Done++
	total := m.Logs.Expected
	progress := fmt.Sprintf("Fetching service logs... %d/%d done", m.Logs.Done, total)
	if m.Logs.Done >= total {
		m.Loading.Message = progress
		return m, func() tea.Msg { return serviceLogsAllDoneMsg{} }
	}
	m.Loading.Message = progress
	return m, nil
}

// handleServiceLogsAllDoneMsg transitions to the InfoList view.
func (m Model) handleServiceLogsAllDoneMsg(msg serviceLogsAllDoneMsg) (tea.Model, tea.Cmd) {
	m.Loading.Active = false
	if msg.err != nil {
		m.UI.ErrorMessage = formatErrorMessage("Service logs fetch failed", msg.err)
		return m, nil
	}
	m.UI.InfoListIndex = 0
	m.ViewState = ViewStateInfoList
	return m, nil
}

// --- Rendering ---

func currentServiceLogView(m Model) *usecase.ServiceLogView {
	if m.Logs == nil || m.UI.InfoListIndex < 0 || m.UI.InfoListIndex >= len(m.Logs.Views) {
		return nil
	}
	return m.Logs.Views[m.UI.InfoListIndex]
}

// buildServiceLogInfoRows builds overview rows for the service logs list.
func buildServiceLogInfoRows(views []*usecase.ServiceLogView) []components.InfoRow {
	var rows []components.InfoRow
	for _, v := range views {
		status := "-"
		if v.Err != "" {
			status = "error"
		} else if v.Status != nil && v.Status.Exists {
			status = v.Status.State
			if v.Status.RestartCount > 0 {
				status = fmt.Sprintf("%s (%d restarts)", status, v.Status.RestartCount)
			}
		} else {
			status = "no container"
		}
		archives := "0"
		if len(v.Archives) > 0 {
			archives = fmt.Sprintf("%d", len(v.Archives))
		}
		name := v.ServiceName
		if v.Err != "" {
			name = "⚠ " + name
		}
		rows = append(rows, components.InfoRow{Cells: []string{name, v.ServerName, status, archives}})
	}
	return rows
}

// renderServiceLogsDetail renders the detail view of the selected service's logs.
func (m Model) renderServiceLogsDetail() string {
	v := currentServiceLogView(m)
	idv := components.NewInfoDetailView("Service Logs")
	if v == nil {
		idv.SetStatLine(styles.MutedStyle.Render("No log data"))
		availableHeight := m.UI.Height - styles.LayoutOverhead - 2
		return idv.Render(availableHeight)
	}

	fields := []components.InfoField{
		{Label: "Server", Value: v.ServerName},
		{Label: "Container", Value: v.Container},
	}
	if v.Status != nil && v.Status.Exists {
		fields = append(fields,
			components.InfoField{Label: "State", Value: v.Status.State},
			components.InfoField{Label: "Restarts", Value: fmt.Sprintf("%d", v.Status.RestartCount)},
			components.InfoField{Label: "Started", Value: v.Status.StartedAt},
		)
	} else {
		fields = append(fields, components.InfoField{Label: "State", Value: "no container"})
	}
	if len(v.Archives) > 0 {
		fields = append(fields, components.InfoField{
			Label: "Archives",
			Value: fmt.Sprintf("%d run(s), newest: %s", len(v.Archives), v.Archives[0]),
		})
	}
	if v.Err != "" {
		fields = append(fields, components.InfoField{Label: "Error", Value: v.Err})
	}
	idv.AddEntity(fmt.Sprintf("SERVICE: %s", v.ServiceName), fields)

	var logLines []string
	if v.Content == "" {
		logLines = append(logLines, "(no logs)")
	} else {
		logLines = strings.Split(v.Content, "\n")
	}
	idv.Entities = append(idv.Entities, components.InfoEntity{Title: "LOG TAIL", Lines: logLines})

	idv.SetStatLine("Logs are read via SSH from docker logs; archives are created before deploy/restart.")
	idv.Cursor = m.UI.InfoDetailCursor
	availableHeight := m.UI.Height - styles.LayoutOverhead - 2
	return idv.Render(availableHeight)
}

// buildServiceLogDetailEntities adapts the log view for the search path.
func (m Model) buildServiceLogDetailEntities() []infoDetailEntity {
	v := currentServiceLogView(m)
	if v == nil {
		return nil
	}
	fields := []components.InfoField{
		{Label: "Server", Value: v.ServerName},
		{Label: "Container", Value: v.Container},
	}
	if v.Status != nil && v.Status.Exists {
		fields = append(fields,
			components.InfoField{Label: "State", Value: v.Status.State},
			components.InfoField{Label: "Restarts", Value: fmt.Sprintf("%d", v.Status.RestartCount)},
		)
	}
	if v.Err != "" {
		fields = append(fields, components.InfoField{Label: "Error", Value: v.Err})
	}
	var lines []string
	if v.Content != "" {
		lines = strings.Split(v.Content, "\n")
	} else {
		lines = []string{"(no logs)"}
	}
	return []infoDetailEntity{
		{Title: fmt.Sprintf("SERVICE: %s", v.ServiceName), Fields: fields},
		{Title: "LOG TAIL", Lines: lines},
	}
}
