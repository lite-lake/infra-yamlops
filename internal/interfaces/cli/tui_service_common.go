package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
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

		client, err := ssh.NewClient(srv.sshHost, srv.sshPort, srv.sshUser, password)
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
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, env, infra.name))
			key := fmt.Sprintf(constants.ServicePrefixFormat, env, infra.name)
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
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, env, svc.name))
			key := fmt.Sprintf(constants.ServicePrefixFormat, env, svc.name)
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
		key := fmt.Sprintf(constants.ServicePrefixFormat, env, node.Name)
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
		result = append(result, serverWithSSH{
			name:        srv.Name,
			sshHost:     srv.SSH.Host,
			sshPort:     srv.SSH.Port,
			sshUser:     srv.SSH.User,
			sshPassword: &srv.SSH.Password,
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
