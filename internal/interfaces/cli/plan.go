package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/domain/valueobject"
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
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().
		WithDomain(filters.Domain).
		WithZone(filters.Zone).
		WithServer(filters.Server).
		WithService(filters.Service)

	executionPlan, _, err := wf.Plan(context.Background(), "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes detected.")
		return
	}

	displayPlan(executionPlan)
}

func displayPlan(p *valueobject.Plan) {
	fmt.Println("Execution Plan:")
	fmt.Println("===============")
	for _, ch := range p.Changes() {
		var prefix string
		switch ch.Type() {
		case valueobject.ChangeTypeCreate:
			prefix = "+"
		case valueobject.ChangeTypeUpdate:
			prefix = "~"
		case valueobject.ChangeTypeDelete:
			prefix = "-"
		default:
			prefix = " "
		}
		fmt.Printf("%s %s: %s\n", prefix, ch.Entity(), ch.Name())
		for _, action := range ch.Actions() {
			fmt.Printf("    - %s\n", action)
		}
	}
}
