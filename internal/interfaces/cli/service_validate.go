package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newServiceValidateCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate service configuration",
		Long:  "Validate service configuration: entity fields, cross-entity references, port conflicts, and name uniqueness.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceValidate(ctx, *filters)
		},
	}
}

func runServiceValidate(ctx *Context, filters ServiceCmdFilters) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadAndValidate(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validating service configuration (%s)...\n\n", ctx.Env)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("Validating service configuration (%s)...\n\n", ctx.Env)

	var errors []validationError
	var warnings []validationError
	checks := 0

	serverMap := cfg.GetServerMap()
	secretMap := cfg.GetSecretsMap()

	serviceTypes, err := parseServiceTypes(filters.Type)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	showBiz := len(serviceTypes) == 0 || containsStr(serviceTypes, "biz")
	showInfra := len(serviceTypes) == 0 || containsStr(serviceTypes, "infra")

	// Name uniqueness across BizService and InfraService
	allNames := make(map[string]string)
	if showBiz {
		for _, svc := range cfg.Services {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			if srv := serverMap[svc.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if existing, ok := allNames[svc.Name]; ok {
				errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': name conflicts with %s", svc.Name, existing), suggestion: fmt.Sprintf("Rename one of the conflicting services in services.yaml")})
			} else {
				allNames[svc.Name] = "biz service"
			}
		}
	}
	if showInfra {
		for _, infra := range cfg.InfraServices {
			if filters.Server != "" && !matchesFilter(infra.Server, filters.Server) {
				continue
			}
			if srv := serverMap[infra.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if existing, ok := allNames[infra.Name]; ok {
				errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': name conflicts with %s", infra.Name, existing), suggestion: fmt.Sprintf("Rename one of the conflicting services in services.yaml")})
			} else {
				allNames[infra.Name] = "infra service"
			}
		}
	}

	// BizService validation
	if showBiz {
		for _, svc := range cfg.Services {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			if srv := serverMap[svc.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}

			// Entity-level validation
			if err := svc.Validate(); err != nil {
				errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': %s", svc.Name, err.Error()), suggestion: fmt.Sprintf("Fix validation error for service '%s' in services.yaml", svc.Name)})
			} else {
				checks++
			}

			// Cross-entity: server exists
			if serverMap[svc.Server] == nil {
				errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': server '%s' not found", svc.Name, svc.Server), suggestion: fmt.Sprintf("Add server '%s' to servers.yaml or fix server reference for service '%s'", svc.Server, svc.Name)})
			} else {
				checks++
			}

			// Cross-entity: secrets exist
			for _, secretName := range svc.Secrets {
				if _, ok := secretMap[secretName]; !ok {
					errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': secret '%s' not found", svc.Name, secretName), suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix secret reference for service '%s'", secretName, svc.Name)})
				} else {
					checks++
				}
			}

			// Cross-entity: env SecretRef resolvable
			for envKey, ref := range svc.Env {
				if ref.Secret() != "" {
					if _, ok := secretMap[ref.Secret()]; !ok {
						errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': env '%s' references unknown secret '%s'", svc.Name, envKey, ref.Secret()), suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix env reference for service '%s'", ref.Secret(), svc.Name)})
					} else {
						checks++
					}
				} else {
					checks++
				}
			}

			// Cross-entity: registry exists (if specified)
			if svc.Registry != "" {
				if cfg.GetRegistryMap()[svc.Registry] == nil {
					errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': registry '%s' not found", svc.Name, svc.Registry), suggestion: fmt.Sprintf("Add registry '%s' to registries.yaml or fix registry reference for service '%s'", svc.Registry, svc.Name)})
				} else {
					checks++
				}
			}

			// Warning: no healthcheck
			if len(svc.Gateways) > 0 && (svc.Healthcheck == nil || svc.Healthcheck.Path == "") {
				warnings = append(warnings, validationError{msg: fmt.Sprintf("Service '%s': no healthcheck configured", svc.Name), suggestion: fmt.Sprintf("Add healthcheck for production service '%s'", svc.Name)})
			}
		}
	}

	// InfraService validation
	if showInfra {
		for _, infra := range cfg.InfraServices {
			if filters.Server != "" && !matchesFilter(infra.Server, filters.Server) {
				continue
			}
			if srv := serverMap[infra.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}

			// Entity-level validation
			if err := infra.Validate(); err != nil {
				errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': %s", infra.Name, err.Error()), suggestion: fmt.Sprintf("Fix validation error for infra service '%s' in services.yaml", infra.Name)})
			} else {
				checks++
			}

			// Cross-entity: server exists
			if serverMap[infra.Server] == nil {
				errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': server '%s' not found", infra.Name, infra.Server), suggestion: fmt.Sprintf("Add server '%s' to servers.yaml or fix server reference for infra service '%s'", infra.Server, infra.Name)})
			} else {
				checks++
			}

			// Cross-entity: secrets exist
			for _, secretName := range infra.Secrets {
				if _, ok := secretMap[secretName]; !ok {
					errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': secret '%s' not found", infra.Name, secretName), suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix secret reference for infra service '%s'", secretName, infra.Name)})
				} else {
					checks++
				}
			}

			// Cross-entity: env SecretRef resolvable
			for envKey, ref := range infra.Env {
				if ref.Secret() != "" {
					if _, ok := secretMap[ref.Secret()]; !ok {
						errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': env '%s' references unknown secret '%s'", infra.Name, envKey, ref.Secret()), suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix env reference for infra service '%s'", ref.Secret(), infra.Name)})
					} else {
						checks++
					}
				} else {
					checks++
				}
			}
		}
	}

	// Port conflict detection (across all services on same server)
	portsByServer := make(map[string]map[int]string)
	if showBiz {
		for _, svc := range cfg.Services {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			if srv := serverMap[svc.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if portsByServer[svc.Server] == nil {
				portsByServer[svc.Server] = make(map[int]string)
			}
			for _, p := range svc.Ports {
				if existing, ok := portsByServer[svc.Server][p.Host]; ok {
					errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': port %d conflicts with '%s' on server '%s'", svc.Name, p.Host, existing, svc.Server), suggestion: fmt.Sprintf("Change port %d in service '%s' to avoid conflict", p.Host, svc.Name)})
				} else {
					portsByServer[svc.Server][p.Host] = svc.Name
					checks++
				}
			}
		}
	}
	if showInfra {
		for _, infra := range cfg.InfraServices {
			if filters.Server != "" && !matchesFilter(infra.Server, filters.Server) {
				continue
			}
			if srv := serverMap[infra.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			if portsByServer[infra.Server] == nil {
				portsByServer[infra.Server] = make(map[int]string)
			}
			if infra.GatewayPorts != nil {
				for _, port := range []int{infra.GatewayPorts.HTTP, infra.GatewayPorts.HTTPS} {
					if port == 0 {
						continue
					}
					if existing, ok := portsByServer[infra.Server][port]; ok {
						errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': port %d conflicts with '%s' on server '%s'", infra.Name, port, existing, infra.Server), suggestion: fmt.Sprintf("Change port %d in infra service '%s' to avoid conflict", port, infra.Name)})
					} else {
						portsByServer[infra.Server][port] = infra.Name
						checks++
					}
				}
			}
			for _, p := range infra.Ports {
				if existing, ok := portsByServer[infra.Server][p.Host]; ok {
					errors = append(errors, validationError{msg: fmt.Sprintf("InfraService '%s': port %d conflicts with '%s' on server '%s'", infra.Name, p.Host, existing, infra.Server), suggestion: fmt.Sprintf("Change port %d in infra service '%s' to avoid conflict", p.Host, infra.Name)})
				} else {
					portsByServer[infra.Server][p.Host] = infra.Name
					checks++
				}
			}
		}
	}

	// Gateway hostname uniqueness (gateway routes are defined on BizService only)
	hostnames := make(map[string]string)
	if showBiz {
		for _, svc := range cfg.Services {
			if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
				continue
			}
			if srv := serverMap[svc.Server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
				continue
			}
			for _, g := range svc.Gateways {
				if existing, ok := hostnames[g.Hostname]; ok {
					errors = append(errors, validationError{msg: fmt.Sprintf("Service '%s': gateway hostname '%s' conflicts with '%s'", svc.Name, g.Hostname, existing), suggestion: fmt.Sprintf("Change gateway hostname in service '%s' to avoid conflict", svc.Name)})
				} else {
					hostnames[g.Hostname] = svc.Name
					checks++
				}
			}
		}
	}

	// Output results
	if len(errors) > 0 {
		fmt.Printf("[OK] %d checks passed\n", checks)
		fmt.Printf("[FAIL] %d errors found\n", len(errors))
		if len(warnings) > 0 {
			warnWord := "warnings"
			if len(warnings) == 1 {
				warnWord = "warning"
			}
			fmt.Printf("[WARN] %d %s\n", len(warnings), warnWord)
		}
		fmt.Println()
		fmt.Println("ERRORS:")
		for i, e := range errors {
			fmt.Printf("  [%d] %s\n", i+1, e.msg)
			fmt.Printf("      Suggestion: %s\n", e.suggestion)
		}
		if len(warnings) > 0 {
			fmt.Println()
			fmt.Println("WARNINGS:")
			for i, w := range warnings {
				fmt.Printf("  [%d] %s\n", i+1, w.msg)
				fmt.Printf("      Suggestion: %s\n", w.suggestion)
			}
		}
		fmt.Println()
		fmt.Println("Result: FAILED (exit code 2)")
		os.Exit(2)
	}

	fmt.Printf("[OK] %d checks passed\n", checks)
	if len(warnings) > 0 {
		warnWord := "warnings"
		if len(warnings) == 1 {
			warnWord = "warning"
		}
		fmt.Printf("[WARN] %d %s\n", len(warnings), warnWord)
	}
	fmt.Println()
	fmt.Println("Result: PASSED")
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func parseServiceTypes(typeStr string) ([]string, error) {
	if typeStr == "" {
		return nil, nil
	}
	parts := strings.Split(typeStr, ",")
	var result []string
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "biz" || p == "infra" {
			if !seen[p] {
				result = append(result, p)
				seen[p] = true
			}
		} else {
			return nil, fmt.Errorf("invalid service type '%s'\nDetails: Allowed values: biz, infra, biz,infra\nSuggestion: Use a valid service type filter", p)
		}
	}
	return result, nil
}
