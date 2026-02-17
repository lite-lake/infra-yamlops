package cli

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [scope]",
	Short: "Generate execution plan",
	Long:  "Generate an execution plan for the specified scope.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := ""
		if len(args) > 0 {
			scope = args[0]
		}
		runPlan(scope, DomainFilter, ZoneFilter, ServerFilter, ServiceFilter)
	},
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVar(&DomainFilter, "domain", "", "Filter by domain")
	planCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")
	planCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	planCmd.Flags().StringVar(&ServiceFilter, "service", "", "Filter by service")
}

func runPlan(scope, domain, zone, server, service string) {
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
