package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
)

func newPlanCommand(ctx *Context) *cobra.Command {
	var filters Filters

	cmd := &cobra.Command{
		Use:   "plan [scope]",
		Short: "Generate execution plan",
		Long:  "Generate an execution plan for the specified scope.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			scope := ""
			if len(args) > 0 {
				scope = args[0]
			}
			runPlan(ctx, scope, filters)
		},
	}

	cmd.Flags().StringVar(&filters.Domain, "domain", "", "Filter by domain")
	cmd.Flags().StringVar(&filters.Zone, "zone", "", "Filter by zone")
	cmd.Flags().StringVar(&filters.Server, "server", "", "Filter by server")
	cmd.Flags().StringVar(&filters.Service, "service", "", "Filter by service")

	return cmd
}

func runPlan(ctx *Context, scope string, filters Filters) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	planner := plan.NewPlanner(cfg, ctx.Env)
	planScope := &valueobject.Scope{
		Domain:  filters.Domain,
		Zone:    filters.Zone,
		Server:  filters.Server,
		Service: filters.Service,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes detected.")
		return
	}

	fmt.Println("Execution Plan:")
	fmt.Println("===============")
	for _, ch := range executionPlan.Changes {
		var prefix string
		switch ch.Type {
		case valueobject.ChangeTypeCreate:
			prefix = "+"
		case valueobject.ChangeTypeUpdate:
			prefix = "~"
		case valueobject.ChangeTypeDelete:
			prefix = "-"
		default:
			prefix = " "
		}
		fmt.Printf("%s %s: %s\n", prefix, ch.Entity, ch.Name)
		for _, action := range ch.Actions {
			fmt.Printf("    - %s\n", action)
		}
	}
}
