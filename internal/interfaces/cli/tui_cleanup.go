package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/ssh"
)

func (m Model) renderServiceCleanup() string {
	availableHeight := m.UI.Height - 10
	if availableHeight < 5 {
		availableHeight = 5
	}

	if m.UI.ErrorMessage != "" {
		var sb strings.Builder
		sb.WriteString(TitleStyle.Render("  Error") + "\n\n")
		sb.WriteString(ChangeDeleteStyle.Render("  "+m.UI.ErrorMessage) + "\n")
		sb.WriteString("\n" + HelpStyle.Render("  Esc back  q quit"))
		return BaseStyle.Render(sb.String())
	}

	totalItems := m.countCleanupItems()
	viewport := NewViewport(0, totalItems, availableHeight)
	viewport.CursorIndex = m.Cleanup.CleanupCursor
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render("  Service Cleanup - Orphan Resources")
	sb.WriteString(title + "\n\n")

	if totalItems == 0 {
		sb.WriteString("  No orphan services found on any server.\n")
	} else {
		itemIndex := 0
		for _, result := range m.Cleanup.CleanupResults {
			sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
			for _, container := range result.OrphanContainers {
				cursor := " "
				if m.Cleanup.CleanupCursor == itemIndex {
					cursor = ">"
				}
				checked := " "
				if m.Cleanup.CleanupSelected[itemIndex] {
					checked = "x"
				}
				line := fmt.Sprintf("  %s [%s] container: %s", cursor, checked, container)
				style := ChangeDeleteStyle
				sb.WriteString(style.Render(line) + "\n")
				itemIndex++
			}
			for _, dir := range result.OrphanDirs {
				cursor := " "
				if m.Cleanup.CleanupCursor == itemIndex {
					cursor = ">"
				}
				checked := " "
				if m.Cleanup.CleanupSelected[itemIndex] {
					checked = "x"
				}
				line := fmt.Sprintf("  %s [%s] directory: %s", cursor, checked, dir)
				style := ChangeDeleteStyle
				sb.WriteString(style.Render(line) + "\n")
				itemIndex++
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ move  Space toggle  Enter confirm  Esc back  q quit"))

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceCleanupConfirm() string {
	var sb strings.Builder
	title := TitleStyle.Render("  Confirm Cleanup")
	sb.WriteString(title + "\n\n")

	selectedCount := 0
	for _, selected := range m.Cleanup.CleanupSelected {
		if selected {
			selectedCount++
		}
	}

	sb.WriteString(fmt.Sprintf("  You are about to remove %d orphan resource(s).\n", selectedCount))
	sb.WriteString("  This action cannot be undone.\n\n")

	options := []string{"Yes, proceed", "Cancel"}
	for i, opt := range options {
		if i == m.Action.ConfirmSelected {
			sb.WriteString(MenuSelectedStyle.Render("> "+opt) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+opt) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceCleanupComplete() string {
	var sb strings.Builder
	title := TitleStyle.Render("  Cleanup Complete")
	sb.WriteString(title + "\n\n")

	for _, result := range m.Cleanup.CleanupResults {
		if len(result.RemovedContainers) > 0 || len(result.RemovedDirs) > 0 ||
			len(result.FailedContainers) > 0 || len(result.FailedDirs) > 0 {
			sb.WriteString(fmt.Sprintf("  [%s]\n", result.ServerName))
			for _, c := range result.RemovedContainers {
				sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ removed container: %s", c)) + "\n")
			}
			for _, d := range result.RemovedDirs {
				sb.WriteString(SuccessStyle.Render(fmt.Sprintf("    ✓ removed directory: %s", d)) + "\n")
			}
			for _, c := range result.FailedContainers {
				sb.WriteString(ChangeDeleteStyle.Render(fmt.Sprintf("    ✗ failed container: %s", c)) + "\n")
			}
			for _, d := range result.FailedDirs {
				sb.WriteString(ChangeDeleteStyle.Render(fmt.Sprintf("    ✗ failed directory: %s", d)) + "\n")
			}
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  Enter back to menu  q quit"))

	return BaseStyle.Render(sb.String())
}

func (m Model) countCleanupItems() int {
	count := 0
	for _, result := range m.Cleanup.CleanupResults {
		count += len(result.OrphanContainers) + len(result.OrphanDirs)
	}
	return count
}

func (m *Model) buildCleanupSelected() {
	m.Cleanup.CleanupSelected = make(map[int]bool)
	itemIndex := 0
	for _, result := range m.Cleanup.CleanupResults {
		for range result.OrphanContainers {
			m.Cleanup.CleanupSelected[itemIndex] = true
			itemIndex++
		}
		for range result.OrphanDirs {
			m.Cleanup.CleanupSelected[itemIndex] = true
			itemIndex++
		}
	}
}

func (m Model) hasSelectedCleanupItems() bool {
	for _, selected := range m.Cleanup.CleanupSelected {
		if selected {
			return true
		}
	}
	return false
}

func (m *Model) scanOrphanServices() {
	m.Cleanup.CleanupResults = nil
	m.UI.ErrorMessage = ""

	secrets := m.Config.GetSecretsMap()
	serviceMap := m.Config.GetServiceMap()
	infraServiceMap := m.Config.GetInfraServiceMap()

	for _, srv := range m.Server.ServerList {
		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Cannot resolve password: %v", srv.Name, err)
			return
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Connection failed: %v", srv.Name, err)
			return
		}

		containerStdout, _, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Failed to list containers: %v", srv.Name, err)
			client.Close()
			return
		}

		dirStdout, _, err := client.Run("sudo ls -1 " + constants.RemoteBaseDir + " 2>/dev/null || true")
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Failed to list directories: %v", srv.Name, err)
			client.Close()
			return
		}

		client.Close()

		result := CleanupResult{ServerName: srv.Name}

		for _, line := range strings.Split(strings.TrimSpace(containerStdout), "\n") {
			if line == "" {
				continue
			}
			var container struct {
				Name string `json:"Names"`
			}
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				continue
			}

			if !strings.HasPrefix(container.Name, "yo-"+string(m.Environment)+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(container.Name, "yo-"+string(m.Environment)+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				result.OrphanContainers = append(result.OrphanContainers, container.Name)
			}
		}

		for _, line := range strings.Split(strings.TrimSpace(dirStdout), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "yo-"+string(m.Environment)+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(line, "yo-"+string(m.Environment)+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				result.OrphanDirs = append(result.OrphanDirs, line)
			}
		}

		if len(result.OrphanContainers) > 0 || len(result.OrphanDirs) > 0 {
			m.Cleanup.CleanupResults = append(m.Cleanup.CleanupResults, result)
		}
	}
}

func (m *Model) executeServiceCleanup() {
	secrets := m.Config.GetSecretsMap()

	for i, result := range m.Cleanup.CleanupResults {
		srv := m.findServerByName(result.ServerName)
		if srv == nil {
			continue
		}

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			for _, c := range result.OrphanContainers {
				m.Cleanup.CleanupResults[i].FailedContainers = append(m.Cleanup.CleanupResults[i].FailedContainers, c)
			}
			for _, d := range result.OrphanDirs {
				m.Cleanup.CleanupResults[i].FailedDirs = append(m.Cleanup.CleanupResults[i].FailedDirs, d)
			}
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			for _, c := range result.OrphanContainers {
				m.Cleanup.CleanupResults[i].FailedContainers = append(m.Cleanup.CleanupResults[i].FailedContainers, c)
			}
			for _, d := range result.OrphanDirs {
				m.Cleanup.CleanupResults[i].FailedDirs = append(m.Cleanup.CleanupResults[i].FailedDirs, d)
			}
			continue
		}

		itemIndex := m.getServerCleanupStartIndex(i)
		for _, container := range result.OrphanContainers {
			if m.Cleanup.CleanupSelected[itemIndex] {
				_, stderr, err := client.Run(fmt.Sprintf("sudo docker rm -f %s", container))
				if err != nil {
					m.Cleanup.CleanupResults[i].FailedContainers = append(m.Cleanup.CleanupResults[i].FailedContainers, container+": "+stderr)
				} else {
					m.Cleanup.CleanupResults[i].RemovedContainers = append(m.Cleanup.CleanupResults[i].RemovedContainers, container)
				}
			}
			itemIndex++
		}
		for _, dir := range result.OrphanDirs {
			if m.Cleanup.CleanupSelected[itemIndex] {
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, dir)
				_, stderr, err := client.Run(fmt.Sprintf("sudo rm -rf %s", remoteDir))
				if err != nil {
					m.Cleanup.CleanupResults[i].FailedDirs = append(m.Cleanup.CleanupResults[i].FailedDirs, dir+": "+stderr)
				} else {
					m.Cleanup.CleanupResults[i].RemovedDirs = append(m.Cleanup.CleanupResults[i].RemovedDirs, dir)
				}
			}
			itemIndex++
		}

		client.Close()
	}
}

func (m *Model) findServerByName(name string) *entity.Server {
	for _, srv := range m.Server.ServerList {
		if srv.Name == name {
			return srv
		}
	}
	return nil
}

func (m *Model) getServerCleanupStartIndex(serverIndex int) int {
	count := 0
	for i := 0; i < serverIndex; i++ {
		count += len(m.Cleanup.CleanupResults[i].OrphanContainers) + len(m.Cleanup.CleanupResults[i].OrphanDirs)
	}
	return count
}
