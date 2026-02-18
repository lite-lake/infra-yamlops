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
	dnsDomainFilter string
	dnsRecordFilter string
	dnsAutoApprove  bool
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS management commands",
	Long:  "Manage domains and DNS records.",
}

var dnsPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate DNS change plan",
	Long:  "Generate a plan for DNS changes.",
	Run: func(cmd *cobra.Command, args []string) {
		runDNSPlan(dnsDomainFilter, dnsRecordFilter)
	},
}

var dnsApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply DNS changes",
	Long:  "Apply DNS changes to providers.",
	Run: func(cmd *cobra.Command, args []string) {
		runDNSApply(dnsDomainFilter, dnsRecordFilter, dnsAutoApprove)
	},
}

var dnsListCmd = &cobra.Command{
	Use:   "list [resource]",
	Short: "List DNS resources",
	Long:  "List domains and DNS records.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resource := ""
		if len(args) > 0 {
			resource = args[0]
		}
		runDNSList(resource)
	},
}

var dnsShowCmd = &cobra.Command{
	Use:   "show <resource> <name>",
	Short: "Show DNS resource details",
	Long:  "Show detailed information for a domain or DNS record.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resource := args[0]
		name := args[1]
		runDNSShow(resource, name)
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	dnsCmd.AddCommand(dnsPlanCmd)
	dnsPlanCmd.Flags().StringVarP(&dnsDomainFilter, "domain", "d", "", "Filter by domain")
	dnsPlanCmd.Flags().StringVarP(&dnsRecordFilter, "record", "r", "", "Filter by record (format: name.domain)")

	dnsCmd.AddCommand(dnsApplyCmd)
	dnsApplyCmd.Flags().StringVarP(&dnsDomainFilter, "domain", "d", "", "Filter by domain")
	dnsApplyCmd.Flags().StringVarP(&dnsRecordFilter, "record", "r", "", "Filter by record (format: name.domain)")
	dnsApplyCmd.Flags().BoolVar(&dnsAutoApprove, "auto-approve", false, "Skip confirmation prompt")

	dnsCmd.AddCommand(dnsListCmd)
	dnsCmd.AddCommand(dnsShowCmd)
}

func runDNSPlan(domain, record string) {
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
		Domain: domain,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	dnsChanges := filterDNSChanges(executionPlan.Changes, domain, record)
	if len(dnsChanges) == 0 {
		fmt.Println("No DNS changes detected.")
		return
	}

	fmt.Println("DNS Change Plan:")
	fmt.Println("================")
	for _, ch := range dnsChanges {
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

func runDNSApply(domain, record string, autoApprove bool) {
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
		Domain: domain,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	dnsChanges := filterDNSChanges(executionPlan.Changes, domain, record)
	if len(dnsChanges) == 0 {
		fmt.Println("No DNS changes to apply.")
		return
	}

	fmt.Println("DNS Changes:")
	for _, ch := range dnsChanges {
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
	}

	if !autoApprove {
		fmt.Print("\nProceed? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return
		}
	}

	filteredPlan := &valueobject.Plan{Changes: dnsChanges}
	if err := planner.GenerateDeployments(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating deployments: %v\n", err)
		os.Exit(1)
	}

	executor := usecase.NewExecutor(filteredPlan, Env)
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
	executor.SetWorkDir(ConfigDir)

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

func runDNSList(resource string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	resource = strings.ToLower(resource)
	switch resource {
	case "", "all":
		printDomains(cfg)
		fmt.Println()
		printDNSRecords(cfg)
	case "domains", "domain":
		printDomains(cfg)
	case "records", "record", "dns":
		printDNSRecords(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown resource type: %s\n", resource)
		fmt.Fprintf(os.Stderr, "Valid types: domains, records\n")
		os.Exit(1)
	}
}

func printDomains(cfg *entity.Config) {
	fmt.Println("DOMAINS:")
	if len(cfg.Domains) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, d := range cfg.Domains {
		parentInfo := ""
		if d.Parent != "" {
			parentInfo = fmt.Sprintf(", parent: %s", d.Parent)
		}
		ispInfo := ""
		if d.ISP != "" {
			ispInfo = fmt.Sprintf(", isp: %s", d.ISP)
		}
		fmt.Printf("  %-20s (dns_isp: %s%s%s)\n", d.Name, d.DNSISP, ispInfo, parentInfo)
	}
}

func printDNSRecords(cfg *entity.Config) {
	fmt.Println("DNS RECORDS:")
	records := cfg.GetAllDNSRecords()
	if len(records) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, r := range records {
		name := r.Name
		if name == "" || name == "@" {
			name = "@"
		}
		fmt.Printf("  %-20s %-6s %-12s -> %-20s (ttl: %d)\n", r.Domain, r.Type, name, r.Value, r.TTL)
	}
}

func runDNSShow(resource, name string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	resource = strings.ToLower(resource)
	var found interface{}

	switch resource {
	case "domain":
		if m := cfg.GetDomainMap(); m[name] != nil {
			found = m[name]
		}
	case "record", "dns":
		for _, r := range cfg.GetAllDNSRecords() {
			fullName := r.Name + "." + r.Domain
			if r.Name == "" || r.Name == "@" {
				fullName = r.Domain
			}
			if fullName == name {
				found = r
				break
			}
			if r.Domain+"::"+r.Name == name {
				found = r
				break
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown resource type: %s\n", resource)
		fmt.Fprintf(os.Stderr, "Valid types: domain, record\n")
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

func filterDNSChanges(changes []*valueobject.Change, domain, record string) []*valueobject.Change {
	var result []*valueobject.Change
	for _, ch := range changes {
		if ch.Entity != "dns_record" && ch.Entity != "domain" {
			continue
		}
		if domain != "" && !strings.Contains(ch.Name, domain) {
			continue
		}
		if record != "" && !strings.Contains(ch.Name, record) {
			continue
		}
		result = append(result, ch)
	}
	return result
}
