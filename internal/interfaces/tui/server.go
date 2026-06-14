package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	serverpkg "github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
)

// generateServerSetupPlan creates a PlanView for server setup.
// All servers are included with "sync" action. Users select/deselect in the Plan view.
// This follows the unified execution mode: Plan → Confirm → Execute.
func (m *Model) generateServerSetupPlan() {
	var items []components.PlanItem
	for _, srv := range m.Server.ServerList {
		items = append(items, components.PlanItem{
			Action:     "sync",
			Name:       srv.Name,
			Server:     srv.Name,
			Details:    fmt.Sprintf("packages: %s (installed)", strings.Join(getServerPackages(srv), ", ")),
			ChangeType: "~",
			Selected:   true,
		})
	}

	if len(items) == 0 {
		return
	}

	pv := components.NewPlanView("PLAN: server setup", string(m.Environment), "", "server_setup", false)
	pv.EnvWarning = true
	pv.SetItems(items)
	m.Action.PlanComponent = pv
	m.Action.PlanResult = valueobject.NewPlan()
	m.Action.ApplyTotal = len(items)
}

func getServerPackages(srv *entity.Server) []string {
	packages := []string{}
	if srv != nil {
		if srv.OS != "" {
			packages = append(packages, srv.OS)
		}
		if len(srv.Environment.Registries) > 0 {
			packages = append(packages, "registries: "+strings.Join(srv.Environment.Registries, ", "))
		}
	}
	if len(packages) == 0 {
		packages = append(packages, "base")
	}
	return packages
}

// executeServerEnvSyncAsync runs server environment sync for selected servers in the Plan view.
// Returns applyCompleteAsyncMsg with unified result format (compatible with ViewStateComplete).
func (m *Model) executeServerEnvSyncAsync() tea.Cmd {
	return func() tea.Msg {
		selectedItems := m.Action.PlanComponent.GetSelectedItems()
		if len(selectedItems) == 0 {
			return applyCompleteAsyncMsg{results: nil}
		}

		selectedSet := make(map[string]bool)
		for _, item := range selectedItems {
			selectedSet[item.Server] = true
		}

		var servers []*entity.Server
		for _, srv := range m.Server.ServerList {
			if selectedSet[srv.Name] {
				servers = append(servers, srv)
			}
		}

		if len(servers) == 0 {
			return applyCompleteAsyncMsg{results: nil}
		}

		concurrency := m.Concurrency
		if concurrency <= 0 {
			concurrency = 5
		}

		secrets := m.Config.GetSecretsMap()

		registries := make([]entity.Registry, 0, len(m.Config.Registries))
		for i := range m.Config.Registries {
			registries = append(registries, m.Config.Registries[i])
		}

		type serverResult struct {
			results []*usecase.Result
		}

		sem := make(chan struct{}, concurrency)
		resultsCh := make(chan serverResult, len(servers))
		var wg sync.WaitGroup

		for _, srv := range servers {
			sem <- struct{}{}
			wg.Add(1)
			go func(s *entity.Server) {
				defer wg.Done()
				defer func() { <-sem }()

				var serverResults []*usecase.Result

				password, err := s.SSH.Password.Resolve(secrets)
				if err != nil {
					serverResults = append(serverResults, &usecase.Result{
						Success: false,
						Error:   fmt.Errorf("cannot resolve password for %s: %w", s.Name, err),
						Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "server_env", s.Name).WithOldState(map[string]interface{}{"server": s.Name}),
					})
					resultsCh <- serverResult{results: serverResults}
					return
				}

				strictHostKeyChecking := true
				if !s.SSH.StrictHostKeyChecking {
					strictHostKeyChecking = false
				}
				sshCfg := &ssh.SSHConfig{
					StrictHostKeyChecking: strictHostKeyChecking,
				}
				client, err := ssh.NewClientWithConfig(s.SSH.Host, s.SSH.Port, s.SSH.User, password, sshCfg)
				if err != nil {
					serverResults = append(serverResults, &usecase.Result{
						Success: false,
						Error:   fmt.Errorf("connection to %s failed: %w", s.Name, err),
						Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "server_env", s.Name).WithOldState(map[string]interface{}{"server": s.Name}),
					})
					resultsCh <- serverResult{results: serverResults}
					return
				}

				syncer := serverpkg.NewSyncer(client, s, string(m.Environment), secrets, registries)
				syncResults := syncer.SyncAll()
				client.Close()

				for _, sr := range syncResults {
					var syncErr error
					if !sr.Success {
						syncErr = fmt.Errorf("%s: %s", sr.Name, sr.Message)
						if sr.Error != nil {
							syncErr = sr.Error
						}
					}
					serverResults = append(serverResults, &usecase.Result{
						Success: sr.Success,
						Error:   syncErr,
						Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "server_env", sr.Name).WithOldState(map[string]interface{}{"server": s.Name}),
					})
				}
				resultsCh <- serverResult{results: serverResults}
			}(srv)
		}

		wg.Wait()
		close(resultsCh)

		var results []*usecase.Result
		for r := range resultsCh {
			results = append(results, r.results...)
		}

		return applyCompleteAsyncMsg{results: results}
	}
}
