package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	serverpkg "github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

// scanServerResourcesBatch returns one tea.Cmd per server so that each
// completion emits its own resourceOneScannedMsg, driving the N/M progress.
func (m *Model) scanServerResourcesBatch() tea.Cmd {
	if m.Config == nil {
		return func() tea.Msg {
			return resourcesAllDoneMsg{err: fmt.Errorf("config not loaded")}
		}
	}
	var cmds []tea.Cmd
	for _, srv := range m.Server.ServerList {
		cmds = append(cmds, m.scanOneServerResource(srv))
	}
	if len(cmds) == 0 {
		return func() tea.Msg { return resourcesAllDoneMsg{} }
	}
	return tea.Batch(cmds...)
}

// scanOneServerResource runs a single-server collection and returns its report.
func (m *Model) scanOneServerResource(srv *entity.Server) tea.Cmd {
	return func() tea.Msg {
		secrets := m.Config.GetSecretsMap()

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			return resourceOneScannedMsg{
				serverName: srv.Name,
				report:     failedResourceReport(srv, string(m.Environment), fmt.Sprintf("cannot resolve password: %v", err)),
			}
		}

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, &ssh.SSHConfig{StrictHostKeyChecking: strictHostKeyChecking, Timeout: constants.DefaultSSHTimeout})
		if err != nil {
			return resourceOneScannedMsg{
				serverName: srv.Name,
				report:     failedResourceReport(srv, string(m.Environment), fmt.Sprintf("connection failed: %v", err)),
			}
		}

		inspector := serverpkg.NewInspector(client, string(m.Environment))
		report, _ := inspector.Inspect(context.Background(), m.Config, srv, secrets)
		client.Close()
		if report == nil {
			report = failedResourceReport(srv, string(m.Environment), "inspector returned nil report")
		}
		return resourceOneScannedMsg{serverName: report.ServerName, report: report}
	}
}

func failedResourceReport(srv *entity.Server, env, scanErr string) *serverpkg.ServerResourceReport {
	return &serverpkg.ServerResourceReport{
		ServerName: srv.Name,
		Zone:       srv.Zone,
		Env:        env,
		Degraded:   true,
		ScanError:  scanErr,
		ScannedAt:  time.Now().UTC().Format(time.RFC3339),
		Segments:   map[string]serverpkg.SegmentState{},
	}
}

func failedResourceReportForName(name, scanErr string) *serverpkg.ServerResourceReport {
	return &serverpkg.ServerResourceReport{
		ServerName: name,
		Degraded:   true,
		ScanError:  scanErr,
		Segments:   map[string]serverpkg.SegmentState{},
	}
}
