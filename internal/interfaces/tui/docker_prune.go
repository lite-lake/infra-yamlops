package tui

import (
	"fmt"
	"sort"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	serverpkg "github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
)

// generateDockerPrunePlan creates a PlanView for docker prune from scan results.
func (m *Model) generateDockerPrunePlan(scanResults []DockerPruneScanResult) {
	var items []components.PlanItem
	for _, r := range scanResults {
		if r.ScanError != "" {
			items = append(items, components.PlanItem{
				Action:     "prune",
				Name:       r.ServerName,
				Server:     r.ServerName,
				Details:    fmt.Sprintf("scan error: %s", r.ScanError),
				ChangeType: "~",
				Selected:   false,
			})
			continue
		}
		reclaimable := r.DiskUsage.TotalReclaimable()
		items = append(items, components.PlanItem{
			Action:     "prune",
			Name:       r.ServerName,
			Server:     r.ServerName,
			Details:    fmt.Sprintf("reclaimable: %s", reclaimable),
			ChangeType: "~",
			Selected:   true,
		})
	}

	if len(items) == 0 {
		return
	}

	pv := components.NewPlanView("PLAN: docker prune", string(m.Environment), "", "docker_prune", false)
	pv.EnvWarning = true
	pv.SetItems(items)
	m.Action.PlanComponent = pv
	m.Action.PlanResult = valueobject.NewPlan()
	m.Action.ApplyTotal = len(items)
}

// scanDockerDiskUsageAsync scans Docker disk usage on all servers.
func (m *Model) scanDockerDiskUsageAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return dockerPruneScannedMsg{err: fmt.Errorf("config not loaded")}
		}

		secrets := m.Config.GetSecretsMap()
		registries := m.Config.Registries

		concurrency := m.Concurrency
		if concurrency <= 0 {
			concurrency = 5
		}

		type scanResult struct {
			index      int
			ServerName string
			DiskUsage  *serverpkg.DockerDiskUsage
			ScanError  string
		}

		servers := m.Server.ServerList

		sem := make(chan struct{}, concurrency)
		resultsCh := make(chan scanResult, len(servers))
		var wg sync.WaitGroup

		for i, srv := range servers {
			sem <- struct{}{}
			wg.Add(1)
			go func(idx int, s *entity.Server) {
				defer wg.Done()
				defer func() { <-sem }()

				password, err := s.SSH.Password.Resolve(secrets)
				if err != nil {
					resultsCh <- scanResult{index: idx, ServerName: s.Name, ScanError: fmt.Sprintf("cannot resolve password: %v", err)}
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
					resultsCh <- scanResult{index: idx, ServerName: s.Name, ScanError: fmt.Sprintf("connection failed: %v", err)}
					return
				}

				syncer := serverpkg.NewSyncer(client, s, string(m.Environment), secrets, registries)
				usage, err := syncer.DockerDiskUsage()
				client.Close()

				if err != nil {
					resultsCh <- scanResult{index: idx, ServerName: s.Name, ScanError: fmt.Sprintf("%v", err)}
					return
				}
				resultsCh <- scanResult{index: idx, ServerName: s.Name, DiskUsage: usage}
			}(i, srv)
		}

		wg.Wait()
		close(resultsCh)

		var allResults []scanResult
		for r := range resultsCh {
			allResults = append(allResults, r)
		}
		sort.Slice(allResults, func(i, j int) bool { return allResults[i].index < allResults[j].index })

		var results []DockerPruneScanResult
		for _, r := range allResults {
			results = append(results, DockerPruneScanResult{
				ServerName: r.ServerName,
				DiskUsage:  r.DiskUsage,
				ScanError:  r.ScanError,
			})
		}

		return dockerPruneScannedMsg{results: results}
	}
}

// executeDockerPruneAsync runs docker prune on selected servers.
func (m *Model) executeDockerPruneAsync() tea.Cmd {
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
		registries := m.Config.Registries

		type pruneResult struct {
			results []*usecase.Result
		}

		sem := make(chan struct{}, concurrency)
		resultsCh := make(chan pruneResult, len(servers))
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
						Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "docker_prune", s.Name).WithOldState(map[string]interface{}{"server": s.Name}),
					})
					resultsCh <- pruneResult{results: serverResults}
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
						Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "docker_prune", s.Name).WithOldState(map[string]interface{}{"server": s.Name}),
					})
					resultsCh <- pruneResult{results: serverResults}
					return
				}

				syncer := serverpkg.NewSyncer(client, s, string(m.Environment), secrets, registries)
				result := syncer.PruneDocker(serverpkg.PruneFilterAll)
				client.Close()

				var pruneErr error
				if !result.Success {
					pruneErr = fmt.Errorf("%s: %s", result.Name, result.Message)
				}
				serverResults = append(serverResults, &usecase.Result{
					Success: result.Success,
					Error:   pruneErr,
					Output:  result.Message,
					Change:  valueobject.NewChange(valueobject.ChangeTypeUpdate, "docker_prune", s.Name).WithOldState(map[string]interface{}{"server": s.Name}),
				})
				resultsCh <- pruneResult{results: serverResults}
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
