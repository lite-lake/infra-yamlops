package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func newDNSCommand(ctx *Context) *cobra.Command {
	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS management commands",
		Long: `DNS management commands.

Commands:
  show      List domains and DNS records (use --detail for record details)
  validate  Validate DNS configuration
  deploy    Deploy DNS changes (Plan -> Confirm -> Execute)
  pull      Pull domains/records from ISP (Plan -> Confirm -> Execute)

Examples:
  yamlops cli dns show -e prod
  yamlops cli dns show -e prod --detail
  yamlops cli dns deploy -e prod --domain example.com --dry-run
  yamlops cli dns pull domains -e prod --isp aliyun --yes
  yamlops cli dns pull records -e prod --domain example.com --yes`,
	}

	dnsCmd.AddCommand(newDNSShowCommand(ctx))
	dnsCmd.AddCommand(newDNSValidateCommand(ctx))
	dnsCmd.AddCommand(newDNSDeployCommand(ctx))
	dnsCmd.AddCommand(newDNSPullCommand(ctx))

	return dnsCmd
}

func newDNSShowCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Domain string
		Record string
	}
	var detail bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "List domains and DNS records",
		Long:  "List domains with ISP and record count. Use --detail to expand record types, names, values, and TTL.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSShow(ctx, filters.Domain, filters.Record, detail)
		},
	}
	cmd.Flags().StringVar(&filters.Domain, "domain", "", "Domain filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Record, "record", "", "Record filter: TYPE:NAME (comma-separated)")
	cmd.Flags().BoolVar(&detail, "detail", false, "Show detailed information")
	return cmd
}

func newDNSDeployCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Domain string
		Record string
	}
	var dryRun, yes, force bool
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy DNS changes",
		Long:  "Deploy DNS changes using unified execution mode: Plan -> Confirm -> Execute.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSDeployUnified(ctx, filters.Domain, filters.Record, dryRun, yes, force)
		},
	}
	cmd.Flags().StringVar(&filters.Domain, "domain", "", "Domain filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Record, "record", "", "Record filter: TYPE:NAME (comma-separated)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	cmd.Flags().BoolVar(&force, "force", false, "Force execution even without changes")
	return cmd
}

func runDNSShow(ctx *Context, domainFilter, recordFilter string, detail bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	type domainRow struct {
		Name    string
		ISP     string
		Records int
	}

	var rows []domainRow
	domains := append([]entity.Domain(nil), cfg.Domains...)
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})

	for _, d := range domains {
		if domainFilter != "" && !matchesFilter(d.Name, domainFilter) {
			continue
		}
		recordCount := 0
		for _, r := range d.Records {
			if recordFilter != "" && !matchesRecordFilter(r, recordFilter) {
				continue
			}
			recordCount++
		}
		if recordFilter != "" && recordCount == 0 {
			continue
		}
		rows = append(rows, domainRow{Name: d.Name, ISP: d.DNSISP, Records: recordCount})
	}

	if len(rows) == 0 {
		fmt.Println("No domains found.")
		return
	}

	totalRecords := 0
	for _, r := range rows {
		totalRecords += r.Records
	}

	if detail {
		fmt.Println()
		fmt.Printf("%-20s %-12s %-12s\n", "DOMAIN", "ISP", "RECORDS")
		for _, r := range rows {
			fmt.Printf("%-20s %-12s %d %s\n", r.Name, r.ISP, r.Records, pluralize(r.Records, "record", "records"))
		}
		fmt.Println()
		first := true
		for _, d := range domains {
			if domainFilter != "" && !matchesFilter(d.Name, domainFilter) {
				continue
			}
			if recordFilter != "" && !hasMatchingRecord(d, recordFilter) {
				continue
			}
			if !first {
				fmt.Println()
			}
			first = false
			fmt.Printf("DOMAIN: %s\n", d.Name)
			fmt.Printf("  ISP: %s\n", d.DNSISP)
			if len(d.Records) > 0 {
				fmt.Println("  Records:")
				fmt.Printf("    %-8s %-12s %-24s %s\n", "TYPE", "NAME", "VALUE", "TTL")
				records := append([]entity.DNSRecord(nil), d.Records...)
				sort.Slice(records, func(i, j int) bool {
					if records[i].Type != records[j].Type {
						return records[i].Type < records[j].Type
					}
					return records[i].Name < records[j].Name
				})
				for _, r := range records {
					if recordFilter != "" && !matchesRecordFilter(r, recordFilter) {
						continue
					}
					name := r.Name
					if name == "" || name == "@" {
						name = "@"
					}
					fmt.Printf("    %-8s %-12s %-24s %d\n", r.Type, name, r.Value, r.TTL)
				}
			}
		}
	} else {
		fmt.Println()
		fmt.Printf("%-20s %-12s %-12s\n", "DOMAIN", "ISP", "RECORDS")
		for _, r := range rows {
			fmt.Printf("%-20s %-12s %d %s\n", r.Name, r.ISP, r.Records, pluralize(r.Records, "record", "records"))
		}
	}

	fmt.Printf("Total: %d %s, %d %s\n", len(rows), pluralize(len(rows), "domain", "domains"), totalRecords, pluralize(totalRecords, "record", "records"))
}

func runDNSDeployUnified(ctx *Context, domainFilter, recordFilter string, dryRun, yes, force bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	scope := valueobject.NewScope()
	if domainFilter != "" {
		scope = scope.WithDomains(splitAndTrim(domainFilter, ","))
	}
	if recordFilter != "" {
		scope = scope.WithDNSRecords(splitAndTrim(recordFilter, ","))
	}
	if force {
		scope = scope.WithForceDeploy(true)
	}

	executionPlan, cfg, err := wf.Plan(context.Background(), "", scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Plan failed for DNS deploy\n")
		fmt.Fprintf(os.Stderr, "Details: %v\n", err)
		fmt.Fprintf(os.Stderr, "Suggestion: Run 'dns validate' to check DNS configuration\n")
		os.Exit(1)
	}

	dnsChanges := filterDNSChangesUnified(executionPlan.Changes(), domainFilter, recordFilter)

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("dns deploy", dryRun, force),
		Env:   ctx.Env,
	})

	if len(dnsChanges) == 0 {
		DisplayNoChanges(dryRun, force)
		return
	}

	var rows []PlanRow
	for _, ch := range dnsChanges {
		domain := ""
		record := ""
		details := ""
		if ch.Entity() == "dns_record" {
			domain, record = parseDNSChangeName(ch)
			details = formatDNSDetails(ch)
		} else if ch.Entity() == "domain" {
			domain = ch.Name()
			record = "-"
		}
		rows = append(rows, PlanRow{
			Action:  changeActionLabel(ch.Type()),
			Name:    domain,
			Server:  record,
			Details: details,
		})
	}
	DisplayPlanTable4Col("ACTION", "DOMAIN", "RECORD", "DETAILS", rows)

	created, updated, deleted := countChanges(dnsChanges)
	DisplaySummary(formatSummary(created, updated, deleted, force))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
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

	DisplayExecuting()
	results := executor.Apply(context.Background())
	succeeded := 0
	failed := 0
	total := len(results)
	for i, result := range results {
		domain, record := parseDNSChangeName(result.Change)
		action := changeActionLabel(result.Change.Type())
		status := changeStatusLabel(result.Change.Type())
		details := formatDNSDetails(result.Change)
		if result.Success {
			DisplayExecuteStep(i+1, total, ExecuteItem{
				Action:  action,
				Name:    domain,
				Record:  record,
				Details: details,
				Status:  status,
				Success: true,
			})
			succeeded++
		} else {
			DisplayExecuteStepWithError(i+1, total, ExecuteItem{
				Action:  action,
				Name:    domain,
				Record:  record,
				Details: details,
				Status:  "failed",
				Success: false,
			}, fmt.Sprintf("%v", result.Error), "")
			failed++
		}
	}
	DisplayResult(succeeded, failed)
}

func filterDNSChangesUnified(changes []*valueobject.Change, domain, record string) []*valueobject.Change {
	var result []*valueobject.Change
	for _, ch := range changes {
		if ch.Entity() != "dns_record" && ch.Entity() != "domain" {
			continue
		}
		if domain != "" {
			match := false
			changeDomain := extractDomainFromChange(ch)
			for _, d := range splitAndTrim(domain, ",") {
				if changeDomain == d {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if record != "" {
			match := false
			if ch.Entity() == "dns_record" {
				recordID := extractRecordID(ch.Name())
				for _, r := range splitAndTrim(record, ",") {
					if strings.TrimSpace(r) == recordID {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, ch)
	}
	return result
}

func parseDNSRecordName(name string) (domain, recordType, recordName string, ok bool) {
	parts := strings.SplitN(name, ":", 3)
	if len(parts) >= 3 {
		return parts[0], parts[1], parts[2], true
	}
	return "", "", "", false
}

func extractDomainFromChange(ch *valueobject.Change) string {
	if ch.Entity() == "dns_record" {
		if domain, _, _, ok := parseDNSRecordName(ch.Name()); ok {
			return domain
		}
	}
	return ch.Name()
}

func extractRecordID(changeName string) string {
	if _, recordType, recordName, ok := parseDNSRecordName(changeName); ok {
		return recordType + ":" + recordName
	}
	return changeName
}

func formatDNSDetails(ch *valueobject.Change) string {
	if ch.Entity() != "dns_record" {
		for _, action := range ch.Actions() {
			return action
		}
		return "-"
	}

	oldRec, _ := ch.OldState().(*entity.DNSRecord)
	newRec, _ := ch.NewState().(*entity.DNSRecord)

	switch ch.Type() {
	case valueobject.ChangeTypeCreate:
		if newRec != nil {
			return fmt.Sprintf("type: %s, value: %s (new)", newRec.Type, newRec.Value)
		}
	case valueobject.ChangeTypeUpdate:
		if oldRec != nil && newRec != nil {
			return fmt.Sprintf("type: %s, value: %s -> %s", newRec.Type, oldRec.Value, newRec.Value)
		}
	case valueobject.ChangeTypeDelete:
		if oldRec != nil {
			return fmt.Sprintf("type: %s, value: %s (deleted)", oldRec.Type, oldRec.Value)
		}
	}
	return "-"
}

func parseDNSChangeName(ch *valueobject.Change) (domain, record string) {
	if ch.Entity() == "dns_record" {
		if d, recordType, recordName, ok := parseDNSRecordName(ch.Name()); ok {
			return d, fmt.Sprintf("%s %s", recordType, recordName)
		}
	}
	return ch.Name(), ""
}

func hasMatchingRecord(d entity.Domain, recordFilter string) bool {
	for _, r := range d.Records {
		if matchesRecordFilter(r, recordFilter) {
			return true
		}
	}
	return false
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func changeStatusLabel(changeType valueobject.ChangeType) string {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "created"
	case valueobject.ChangeTypeUpdate:
		return "updated"
	case valueobject.ChangeTypeDelete:
		return "deleted"
	default:
		return "applied"
	}
}
