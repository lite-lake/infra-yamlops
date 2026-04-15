package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

// StateFetcher 负责从远程服务器获取部署状态
type StateFetcher struct {
	env       string
	configDir string
	sshPool   *usecase.SSHPool
}

// NewStateFetcher 创建状态获取器
func NewStateFetcher(env, configDir string) *StateFetcher {
	return NewStateFetcherWithPool(env, configDir, usecase.NewSSHPool())
}

// NewStateFetcherWithPool 使用指定的 SSH 池创建状态获取器
func NewStateFetcherWithPool(env, configDir string, pool *usecase.SSHPool) *StateFetcher {
	return &StateFetcher{
		env:       env,
		configDir: configDir,
		sshPool:   pool,
	}
}

// Fetch 从所有服务器并发获取部署状态
func (f *StateFetcher) Fetch(ctx context.Context, cfg *entity.Config) *repository.DeploymentState {
	state := repository.NewDeploymentState()

	for _, zone := range cfg.Zones {
		state.Zones[zone.Name] = &zone
	}

	secrets := cfg.GetSecretsMap()
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, srv := range cfg.Servers {
		mu.Lock()
		state.Servers[srv.Name] = &srv
		mu.Unlock()

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			logger.Warn("failed to resolve SSH password", "server", srv.Name, "error", err)
			continue
		}

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}

		info := &usecase.ServerInfo{
			Host:                  srv.SSH.Host,
			Port:                  srv.SSH.Port,
			User:                  srv.SSH.User,
			Password:              password,
			StrictHostKeyChecking: strictHostKeyChecking,
		}

		client, err := f.sshPool.Get(info)
		if err != nil {
			logger.Warn("failed to get SSH client from pool", "server", srv.Name, "error", err)
			continue
		}

		wg.Add(1)
		go func(serverName string, sshClient contract.SSHClient, config *entity.Config) {
			defer wg.Done()
			f.fetchServerServicesState(sshClient, serverName, config, state, &mu)
		}(srv.Name, client, cfg)
	}

	wg.Wait()
	f.sshPool.CloseAll()
	return state
}

// fetchServerServicesState 获取单个服务器上的服务状态
func (f *StateFetcher) fetchServerServicesState(client contract.SSHClient, serverName string, cfg *entity.Config, state *repository.DeploymentState, mu *sync.Mutex) {
	stdout, _, err := client.Run("sudo docker compose ls -a --format json 2>/dev/null || sudo docker compose ls -a --format json")
	if err != nil {
		logger.Warn("failed to list docker compose projects", "server", serverName, "error", err)
		return
	}
	if stdout == "" {
		return
	}

	type composeProject struct {
		Name string `json:"Name"`
	}
	var projects []composeProject
	if err := json.Unmarshal([]byte(stdout), &projects); err != nil {
		logger.Debug("failed to parse docker compose output as array, trying line-by-line", "server", serverName, "error", err)
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var proj composeProject
			if err := json.Unmarshal([]byte(line), &proj); err != nil {
				logger.Warn("failed to parse docker compose project line", "server", serverName, "line", line, "error", err)
				continue
			}
			if proj.Name != "" {
				projects = append(projects, proj)
			}
		}
	}

	deployedServices := make(map[string]bool)
	for _, proj := range projects {
		deployedServices[proj.Name] = true
	}

	for _, svc := range cfg.Services {
		if svc.Server != serverName {
			continue
		}
		f.processService(client, serverName, svc.Name, deployedServices, func(exists bool, remoteHash, localHash string) {
			if exists {
				mu.Lock()
				if remoteHash != "" && localHash != "" && remoteHash == localHash {
					state.Services[svc.Name] = &entity.BizService{
						ServiceBase: entity.ServiceBase{
							Server: svc.ServiceBase.Server,
						},
						Name:     svc.Name,
						Image:    svc.Image,
						Ports:    svc.Ports,
						Env:      svc.Env,
						Gateways: svc.Gateways,
					}
				} else {
					state.Services[svc.Name] = &entity.BizService{
						ServiceBase: entity.ServiceBase{
							Server: svc.ServiceBase.Server,
						},
						Name: svc.Name,
					}
				}
				mu.Unlock()
			}
		})
	}

	for _, infra := range cfg.InfraServices {
		if infra.Server != serverName {
			continue
		}
		f.processService(client, serverName, infra.Name, deployedServices, func(exists bool, remoteHash, localHash string) {
			if exists {
				mu.Lock()
				if remoteHash != "" && localHash != "" && remoteHash == localHash {
					state.InfraServices[infra.Name] = &entity.InfraService{
						ServiceBase: entity.ServiceBase{
							Server: infra.ServiceBase.Server,
						},
						Name:            infra.Name,
						Type:            infra.Type,
						Image:           infra.Image,
						GatewayLogLevel: infra.GatewayLogLevel,
						GatewayPorts:    infra.GatewayPorts,
						GatewaySSL:      infra.GatewaySSL,
						GatewayWAF:      infra.GatewayWAF,
						SSLConfig:       infra.SSLConfig,
					}
				} else {
					state.InfraServices[infra.Name] = &entity.InfraService{
						ServiceBase: entity.ServiceBase{
							Server: infra.ServiceBase.Server,
						},
						Name: infra.Name,
					}
				}
				mu.Unlock()
			}
		})
	}
}

// serviceProcessor 服务状态处理函数类型
type serviceProcessor func(exists bool, remoteHash, localHash string)

// processService 处理单个服务的状态
func (f *StateFetcher) processService(client contract.SSHClient, serverName, serviceName string, deployedServices map[string]bool, processor serviceProcessor) {
	remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, f.env, serviceName))
	key := fmt.Sprintf(constants.ServiceNameFormat, f.env, serviceName)

	exists := deployedServices[key]
	if !exists {
		checkStdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
		if err != nil {
			logger.Debug("failed to check remote directory", "server", serverName, "service", serviceName, "dir", remoteDir, "error", err)
		}
		exists = strings.TrimSpace(checkStdout) == "exists"
	}

	if exists {
		composePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
		remoteContent, _, err := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo ''", composePath))
		if err != nil {
			logger.Debug("failed to read remote compose file", "server", serverName, "service", serviceName, "path", composePath, "error", err)
		}
		remoteHash := hashString(strings.TrimSpace(remoteContent))

		localComposePath := fmt.Sprintf("%s/deployments/%s/%s.compose.yaml", f.configDir, serverName, serviceName)
		localContent, err := readFileContent(localComposePath)
		if err != nil {
			logger.Debug("failed to read local compose file", "server", serverName, "service", serviceName, "path", localComposePath, "error", err)
		}
		localHash := hashString(strings.TrimSpace(localContent))

		processor(exists, remoteHash, localHash)
	} else {
		processor(exists, "", "")
	}
}
