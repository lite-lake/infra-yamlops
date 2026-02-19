package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/litelake/yamlops/internal/application/usecase"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/plan"
)

type AppFilters struct {
	Zone   string
	Server string
	Infra  string
	Biz    string
}

func newAppCommand(ctx *Context) *cobra.Command {
	var filters AppFilters
	var autoApprove bool

	appCmd := &cobra.Command{
		Use:   "app",
		Short: "Manage application resources",
		Long:  "Manage zones, servers, infra services, and business services.",
	}

	appPlanCmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate deployment plan",
		Long:  "Generate a deployment plan for application resources.",
		Run: func(cmd *cobra.Command, args []string) {
			runAppPlan(ctx, filters)
		},
	}

	appApplyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply deployment",
		Long:  "Apply the deployment for application resources.",
		Run: func(cmd *cobra.Command, args []string) {
			runAppApply(ctx, filters, autoApprove)
		},
	}

	appListCmd := &cobra.Command{
		Use:   "list [resource]",
		Short: "List resources",
		Long:  "List application resources (zones, servers, infra, biz).",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			resource := ""
			if len(args) > 0 {
				resource = args[0]
			}
			runAppList(ctx, filters, resource)
		},
	}

	appShowCmd := &cobra.Command{
		Use:   "show <resource> <name>",
		Short: "Show resource details",
		Long:  "Show detailed information for a specific resource.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			resource := args[0]
			name := args[1]
			runAppShow(ctx, resource, name)
		},
	}

	appCmd.PersistentFlags().StringVarP(&filters.Zone, "zone", "z", "", "Filter by zone")
	appCmd.PersistentFlags().StringVarP(&filters.Server, "server", "s", "", "Filter by server")
	appCmd.PersistentFlags().StringVarP(&filters.Infra, "infra", "i", "", "Filter by infra service")
	appCmd.PersistentFlags().StringVarP(&filters.Biz, "biz", "b", "", "Filter by business service")

	appApplyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Auto approve changes")

	appCmd.AddCommand(appPlanCmd)
	appCmd.AddCommand(appApplyCmd)
	appCmd.AddCommand(appListCmd)
	appCmd.AddCommand(appShowCmd)

	return appCmd
}

func runAppPlan(ctx *Context, filters AppFilters) {
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
		Zone:    filters.Zone,
		Server:  filters.Server,
		Service: filters.Biz,
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
		if filters.Infra != "" && ch.Entity != "infra_service" {
			continue
		}
		if filters.Biz != "" && ch.Entity != "service" {
			continue
		}
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

func runAppApply(ctx *Context, filters AppFilters, autoApprove bool) {
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
		Zone:    filters.Zone,
		Server:  filters.Server,
		Service: filters.Biz,
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

	if !autoApprove {
		fmt.Print("Do you want to apply these changes? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return
		}
	}

	if err := planner.GenerateDeployments(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating deployments: %v\n", err)
		os.Exit(1)
	}

	executor := usecase.NewExecutor(executionPlan, ctx.Env)
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
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

	hasError := false
	for _, result := range results {
		if filters.Infra != "" && result.Change.Entity != "infra_service" {
			continue
		}
		if filters.Biz != "" && result.Change.Entity != "service" {
			continue
		}
		if result.Success {
			fmt.Printf("✓ %s: %s\n", result.Change.Entity, result.Change.Name)
			for _, w := range result.Warnings {
				fmt.Printf("  ⚠ %s\n", w)
			}
		} else {
			fmt.Printf("✗ %s: %s - %v\n", result.Change.Entity, result.Change.Name, result.Error)
			hasError = true
		}
	}

	if hasError {
		os.Exit(1)
	}
}

func runAppList(ctx *Context, filters AppFilters, resource string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	resource = strings.ToLower(resource)
	switch resource {
	case "", "all":
		listZones(cfg)
		listServers(filters, cfg)
		listInfraServices(filters, cfg)
		listBizServices(filters, cfg)
	case "zones", "zone":
		listZones(cfg)
	case "servers", "server":
		listServers(filters, cfg)
	case "infra", "infra_service", "infra_services":
		listInfraServices(filters, cfg)
	case "biz", "business", "services", "service":
		listBizServices(filters, cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown resource type: %s\n", resource)
		fmt.Fprintf(os.Stderr, "Valid types: zones, servers, infra, biz\n")
		os.Exit(1)
	}
}

func listZones(cfg *entity.Config) {
	if len(cfg.Zones) == 0 {
		return
	}
	fmt.Println("Zones:")
	for _, z := range cfg.Zones {
		fmt.Printf("  - %s (isp: %s, region: %s)\n", z.Name, z.ISP, z.Region)
	}
}

func listServers(filters AppFilters, cfg *entity.Config) {
	if len(cfg.Servers) == 0 {
		return
	}
	fmt.Println("Servers:")
	for _, s := range cfg.Servers {
		if filters.Zone != "" && s.Zone != filters.Zone {
			continue
		}
		fmt.Printf("  - %s (zone: %s, ip: %s)\n", s.Name, s.Zone, s.IP.Public)
	}
}

func listInfraServices(filters AppFilters, cfg *entity.Config) {
	if len(cfg.InfraServices) == 0 {
		return
	}
	fmt.Println("Infra Services:")
	for _, infra := range cfg.InfraServices {
		if filters.Server != "" && infra.Server != filters.Server {
			continue
		}
		fmt.Printf("  - %s (server: %s, type: %s)\n", infra.Name, infra.Server, infra.Type)
	}
}

func listBizServices(filters AppFilters, cfg *entity.Config) {
	if len(cfg.Services) == 0 {
		return
	}
	fmt.Println("Business Services:")
	for _, s := range cfg.Services {
		if filters.Server != "" && s.Server != filters.Server {
			continue
		}
		portStr := ""
		for i, p := range s.Ports {
			if i > 0 {
				portStr += ", "
			}
			portStr += fmt.Sprintf("%d->%d", p.Host, p.Container)
		}
		fmt.Printf("  - %s (server: %s, ports: %s)\n", s.Name, s.Server, portStr)
	}
}

func runAppShow(ctx *Context, resource, name string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	resource = strings.ToLower(resource)
	var found interface{}

	switch resource {
	case "zone":
		if m := cfg.GetZoneMap(); m[name] != nil {
			found = m[name]
		}
	case "server":
		if m := cfg.GetServerMap(); m[name] != nil {
			found = m[name]
		}
	case "infra", "infra_service":
		if m := cfg.GetInfraServiceMap(); m[name] != nil {
			found = m[name]
		}
	case "biz", "business", "service":
		if m := cfg.GetServiceMap(); m[name] != nil {
			found = m[name]
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown resource type: %s\n", resource)
		fmt.Fprintf(os.Stderr, "Valid types: zone, server, infra, biz\n")
		os.Exit(1)
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "%s '%s' not found\n", resource, name)
		os.Exit(1)
	}

	data, err := yaml.Marshal(found)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling resource: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", strings.Title(resource), name)
	fmt.Println(string(data))
}
