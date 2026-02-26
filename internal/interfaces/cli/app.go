package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
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
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().
		WithZone(filters.Zone).
		WithServer(filters.Server).
		WithService(filters.Biz)

	executionPlan, _, err := wf.Plan(context.Background(), "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes detected.")
		return
	}

	fmt.Println("Execution Plan:")
	fmt.Println("===============")
	for _, ch := range executionPlan.Changes() {
		if filters.Infra != "" && ch.Entity() != "infra_service" {
			continue
		}
		if filters.Biz != "" && ch.Entity() != "service" {
			continue
		}
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

func runAppApply(ctx *Context, filters AppFilters, autoApprove bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().
		WithZone(filters.Zone).
		WithServer(filters.Server).
		WithService(filters.Biz)

	executionPlan, cfg, err := wf.Plan(nil, "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes to apply.")
		return
	}

	if !autoApprove {
		if !Confirm("Do you want to apply these changes?", false) {
			fmt.Println("Cancelled.")
			return
		}
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

	hasError := false
	for _, result := range results {
		if filters.Infra != "" && result.Change.Entity() != "infra_service" {
			continue
		}
		if filters.Biz != "" && result.Change.Entity() != "service" {
			continue
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

func runAppList(ctx *Context, filters AppFilters, resource string) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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
	resource = strings.ToLower(resource)

	finder := func(cfg *entity.Config, name string) (interface{}, FindResult) {
		switch resource {
		case "zone":
			if m := cfg.GetZoneMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "server":
			if m := cfg.GetServerMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "infra", "infra_service":
			if m := cfg.GetInfraServiceMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "biz", "business", "service":
			if m := cfg.GetServiceMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		default:
			return nil, FindResultUnknownType
		}
	}

	validTypes := []string{"zone", "server", "infra", "biz"}
	showEntity(ctx, resource, name, finder, WithValidTypes(validTypes))
}
