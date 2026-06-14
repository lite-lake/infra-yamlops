package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

type ServiceCmdFilters struct {
	Type    string
	Zone    string
	Server  string
	Service string
}

func buildServiceScope(filters ServiceCmdFilters, forceDeploy bool) (*valueobject.Scope, error) {
	scope := valueobject.NewScope()
	if filters.Zone != "" {
		scope = scope.WithZones(splitAndTrim(filters.Zone, ","))
	}
	if filters.Server != "" {
		scope = scope.WithServers(splitAndTrim(filters.Server, ","))
	}
	if filters.Service != "" {
		names := splitAndTrim(filters.Service, ",")
		scope = scope.WithBizServices(names).WithInfraServices(names)
	}
	serviceTypes, err := parseServiceTypes(filters.Type)
	if err != nil {
		return nil, err
	}
	if len(serviceTypes) > 0 {
		scope = scope.WithServiceTypes(serviceTypes)
	}
	if forceDeploy {
		scope = scope.WithForceDeploy(true)
	}
	return scope, nil
}

func newServiceCommand(ctx *Context) *cobra.Command {
	var filters ServiceCmdFilters

	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Service management commands",
		Long: `Service management commands.

Commands:
  show      List services (use --detail for detailed view)
  validate  Validate service configuration
  deploy    Deploy services (Plan -> Confirm -> Execute)
  stop      Stop services (Plan -> Confirm -> Execute)
  restart   Restart services (Plan -> Confirm -> Execute)
  cleanup   Clean up orphan resources (Plan -> Confirm -> Execute)

Flags:
  --type string         Service type filter: biz, infra, biz,infra (default: all)
  --service string      Service name filter (comma-separated)
  --detail              Show detailed information (for show command)
  --concurrency int     Concurrency for server operations (default: 5)

Examples:
  yamlops cli service show -e prod --type biz
  yamlops cli service deploy -e prod --type biz --dry-run
  yamlops cli service deploy -e prod --service my-api,my-worker
  yamlops cli service stop -e prod --type infra --yes
  yamlops cli service cleanup -e prod --dry-run`,
	}

	serviceCmd.PersistentFlags().StringVar(&filters.Type, "type", "", "Service type filter: biz, infra, biz,infra (default: all)")
	serviceCmd.PersistentFlags().StringVar(&filters.Zone, "zone", "", "Zone filter (comma-separated)")
	serviceCmd.PersistentFlags().StringVar(&filters.Server, "server", "", "Server filter (comma-separated)")
	serviceCmd.PersistentFlags().StringVar(&filters.Service, "service", "", "Service name filter (comma-separated)")

	serviceCmd.AddCommand(newServiceShowCommand(ctx, &filters))
	serviceCmd.AddCommand(newServiceValidateCommand(ctx, &filters))
	serviceCmd.AddCommand(newServiceDeployCommand(ctx, &filters))
	serviceCmd.AddCommand(newServiceStopCommand(ctx, &filters))
	serviceCmd.AddCommand(newServiceRestartCommand(ctx, &filters))
	serviceCmd.AddCommand(newServiceCleanupCommand(ctx, &filters))

	return serviceCmd
}

func newServiceDeployCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var dryRun, yes, force bool
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy services",
		Long: `Deploy services using unified execution mode: Plan -> Confirm -> Execute.

Plan stage generates the deployment plan. --dry-run stops after Plan.
--yes skips the Confirm stage and executes all changes immediately.
--force generates a deployment plan even when configuration has no changes.`,
		Run: func(cmd *cobra.Command, args []string) {
			runServiceDeployUnified(ctx, *filters, dryRun, yes, force, ctx.Concurrency)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	cmd.Flags().BoolVar(&force, "force", false, "Force execution even without changes")
	return cmd
}

func newServiceStopCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop services",
		Long: `Stop running services using unified execution mode: Plan -> Confirm -> Execute.

Plan stage identifies running services to stop. --dry-run stops after Plan.
--yes skips the Confirm stage and executes all changes immediately.`,
		Run: func(cmd *cobra.Command, args []string) {
			runServiceStopUnified(ctx, *filters, dryRun, yes, ctx.Concurrency)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	return cmd
}

func newServiceRestartCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart services",
		Long: `Restart services using unified execution mode: Plan -> Confirm -> Execute.

Plan stage identifies services to restart. --dry-run stops after Plan.
--yes skips the Confirm stage and executes all changes immediately.`,
		Run: func(cmd *cobra.Command, args []string) {
			runServiceRestartUnified(ctx, *filters, dryRun, yes, ctx.Concurrency)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	return cmd
}

func newServiceCleanupCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up orphan resources",
		Long: `Scan and remove orphan containers and directories using unified execution mode:
Plan -> Confirm -> Execute.

Plan stage scans all servers for orphan resources (containers and directories
prefixed with yo-{env}- whose service name no longer exists in config).
--dry-run stops after Plan. --yes skips the Confirm stage and executes all
changes immediately.

Note: --type flag is ignored. Orphan resource names (yo-{env}-{name}) do not
contain service type information, so Biz vs Infra cannot be distinguished.
All orphan resources are scanned regardless of --type.`,
		Run: func(cmd *cobra.Command, args []string) {
			runServiceCleanupUnified(ctx, *filters, dryRun, yes, ctx.Concurrency)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	return cmd
}

// --- Service Deploy (unified execution mode) ---

func runServiceDeployUnified(ctx *Context, filters ServiceCmdFilters, dryRun, yes, force bool, concurrency int) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	scope, err := buildServiceScope(filters, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	executionPlan, cfg, err := wf.Plan(context.Background(), "", scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var serviceChanges []*valueobject.Change
	for _, ch := range executionPlan.Changes() {
		if ch.Entity() == "service" || ch.Entity() == "infra_service" {
			serviceChanges = append(serviceChanges, ch)
		}
	}

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("service deploy", dryRun, force),
		Env:   ctx.Env,
		Extra: planTypeAndServiceExtra(filters.Type, filters.Service),
	})

	if len(serviceChanges) == 0 {
		DisplayNoChanges(dryRun, force)
		if dryRun {
			DisplayDryRun()
		}
		return
	}

	var rows []PlanRow
	for _, ch := range serviceChanges {
		rows = append(rows, PlanRow{
			Action:  changeActionLabel(ch.Type()),
			Name:    ch.Name(),
			Server:  extractServerFromChange(ch, cfg),
			Details: formatServiceDeployDetails(ch),
		})
	}
	DisplayPlanTable4Col("ACTION", "NAME", "SERVER", "DETAILS", rows)

	created, updated, deleted := countChanges(serviceChanges)
	DisplaySummary(formatSummary(created, updated, deleted, force))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
	}

	filteredPlan := valueobject.NewPlan()
	for _, ch := range serviceChanges {
		filteredPlan.AddChange(ch)
	}

	DisplayExecuting()
	executeServicePlan(ctx, cfg, filteredPlan, scope, concurrency)
}

// --- Service Stop (unified) ---

func runServiceStopUnified(ctx *Context, filters ServiceCmdFilters, dryRun, yes bool, concurrency int) {
	cfg, err := loadConfigFromContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	targetServices, err := collectTargetServicesUnified(cfg, filters)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(targetServices) == 0 {
		DisplayInfo("No services matched for stop.")
		return
	}

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("service stop", dryRun, false),
		Env:   ctx.Env,
		Extra: planTypeAndServiceExtra(filters.Type, filters.Service),
	})

	var rows []PlanRow
	for _, svc := range targetServices {
		// NOTE: stop/restart uses collectTargetServicesUnified (config-only) rather than
		// wf.Plan() which would query remote Docker state via SSH. The Details below
		// reflect the *expected* state transition, not the actual remote container status.
		// This is acceptable because docker compose stop/restart is idempotent (§6.6.7).
		rows = append(rows, PlanRow{
			Action:  "stop",
			Name:    svc.Name,
			Server:  svc.Server,
			Details: "status: * -> stopped",
		})
	}
	DisplayPlanTable4Col("ACTION", "NAME", "SERVER", "DETAILS", rows)
	DisplaySummary(formatSummaryCount("stopped", len(targetServices)))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
	}

	DisplayExecuting()
	executeServiceOperationUnified(ctx, cfg, targetServices, stopServiceOperation, "stop", "stopped", concurrency)
}

// --- Service Restart (unified) ---

func runServiceRestartUnified(ctx *Context, filters ServiceCmdFilters, dryRun, yes bool, concurrency int) {
	cfg, err := loadConfigFromContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	targetServices, err := collectTargetServicesUnified(cfg, filters)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(targetServices) == 0 {
		DisplayInfo("No services matched for restart.")
		return
	}

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("service restart", dryRun, false),
		Env:   ctx.Env,
		Extra: planTypeAndServiceExtra(filters.Type, filters.Service),
	})

	var rows []PlanRow
	for _, svc := range targetServices {
		// NOTE: restart uses collectTargetServicesUnified (config-only) rather than
		// wf.Plan() which would query remote Docker state via SSH. The Details below
		// reflect the *expected* state transition, not the actual remote container status.
		// This is acceptable because docker compose restart is idempotent (§6.6.7).
		rows = append(rows, PlanRow{
			Action:  "restart",
			Name:    svc.Name,
			Server:  svc.Server,
			Details: "status: * -> restarted",
		})
	}
	DisplayPlanTable4Col("ACTION", "NAME", "SERVER", "DETAILS", rows)
	DisplaySummary(formatSummaryCount("restarted", len(targetServices)))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
	}

	DisplayExecuting()
	executeServiceOperationUnified(ctx, cfg, targetServices, restartServiceOperation, "restart", "restarted", concurrency)
}

// --- Service Cleanup (unified) ---

func runServiceCleanupUnified(ctx *Context, filters ServiceCmdFilters, dryRun, yes bool, concurrency int) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if filters.Type != "" {
		fmt.Fprintln(os.Stderr, "Warning: --type is ignored for cleanup: orphan resource names (containers/directories) do not contain service type info, so Biz vs Infra cannot be determined. Scanning all orphan resources.")
	}

	orphanResources, err := scanOrphanResourcesCLIUnified(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(orphanResources) == 0 {
		DisplayInfo("No orphan resources matched for cleanup.")
		if dryRun {
			DisplayDryRun()
		}
		return
	}

	totalCount := 0
	for _, r := range orphanResources {
		totalCount += len(r.Containers) + len(r.Dirs)
	}

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("service cleanup", dryRun, false),
		Env:   ctx.Env,
		Extra: planTypeAndServiceExtra(filters.Type, filters.Service),
	})

	envPrefix := "yo-" + ctx.Env + "-"
	var rows []PlanRow
	for _, r := range orphanResources {
		for _, c := range r.Containers {
			rows = append(rows, PlanRow{
				Action:  "cleanup",
				Name:    strings.TrimPrefix(c, envPrefix),
				Server:  r.ServerName,
				Details: fmt.Sprintf("container: %s", c),
			})
		}
		for _, d := range r.Dirs {
			rows = append(rows, PlanRow{
				Action:  "cleanup",
				Name:    strings.TrimPrefix(d, envPrefix),
				Server:  r.ServerName,
				Details: fmt.Sprintf("directory: /data/yamlops/%s", strings.TrimPrefix(d, envPrefix)),
			})
		}
	}
	DisplayPlanTable4Col("ACTION", "NAME", "SERVER", "DETAILS", rows)
	DisplaySummary(formatSummaryCount("cleaned", totalCount))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
	}

	DisplayExecuting()
	executeServiceCleanupUnifiedExec(ctx, cfg, orphanResources, concurrency)
}

// --- Helper functions ---

func planTypeExtra(typeFilter string) []PlanHeaderExtra {
	if typeFilter == "" {
		return nil
	}
	return []PlanHeaderExtra{{Label: "TYPE", Value: typeFilter}}
}

func planServiceExtra(serviceFilter string) []PlanHeaderExtra {
	if serviceFilter == "" {
		return nil
	}
	return []PlanHeaderExtra{{Label: "SERVICE", Value: serviceFilter}}
}

func planTypeAndServiceExtra(typeFilter, serviceFilter string) []PlanHeaderExtra {
	var extra []PlanHeaderExtra
	if typeFilter != "" {
		extra = append(extra, PlanHeaderExtra{Label: "TYPE", Value: typeFilter})
	}
	if serviceFilter != "" {
		extra = append(extra, PlanHeaderExtra{Label: "SERVICE", Value: serviceFilter})
	}
	return extra
}

func loadConfigFromContext(ctx *Context) (*entity.Config, error) {
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

type targetServiceUnified struct {
	Name    string
	Server  string
	IsInfra bool
}

func collectTargetServicesUnified(cfg *entity.Config, filters ServiceCmdFilters) ([]targetServiceUnified, error) {
	var result []targetServiceUnified
	serviceTypes, err := parseServiceTypes(filters.Type)
	if err != nil {
		return nil, err
	}
	showBiz := len(serviceTypes) == 0 || containsStr(serviceTypes, "biz")
	showInfra := len(serviceTypes) == 0 || containsStr(serviceTypes, "infra")

	if showBiz {
		for _, svc := range cfg.Services {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			srv := cfg.GetServerMap()[svc.Server]
			if srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if filters.Service != "" && !matchesFilter(svc.Name, filters.Service) {
				continue
			}
			result = append(result, targetServiceUnified{Name: svc.Name, Server: svc.Server, IsInfra: false})
		}
	}
	if showInfra {
		for _, svc := range cfg.InfraServices {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			srv := cfg.GetServerMap()[svc.Server]
			if srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if filters.Service != "" && !matchesFilter(svc.Name, filters.Service) {
				continue
			}
			result = append(result, targetServiceUnified{Name: svc.Name, Server: svc.Server, IsInfra: true})
		}
	}
	return result, nil
}

func executeServicePlan(ctx *Context, cfg *entity.Config, plan *valueobject.Plan, scope *valueobject.Scope, concurrency int) {
	executor := usecase.NewExecutor(&usecase.ExecutorConfig{
		Plan:        plan,
		Env:         ctx.Env,
		Concurrency: concurrency,
	})
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetServerEntities(cfg.GetServerMap())
	executor.SetWorkDir(ctx.ConfigDir)

	relevantServers := make(map[string]bool)
	for _, svc := range cfg.Services {
		if scope.MatchesBizService(svc.Name) {
			relevantServers[svc.Server] = true
		}
	}
	for _, svc := range cfg.InfraServices {
		if scope.MatchesInfraService(svc.Name) {
			relevantServers[svc.Server] = true
		}
	}

	for _, srv := range cfg.Servers {
		if !relevantServers[srv.Name] && scope.HasServices() {
			continue
		}
		password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
		if err != nil {
			continue
		}
		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		executor.RegisterServer(srv.Name, srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, strictHostKeyChecking)
	}

	results := executor.Apply(context.Background())
	succeeded := 0
	failed := 0
	total := len(results)
	for i, result := range results {
		server := extractServerFromChange(result.Change, cfg)
		action := changeActionLabel(result.Change.Type())
		if result.Success {
			DisplayExecuteStep(i+1, total, ExecuteItem{
				Action:  action,
				Name:    result.Change.Name(),
				Server:  server,
				Status:  "deployed",
				Success: true,
			})
			succeeded++
		} else {
			DisplayExecuteStepWithError(i+1, total, ExecuteItem{
				Action:  action,
				Name:    result.Change.Name(),
				Server:  server,
				Status:  "failed",
				Success: false,
			}, fmt.Sprintf("%v", result.Error), "Check server and service configuration")
			failed++
		}
	}

	DisplayResult(succeeded, failed)
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

func executeServiceOperationUnified(ctx *Context, cfg *entity.Config, services []targetServiceUnified, opFunc serviceOperationFunc, actionLabel, statusLabel string, concurrency int) {
	succeeded := 0
	failed := 0
	total := len(services)
	serverMap := cfg.GetServerMap()
	secrets := cfg.GetSecretsMap()

	if concurrency <= 0 {
		concurrency = 5
	}

	type serviceResult struct {
		index   int
		success bool
		errMsg  string
	}

	sem := make(chan struct{}, concurrency)
	resultsCh := make(chan serviceResult, total)
	var wg sync.WaitGroup

	for i, svc := range services {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, s targetServiceUnified) {
			defer wg.Done()
			defer func() { <-sem }()

			srv, ok := serverMap[s.Server]
			if !ok {
				resultsCh <- serviceResult{index: idx, success: false, errMsg: fmt.Sprintf("server not found: %s", s.Server)}
				return
			}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				resultsCh <- serviceResult{index: idx, success: false, errMsg: fmt.Sprintf("cannot resolve password: %v", err)}
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
				resultsCh <- serviceResult{index: idx, success: false, errMsg: fmt.Sprintf("connection failed: %v", err)}
				return
			}

			remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, ctx.Env, s.Name))
			_, err = opFunc(client, remoteDir)
			client.Close()

			if err != nil {
				resultsCh <- serviceResult{index: idx, success: false, errMsg: fmt.Sprintf("%v", err)}
			} else {
				resultsCh <- serviceResult{index: idx, success: true}
			}
		}(i, svc)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var allResults []serviceResult
	for r := range resultsCh {
		allResults = append(allResults, r)
	}
	sort.Slice(allResults, func(i, j int) bool { return allResults[i].index < allResults[j].index })
	for _, r := range allResults {
		if r.success {
			DisplayExecuteStep(r.index+1, total, ExecuteItem{
				Action: actionLabel, Name: services[r.index].Name, Server: services[r.index].Server, Status: statusLabel, Success: true,
			})
			succeeded++
		} else {
			DisplayExecuteStepWithError(r.index+1, total, ExecuteItem{
				Action: actionLabel, Name: services[r.index].Name, Server: services[r.index].Server, Status: "failed", Success: false,
			}, r.errMsg, "")
			failed++
		}
	}

	DisplayResult(succeeded, failed)
}

func scanOrphanResourcesCLIUnified(ctx *Context, cfg *entity.Config) ([]orphanResource, error) {
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

		strictHostKeyChecking := true
		if !srv.SSH.StrictHostKeyChecking {
			strictHostKeyChecking = false
		}
		sshCfg := &ssh.SSHConfig{
			StrictHostKeyChecking: strictHostKeyChecking,
		}
		client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
		if err != nil {
			return nil, fmt.Errorf("[%s] connection failed: %w", srv.Name, err)
		}

		containerStdout, containerStderr, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("[%s] failed to list containers: %w, stderr: %s", srv.Name, err, containerStderr)
		}

		dirStdout, dirStderr, err := client.Run("sudo ls -1 " + constants.RemoteBaseDir + " 2>/dev/null || true")
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("[%s] failed to list directories: %w, stderr: %s", srv.Name, err, dirStderr)
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

func executeServiceCleanupUnifiedExec(ctx *Context, cfg *entity.Config, resources []orphanResource, concurrency int) {
	total := 0
	for _, r := range resources {
		total += len(r.Containers) + len(r.Dirs)
	}
	if total == 0 {
		DisplayResult(0, 0)
		return
	}
	if concurrency <= 0 {
		concurrency = 5
	}

	secrets := cfg.GetSecretsMap()
	serverMap := cfg.GetServerMap()

	type cleanupItem struct {
		action string
		name   string
		server string
	}

	type serverCleanupResult struct {
		index int
		items []cleanupItem
		errs  []error
	}

	sem := make(chan struct{}, concurrency)
	resultsCh := make(chan serverCleanupResult, len(resources))
	var wg sync.WaitGroup

	for idx, r := range resources {
		sem <- struct{}{}
		wg.Add(1)
		go func(resIdx int, res orphanResource) {
			defer wg.Done()
			defer func() { <-sem }()

			var items []cleanupItem
			var errs []error

			srv, ok := serverMap[res.ServerName]
			if !ok {
				for _, c := range res.Containers {
					items = append(items, cleanupItem{action: "cleanup", name: c, server: res.ServerName})
					errs = append(errs, fmt.Errorf("server not found: %s", res.ServerName))
				}
				for _, d := range res.Dirs {
					items = append(items, cleanupItem{action: "cleanup", name: d, server: res.ServerName})
					errs = append(errs, fmt.Errorf("server not found: %s", res.ServerName))
				}
				resultsCh <- serverCleanupResult{index: resIdx, items: items, errs: errs}
				return
			}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				for _, c := range res.Containers {
					items = append(items, cleanupItem{action: "cleanup", name: c, server: res.ServerName})
					errs = append(errs, fmt.Errorf("cannot resolve password: %v", err))
				}
				for _, d := range res.Dirs {
					items = append(items, cleanupItem{action: "cleanup", name: d, server: res.ServerName})
					errs = append(errs, fmt.Errorf("cannot resolve password: %v", err))
				}
				resultsCh <- serverCleanupResult{index: resIdx, items: items, errs: errs}
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
				for _, c := range res.Containers {
					items = append(items, cleanupItem{action: "cleanup", name: c, server: res.ServerName})
					errs = append(errs, fmt.Errorf("connection failed: %v", err))
				}
				for _, d := range res.Dirs {
					items = append(items, cleanupItem{action: "cleanup", name: d, server: res.ServerName})
					errs = append(errs, fmt.Errorf("connection failed: %v", err))
				}
				resultsCh <- serverCleanupResult{index: resIdx, items: items, errs: errs}
				return
			}

			for _, c := range res.Containers {
				cmd := fmt.Sprintf("sudo docker rm -f %s", c)
				_, stderr, err := client.Run(cmd)
				items = append(items, cleanupItem{action: "cleanup", name: c, server: res.ServerName})
				if err != nil {
					errs = append(errs, fmt.Errorf("%s (stderr: %s)", err, stderr))
				} else {
					errs = append(errs, nil)
				}
			}

			for _, d := range res.Dirs {
				remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, d)
				cmd := fmt.Sprintf("sudo rm -rf %s", remoteDir)
				_, stderr, err := client.Run(cmd)
				items = append(items, cleanupItem{action: "cleanup", name: d, server: res.ServerName})
				if err != nil {
					errs = append(errs, fmt.Errorf("%s (stderr: %s)", err, stderr))
				} else {
					errs = append(errs, nil)
				}
			}

			client.Close()
			resultsCh <- serverCleanupResult{index: resIdx, items: items, errs: errs}
		}(idx, r)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var allResults []serverCleanupResult
	for r := range resultsCh {
		allResults = append(allResults, r)
	}
	sort.Slice(allResults, func(i, j int) bool { return allResults[i].index < allResults[j].index })

	succeeded := 0
	failed := 0
	current := 0
	for _, r := range allResults {
		for i, item := range r.items {
			current++
			if r.errs[i] != nil {
				DisplayExecuteStepWithError(current, total, ExecuteItem{
					Action: item.action, Name: item.name, Server: item.server, Status: "failed", Success: false,
				}, fmt.Sprintf("%v", r.errs[i]), "")
				failed++
			} else {
				DisplayExecuteStep(current, total, ExecuteItem{
					Action: item.action, Name: item.name, Server: item.server, Status: "removed", Success: true,
				})
				succeeded++
			}
		}
	}

	DisplayResult(succeeded, failed)
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

func changeActionLabel(changeType valueobject.ChangeType) string {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "create"
	case valueobject.ChangeTypeUpdate:
		return "update"
	case valueobject.ChangeTypeDelete:
		return "delete"
	default:
		return ""
	}
}

func formatServiceDeployDetails(ch *valueobject.Change) string {
	var details string

	switch {
	case ch.Entity() == "service":
		newSvc, newOk := ch.NewState().(*entity.BizService)
		if !newOk {
			break
		}
		if ch.Type() == valueobject.ChangeTypeUpdate && ch.OldState() != nil {
			if oldSvc, ok := ch.OldState().(*entity.BizService); ok {
				details = formatServiceChangedFields(oldSvc.Image, newSvc.Image, oldSvc.Ports, newSvc.Ports)
			}
		}
		if details == "" {
			details = formatServiceNewFields(newSvc.Image, newSvc.Ports)
		}
	case ch.Entity() == "infra_service":
		newInfra, newOk := ch.NewState().(*entity.InfraService)
		if !newOk {
			break
		}
		if ch.Type() == valueobject.ChangeTypeUpdate && ch.OldState() != nil {
			if oldInfra, ok := ch.OldState().(*entity.InfraService); ok {
				details = formatInfraChangedFields(oldInfra.Image, newInfra.Image)
			}
		}
		if details == "" && newInfra.Image != "" {
			details = fmt.Sprintf("image: %s", newInfra.Image)
		}
	}

	if details == "" {
		details = "-"
	}
	if ch.ForcedNoChange() {
		details += " (no change, forced)"
	}
	return details
}

func formatServiceChangedFields(oldImage, newImage string, oldPorts, newPorts []entity.ServicePort) string {
	var parts []string
	if newImage != "" {
		if oldImage != "" && oldImage != newImage {
			parts = append(parts, fmt.Sprintf("image: %s -> %s", oldImage, newImage))
		} else {
			parts = append(parts, fmt.Sprintf("image: %s", newImage))
		}
	}
	newPortsStr := formatPorts(newPorts)
	oldPortsStr := formatPorts(oldPorts)
	if len(newPorts) > 0 {
		if oldPortsStr != "" && oldPortsStr != newPortsStr {
			parts = append(parts, fmt.Sprintf("ports: %s -> %s", oldPortsStr, newPortsStr))
		} else {
			parts = append(parts, fmt.Sprintf("ports: %s", newPortsStr))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return ""
}

func formatServiceNewFields(image string, ports []entity.ServicePort) string {
	var parts []string
	if image != "" {
		parts = append(parts, fmt.Sprintf("image: %s", image))
	}
	if len(ports) > 0 {
		parts = append(parts, fmt.Sprintf("ports: %s", formatPorts(ports)))
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return ""
}

func formatInfraChangedFields(oldImage, newImage string) string {
	if newImage == "" {
		return ""
	}
	if oldImage != "" && oldImage != newImage {
		return fmt.Sprintf("image: %s -> %s", oldImage, newImage)
	}
	return fmt.Sprintf("image: %s", newImage)
}

func formatPorts(ports []entity.ServicePort) string {
	var parts []string
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d:%d", p.Host, p.Container))
	}
	return strings.Join(parts, ", ")
}

func countChanges(changes []*valueobject.Change) (created, updated, deleted int) {
	for _, ch := range changes {
		switch ch.Type() {
		case valueobject.ChangeTypeCreate:
			created++
		case valueobject.ChangeTypeUpdate:
			updated++
		case valueobject.ChangeTypeDelete:
			deleted++
		}
	}
	return
}

type orphanResource struct {
	ServerName string
	Containers []string
	Dirs       []string
}
