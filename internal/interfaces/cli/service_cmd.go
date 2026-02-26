package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/constants"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

type ServiceFilters struct {
	Server string
	Infra  string
	Biz    string
}

func newServiceCommand(ctx *Context) *cobra.Command {
	var filters ServiceFilters
	var autoApprove bool

	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage services (deploy, stop, restart, cleanup)",
		Long:  "Manage services: deploy new or updated services, stop, restart, and cleanup orphan resources.",
	}

	serviceDeployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy services",
		Long:  "Deploy services. If services already exist, they will be restarted with updated files.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceDeploy(ctx, filters, autoApprove)
		},
	}

	serviceStopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop services",
		Long:  "Stop running services. Data is preserved.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceStop(ctx, filters, autoApprove)
		},
	}

	serviceRestartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart services",
		Long:  "Restart services without pulling images or syncing files.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceRestart(ctx, filters, autoApprove)
		},
	}

	serviceCleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup orphan resources",
		Long:  "Scan and remove orphan containers and directories that are not in the configuration.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceCleanup(ctx, filters, autoApprove)
		},
	}

	serviceCmd.PersistentFlags().StringVarP(&filters.Server, "server", "s", "", "Filter by server")
	serviceCmd.PersistentFlags().StringVarP(&filters.Infra, "infra", "i", "", "Filter by infra service")
	serviceCmd.PersistentFlags().StringVarP(&filters.Biz, "biz", "b", "", "Filter by business service")

	serviceDeployCmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto approve without confirmation")
	serviceStopCmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto approve without confirmation")
	serviceRestartCmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto approve without confirmation")
	serviceCleanupCmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Auto approve without confirmation")

	serviceCmd.AddCommand(serviceDeployCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceCleanupCmd)

	return serviceCmd
}

func runServiceDeploy(ctx *Context, filters ServiceFilters, autoApprove bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)

	planScope := valueobject.NewScope().
		WithServer(filters.Server).
		WithService(filters.Biz).
		WithInfraServices(strings.Split(filters.Infra, ","))

	executionPlan, cfg, err := wf.Plan(context.Background(), "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Plan error: %v\n", err)
		os.Exit(1)
	}

	var targetChanges []*valueobject.Change
	for _, ch := range executionPlan.Changes() {
		if ch.Entity() != "service" && ch.Entity() != "infra_service" {
			continue
		}
		if filters.Server != "" {
			serverName := extractServerFromChange(ch, cfg)
			if serverName != filters.Server {
				continue
			}
		}
		if filters.Biz != "" && ch.Entity() != "service" {
			continue
		}
		if filters.Infra != "" && ch.Entity() != "infra_service" {
			continue
		}
		targetChanges = append(targetChanges, ch)
	}

	if len(targetChanges) == 0 {
		fmt.Println("No service changes to deploy.")
		return
	}

	fmt.Println("Deploy Plan:")
	fmt.Println("============")
	for _, ch := range targetChanges {
		fmt.Printf("  %s %s: %s\n", changeTypeIcon(ch.Type()), ch.Entity(), ch.Name())
	}

	if !autoApprove {
		if !Confirm("Do you want to deploy these services?", false) {
			fmt.Println("Cancelled.")
			return
		}
	}

	if err := wf.GenerateDeployments(cfg, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Generate deployments error: %v\n", err)
		os.Exit(1)
	}

	executor := usecase.NewExecutor(&usecase.ExecutorConfig{
		Plan: executionPlan,
		Env:  ctx.Env,
	})
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetServerEntities(cfg.GetServerMap())
	executor.SetWorkDir(ctx.ConfigDir)

	for _, srv := range cfg.Servers {
		if filters.Server != "" && srv.Name != filters.Server {
			continue
		}
		password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving password for server %s: %v\n", srv.Name, err)
			continue
		}
		executor.RegisterServer(srv.Name, srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	}

	results := executor.Apply()

	hasError := false
	for _, result := range results {
		if result.Change.Entity() != "service" && result.Change.Entity() != "infra_service" {
			continue
		}
		if filters.Server != "" {
			serverName := extractServerFromChange(result.Change, cfg)
			if serverName != filters.Server {
				continue
			}
		}
		if result.Success {
			fmt.Printf("✓ %s: %s\n", result.Change.Entity(), result.Change.Name())
			for _, w := range result.Warnings {
				fmt.Printf("  ⚠ %s\n", w)
			}
		} else {
			fmt.Printf("✗ %s: %s - %v\n", result.Change.Entity(), result.Change.Name(), result.Error)
			hasError = true
		}
	}

	if hasError {
		os.Exit(1)
	}
}

func runServiceStop(ctx *Context, filters ServiceFilters, autoApprove bool) {
	cfg, err := loadConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Load config error: %v\n", err)
		os.Exit(1)
	}

	targetServices := collectTargetServices(cfg, filters)
	if len(targetServices) == 0 {
		fmt.Println("No services to stop.")
		return
	}

	fmt.Println("Stop Plan:")
	fmt.Println("==========")
	for _, svc := range targetServices {
		fmt.Printf("  Stop %s (%s)\n", svc.Name, svc.Server)
	}

	if !autoApprove {
		if !Confirm(fmt.Sprintf("Do you want to stop %d service(s)?", len(targetServices)), false) {
			fmt.Println("Cancelled.")
			return
		}
	}

	executeServiceOperation(ctx, cfg, targetServices, stopServiceOperation, false)
}

func runServiceRestart(ctx *Context, filters ServiceFilters, autoApprove bool) {
	cfg, err := loadConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Load config error: %v\n", err)
		os.Exit(1)
	}

	targetServices := collectTargetServices(cfg, filters)
	if len(targetServices) == 0 {
		fmt.Println("No services to restart.")
		return
	}

	fmt.Println("Restart Plan:")
	fmt.Println("=============")
	for _, svc := range targetServices {
		fmt.Printf("  Restart %s (%s)\n", svc.Name, svc.Server)
	}

	if !autoApprove {
		if !Confirm(fmt.Sprintf("Do you want to restart %d service(s)?", len(targetServices)), false) {
			fmt.Println("Cancelled.")
			return
		}
	}

	executeServiceOperation(ctx, cfg, targetServices, restartServiceOperation, false)
}

func runServiceCleanup(ctx *Context, filters ServiceFilters, autoApprove bool) {
	cfg, err := loadConfig(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Load config error: %v\n", err)
		os.Exit(1)
	}

	orphanResources, err := scanOrphanResourcesCLI(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	if len(orphanResources) == 0 {
		fmt.Println("No orphan resources found.")
		return
	}

	fmt.Println("Orphan Resources:")
	fmt.Println("==================")
	totalCount := 0
	for _, r := range orphanResources {
		fmt.Printf("  [%s]\n", r.ServerName)
		for _, c := range r.Containers {
			fmt.Printf("    container: %s\n", c)
			totalCount++
		}
		for _, d := range r.Dirs {
			fmt.Printf("    directory: %s\n", d)
			totalCount++
		}
	}

	if !autoApprove {
		if !Confirm(fmt.Sprintf("Do you want to remove %d orphan resource(s)?", totalCount), false) {
			fmt.Println("Cancelled.")
			return
		}
	}

	executeServiceCleanup(ctx, cfg, orphanResources)
}

func loadConfig(ctx *Context) (*entity.Config, error) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		return nil, err
	}
	if err := wf.ResolveSecrets(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

type targetService struct {
	Name    string
	Server  string
	IsInfra bool
}

func collectTargetServices(cfg *entity.Config, filters ServiceFilters) []targetService {
	var result []targetService

	for _, svc := range cfg.Services {
		if filters.Server != "" && svc.Server != filters.Server {
			continue
		}
		if filters.Biz != "" && svc.Name != filters.Biz {
			continue
		}
		result = append(result, targetService{
			Name:    svc.Name,
			Server:  svc.Server,
			IsInfra: false,
		})
	}

	for _, svc := range cfg.InfraServices {
		if filters.Server != "" && svc.Server != filters.Server {
			continue
		}
		if filters.Infra != "" && svc.Name != filters.Infra {
			continue
		}
		result = append(result, targetService{
			Name:    svc.Name,
			Server:  svc.Server,
			IsInfra: true,
		})
	}

	return result
}

type serviceOperationFunc func(client *ssh.Client, remoteDir string) (string, error)

var stopServiceOperation serviceOperationFunc = func(client *ssh.Client, remoteDir string) (string, error) {
	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml stop 2>&1", remoteDir)
	_, stderr, err := client.Run(cmd)
	return stderr, err
}

var restartServiceOperation serviceOperationFunc = func(client *ssh.Client, remoteDir string) (string, error) {
	cmd := fmt.Sprintf("sudo docker compose -f %s/docker-compose.yml restart 2>&1", remoteDir)
	_, stderr, err := client.Run(cmd)
	return stderr, err
}

func executeServiceOperation(ctx *Context, cfg *entity.Config, services []targetService, opFunc serviceOperationFunc, withSync bool) {
	hasError := false
	serverMap := cfg.GetServerMap()

	for _, svc := range services {
		srv, ok := serverMap[svc.Server]
		if !ok {
			fmt.Printf("✗ %s: server not found: %s\n", svc.Name, svc.Server)
			hasError = true
			continue
		}

		password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
		if err != nil {
			fmt.Printf("✗ %s: cannot resolve password: %v\n", svc.Name, err)
			hasError = true
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			fmt.Printf("✗ %s: connection failed: %v\n", svc.Name, err)
			hasError = true
			continue
		}

		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceDirPattern, ctx.Env, svc.Name))
		stderr, err := opFunc(client, remoteDir)

		client.Close()

		if err != nil {
			fmt.Printf("✗ %s: %s\n", svc.Name, stderr)
			hasError = true
		} else {
			fmt.Printf("✓ %s\n", svc.Name)
		}
	}

	if hasError {
		os.Exit(1)
	}
}

type orphanResource struct {
	ServerName string
	Containers []string
	Dirs       []string
}

func scanOrphanResourcesCLI(ctx *Context, cfg *entity.Config) ([]orphanResource, error) {
	var results []orphanResource
	secrets := cfg.GetSecretsMap()
	serviceMap := cfg.GetServiceMap()
	infraServiceMap := cfg.GetInfraServiceMap()
	envPrefix := "yo-" + ctx.Env + "-"

	for _, srv := range cfg.Servers {
		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("[%s] cannot resolve password: %w", srv.Name, err)
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			return nil, fmt.Errorf("[%s] connection failed: %w", srv.Name, err)
		}

		containerStdout, _, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("[%s] failed to list containers: %w", srv.Name, err)
		}

		dirStdout, _, err := client.Run("sudo ls -1 " + constants.RemoteBaseDir + " 2>/dev/null || true")
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("[%s] failed to list directories: %w", srv.Name, err)
		}

		client.Close()

		r := orphanResource{ServerName: srv.Name}

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

			if !strings.HasPrefix(container.Name, envPrefix) {
				continue
			}
			serviceName := strings.TrimPrefix(container.Name, envPrefix)
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				r.Containers = append(r.Containers, container.Name)
			}
		}

		for _, line := range strings.Split(strings.TrimSpace(dirStdout), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, envPrefix) {
				continue
			}
			serviceName := strings.TrimPrefix(line, envPrefix)
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				r.Dirs = append(r.Dirs, line)
			}
		}

		if len(r.Containers) > 0 || len(r.Dirs) > 0 {
			results = append(results, r)
		}
	}

	return results, nil
}

func executeServiceCleanup(ctx *Context, cfg *entity.Config, resources []orphanResource) {
	hasError := false
	secrets := cfg.GetSecretsMap()

	for _, r := range resources {
		srv, ok := cfg.GetServerMap()[r.ServerName]
		if !ok {
			continue
		}

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			for _, c := range r.Containers {
				fmt.Printf("✗ %s: cannot resolve password: %v\n", c, err)
			}
			for _, d := range r.Dirs {
				fmt.Printf("✗ %s: cannot resolve password: %v\n", d, err)
			}
			hasError = true
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			for _, c := range r.Containers {
				fmt.Printf("✗ %s: connection failed: %v\n", c, err)
			}
			for _, d := range r.Dirs {
				fmt.Printf("✗ %s: connection failed: %v\n", d, err)
			}
			hasError = true
			continue
		}

		for _, c := range r.Containers {
			cmd := fmt.Sprintf("sudo docker rm -f %s", c)
			_, stderr, err := client.Run(cmd)
			if err != nil {
				fmt.Printf("✗ %s: %s\n", c, stderr)
				hasError = true
			} else {
				fmt.Printf("✓ removed container: %s\n", c)
			}
		}

		for _, d := range r.Dirs {
			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, d)
			cmd := fmt.Sprintf("sudo rm -rf %s", remoteDir)
			_, stderr, err := client.Run(cmd)
			if err != nil {
				fmt.Printf("✗ %s: %s\n", d, stderr)
				hasError = true
			} else {
				fmt.Printf("✓ removed directory: %s\n", d)
			}
		}

		client.Close()
	}

	if hasError {
		os.Exit(1)
	}
}

func extractServerFromChange(change *valueobject.Change, cfg *entity.Config) string {
	switch change.Entity() {
	case "service":
		if svc, ok := change.NewState().(*entity.BizService); ok {
			return svc.Server
		}
		if change.OldState() != nil {
			if svc, ok := change.OldState().(*entity.BizService); ok {
				return svc.Server
			}
		}
	case "infra_service":
		if infra, ok := change.NewState().(*entity.InfraService); ok {
			return infra.Server
		}
		if change.OldState() != nil {
			if infra, ok := change.OldState().(*entity.InfraService); ok {
				return infra.Server
			}
		}
	}
	return ""
}

func changeTypeIcon(changeType valueobject.ChangeType) string {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "+"
	case valueobject.ChangeTypeUpdate:
		return "~"
	case valueobject.ChangeTypeDelete:
		return "-"
	default:
		return " "
	}
}
