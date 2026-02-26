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

func newDNSCommand(ctx *Context) *cobra.Command {
	var (
		dnsDomainFilter string
		dnsRecordFilter string
		dnsAutoApprove  bool
	)

	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS management commands",
		Long:  "Manage domains and DNS records.",
	}

	dnsPlanCmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate DNS change plan",
		Long:  "Generate a plan for DNS changes.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSPlan(ctx, dnsDomainFilter, dnsRecordFilter)
		},
	}

	dnsApplyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply DNS changes",
		Long:  "Apply DNS changes to providers.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSApply(ctx, dnsDomainFilter, dnsRecordFilter, dnsAutoApprove)
		},
	}

	dnsListCmd := &cobra.Command{
		Use:   "list [resource]",
		Short: "List DNS resources",
		Long:  "List domains and DNS records.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			resource := ""
			if len(args) > 0 {
				resource = args[0]
			}
			runDNSList(ctx, resource)
		},
	}

	dnsShowCmd := &cobra.Command{
		Use:   "show <resource> <name>",
		Short: "Show DNS resource details",
		Long:  "Show detailed information for a domain or DNS record.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			resource := args[0]
			name := args[1]
			runDNSShow(ctx, resource, name)
		},
	}

	dnsPlanCmd.Flags().StringVarP(&dnsDomainFilter, "domain", "d", "", "Filter by domain")
	dnsPlanCmd.Flags().StringVarP(&dnsRecordFilter, "record", "r", "", "Filter by record (format: name.domain)")

	dnsApplyCmd.Flags().StringVarP(&dnsDomainFilter, "domain", "d", "", "Filter by domain")
	dnsApplyCmd.Flags().StringVarP(&dnsRecordFilter, "record", "r", "", "Filter by record (format: name.domain)")
	dnsApplyCmd.Flags().BoolVar(&dnsAutoApprove, "auto-approve", false, "Skip confirmation prompt")

	dnsCmd.AddCommand(dnsPlanCmd)
	dnsCmd.AddCommand(dnsApplyCmd)
	dnsCmd.AddCommand(dnsListCmd)
	dnsCmd.AddCommand(dnsShowCmd)

	dnsCmd.AddCommand(newDNSPullCommand(ctx))

	return dnsCmd
}

func runDNSPlan(ctx *Context, domain, record string) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().WithDomain(domain)

	executionPlan, _, err := wf.Plan(context.Background(), "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	dnsChanges := filterDNSChanges(executionPlan.Changes(), domain, record)
	if len(dnsChanges) == 0 {
		fmt.Println("No DNS changes detected.")
		return
	}

	fmt.Println("DNS Change Plan:")
	fmt.Println("================")
	displayChanges(dnsChanges)
}

func runDNSApply(ctx *Context, domain, record string, autoApprove bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	planScope := valueobject.NewScope().WithDomain(domain)

	executionPlan, cfg, err := wf.Plan(nil, "", planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	dnsChanges := filterDNSChanges(executionPlan.Changes(), domain, record)
	if len(dnsChanges) == 0 {
		fmt.Println("No DNS changes to apply.")
		return
	}

	fmt.Println("DNS Changes:")
	displayChanges(dnsChanges)

	if !autoApprove {
		if !Confirm("\nProceed?", false) {
			fmt.Println("Cancelled.")
			return
		}
	}

	if err := wf.GenerateDeployments(cfg, ""); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	filteredPlan := valueobject.NewPlan()
	for _, ch := range dnsChanges {
		filteredPlan.AddChange(ch)
	}
	executor := usecase.NewExecutor(&usecase.ExecutorConfig{
		Plan: filteredPlan,
		Env:  ctx.Env,
	})
	executor.SetSecrets(cfg.GetSecretsMap())
	executor.SetDomains(cfg.GetDomainMap())
	executor.SetISPs(cfg.GetISPMap())
	executor.SetWorkDir(ctx.ConfigDir)

	results := executor.Apply()
	displayResults(results)
}

func displayChanges(changes []*valueobject.Change) {
	for _, ch := range changes {
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

func runDNSList(ctx *Context, resource string) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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

func runDNSShow(ctx *Context, resource, name string) {
	resource = strings.ToLower(resource)

	finder := func(cfg *entity.Config, name string) (interface{}, FindResult) {
		switch resource {
		case "domain":
			if m := cfg.GetDomainMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "record", "dns":
			for _, r := range cfg.GetAllDNSRecords() {
				fullName := r.Name + "." + r.Domain
				if r.Name == "" || r.Name == "@" {
					fullName = r.Domain
				}
				if fullName == name {
					return r, FindResultFound
				}
				if r.Domain+"::"+r.Name == name {
					return r, FindResultFound
				}
			}
			return nil, FindResultNotFound
		default:
			return nil, FindResultUnknownType
		}
	}

	validTypes := []string{"domain", "record"}
	showEntity(ctx, resource, name, finder, WithValidTypes(validTypes))
}

func filterDNSChanges(changes []*valueobject.Change, domain, record string) []*valueobject.Change {
	var result []*valueobject.Change
	for _, ch := range changes {
		if ch.Entity() != "dns_record" && ch.Entity() != "domain" {
			continue
		}
		if domain != "" && !strings.Contains(ch.Name(), domain) {
			continue
		}
		if record != "" && !strings.Contains(ch.Name(), record) {
			continue
		}
		result = append(result, ch)
	}
	return result
}
