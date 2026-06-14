package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

type ServiceStatusFetchResult struct {
	StatusMap map[string]NodeStatus
}

type serviceInfo struct {
	Name   string
	Server string
	Type   NodeType
}

func fetchServiceStatus(servers []serverWithSSH, infraServices []serviceWithServer, bizServices []serviceWithServer, secrets map[string]string, env string) map[string]NodeStatus {
	statusMap := make(map[string]NodeStatus)

	for _, srv := range servers {
		password, err := srv.sshPassword.Resolve(secrets)
		if err != nil {
			continue
		}

		client, err := ssh.NewClientWithConfig(srv.sshHost, srv.sshPort, srv.sshUser, password, &ssh.SSHConfig{
			StrictHostKeyChecking: srv.strictHostKeyChecking,
		})
		if err != nil {
			continue
		}

		stdout, _, err := client.Run("sudo docker compose ls -a --format json 2>/dev/null || sudo docker compose ls -a --format json")
		if err != nil || stdout == "" {
			client.Close()
			continue
		}

		type composeProject struct {
			Name string `json:"Name"`
		}
		var projects []composeProject
		if err := json.Unmarshal([]byte(stdout), &projects); err != nil {
			for _, line := range strings.Split(stdout, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var proj composeProject
				if err := json.Unmarshal([]byte(line), &proj); err == nil && proj.Name != "" {
					projects = append(projects, proj)
				}
			}
		}

		for _, proj := range projects {
			statusMap[proj.Name] = StatusRunning
		}

		for _, infra := range infraServices {
			if infra.serverName != srv.name {
				continue
			}
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, env, infra.name))
			key := fmt.Sprintf(constants.ServiceNameFormat, env, infra.name)
			if _, exists := statusMap[key]; !exists {
				stdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if err != nil {
					statusMap[key] = StatusError
				} else if strings.TrimSpace(stdout) == "exists" {
					statusMap[key] = StatusStopped
				}
			}
		}

		for _, svc := range bizServices {
			if svc.serverName != srv.name {
				continue
			}
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, env, svc.name))
			key := fmt.Sprintf(constants.ServiceNameFormat, env, svc.name)
			if _, exists := statusMap[key]; !exists {
				stdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
				if err != nil {
					statusMap[key] = StatusError
				} else if strings.TrimSpace(stdout) == "exists" {
					statusMap[key] = StatusStopped
				}
			}
		}

		client.Close()
	}

	return statusMap
}

type serverWithSSH struct {
	name        string
	sshHost     string
	sshPort     int
	sshUser     string
	sshPassword interface {
		Resolve(map[string]string) (string, error)
	}
	strictHostKeyChecking bool
}

type serviceWithServer struct {
	name       string
	serverName string
}

func applyStatusToNodes(nodes []*TreeNode, statusMap map[string]NodeStatus, env string) {
	for _, node := range nodes {
		applyStatusToNode(node, statusMap, env)
	}
}

func applyStatusToNode(node *TreeNode, statusMap map[string]NodeStatus, env string) {
	if node.Type == NodeTypeInfra || node.Type == NodeTypeBiz {
		key := fmt.Sprintf(constants.ServiceNameFormat, env, node.Name)
		if status, exists := statusMap[key]; exists {
			node.Status = status
		}
	}
	for _, child := range node.Children {
		applyStatusToNode(child, statusMap, env)
	}
}

func getSelectedServicesInfo(nodes []*TreeNode) []serviceInfo {
	var services []serviceInfo
	serviceSet := make(map[string]bool)

	for _, node := range nodes {
		leaves := node.GetSelectedLeaves()
		for _, leaf := range leaves {
			var serverName string
			if leaf.Parent != nil {
				serverName = leaf.Parent.Name
			}
			switch leaf.Type {
			case NodeTypeInfra:
				if !serviceSet[leaf.Name] {
					services = append(services, serviceInfo{Name: leaf.Name, Server: serverName, Type: NodeTypeInfra})
					serviceSet[leaf.Name] = true
				}
			case NodeTypeBiz:
				if !serviceSet[leaf.Name] {
					services = append(services, serviceInfo{Name: leaf.Name, Server: serverName, Type: NodeTypeBiz})
					serviceSet[leaf.Name] = true
				}
			}
		}
	}
	return services
}

type secretResolver interface {
	Resolve(map[string]string) (string, error)
}

func (m *Model) buildServerWithSSHList() []serverWithSSH {
	var result []serverWithSSH
	if m.Config == nil {
		return result
	}
	for i := range m.Config.Servers {
		srv := &m.Config.Servers[i]
		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		result = append(result, serverWithSSH{
			name:                  srv.Name,
			sshHost:               srv.SSH.Host,
			sshPort:               srv.SSH.Port,
			sshUser:               srv.SSH.User,
			sshPassword:           &srv.SSH.Password,
			strictHostKeyChecking: strictHostKeyChecking,
		})
	}
	return result
}

func (m *Model) buildInfraServicesList() []serviceWithServer {
	var result []serviceWithServer
	if m.Config == nil {
		return result
	}
	for _, svc := range m.Config.InfraServices {
		result = append(result, serviceWithServer{
			name:       svc.Name,
			serverName: svc.Server,
		})
	}
	return result
}

func (m *Model) buildBizServicesList() []serviceWithServer {
	var result []serviceWithServer
	if m.Config == nil {
		return result
	}
	for _, svc := range m.Config.Services {
		result = append(result, serviceWithServer{
			name:       svc.Name,
			serverName: svc.Server,
		})
	}
	return result
}

func (m *Model) fetchServiceStatusAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return serviceStatusFetchedMsg{statusMap: make(map[string]NodeStatus)}
		}
		secrets := m.Config.GetSecretsMap()
		servers := m.buildServerWithSSHList()
		infraServices := m.buildInfraServicesList()
		bizServices := m.buildBizServicesList()
		statusMap := fetchServiceStatus(servers, infraServices, bizServices, secrets, string(m.Environment))
		return serviceStatusFetchedMsg{statusMap: statusMap}
	}
}

func (m *Model) fetchRestartServiceStatusAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return restartStatusFetchedMsg{statusMap: make(map[string]NodeStatus)}
		}
		secrets := m.Config.GetSecretsMap()
		servers := m.buildServerWithSSHList()
		infraServices := m.buildInfraServicesList()
		bizServices := m.buildBizServicesList()
		statusMap := fetchServiceStatus(servers, infraServices, bizServices, secrets, string(m.Environment))
		return restartStatusFetchedMsg{statusMap: statusMap}
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

func (m Model) applyServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Stop.ServiceStatusMap, string(m.Environment))
}

func (m Model) applyRestartServiceStatusToTree() {
	applyStatusToNodes(m.Tree.TreeNodes, m.Restart.ServiceStatusMap, string(m.Environment))
}

func (m Model) countSelectedServices() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotalServices() int {
	count := 0
	for _, node := range m.Tree.TreeNodes {
		count += node.CountTotal()
	}
	return count
}

func (m Model) hasSelectedServices() bool {
	for _, node := range m.Tree.TreeNodes {
		if node.CountSelected() > 0 {
			return true
		}
	}
	return false
}

func (m Model) hasSelectedStopServices() bool {
	return m.hasSelectedServices()
}

func (m Model) hasSelectedRestartServices() bool {
	return m.hasSelectedServices()
}

func (m Model) countSelectedForStop() int {
	return m.countSelectedServices()
}

func (m Model) countTotalForStop() int {
	return m.countTotalServices()
}

func (m Model) countSelectedForRestart() int {
	return m.countSelectedServices()
}

func (m Model) countTotalForRestart() int {
	return m.countTotalServices()
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

func (m *Model) generateStopPlan() {
	plan := valueobject.NewPlan()
	services := getSelectedServicesInfo(m.Tree.TreeNodes)
	for _, svc := range services {
		plan.AddChange(valueobject.NewChange(
			valueobject.ChangeTypeUpdate,
			"service",
			svc.Name,
		).WithActions("stop").WithRemoteExists(true).WithOldState(map[string]interface{}{
			"server": svc.Server,
			"status": "running",
		}).WithNewState(map[string]interface{}{
			"status": "stopped",
		}))
	}
	m.Action.PlanResult = plan
	m.Action.ApplyTotal = len(plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
}

func (m *Model) generateRestartPlan() {
	plan := valueobject.NewPlan()
	services := getSelectedServicesInfo(m.Tree.TreeNodes)
	for _, svc := range services {
		plan.AddChange(valueobject.NewChange(
			valueobject.ChangeTypeUpdate,
			"service",
			svc.Name,
		).WithActions("restart").WithRemoteExists(true).WithOldState(map[string]interface{}{
			"server": svc.Server,
			"status": "running",
		}).WithNewState(map[string]interface{}{
			"status": "restarted",
		}))
	}
	m.Action.PlanResult = plan
	m.Action.ApplyTotal = len(plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
}

func (m *Model) generateCleanupPlan() {
	plan := valueobject.NewPlan()
	itemIndex := 0
	for _, result := range m.Cleanup.CleanupResults {
		for _, container := range result.OrphanContainers {
			if m.Cleanup.CleanupSelected[itemIndex] {
				plan.AddChange(valueobject.NewChange(
					valueobject.ChangeTypeDelete,
					"container",
					container,
				).WithActions("cleanup").WithRemoteExists(true).WithOldState(map[string]interface{}{
					"server": result.ServerName,
				}))
			}
			itemIndex++
		}
		for _, dir := range result.OrphanDirs {
			if m.Cleanup.CleanupSelected[itemIndex] {
				plan.AddChange(valueobject.NewChange(
					valueobject.ChangeTypeDelete,
					"directory",
					dir,
				).WithActions("cleanup").WithRemoteExists(true).WithOldState(map[string]interface{}{
					"server": result.ServerName,
				}))
			}
			itemIndex++
		}
	}
	m.Action.PlanResult = plan
	m.Action.ApplyTotal = len(plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
}

func (m *Model) getServerCleanupStartIndex(serverIndex int) int {
	count := 0
	for i := 0; i < serverIndex; i++ {
		count += len(m.Cleanup.CleanupResults[i].OrphanContainers) + len(m.Cleanup.CleanupResults[i].OrphanDirs)
	}
	return count
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

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		sshCfg := &ssh.SSHConfig{
			StrictHostKeyChecking: strictHostKeyChecking,
		}
		client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Connection failed: %v", srv.Name, err)
			return
		}

		containerStdout, containerStderr, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Failed to list containers: %v, stderr: %s", srv.Name, err, containerStderr)
			client.Close()
			return
		}

		dirStdout, dirStderr, err := client.Run("sudo ls -1 " + constants.RemoteBaseDir + " 2>/dev/null || true")
		if err != nil {
			m.UI.ErrorMessage = fmt.Sprintf("[%s] Failed to list directories: %v, stderr: %s", srv.Name, err, dirStderr)
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

func (m *Model) scanOrphanServicesAsync() tea.Cmd {
	return func() tea.Msg {
		var results []CleanupResult

		secrets := m.Config.GetSecretsMap()
		serviceMap := m.Config.GetServiceMap()
		infraServiceMap := m.Config.GetInfraServiceMap()

		for _, srv := range m.Server.ServerList {
			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				return orphanServicesScannedMsg{err: fmt.Errorf("[%s] Cannot resolve password: %v", srv.Name, err)}
			}

			strictHostKeyChecking := true
			if !srv.SSH.StrictHostKeyChecking {
				strictHostKeyChecking = false
			}
			sshCfg := &ssh.SSHConfig{
				StrictHostKeyChecking: strictHostKeyChecking,
			}
			client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
			if err != nil {
				return orphanServicesScannedMsg{err: fmt.Errorf("[%s] Connection failed: %v", srv.Name, err)}
			}

			containerStdout, containerStderr, err := client.Run("sudo docker ps -a --format '{{json .}}'")
			if err != nil {
				client.Close()
				return orphanServicesScannedMsg{err: fmt.Errorf("[%s] Failed to list containers: %v, stderr: %s", srv.Name, err, containerStderr)}
			}

			dirStdout, dirStderr, err := client.Run("sudo ls -1 " + constants.RemoteBaseDir + " 2>/dev/null || true")
			if err != nil {
				client.Close()
				return orphanServicesScannedMsg{err: fmt.Errorf("[%s] Failed to list directories: %v, stderr: %s", srv.Name, err, dirStderr)}
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
				results = append(results, result)
			}
		}

		return orphanServicesScannedMsg{results: results}
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

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		sshCfg := &ssh.SSHConfig{
			StrictHostKeyChecking: strictHostKeyChecking,
		}
		client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
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

func (m *Model) executeServiceCleanupAsync() tea.Cmd {
	return func() tea.Msg {
		secrets := m.Config.GetSecretsMap()
		results := make([]CleanupResult, len(m.Cleanup.CleanupResults))
		copy(results, m.Cleanup.CleanupResults)

		for i, result := range results {
			srv := m.findServerByName(result.ServerName)
			if srv == nil {
				continue
			}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				for _, c := range result.OrphanContainers {
					results[i].FailedContainers = append(results[i].FailedContainers, c)
				}
				for _, d := range result.OrphanDirs {
					results[i].FailedDirs = append(results[i].FailedDirs, d)
				}
				continue
			}

			strictHostKeyChecking := true
			if !srv.SSH.StrictHostKeyChecking {
				strictHostKeyChecking = false
			}
			sshCfg := &ssh.SSHConfig{
				StrictHostKeyChecking: strictHostKeyChecking,
			}
			client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
			if err != nil {
				for _, c := range result.OrphanContainers {
					results[i].FailedContainers = append(results[i].FailedContainers, c)
				}
				for _, d := range result.OrphanDirs {
					results[i].FailedDirs = append(results[i].FailedDirs, d)
				}
				continue
			}

			itemIndex := m.getServerCleanupStartIndex(i)
			for _, container := range result.OrphanContainers {
				if m.Cleanup.CleanupSelected[itemIndex] {
					_, stderr, err := client.Run(fmt.Sprintf("sudo docker rm -f %s", container))
					if err != nil {
						results[i].FailedContainers = append(results[i].FailedContainers, container+": "+stderr)
					} else {
						results[i].RemovedContainers = append(results[i].RemovedContainers, container)
					}
				}
				itemIndex++
			}
			for _, dir := range result.OrphanDirs {
				if m.Cleanup.CleanupSelected[itemIndex] {
					remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, dir)
					_, stderr, err := client.Run(fmt.Sprintf("sudo rm -rf %s", remoteDir))
					if err != nil {
						results[i].FailedDirs = append(results[i].FailedDirs, dir+": "+stderr)
					} else {
						results[i].RemovedDirs = append(results[i].RemovedDirs, dir)
					}
				}
				itemIndex++
			}

			client.Close()
		}

		return serviceCleanupCompleteMsg{results: results}
	}
}

func (m *Model) validateServicesAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return validateCompleteMsg{module: "service", err: fmt.Errorf("config not loaded")}
		}

		passed := 0
		failed := 0
		warnings := 0
		var errs []ValidateErrorItem

		serverMap := m.Config.GetServerMap()
		secretsMap := m.Config.GetSecretsMap()

		// 唯一性约束：BizService 和 InfraService 同名冲突
		nameSet := make(map[string]bool)
		for _, svc := range m.Config.Services {
			nameSet[svc.Name] = true
		}
		for _, infra := range m.Config.InfraServices {
			if nameSet[infra.Name] {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Duplicate service name: '%s' exists in both BizService and InfraService", infra.Name),
					Suggestion: fmt.Sprintf("Rename one of the services '%s' to avoid collision across service types", infra.Name),
				})
				failed++
			}
		}

		// BizService 验证：调用 entity.Validate() + 跨实体引用 + 冲突检测
		for _, svc := range m.Config.Services {
			if err := svc.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("BizService '%s': %v", svc.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in BizService '%s' configuration (services.yaml)", svc.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：server 必须存在
			if _, exists := serverMap[svc.Server]; !exists {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("BizService '%s' references non-existent server '%s'", svc.Name, svc.Server),
					Suggestion: fmt.Sprintf("Ensure server '%s' is defined in servers.yaml, or update BizService '%s' to use an existing server", svc.Server, svc.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：secrets 列表中每个 secret 必须存在
			for _, secretName := range svc.Secrets {
				if _, exists := secretsMap[secretName]; !exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("BizService '%s' references non-existent secret '%s'", svc.Name, secretName),
						Suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml, or remove it from BizService '%s' secrets list", secretName, svc.Name),
					})
					failed++
				}
			}
			// 跨实体引用：env 中 SecretRef 引用的 secret 必须存在
			for key, ref := range svc.Env {
				if ref.Secret() != "" {
					if _, exists := secretsMap[ref.Secret()]; !exists {
						errs = append(errs, ValidateErrorItem{
							Level:      "error",
							Message:    fmt.Sprintf("BizService '%s' env[%s] references non-existent secret '%s'", svc.Name, key, ref.Secret()),
							Suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml, or update env[%s] in BizService '%s'", ref.Secret(), key, svc.Name),
						})
						failed++
					}
				}
			}
			// 警告检查：生产环境服务未配置 healthcheck
			if svc.Healthcheck == nil || (svc.Healthcheck.Path == "" && svc.Healthcheck.Interval == "") {
				errs = append(errs, ValidateErrorItem{
					Level:      "warning",
					Message:    fmt.Sprintf("Service '%s': no healthcheck configured", svc.Name),
					Suggestion: "Add healthcheck for production services",
				})
				warnings++
			}
			passed++
		}

		// InfraService 验证：调用 entity.Validate() + 跨实体引用
		for _, infra := range m.Config.InfraServices {
			if err := infra.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("InfraService '%s': %v", infra.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in InfraService '%s' configuration (infra-services.yaml)", infra.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：server 必须存在
			if _, exists := serverMap[infra.Server]; !exists {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("InfraService '%s' references non-existent server '%s'", infra.Name, infra.Server),
					Suggestion: fmt.Sprintf("Ensure server '%s' is defined in servers.yaml, or update InfraService '%s' to use an existing server", infra.Server, infra.Name),
				})
				failed++
				continue
			}
			passed++
		}

		// 冲突检测：同一服务器上端口不能冲突（host 端口）
		serverPorts := make(map[string]map[int]string) // server -> hostPort -> serviceName
		for _, svc := range m.Config.Services {
			if _, exists := serverMap[svc.Server]; !exists {
				continue
			}
			if serverPorts[svc.Server] == nil {
				serverPorts[svc.Server] = make(map[int]string)
			}
			for _, port := range svc.Ports {
				if existing, exists := serverPorts[svc.Server][port.Host]; exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Port conflict: server '%s' host port %d used by both '%s' and '%s'", svc.Server, port.Host, existing, svc.Name),
						Suggestion: fmt.Sprintf("Change the host port for '%s' or '%s' on server '%s' to avoid the conflict", existing, svc.Name, svc.Server),
					})
					failed++
				} else {
					serverPorts[svc.Server][port.Host] = svc.Name
				}
			}
		}
		for _, infra := range m.Config.InfraServices {
			if _, exists := serverMap[infra.Server]; !exists {
				continue
			}
			if serverPorts[infra.Server] == nil {
				serverPorts[infra.Server] = make(map[int]string)
			}
			if infra.GatewayPorts != nil {
				for _, hostPort := range []int{infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTPS} {
					if existing, exists := serverPorts[infra.Server][hostPort]; exists {
						errs = append(errs, ValidateErrorItem{
							Level:      "error",
							Message:    fmt.Sprintf("Port conflict: server '%s' host port %d used by both '%s' and '%s'", infra.Server, hostPort, existing, infra.Name),
							Suggestion: fmt.Sprintf("Change the host port for '%s' or '%s' on server '%s' to avoid the conflict", existing, infra.Name, infra.Server),
						})
						failed++
					} else {
						serverPorts[infra.Server][hostPort] = infra.Name
					}
				}
			}
		}

		// 冲突检测：不同 service 的 gateway 路由中 hostname 不能重复
		hostnameServices := make(map[string]string) // hostname -> serviceName
		for _, svc := range m.Config.Services {
			for _, gw := range svc.Gateways {
				if existing, exists := hostnameServices[gw.Hostname]; exists && existing != svc.Name {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Gateway hostname conflict: '%s' used by both '%s' and '%s'", gw.Hostname, existing, svc.Name),
						Suggestion: fmt.Sprintf("Assign a unique hostname for each service gateway route; '%s' is already used by '%s'", gw.Hostname, existing),
					})
					failed++
				} else {
					hostnameServices[gw.Hostname] = svc.Name
				}
			}
		}

		return validateCompleteMsg{
			module:   "service",
			passed:   passed,
			failed:   failed,
			warnings: warnings,
			errors:   errs,
		}
	}
}

func (m *Model) validateServersAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return validateCompleteMsg{module: "server", err: fmt.Errorf("config not loaded")}
		}

		passed := 0
		failed := 0
		warnings := 0
		var errs []ValidateErrorItem

		zoneMap := m.Config.GetZoneMap()
		ispMap := m.Config.GetISPMap()
		secretsMap := m.Config.GetSecretsMap()

		for _, srv := range m.Config.Servers {
			// 调用 entity.Server.Validate() 覆盖自验证：
			// name 非空、zone 非空、IP 合法、ssh.host 非空、ssh.port 1-65535、
			// ssh.user 非空、ssh.password 合法、network name 非空、network type bridge/overlay
			if err := srv.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Server '%s': %v", srv.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in server '%s' configuration (servers.yaml)", srv.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：zone 必须存在
			if _, exists := zoneMap[srv.Zone]; !exists {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Server '%s' references non-existent zone '%s'", srv.Name, srv.Zone),
					Suggestion: fmt.Sprintf("Add zone '%s' to zones.yaml, or update server '%s' to use an existing zone", srv.Zone, srv.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：isp 如果非空必须存在于 isps 中
			if srv.ISP != "" {
				if _, exists := ispMap[srv.ISP]; !exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Server '%s' references non-existent ISP '%s'", srv.Name, srv.ISP),
						Suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml, or update server '%s' to use an existing ISP", srv.ISP, srv.Name),
					})
					failed++
					continue
				}
			}
			// 跨实体引用：SSH password 引用的 secret 必须存在
			if srv.SSH.Password.Secret() != "" {
				if _, exists := secretsMap[srv.SSH.Password.Secret()]; !exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Server '%s' SSH password references non-existent secret '%s'", srv.Name, srv.SSH.Password.Secret()),
						Suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml, or update server '%s' SSH password configuration", srv.SSH.Password.Secret(), srv.Name),
					})
					failed++
					continue
				}
			}
			passed++
		}

		return validateCompleteMsg{
			module:   "server",
			passed:   passed,
			failed:   failed,
			warnings: warnings,
			errors:   errs,
		}
	}
}

func (m *Model) validateDNSAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return validateCompleteMsg{module: "dns", err: fmt.Errorf("config not loaded")}
		}

		passed := 0
		failed := 0
		warnings := 0
		var errs []ValidateErrorItem

		ispMap := m.Config.GetISPMap()
		domainMap := m.Config.GetDomainMap()

		// 冲突检测：domain name 不能重复定义
		domainNames := make(map[string]bool)

		for _, domain := range m.Config.Domains {
			// 调用 entity.Domain.Validate() 覆盖自验证：
			// name 非空且符合域名正则（支持通配符 *.）、dns_isp 非空、每条 DNS record 合法
			if err := domain.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Domain '%s': %v", domain.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in domain '%s' configuration (dns.yaml)", domain.Name),
				})
				failed++
				continue
			}
			if domainNames[domain.Name] {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Duplicate domain: %s", domain.Name),
					Suggestion: fmt.Sprintf("Remove the duplicate definition of domain '%s' in dns.yaml", domain.Name),
				})
				failed++
				continue
			}
			domainNames[domain.Name] = true

			// 跨实体引用：dns_isp 必须存在于 isps 中
			if _, exists := ispMap[domain.DNSISP]; !exists {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Domain '%s' references non-existent dns_isp '%s'", domain.Name, domain.DNSISP),
					Suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml, or update domain '%s' dns_isp to an existing ISP", domain.DNSISP, domain.Name),
				})
				failed++
				continue
			}
			// 跨实体引用：isp 如果非空必须存在于 isps 中
			if domain.ISP != "" {
				if _, exists := ispMap[domain.ISP]; !exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Domain '%s' references non-existent isp '%s'", domain.Name, domain.ISP),
						Suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml, or update domain '%s' isp to an existing ISP", domain.ISP, domain.Name),
					})
					failed++
					continue
				}
			}
			// 跨实体引用：parent 如果非空必须存在于 domains 中
			if domain.Parent != "" {
				if _, exists := domainMap[domain.Parent]; !exists {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Domain '%s' references non-existent parent '%s'", domain.Name, domain.Parent),
						Suggestion: fmt.Sprintf("Add domain '%s' to dns.yaml, or update domain '%s' parent to an existing domain", domain.Parent, domain.Name),
					})
					failed++
					continue
				}
			}

			// 冲突检测：DNS record 不能重复（key = type:name:value）
			recordKeys := make(map[string]bool)
			for _, record := range domain.Records {
				key := fmt.Sprintf("%s:%s:%s", record.Type, record.Name, record.Value)
				if recordKeys[key] {
					errs = append(errs, ValidateErrorItem{
						Level:      "error",
						Message:    fmt.Sprintf("Duplicate record in domain '%s': %s", domain.Name, key),
						Suggestion: fmt.Sprintf("Remove the duplicate DNS record '%s' from domain '%s'", key, domain.Name),
					})
					failed++
				}
				recordKeys[key] = true
			}
			passed++
		}

		return validateCompleteMsg{
			module:   "dns",
			passed:   passed,
			failed:   failed,
			warnings: warnings,
			errors:   errs,
		}
	}
}

func (m *Model) validateConfigAsync() tea.Cmd {
	return func() tea.Msg {
		if m.Config == nil {
			return validateCompleteMsg{module: "config", err: fmt.Errorf("config not loaded")}
		}

		passed := 0
		failed := 0
		warnings := 0
		var errs []ValidateErrorItem

		// ISP 配置验证：调用 entity.ISP.Validate() 覆盖自验证
		// name 非空、type 合法、services 非空且值合法、endpoint 非空、凭证字段已配置
		for _, isp := range m.Config.ISPs {
			if err := isp.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("ISP '%s': %v", isp.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in ISP '%s' configuration (isps.yaml)", isp.Name),
				})
				failed++
				continue
			}
			passed++
		}

		// Registry 配置验证：调用 entity.Registry.Validate() 覆盖自验证
		// name 非空、url 非空且格式合法、namespace 非空、认证信息已配置
		for _, reg := range m.Config.Registries {
			if err := reg.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Registry '%s': %v", reg.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in registry '%s' configuration (registries.yaml)", reg.Name),
				})
				failed++
				continue
			}
			passed++
		}

		// Secret 配置验证：key 非空且符合命名规范
		secretNames := make(map[string]bool)
		for _, secret := range m.Config.Secrets {
			if err := secret.Validate(); err != nil {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Secret '%s': %v", secret.Name, err),
					Suggestion: fmt.Sprintf("Fix the invalid fields in secret '%s' configuration (secrets.yaml)", secret.Name),
				})
				failed++
				continue
			}
			if !isValidSecretName(secret.Name) {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Secret '%s': invalid name format (must be lowercase, digits, underscores)", secret.Name),
					Suggestion: fmt.Sprintf("Rename secret '%s' to use only lowercase letters, digits, and underscores", secret.Name),
				})
				failed++
				continue
			}
			if secretNames[secret.Name] {
				errs = append(errs, ValidateErrorItem{
					Level:      "error",
					Message:    fmt.Sprintf("Duplicate secret: '%s'", secret.Name),
					Suggestion: fmt.Sprintf("Remove the duplicate definition of secret '%s' in secrets.yaml", secret.Name),
				})
				failed++
				continue
			}
			secretNames[secret.Name] = true
			passed++
		}

		return validateCompleteMsg{
			module:   "config",
			passed:   passed,
			failed:   failed,
			warnings: warnings,
			errors:   errs,
		}
	}
}

func isValidSecretName(name string) bool {
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return len(name) > 0
}

func (m *Model) buildFilterView() {
	if m.Config == nil {
		return
	}

	title := "Select filters to narrow scope:"
	sv := components.NewSelectionView(title)

	switch m.Action.OperationType {
	case "show":
		sv.AddGroup("Type", []components.SelectionItem{
			{Label: "biz", Selected: true},
			{Label: "infra", Selected: true},
		})
	case "stop", "restart":
		sv.AddGroup("Type", []components.SelectionItem{
			{Label: "biz", Selected: true},
			{Label: "infra", Selected: true},
		})

		var zoneItems []components.SelectionItem
		for _, zone := range m.Config.Zones {
			zoneItems = append(zoneItems, components.SelectionItem{
				Label:    zone.Name,
				Selected: true,
			})
		}
		if len(zoneItems) > 0 {
			sv.AddGroup("Zones", zoneItems)
		}

		var serverItems []components.SelectionItem
		for _, srv := range m.Config.Servers {
			serverItems = append(serverItems, components.SelectionItem{
				Label:    srv.Name,
				Selected: true,
			})
		}
		if len(serverItems) > 0 {
			sv.AddGroup("Servers", serverItems)
		}
	case "cleanup":
		sv.AddGroup("Type", []components.SelectionItem{
			{Label: "biz", Selected: true},
			{Label: "infra", Selected: true},
		})

		var zoneItems []components.SelectionItem
		for _, zone := range m.Config.Zones {
			zoneItems = append(zoneItems, components.SelectionItem{
				Label:    zone.Name,
				Selected: true,
			})
		}
		if len(zoneItems) > 0 {
			sv.AddGroup("Zones", zoneItems)
		}

		var serverItems []components.SelectionItem
		for _, srv := range m.Config.Servers {
			serverItems = append(serverItems, components.SelectionItem{
				Label:    srv.Name,
				Selected: true,
			})
		}
		if len(serverItems) > 0 {
			sv.AddGroup("Servers", serverItems)
		}
	}

	m.Action.FilterView = sv
	m.updateFilterMatchedLine()
}

func (m *Model) updateFilterMatchedLine() {
	if m.Action.FilterView == nil || m.Config == nil {
		return
	}

	switch m.Action.OperationType {
	case "show":
		bizCount, infraCount := m.getSelectedTypeCounts()
		total := bizCount + infraCount
		typeLabel := m.getSelectedTypeLabel()
		if typeLabel == "" {
			typeLabel = "all"
		}
		m.Action.FilterView.MatchedLine = styles.MutedStyle.Render(
			fmt.Sprintf("Type: %s | %d services", typeLabel, total),
		)
	case "stop", "restart":
		services := m.getFilteredServices()
		serverSet := make(map[string]bool)
		for _, svc := range services {
			serverSet[svc.Server] = true
		}
		m.Action.FilterView.MatchedLine = styles.MutedStyle.Render(
			fmt.Sprintf("Matched services: %d services across %d servers", len(services), len(serverSet)),
		)
	case "cleanup":
		filteredServers := m.getFilteredServersForCleanup()
		serverSet := make(map[string]bool)
		for _, s := range filteredServers {
			serverSet[s] = true
		}
		count := 0
		for _, result := range m.Cleanup.CleanupResults {
			if serverSet[result.ServerName] {
				count += len(result.OrphanContainers) + len(result.OrphanDirs)
			}
		}
		m.Action.FilterView.MatchedLine = styles.MutedStyle.Render(
			fmt.Sprintf("Matched items: %d items across %d servers", count, len(filteredServers)),
		)
	}
}

func (m *Model) getFilteredServices() []serviceInfo {
	sv := m.Action.FilterView
	if sv == nil || m.Config == nil {
		return nil
	}

	typeBiz := false
	typeInfra := false
	for _, label := range sv.GetSelectedLabels(0) {
		switch label {
		case "biz":
			typeBiz = true
		case "infra":
			typeInfra = true
		}
	}

	selectedZones := make(map[string]bool)
	zoneIdx := m.getFilterGroupIndex("Zones")
	if zoneIdx >= 0 {
		for _, label := range sv.GetSelectedLabels(zoneIdx) {
			selectedZones[label] = true
		}
	}

	selectedServers := make(map[string]bool)
	serverIdx := m.getFilterGroupIndex("Servers")
	if serverIdx >= 0 {
		for _, label := range sv.GetSelectedLabels(serverIdx) {
			selectedServers[label] = true
		}
	}

	serverZoneMap := make(map[string]string)
	for _, srv := range m.Config.Servers {
		serverZoneMap[srv.Name] = srv.Zone
	}

	var services []serviceInfo
	if typeBiz {
		for _, svc := range m.Config.Services {
			zone := serverZoneMap[svc.Server]
			if selectedZones[zone] && selectedServers[svc.Server] {
				services = append(services, serviceInfo{
					Name:   svc.Name,
					Server: svc.Server,
					Type:   NodeTypeBiz,
				})
			}
		}
	}
	if typeInfra {
		for _, svc := range m.Config.InfraServices {
			zone := serverZoneMap[svc.Server]
			if selectedZones[zone] && selectedServers[svc.Server] {
				services = append(services, serviceInfo{
					Name:   svc.Name,
					Server: svc.Server,
					Type:   NodeTypeInfra,
				})
			}
		}
	}

	return services
}

func (m *Model) getSelectedServersFromFilter() []string {
	sv := m.Action.FilterView
	if sv == nil {
		return nil
	}
	serverIdx := m.getFilterGroupIndex("Servers")
	if serverIdx < 0 {
		return nil
	}
	return sv.GetSelectedLabels(serverIdx)
}

func (m *Model) getFilteredServersForCleanup() []string {
	sv := m.Action.FilterView
	if sv == nil || m.Config == nil {
		return nil
	}

	selectedZones := make(map[string]bool)
	zoneIdx := m.getFilterGroupIndex("Zones")
	if zoneIdx >= 0 {
		for _, label := range sv.GetSelectedLabels(zoneIdx) {
			selectedZones[label] = true
		}
	} else {
		for _, zone := range m.Config.Zones {
			selectedZones[zone.Name] = true
		}
	}

	selectedServers := make(map[string]bool)
	serverIdx := m.getFilterGroupIndex("Servers")
	if serverIdx >= 0 {
		for _, label := range sv.GetSelectedLabels(serverIdx) {
			selectedServers[label] = true
		}
	}

	serverZoneMap := make(map[string]string)
	for _, srv := range m.Config.Servers {
		serverZoneMap[srv.Name] = srv.Zone
	}

	var servers []string
	for _, srv := range m.Config.Servers {
		zone := serverZoneMap[srv.Name]
		if selectedZones[zone] && selectedServers[srv.Name] {
			servers = append(servers, srv.Name)
		}
	}
	return servers
}

func (m *Model) getFilterGroupIndex(title string) int {
	if m.Action.FilterView == nil {
		return -1
	}
	for i, group := range m.Action.FilterView.Groups {
		if group.Title == title {
			return i
		}
	}
	return -1
}

func (m *Model) getSelectedTypeLabel() string {
	if m.Action.FilterView == nil {
		return ""
	}
	selectedTypes := m.Action.FilterView.GetSelectedLabels(0)
	if len(selectedTypes) == 0 {
		return ""
	}
	if len(selectedTypes) == 1 {
		return selectedTypes[0]
	}
	return strings.Join(selectedTypes, ",")
}

func (m *Model) getSelectedTypeCounts() (bizCount, infraCount int) {
	typeBiz := false
	typeInfra := false
	if m.Action.FilterView != nil {
		for _, label := range m.Action.FilterView.GetSelectedLabels(0) {
			switch label {
			case "biz":
				typeBiz = true
			case "infra":
				typeInfra = true
			}
		}
	}
	if typeBiz {
		bizCount = len(m.Config.Services)
	}
	if typeInfra {
		infraCount = len(m.Config.InfraServices)
	}
	return
}

func (m *Model) generatePlanFromFilter() {
	plan := valueobject.NewPlan()

	switch m.Action.OperationType {
	case "stop":
		for _, svc := range m.getFilteredServices() {
			plan.AddChange(valueobject.NewChange(
				valueobject.ChangeTypeUpdate,
				"service",
				svc.Name,
			).WithActions("stop").WithRemoteExists(true).WithOldState(map[string]interface{}{
				"server": svc.Server,
				"status": "running",
			}).WithNewState(map[string]interface{}{
				"status": "stopped",
			}))
		}
	case "restart":
		for _, svc := range m.getFilteredServices() {
			plan.AddChange(valueobject.NewChange(
				valueobject.ChangeTypeUpdate,
				"service",
				svc.Name,
			).WithActions("restart").WithRemoteExists(true).WithOldState(map[string]interface{}{
				"server": svc.Server,
				"status": "running",
			}).WithNewState(map[string]interface{}{
				"status": "restarted",
			}))
		}
	case "cleanup":
		filteredServers := m.getFilteredServersForCleanup()
		serverSet := make(map[string]bool)
		for _, s := range filteredServers {
			serverSet[s] = true
		}
		itemIndex := 0
		for _, result := range m.Cleanup.CleanupResults {
			if !serverSet[result.ServerName] {
				itemIndex += len(result.OrphanContainers) + len(result.OrphanDirs)
				continue
			}
			for _, container := range result.OrphanContainers {
				if m.Cleanup.CleanupSelected[itemIndex] {
					plan.AddChange(valueobject.NewChange(
						valueobject.ChangeTypeDelete,
						"container",
						container,
					).WithActions("cleanup").WithRemoteExists(true).WithOldState(map[string]interface{}{
						"server": result.ServerName,
					}))
				}
				itemIndex++
			}
			for _, dir := range result.OrphanDirs {
				if m.Cleanup.CleanupSelected[itemIndex] {
					plan.AddChange(valueobject.NewChange(
						valueobject.ChangeTypeDelete,
						"directory",
						dir,
					).WithActions("cleanup").WithRemoteExists(true).WithOldState(map[string]interface{}{
						"server": result.ServerName,
					}))
				}
				itemIndex++
			}
		}
	}

	m.Action.PlanResult = plan
	m.Action.ApplyTotal = len(plan.Changes())
	if m.Action.ApplyTotal == 0 {
		m.Action.ApplyTotal = 1
	}
}
