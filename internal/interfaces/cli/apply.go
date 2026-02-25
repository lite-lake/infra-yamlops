package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func newApplyCommand(ctx *Context) *cobra.Command {
	var filters Filters

	cmd := &cobra.Command{
		Use:   "apply [scope]",
		Short: "Apply changes",
		Long:  "Apply infrastructure changes for the specified scope.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			scope := ""
			if len(args) > 0 {
				scope = args[0]
			}
			runApply(ctx, scope, filters)
		},
	}

	cmd.Flags().StringVar(&filters.Domain, "domain", "", "Filter by domain")
	cmd.Flags().StringVar(&filters.Zone, "zone", "", "Filter by zone")
	cmd.Flags().StringVar(&filters.Server, "server", "", "Filter by server")
	cmd.Flags().StringVar(&filters.Service, "service", "", "Filter by service")

	return cmd
}

func runApply(ctx *Context, scope string, filters Filters) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().
		WithDomain(filters.Domain).
		WithZone(filters.Zone).
		WithServer(filters.Server).
		WithService(filters.Service)

	executionPlan, cfg, err := wf.Plan(nil, "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes to apply.")
		return
	}

	displayPlan(executionPlan)
	if !Confirm("\nDo you want to apply these changes?", false) {
		fmt.Println("Cancelled.")
		return
	}

	if err := wf.GenerateDeployments(cfg, ""); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	executor := usecase.NewExecutor(&usecase.ExecutorConfig{
		Plan: executionPlan,
		Env:  ctx.Env,
	})
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
	executor.SetServerEntities(cfg.GetServerMap())
	executor.SetWorkDir(ctx.ConfigDir)

	for _, srv := range cfg.Servers {
		if filters.Server != "" && srv.Name != filters.Server {
			continue
		}
		if filters.Zone != "" && srv.Zone != filters.Zone {
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
	displayResults(results)
}

func displayResults(results []*handler.Result) {
	hasError := false
	for _, result := range results {
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
