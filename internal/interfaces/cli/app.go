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

var (
	appZoneFilter   string
	appServerFilter string
	appInfraFilter  string
	appBizFilter    string
	appAutoApprove  bool
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage application resources",
	Long:  "Manage zones, servers, infra services, and business services.",
}

var appPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate deployment plan",
	Long:  "Generate a deployment plan for application resources.",
	Run: func(cmd *cobra.Command, args []string) {
		runAppPlan()
	},
}

var appApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply deployment",
	Long:  "Apply the deployment for application resources.",
	Run: func(cmd *cobra.Command, args []string) {
		runAppApply()
	},
}

var appListCmd = &cobra.Command{
	Use:   "list [resource]",
	Short: "List resources",
	Long:  "List application resources (zones, servers, infra, biz).",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resource := ""
		if len(args) > 0 {
			resource = args[0]
		}
		runAppList(resource)
	},
}

var appShowCmd = &cobra.Command{
	Use:   "show <resource> <name>",
	Short: "Show resource details",
	Long:  "Show detailed information for a specific resource.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resource := args[0]
		name := args[1]
		runAppShow(resource, name)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)

	appCmd.PersistentFlags().StringVarP(&appZoneFilter, "zone", "z", "", "Filter by zone")
	appCmd.PersistentFlags().StringVarP(&appServerFilter, "server", "s", "", "Filter by server")
	appCmd.PersistentFlags().StringVarP(&appInfraFilter, "infra", "i", "", "Filter by infra service")
	appCmd.PersistentFlags().StringVarP(&appBizFilter, "biz", "b", "", "Filter by business service")

	appApplyCmd.Flags().BoolVar(&appAutoApprove, "auto-approve", false, "Auto approve changes")

	appCmd.AddCommand(appPlanCmd)
	appCmd.AddCommand(appApplyCmd)
	appCmd.AddCommand(appListCmd)
	appCmd.AddCommand(appShowCmd)
}

func runAppPlan() {
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
		Zone:    appZoneFilter,
		Server:  appServerFilter,
		Service: appBizFilter,
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
		if appInfraFilter != "" && ch.Entity != "infra_service" {
			continue
		}
		if appBizFilter != "" && ch.Entity != "service" {
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

func runAppApply() {
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
		Zone:    appZoneFilter,
		Server:  appServerFilter,
		Service: appBizFilter,
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

	if !appAutoApprove {
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

	executor := usecase.NewExecutor(executionPlan, Env)
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
	executor.SetWorkDir(ConfigDir)

	for _, srv := range cfg.Servers {
		if appServerFilter != "" && srv.Name != appServerFilter {
			continue
		}
		if appZoneFilter != "" && srv.Zone != appZoneFilter {
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
		if appInfraFilter != "" && result.Change.Entity != "infra_service" {
			continue
		}
		if appBizFilter != "" && result.Change.Entity != "service" {
			continue
		}
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

func runAppList(resource string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	resource = strings.ToLower(resource)
	switch resource {
	case "", "all":
		listZones(cfg)
		listServers(cfg)
		listInfraServices(cfg)
		listBizServices(cfg)
	case "zones", "zone":
		listZones(cfg)
	case "servers", "server":
		listServers(cfg)
	case "infra", "infra_service", "infra_services":
		listInfraServices(cfg)
	case "biz", "business", "services", "service":
		listBizServices(cfg)
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

func listServers(cfg *entity.Config) {
	if len(cfg.Servers) == 0 {
		return
	}
	fmt.Println("Servers:")
	for _, s := range cfg.Servers {
		if appZoneFilter != "" && s.Zone != appZoneFilter {
			continue
		}
		fmt.Printf("  - %s (zone: %s, ip: %s)\n", s.Name, s.Zone, s.IP.Public)
	}
}

func listInfraServices(cfg *entity.Config) {
	if len(cfg.InfraServices) == 0 {
		return
	}
	fmt.Println("Infra Services:")
	for _, infra := range cfg.InfraServices {
		if appServerFilter != "" && infra.Server != appServerFilter {
			continue
		}
		fmt.Printf("  - %s (server: %s, type: %s)\n", infra.Name, infra.Server, infra.Type)
	}
}

func listBizServices(cfg *entity.Config) {
	if len(cfg.Services) == 0 {
		return
	}
	fmt.Println("Business Services:")
	for _, s := range cfg.Services {
		if appServerFilter != "" && s.Server != appServerFilter {
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

func runAppShow(resource, name string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
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
