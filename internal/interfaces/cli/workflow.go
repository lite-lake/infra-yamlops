package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/litelake/yamlops/internal/config"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/repository"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/litelake/yamlops/internal/ssh"
)

type Workflow struct {
	env       string
	configDir string
	loader    repository.ConfigLoader
}

func NewWorkflow(env, configDir string) *Workflow {
	return &Workflow{
		env:       env,
		configDir: configDir,
		loader:    persistence.NewConfigLoader(configDir),
	}
}

func (w *Workflow) Env() string { return w.env }

func (w *Workflow) LoadConfig(ctx context.Context) (*entity.Config, error) {
	cfg, err := w.loader.Load(ctx, w.env)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error) {
	cfg, err := w.LoadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.loader.Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func (w *Workflow) ResolveSecrets(cfg *entity.Config) error {
	secrets := make([]*entity.Secret, len(cfg.Secrets))
	for i := range cfg.Secrets {
		secrets[i] = &cfg.Secrets[i]
	}
	resolver := config.NewSecretResolver(secrets)
	return resolver.ResolveAll(cfg)
}

func (w *Workflow) CreatePlanner(cfg *entity.Config, outputDir string) *plan.Planner {
	planner := plan.NewPlanner(cfg, w.env)
	if outputDir != "" {
		planner.SetOutputDir(outputDir)
	}
	return planner
}

func (w *Workflow) Plan(ctx context.Context, outputDir string, scope *valueobject.Scope) (*valueobject.Plan, *entity.Config, error) {
	cfg, err := w.LoadAndValidate(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := w.ResolveSecrets(cfg); err != nil {
		return nil, nil, fmt.Errorf("resolve secrets: %w", err)
	}

	if err := w.GenerateDeployments(cfg, outputDir); err != nil {
		return nil, nil, fmt.Errorf("generate deployments: %w", err)
	}

	remoteState := w.FetchRemoteState(ctx, cfg)
	planner := plan.NewPlannerWithState(cfg, remoteState, w.env)
	if outputDir != "" {
		planner.SetOutputDir(outputDir)
	}
	p, err := planner.Plan(scope)
	if err != nil {
		return nil, nil, fmt.Errorf("plan: %w", err)
	}
	return p, cfg, nil
}

func (w *Workflow) GenerateDeployments(cfg *entity.Config, outputDir string) error {
	planner := w.CreatePlanner(cfg, outputDir)
	return planner.GenerateDeployments()
}

func (w *Workflow) FetchRemoteState(ctx context.Context, cfg *entity.Config) *repository.DeploymentState {
	state := repository.NewDeploymentState()

	for _, zone := range cfg.Zones {
		state.Zones[zone.Name] = &zone
	}

	secrets := cfg.GetSecretsMap()
	for _, srv := range cfg.Servers {
		state.Servers[srv.Name] = &srv

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			continue
		}

		w.fetchServerServicesState(client, srv.Name, cfg, state)

		client.Close()
	}

	return state
}

func (w *Workflow) fetchServerServicesState(client *ssh.Client, serverName string, cfg *entity.Config, state *repository.DeploymentState) {
	stdout, _, err := client.Run("sudo docker compose ls -a --format json 2>/dev/null || sudo docker compose ls -a --format json")
	if err != nil || stdout == "" {
		return
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

	deployedServices := make(map[string]bool)
	for _, proj := range projects {
		deployedServices[proj.Name] = true
	}

	for _, svc := range cfg.Services {
		if svc.Server != serverName {
			continue
		}
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, w.env, svc.Name))
		key := fmt.Sprintf(constants.ServicePrefixFormat, w.env, svc.Name)

		exists := deployedServices[key]
		if !exists {
			checkStdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
			exists = strings.TrimSpace(checkStdout) == "exists"
		}

		if exists {
			composePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
			remoteContent, _, _ := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo ''", composePath))
			remoteHash := hashString(strings.TrimSpace(remoteContent))

			localComposePath := fmt.Sprintf("%s/deployments/%s/%s.compose.yaml", w.configDir, serverName, svc.Name)
			localContent, _ := readFileContent(localComposePath)
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
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, w.env, infra.Name))
		key := fmt.Sprintf(constants.ServicePrefixFormat, w.env, infra.Name)

		exists := deployedServices[key]
		if !exists {
			checkStdout, _, _ := client.Run(fmt.Sprintf("sudo test -d %s && echo exists || echo notfound", remoteDir))
			exists = strings.TrimSpace(checkStdout) == "exists"
		}

		if exists {
			composePath := fmt.Sprintf("%s/docker-compose.yml", remoteDir)
			remoteContent, _, _ := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo ''", composePath))
			remoteHash := hashString(strings.TrimSpace(remoteContent))

			localComposePath := fmt.Sprintf("%s/deployments/%s/%s.compose.yaml", w.configDir, serverName, infra.Name)
			localContent, _ := readFileContent(localComposePath)
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

func hashString(s string) string {
	if s == "" {
		return ""
	}
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
