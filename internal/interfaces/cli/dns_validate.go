package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
)

func newDNSValidateCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Domain string
		Record string
	}
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate DNS configuration",
		Long:  "Validate DNS configuration: domain names, record types, ISP references, and uniqueness constraints.",
		Run: func(cmd *cobra.Command, args []string) {
			runDNSValidate(ctx, filters.Domain, filters.Record)
		},
	}
	cmd.Flags().StringVar(&filters.Domain, "domain", "", "Domain filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Record, "record", "", "Record filter: TYPE:NAME (comma-separated)")
	return cmd
}

func runDNSValidate(ctx *Context, domainFilter, recordFilter string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validating DNS configuration (%s)...\n\n", ctx.Env)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("Validating DNS configuration (%s)...\n\n", ctx.Env)

	var errs []validationError
	var warns []validationError
	checks := 0
	ispMap := cfg.GetISPMap()
	domainMap := cfg.GetDomainMap()

	for _, d := range cfg.Domains {
		if domainFilter != "" && !matchesFilter(d.Name, domainFilter) {
			continue
		}

		// Entity-level validation (domain + records)
		checks++
		if err := d.Validate(); err != nil {
			errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': %s", d.Name, err.Error()), suggestion: fmt.Sprintf("Fix validation error for domain '%s' in domains.yaml", d.Name)})
		}

		// Skip cross-entity checks only if domain name is empty (cannot resolve references)
		if d.Name == "" {
			continue
		}

		// Cross-entity: dns_isp exists
		checks++
		if ispMap[d.DNSISP] == nil {
			errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': dns_isp '%s' not found", d.Name, d.DNSISP), suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix dns_isp reference in domains.yaml", d.DNSISP)})
		} else {
			// Cross-entity: ISP supports DNS service
			isp := ispMap[d.DNSISP]
			checks++
			if !isp.HasService(entity.ISPServiceDNS) {
				errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': isp '%s' does not support DNS service", d.Name, d.DNSISP), suggestion: fmt.Sprintf("Check ISP configuration in isps.yaml for ISP '%s'", d.DNSISP)})
			}
		}

		// Cross-entity: parent domain exists
		if d.Parent != "" {
			checks++
			if domainMap[d.Parent] == nil {
				errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': parent '%s' not found", d.Name, d.Parent), suggestion: fmt.Sprintf("Add parent domain '%s' to domains.yaml or fix parent reference for domain '%s'", d.Parent, d.Name)})
			}
		}

		// Cross-entity: ISP field exists (if specified)
		if d.ISP != "" {
			checks++
			if ispMap[d.ISP] == nil {
				errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': isp '%s' not found", d.Name, d.ISP), suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix ISP reference in domains.yaml for domain '%s'", d.ISP, d.Name)})
			}
		}

	}

	// Domain name uniqueness
	domainNames := make(map[string]bool)
	for _, d := range cfg.Domains {
		if domainFilter != "" && !matchesFilter(d.Name, domainFilter) {
			continue
		}
		checks++
		if domainNames[d.Name] {
			errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': duplicate definition", d.Name), suggestion: fmt.Sprintf("Remove duplicate domain '%s' entry from domains.yaml", d.Name)})
		}
		domainNames[d.Name] = true
	}

	// DNS record uniqueness (key = domain:type:name:value)
	recordKeys := make(map[string]bool)
	for _, d := range cfg.Domains {
		if domainFilter != "" && !matchesFilter(d.Name, domainFilter) {
			continue
		}
		for _, r := range d.Records {
			if recordFilter != "" && !matchesRecordFilter(r, recordFilter) {
				continue
			}
			checks++
			key := fmt.Sprintf("%s:%s:%s:%s", d.Name, r.Type, r.Name, r.Value)
			if recordKeys[key] {
				errs = append(errs, validationError{msg: fmt.Sprintf("Domain '%s': duplicate record %s %s -> %s", d.Name, r.Type, r.Name, r.Value), suggestion: fmt.Sprintf("Remove duplicate record %s %s from domains.yaml for domain '%s'", r.Type, r.Name, d.Name)})
			}
			recordKeys[key] = true
		}
	}

	if len(errs) > 0 {
		fmt.Printf("[OK] %d checks passed\n", checks-len(errs))
		fmt.Printf("[FAIL] %d errors found\n", len(errs))
		if len(warns) > 0 {
			fmt.Printf("[WARN] %d warnings\n", len(warns))
		}
		fmt.Println()
		fmt.Println("ERRORS:")
		for i, e := range errs {
			fmt.Printf("  [%d] %s\n", i+1, e.msg)
			fmt.Printf("      Suggestion: %s\n", e.suggestion)
		}
		if len(warns) > 0 {
			fmt.Println()
			fmt.Println("WARNINGS:")
			for i, w := range warns {
				fmt.Printf("  [%d] %s\n", i+1, w.msg)
				fmt.Printf("      Suggestion: %s\n", w.suggestion)
			}
		}
		fmt.Println()
		fmt.Println("Result: FAILED (exit code 2)")
		os.Exit(2)
	}

	fmt.Printf("[OK] %d checks passed\n", checks)
	if len(warns) > 0 {
		fmt.Printf("[WARN] %d warnings\n", len(warns))
	}
	fmt.Println()
	fmt.Println("Result: PASSED")
}

func matchesRecordFilter(r entity.DNSRecord, filter string) bool {
	if filter == "" {
		return true
	}
	recordID := fmt.Sprintf("%s:%s", r.Type, r.Name)
	for _, f := range strings.Split(filter, ",") {
		if strings.TrimSpace(f) == recordID {
			return true
		}
	}
	return false
}
