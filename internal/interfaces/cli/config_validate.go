package cli

import (
	"fmt"
	"net/url"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
)

var validISPTypes = map[entity.ISPType]bool{
	entity.ISPTypeAliyun:     true,
	entity.ISPTypeCloudflare: true,
	entity.ISPTypeTencent:    true,
}

type validationError struct {
	msg        string
	suggestion string
}

func newConfigValidateCommand(ctx *Context) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Long:  "Validate ISP, registry, and secret configuration completeness: fields, references, and naming conventions.",
		Run: func(cmd *cobra.Command, args []string) {
			runConfigValidate(ctx)
		},
	}
}

func runConfigValidate(ctx *Context) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validating configuration (%s)...\n\n", ctx.Env)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("Validating configuration (%s)...\n\n", ctx.Env)

	var errs []validationError
	checks := 0

	// ISP validation
	ispNames := make(map[string]bool)
	for _, isp := range cfg.ISPs {
		checks++
		if isp.Name == "" {
			errs = append(errs, validationError{msg: "ISP: name is empty", suggestion: "Add a name field to the ISP definition in isps.yaml"})
			continue
		}
		if ispNames[isp.Name] {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': duplicate definition", isp.Name),
				suggestion: fmt.Sprintf("Remove duplicate ISP '%s' entry from isps.yaml", isp.Name),
			})
		}
		ispNames[isp.Name] = true

		checks++
		if isp.Type == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': type is empty", isp.Name),
				suggestion: fmt.Sprintf("Add type to isps.yaml for ISP '%s' (aliyun, cloudflare, or tencent)", isp.Name),
			})
		} else if !validISPTypes[isp.Type] {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': invalid type '%s' (must be one of: aliyun, cloudflare, tencent)", isp.Name, isp.Type),
				suggestion: fmt.Sprintf("Fix type in isps.yaml for ISP '%s'", isp.Name),
			})
		}
		checks++
		if len(isp.Services) == 0 {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': services list is empty", isp.Name),
				suggestion: fmt.Sprintf("Add services to isps.yaml for ISP '%s'", isp.Name),
			})
		}
		checks++
		if len(isp.Credentials) == 0 {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': credentials not configured", isp.Name),
				suggestion: fmt.Sprintf("Add credentials to isps.yaml for ISP '%s'", isp.Name),
			})
		} else {
			for key, ref := range isp.Credentials {
				checks++
				if err := ref.Validate(); err != nil {
					errs = append(errs, validationError{
						msg:        fmt.Sprintf("ISP '%s': credential '%s' is invalid: %v", isp.Name, key, err),
						suggestion: fmt.Sprintf("Fix credential '%s' in isps.yaml for ISP '%s'", key, isp.Name),
					})
				}
			}
		}
		checks++
		endpoint, hasEndpoint := isp.Credentials["endpoint"]
		if !hasEndpoint || (endpoint.Plain() == "" && endpoint.Secret() == "") {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("ISP '%s': missing required field 'endpoint'", isp.Name),
				suggestion: fmt.Sprintf("Add endpoint to isps.yaml for ISP '%s'", isp.Name),
			})
		}
	}

	// Registry validation
	registryNames := make(map[string]bool)
	for _, reg := range cfg.Registries {
		checks++
		if reg.Name == "" {
			errs = append(errs, validationError{msg: "Registry: name is empty", suggestion: "Add a name field to the registry definition in registries.yaml"})
			continue
		}
		if registryNames[reg.Name] {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': duplicate definition", reg.Name),
				suggestion: fmt.Sprintf("Remove duplicate registry '%s' entry from registries.yaml", reg.Name),
			})
		}
		registryNames[reg.Name] = true

		checks++
		if reg.URL == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': url is empty", reg.Name),
				suggestion: fmt.Sprintf("Add url to registries.yaml for registry '%s'", reg.Name),
			})
		} else if _, err := url.Parse(reg.URL); err != nil {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': url format is invalid: %v", reg.Name, err),
				suggestion: fmt.Sprintf("Fix url in registries.yaml for registry '%s'", reg.Name),
			})
		}
		checks++
		if reg.Namespace == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': namespace is empty", reg.Name),
				suggestion: fmt.Sprintf("Add namespace to registries.yaml for registry '%s'", reg.Name),
			})
		}
		checks++
		if reg.Credentials.Username.Secret() == "" && reg.Credentials.Username.Plain() == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': username not configured", reg.Name),
				suggestion: fmt.Sprintf("Add username to registries.yaml for registry '%s'", reg.Name),
			})
		}
		checks++
		if reg.Credentials.Password.Secret() == "" && reg.Credentials.Password.Plain() == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Registry '%s': password not configured", reg.Name),
				suggestion: fmt.Sprintf("Add password to registries.yaml for registry '%s'", reg.Name),
			})
		}
	}

	// Secret validation
	secretNames := make(map[string]bool)
	validKeyRegex := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, s := range cfg.Secrets {
		checks++
		if s.Name == "" {
			errs = append(errs, validationError{msg: "Secret: key is empty", suggestion: "Add a key field to the secret definition in secrets.yaml"})
			continue
		}
		if secretNames[s.Name] {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Secret '%s': duplicate definition", s.Name),
				suggestion: fmt.Sprintf("Remove duplicate secret '%s' entry from secrets.yaml", s.Name),
			})
		}
		secretNames[s.Name] = true

		checks++
		if !isValidSecretKey(s.Name) {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Secret '%s': key contains invalid characters (use lowercase, digits, underscores, must start with letter)", s.Name),
				suggestion: fmt.Sprintf("Rename secret key '%s' to use only lowercase letters, digits, and underscores in secrets.yaml", s.Name),
			})
		} else if !validKeyRegex.MatchString(s.Name) {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Secret '%s': key must start with a lowercase letter", s.Name),
				suggestion: fmt.Sprintf("Rename secret key '%s' to start with a lowercase letter in secrets.yaml", s.Name),
			})
		}

		checks++
		if s.Source == "" {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Secret '%s': source is empty", s.Name),
				suggestion: fmt.Sprintf("Add source file for secret '%s' in secrets.yaml", s.Name),
			})
		} else if _, statErr := os.Stat(s.Source); os.IsNotExist(statErr) {
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Secret '%s' references source file '%s' that does not exist", s.Name, s.Source),
				suggestion: "Create the source file or update the secret configuration",
			})
		}
	}

	// Cross-reference validation
	ispMap := cfg.GetISPMap()
	registryMap := cfg.GetRegistryMap()

	// Server -> ISP/Registry references
	zoneNames := make(map[string]bool)
	for _, z := range cfg.Zones {
		zoneNames[z.Name] = true
	}
	for _, srv := range cfg.Servers {
		if srv.Zone != "" && !zoneNames[srv.Zone] {
			checks++
			errs = append(errs, validationError{
				msg:        fmt.Sprintf("Server '%s': zone '%s' does not exist", srv.Name, srv.Zone),
				suggestion: fmt.Sprintf("Add zone '%s' to zones.yaml or fix zone reference in servers.yaml for server '%s'", srv.Zone, srv.Name),
			})
		}
		if srv.ISP != "" {
			checks++
			if _, ok := ispMap[srv.ISP]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Server '%s': isp '%s' does not exist", srv.Name, srv.ISP),
					suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix ISP reference in servers.yaml for server '%s'", srv.ISP, srv.Name),
				})
			}
		}
		for _, regName := range srv.Environment.Registries {
			checks++
			if _, ok := registryMap[regName]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Server '%s': registry '%s' does not exist", srv.Name, regName),
					suggestion: fmt.Sprintf("Add registry '%s' to registries.yaml or fix reference in servers.yaml for server '%s'", regName, srv.Name),
				})
			}
		}
	}

	// BizService -> Registry/Secret references
	for _, svc := range cfg.Services {
		if svc.Registry != "" {
			checks++
			if _, ok := registryMap[svc.Registry]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Service '%s': registry '%s' does not exist", svc.Name, svc.Registry),
					suggestion: fmt.Sprintf("Add registry '%s' to registries.yaml or fix registry reference for service '%s'", svc.Registry, svc.Name),
				})
			}
		}
		for _, secretName := range svc.Secrets {
			checks++
			if _, ok := secretNames[secretName]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Service '%s': secret '%s' does not exist", svc.Name, secretName),
					suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix secret reference for service '%s'", secretName, svc.Name),
				})
			}
		}
	}

	// InfraService -> Secret references
	for _, infra := range cfg.InfraServices {
		for _, secretName := range infra.Secrets {
			checks++
			if _, ok := secretNames[secretName]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("InfraService '%s': secret '%s' does not exist", infra.Name, secretName),
					suggestion: fmt.Sprintf("Add secret '%s' to secrets.yaml or fix secret reference for infra service '%s'", secretName, infra.Name),
				})
			}
		}
	}

	// Domain -> ISP references
	for _, d := range cfg.Domains {
		if d.ISP != "" {
			checks++
			if _, ok := ispMap[d.ISP]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Domain '%s': isp '%s' does not exist", d.Name, d.ISP),
					suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix ISP reference in domains.yaml for domain '%s'", d.ISP, d.Name),
				})
			}
		}
		if d.DNSISP != "" {
			checks++
			if _, ok := ispMap[d.DNSISP]; !ok {
				errs = append(errs, validationError{
					msg:        fmt.Sprintf("Domain '%s': dns_isp '%s' does not exist", d.Name, d.DNSISP),
					suggestion: fmt.Sprintf("Add ISP '%s' to isps.yaml or fix dns_isp reference in domains.yaml for domain '%s'", d.DNSISP, d.Name),
				})
			}
		}
	}

	if len(errs) > 0 {
		fmt.Printf("[OK] %d checks passed\n", checks-len(errs))
		fmt.Printf("[FAIL] %d errors found\n", len(errs))
		fmt.Println()
		fmt.Println("ERRORS:")
		for i, e := range errs {
			fmt.Printf("  [%d] %s\n", i+1, e.msg)
			fmt.Printf("      Suggestion: %s\n", e.suggestion)
		}
		fmt.Println()
		fmt.Println("Result: FAILED (exit code 2)")
		os.Exit(2)
	}

	fmt.Printf("[OK] %d checks passed\n", checks)
	fmt.Println()
	fmt.Println("Result: PASSED")
}

func isValidSecretKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
