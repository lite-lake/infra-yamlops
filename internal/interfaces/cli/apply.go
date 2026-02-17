package cli

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply [scope]",
	Short: "Apply changes",
	Long:  "Apply infrastructure changes for the specified scope.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := ""
		if len(args) > 0 {
			scope = args[0]
		}
		runApply(scope, DomainFilter, ZoneFilter, ServerFilter, ServiceFilter)
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringVar(&DomainFilter, "domain", "", "Filter by domain")
	applyCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")
	applyCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	applyCmd.Flags().StringVar(&ServiceFilter, "service", "", "Filter by service")
}

func runApply(scope, domain, zone, server, service string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	planner := plan.NewPlanner(cfg, Env)
	planScope := &valueobject.Scope{
		Domain:  domain,
		Zone:    zone,
		Server:  server,
		Service: service,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes to apply.")
		return
	}

	if err := planner.GenerateDeployments(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating deployments: %v\n", err)
		os.Exit(1)
	}

	executor := usecase.NewExecutor(executionPlan, Env)
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
	executor.SetWorkDir(ConfigDir)

	for _, srv := range cfg.Servers {
		if server != "" && srv.Name != server {
			continue
		}
		if zone != "" && srv.Zone != zone {
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
		if result.Success {
			fmt.Printf("✓ %s: %s\n", result.Change.Entity, result.Change.Name)
		} else {
			fmt.Printf("✗ %s: %s - %v\n", result.Change.Entity, result.Change.Name, result.Error)
			hasError = true
		}
	}

	if hasError {
		os.Exit(1)
	}
}
