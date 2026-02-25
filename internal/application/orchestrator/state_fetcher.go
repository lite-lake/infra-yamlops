package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/infrastructure/logger"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

type StateFetcher struct {
	env       string
	configDir string
}

func NewStateFetcher(env, configDir string) *StateFetcher {
	return &StateFetcher{
		env:       env,
		configDir: configDir,
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

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			logger.Warn("failed to create SSH client", "server", srv.Name, "error", err)
			continue
		}

		func() {
			defer client.Close()
			f.fetchServerServicesState(client, srv.Name, cfg, state)
		}()
	}

	return state
}

func (f *StateFetcher) fetchServerServicesState(client *ssh.Client, serverName string, cfg *entity.Config, state *repository.DeploymentState) {
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
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, f.env, svc.Name))
		key := fmt.Sprintf(constants.ServicePrefixFormat, f.env, svc.Name)

		exists := deployedServices[key]
		if !exists {
			checkStdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
			if err != nil {
				logger.Debug("failed to check remote directory", "server", serverName, "service", svc.Name, "dir", remoteDir, "error", err)
			}
			exists = strings.TrimSpace(checkStdout) == "exists"
		}

		if exists {
			composePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
			remoteContent, _, err := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo ''", composePath))
			if err != nil {
				logger.Debug("failed to read remote compose file", "server", serverName, "service", svc.Name, "path", composePath, "error", err)
			}
			remoteHash := hashString(strings.TrimSpace(remoteContent))

			localComposePath := fmt.Sprintf("%s/deployments/%s/%s.compose.yaml", f.configDir, serverName, svc.Name)
			localContent, err := readFileContent(localComposePath)
			if err != nil {
				logger.Debug("failed to read local compose file", "server", serverName, "service", svc.Name, "path", localComposePath, "error", err)
			}
			localHash := hashString(strings.TrimSpace(localContent))

			if remoteHash != "" && localHash != "" && remoteHash == localHash {
				state.Services[svc.Name] = &entity.BizService{
					Name:     svc.Name,
					Server:   svc.Server,
					Image:    svc.Image,
					Ports:    svc.Ports,
					Env:      svc.Env,
					Gateways: svc.Gateways,
				}
			} else {
				state.Services[svc.Name] = &entity.BizService{
					Name:   svc.Name,
					Server: svc.Server,
				}
			}
		}
	}

	for _, infra := range cfg.InfraServices {
		if infra.Server != serverName {
			continue
		}
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, f.env, infra.Name))
		key := fmt.Sprintf(constants.ServicePrefixFormat, f.env, infra.Name)

		exists := deployedServices[key]
		if !exists {
			checkStdout, _, err := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
			if err != nil {
				logger.Debug("failed to check remote directory", "server", serverName, "service", infra.Name, "dir", remoteDir, "error", err)
			}
			exists = strings.TrimSpace(checkStdout) == "exists"
		}

		if exists {
			composePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
			remoteContent, _, err := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo ''", composePath))
			if err != nil {
				logger.Debug("failed to read remote compose file", "server", serverName, "service", infra.Name, "path", composePath, "error", err)
			}
			remoteHash := hashString(strings.TrimSpace(remoteContent))

			localComposePath := fmt.Sprintf("%s/deployments/%s/%s.compose.yaml", f.configDir, serverName, infra.Name)
			localContent, err := readFileContent(localComposePath)
			if err != nil {
				logger.Debug("failed to read local compose file", "server", serverName, "service", infra.Name, "path", localComposePath, "error", err)
			}
			localHash := hashString(strings.TrimSpace(localContent))

			if remoteHash != "" && localHash != "" && remoteHash == localHash {
				state.InfraServices[infra.Name] = &entity.InfraService{
					Name:            infra.Name,
					Server:          infra.Server,
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
					Name:   infra.Name,
					Server: infra.Server,
				}
			}
		}
	}
}
