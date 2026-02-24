package handler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/litelake/yamlops/internal/constants"
	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/network"
)

type NetworkGetter interface {
	GetNetworks() []string
}

func GetRequiredNetworks(change *valueobject.Change, deps DepsProvider, serverName string) ([]entity.ServerNetwork, error) {
	server, ok := deps.Server(serverName)
	if !ok {
		return nil, fmt.Errorf("%w: %s", domainerr.ErrServerNotRegistered, serverName)
	}

	var serviceNetworks []string
	if change.NewState != nil {
		if getter, ok := change.NewState.(NetworkGetter); ok {
			serviceNetworks = getter.GetNetworks()
		} else if svc, ok := change.NewState.(map[string]interface{}); ok {
			if nets, ok := svc["networks"].([]interface{}); ok {
				for _, n := range nets {
					if name, ok := n.(string); ok {
						serviceNetworks = append(serviceNetworks, name)
					}
				}
			}
		}
	}

	if len(serviceNetworks) == 0 {
		if len(server.Networks) > 0 {
			return server.Networks[:1], nil
		}
		return []entity.ServerNetwork{{Name: fmt.Sprintf("yamlops-%s", deps.Env()), Type: entity.NetworkTypeBridge}}, nil
	}

	var requiredNetworks []entity.ServerNetwork
	for _, netName := range serviceNetworks {
		if server.HasNetwork(netName) {
			requiredNetworks = append(requiredNetworks, *server.GetNetwork(netName))
		} else {
			requiredNetworks = append(requiredNetworks, entity.ServerNetwork{Name: netName, Type: entity.NetworkTypeBridge})
		}
	}
	return requiredNetworks, nil
}

func EnsureNetworks(client SSHClient, networks []entity.ServerNetwork) error {
	if len(networks) == 0 {
		return nil
	}
	netMgr := network.NewManager(client)
	for _, netSpec := range networks {
		if err := netMgr.Ensure(&netSpec); err != nil {
			return fmt.Errorf("ensure network %s: %w", netSpec.Name, err)
		}
	}
	return nil
}

func DeleteServiceRemote(change *valueobject.Change, client SSHClient, remoteDir string) (*Result, error) {
	result := &Result{Change: change, Success: false}

	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml down -v 2>/dev/null || true", remoteDir)
	stdout, stderr, _ := client.Run(cmd)

	rmCmd := fmt.Sprintf("sudo rm -rf %s", remoteDir)
	stdout2, stderr2, err := client.Run(rmCmd)
	if err != nil {
		result.Error = fmt.Errorf("%w: %w, stderr: %s", domainerr.ErrDirectoryRemoveFailed, err, stderr2)
		result.Output = stdout + "\n" + stderr + "\n" + stdout2 + "\n" + stderr2
		return result, nil
	}

	result.Success = true
	result.Output = stdout + "\n" + stdout2
	return result, nil
}

func GetComposeFilePath(ch *valueobject.Change, deps DepsProvider) string {
	serverName := ExtractServerFromChange(ch)
	if serverName == "" {
		return ""
	}
	return filepath.Join(deps.WorkDir(), "deployments", serverName, ch.Name+".compose.yaml")
}

type DeployComposeConfig struct {
	RemoteDir      string
	ComposeFile    string
	Env            string
	ServiceName    string
	RestartAfterUp bool
}

func DeployComposeFile(client SSHClient, cfg *DeployComposeConfig, result *Result) bool {
	if cfg.ComposeFile == "" {
		return true
	}

	if _, err := os.Stat(cfg.ComposeFile); err != nil {
		return true
	}

	content, err := os.ReadFile(cfg.ComposeFile)
	if err != nil {
		result.Error = fmt.Errorf("%w: compose file %s: %w", domainerr.ErrFileReadFailed, cfg.ComposeFile, err)
		return false
	}
	if err := SyncContent(client, string(content), cfg.RemoteDir+"/docker-compose.yml"); err != nil {
		result.Error = fmt.Errorf("%w: compose file %s to %s/docker-compose.yml: %w", domainerr.ErrComposeSyncFailed, cfg.ComposeFile, cfg.RemoteDir, err)
		return false
	}

	pullCmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml pull", cfg.RemoteDir)
	_, pullStderr, pullErr := client.Run(pullCmd)
	if pullErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("image pull failed: %s", pullStderr))
	}

	var cmd string
	if cfg.RestartAfterUp {
		containerName := fmt.Sprintf(constants.ServicePrefixFormat, cfg.Env, cfg.ServiceName)
		cmd = fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d && sudo docker restart %s", cfg.RemoteDir, containerName)
	} else {
		cmd = fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml up -d", cfg.RemoteDir)
	}

	stdout, stderr, err := client.Run(cmd)
	if err != nil {
		result.Error = fmt.Errorf("%w: in %s: %w, stderr: %s", domainerr.ErrDockerComposeFailed, cfg.RemoteDir, err, stderr)
		result.Output = stdout + "\n" + stderr
		return false
	}
	result.Output = stdout
	return true
}

func GetRemoteDir(deps DepsProvider, serviceName string) string {
	return fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, deps.Env(), serviceName))
}

func EnsureRemoteDir(client SSHClient, remoteDir string) error {
	return client.MkdirAllSudoWithPerm(remoteDir, "755")
}

type ServiceDeployContext struct {
	ServerName string
	Client     SSHClient
	RemoteDir  string
}

type DeployServiceOptions struct {
	PreDeployHook  func(result *Result) error
	PostDeployHook func(result *Result) error
	RestartAfterUp bool
}

func PrepareServiceDeploy(change *valueobject.Change, deps DepsProvider) (*ServiceDeployContext, *Result) {
	result := &Result{Change: change, Success: false}

	serverName := ExtractServerFromChange(change)
	if serverName == "" {
		result.Error = fmt.Errorf("cannot determine server for change %s", change.Name)
		return nil, result
	}

	client, err := deps.SSHClient(serverName)
	if err != nil {
		result.Error = err
		return nil, result
	}

	remoteDir := GetRemoteDir(deps, change.Name)

	return &ServiceDeployContext{
		ServerName: serverName,
		Client:     client,
		RemoteDir:  remoteDir,
	}, nil
}

func ExecuteServiceDeploy(change *valueobject.Change, ctx *ServiceDeployContext, deps DepsProvider, opts DeployServiceOptions) (*Result, error) {
	result := &Result{Change: change, Success: false}

	if opts.PreDeployHook != nil {
		if err := opts.PreDeployHook(result); err != nil {
			return result, nil
		}
	}

	requiredNetworks, err := GetRequiredNetworks(change, deps, ctx.ServerName)
	if err != nil {
		result.Error = err
		return result, nil
	}

	if err := EnsureNetworks(ctx.Client, requiredNetworks); err != nil {
		result.Error = fmt.Errorf("ensuring networks on server %s: %w", ctx.ServerName, err)
		return result, nil
	}

	if err := EnsureRemoteDir(ctx.Client, ctx.RemoteDir); err != nil {
		result.Error = fmt.Errorf("%w: %s: %w", domainerr.ErrDirectoryCreateFailed, ctx.RemoteDir, err)
		return result, nil
	}

	if opts.PostDeployHook != nil {
		if err := opts.PostDeployHook(result); err != nil {
			return result, nil
		}
	}

	composeFile := GetComposeFilePath(change, deps)
	if !DeployComposeFile(ctx.Client, &DeployComposeConfig{
		RemoteDir:      ctx.RemoteDir,
		ComposeFile:    composeFile,
		Env:            deps.Env(),
		ServiceName:    change.Name,
		RestartAfterUp: opts.RestartAfterUp,
	}, result) {
		return result, nil
	}

	result.Success = true
	return result, nil
}
