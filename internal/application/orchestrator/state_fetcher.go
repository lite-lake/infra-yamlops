package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/handler"
	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

type StateFetcher struct {
	env       string
	configDir string
	sshPool   *usecase.SSHPool
}

func NewStateFetcher(env, configDir string) *StateFetcher {
	return NewStateFetcherWithPool(env, configDir, usecase.NewSSHPool())
}

func NewStateFetcherWithPool(env, configDir string, pool *usecase.SSHPool) *StateFetcher {
	return &StateFetcher{
		env:       env,
		configDir: configDir,
		sshPool:   pool,
	}
}

func (f *StateFetcher) Fetch(ctx context.Context, cfg *entity.Config) *repository.DeploymentState {
	state := repository.NewDeploymentState()

	for _, zone := range cfg.Zones {
		state.Zones[zone.Name] = &zone
	}

	secrets := cfg.GetSecretsMap()
	for _, srv := range cfg.Servers {
		state.Servers[srv.Name] = &srv

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			logger.Warn("failed to resolve SSH password", "server", srv.Name, "error", err)
			continue
		}

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}

		info := &handler.ServerInfo{
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

		f.fetchServerServicesState(client, srv.Name, cfg, state)
	}

	f.sshPool.CloseAll()
	return state
}

func (f *StateFetcher) fetchServerServicesState(client contract.SSHClient, serverName string, cfg *entity.Config, state *repository.DeploymentState) {
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
			}
		})
	}

	for _, infra := range cfg.InfraServices {
		if infra.Server != serverName {
			continue
		}
		f.processService(client, serverName, infra.Name, deployedServices, func(exists bool, remoteHash, localHash string) {
			if exists {
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
						GatewayConfig:   infra.GatewayConfig,
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
			}
		})
	}
}

type serviceProcessor func(exists bool, remoteHash, localHash string)

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
