package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
)

func newServerValidateCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Zone   string
		Server string
	}
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate server configuration",
		Long:  "Validate server configuration: name, zone, IP, SSH, networks, and cross-entity references.",
		Run: func(cmd *cobra.Command, args []string) {
			runServerValidate(ctx, filters.Zone, filters.Server)
		},
	}
	cmd.Flags().StringVar(&filters.Zone, "zone", "", "Zone filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Server, "server", "", "Server filter (comma-separated)")
	return cmd
}

func runServerValidate(ctx *Context, zoneFilter, serverFilter string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Printf("Validating server configuration (%s)...\n\n", ctx.Env)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("Validating server configuration (%s)...\n\n", ctx.Env)

	var valErrors []struct {
		msg        string
		suggestion string
	}
	checks := 0
	secretMap := cfg.GetSecretsMap()
	ispMap := cfg.GetISPMap()
	zoneMap := cfg.GetZoneMap()

	for _, srv := range cfg.Servers {
		if serverFilter != "" && !matchesFilter(srv.Name, serverFilter) {
			continue
		}
		if zoneFilter != "" && !matchesFilter(srv.Zone, zoneFilter) {
			continue
		}

		if srv.Name == "" {
			checks++
			valErrors = append(valErrors, struct {
				msg        string
				suggestion string
			}{
				msg:        "Server: name is empty",
				suggestion: "Add a 'name' field to the server definition in servers.yaml",
			})
			continue
		}

		// Entity-level validation
		if err := srv.Validate(); err != nil {
			checks++
			valErrors = append(valErrors, struct {
				msg        string
				suggestion string
			}{
				msg:        fmt.Sprintf("Server '%s': %s", srv.Name, err.Error()),
				suggestion: fmt.Sprintf("Fix the validation error in servers.yaml for server '%s'", srv.Name),
			})
		} else {
			checks++
		}

		// Cross-entity: zone exists
		checks++
		if srv.Zone != "" && zoneMap[srv.Zone] == nil {
			valErrors = append(valErrors, struct {
				msg        string
				suggestion string
			}{
				msg:        fmt.Sprintf("Server '%s': zone '%s' not found", srv.Name, srv.Zone),
				suggestion: fmt.Sprintf("Add zone '%s' to zones.yaml or fix the zone name in servers.yaml", srv.Zone),
			})
		}

		// Cross-entity: ISP exists
		if srv.ISP != "" {
			checks++
			if ispMap[srv.ISP] == nil {
				valErrors = append(valErrors, struct {
					msg        string
					suggestion string
				}{
					msg:        fmt.Sprintf("Server '%s': isp '%s' not found", srv.Name, srv.ISP),
					suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix the isp name in servers.yaml", srv.ISP),
				})
			}
		}

		// Cross-entity: SSH password secret exists
		if srv.SSH.Password.Secret() != "" {
			checks++
			if _, ok := secretMap[srv.SSH.Password.Secret()]; !ok {
				valErrors = append(valErrors, struct {
					msg        string
					suggestion string
				}{
					msg:        fmt.Sprintf("Server '%s': ssh password secret '%s' not found", srv.Name, srv.SSH.Password.Secret()),
					suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml", srv.SSH.Password.Secret()),
				})
			}
		}

		// Cross-entity: SSH password resolvable
		if srv.SSH.Password.Secret() != "" || srv.SSH.Password.Plain() != "" {
			checks++
			if _, err := srv.SSH.Password.Resolve(secretMap); err != nil {
				valErrors = append(valErrors, struct {
					msg        string
					suggestion string
				}{
					msg:        fmt.Sprintf("Server '%s': ssh password cannot be resolved: %v", srv.Name, err),
					suggestion: fmt.Sprintf("Check secret value '%s' in secrets.yaml for server '%s'", srv.SSH.Password.Secret(), srv.Name),
				})
			}
		}
	}

	if len(valErrors) > 0 {
		fmt.Printf("[OK] %d checks passed\n", checks-len(valErrors))
		fmt.Printf("[FAIL] %d errors found\n", len(valErrors))
		fmt.Println()
		fmt.Println("ERRORS:")
		for i, e := range valErrors {
			fmt.Printf("  [%d] %s\n", i+1, e.msg)
			fmt.Printf("      Suggestion: %s\n", e.suggestion)
			fmt.Println()
		}
		fmt.Println("Result: FAILED (exit code 2)")
		os.Exit(2)
	}

	fmt.Printf("[OK] %d checks passed\n", checks)
	fmt.Println()
	fmt.Println("Result: PASSED")
}

func matchesFilter(value, filter string) bool {
	if filter == "" {
		return true
	}
	for _, f := range strings.Split(filter, ",") {
		if strings.TrimSpace(f) == value {
			return true
		}
	}
	return false
}
