package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
)

func newConfigCommand(ctx *Context) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "View configuration",
		Long: `View and validate configuration: ISPs, registries, secrets.

Commands:
  show <entity> [name]  Show configuration (isps, registries, secrets)
  validate              Validate ISP/Registry/Secret configuration

Examples:
  yamlops cli config show isps -e prod
  yamlops cli config show isps -e prod --detail
  yamlops cli config show registries -e prod
  yamlops cli config show secrets -e prod
  yamlops cli config validate -e prod`,
	}

	configCmd.AddCommand(newConfigShowCommand(ctx))
	configCmd.AddCommand(newConfigValidateCommand(ctx))

	return configCmd
}

func newConfigShowCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <entity> [name]",
		Short: "Show configuration details",
		Long:  "Show configuration for ISPs, registries, or secrets. For isps/registries, use name to show a specific entry. Use --detail for detailed view.",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			entityType := strings.ToLower(args[0])
			name := ""
			if len(args) > 1 {
				if entityType == "secrets" || entityType == "secret" {
					fmt.Fprintf(os.Stderr, "Error: 'secrets' does not support name argument\n")
					fmt.Fprintf(os.Stderr, "Suggestion: Use 'config show secrets -e <env>' to list all secrets\n")
					os.Exit(1)
				}
				name = args[1]
			}
			detail, _ := cmd.Flags().GetBool("detail")
			runConfigShowUnified(ctx, entityType, name, detail)
		},
	}
	cmd.Flags().Bool("detail", false, "Show detailed information")
	return cmd
}

func runConfigShowUnified(ctx *Context, entityType, name string, detail bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch entityType {
	case "isps", "isp":
		showISPs(cfg, name, detail)
	case "registries", "registry":
		showRegistries(cfg, name, detail)
	case "secrets", "secret":
		showSecrets(cfg, name, detail)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown entity type '%s'\n", entityType)
		fmt.Fprintf(os.Stderr, "Suggestion: Use 'isps', 'registries', or 'secrets'\n")
		os.Exit(1)
	}
}

func showISPs(cfg *entity.Config, name string, detail bool) {
	var isps []entity.ISP
	if name != "" {
		ispMap := cfg.GetISPMap()
		isp := ispMap[name]
		if isp == nil {
			var names []string
			for n := range ispMap {
				names = append(names, n)
			}
			sort.Strings(names)
			fmt.Fprintf(os.Stderr, "Error: ISP '%s' not found\n", name)
			fmt.Fprintf(os.Stderr, "Details: Available ISPs: %s\n", strings.Join(names, ", "))
			fmt.Fprintf(os.Stderr, "Suggestion: Run 'config show isps' to list available ISPs\n")
			os.Exit(1)
		}
		isps = append(isps, *isp)
	} else {
		isps = cfg.ISPs
	}

	sort.Slice(isps, func(i, j int) bool { return isps[i].Name < isps[j].Name })

	fmt.Printf("%-16s %-12s %s\n", "ISP", "TYPE", "SERVICES")
	if len(isps) == 0 {
		fmt.Printf("\nTotal: %d ISPs\n", 0)
		return
	}
	for _, isp := range isps {
		services := formatISPServicesShort(isp.Services)
		fmt.Printf("%-16s %-12s %s\n", isp.Name, isp.Type, services)
	}

	if detail {
		fmt.Println()
		for i, isp := range isps {
			fmt.Printf("ISP: %s\n", isp.Name)
			fmt.Printf("  Type:        %s\n", isp.Type)
			fmt.Printf("  Services:    %s\n", formatISPServicesShort(isp.Services))
			if len(isp.Regions) > 0 {
				fmt.Printf("  Regions:     %s\n", strings.Join(isp.Regions, ", "))
			}
			fmt.Printf("  Endpoint:    %s\n", getISPEndpoint(isp))
			if i < len(isps)-1 {
				fmt.Println()
			}
		}
	}

	fmt.Printf("\nTotal: %d ISPs\n", len(isps))
}

func showRegistries(cfg *entity.Config, name string, detail bool) {
	var registries []entity.Registry
	if name != "" {
		reg := cfg.GetRegistryMap()[name]
		if reg == nil {
			var names []string
			for n := range cfg.GetRegistryMap() {
				names = append(names, n)
			}
			sort.Strings(names)
			fmt.Fprintf(os.Stderr, "Error: Registry '%s' not found\n", name)
			fmt.Fprintf(os.Stderr, "Details: Available registries: %s\n", strings.Join(names, ", "))
			fmt.Fprintf(os.Stderr, "Suggestion: Run 'config show registries' to list available registries\n")
			os.Exit(1)
		}
		registries = append(registries, *reg)
	} else {
		registries = cfg.Registries
	}

	sort.Slice(registries, func(i, j int) bool { return registries[i].Name < registries[j].Name })

	fmt.Printf("%-16s %-26s %s\n", "REGISTRY", "URL", "NAMESPACE")
	if len(registries) == 0 {
		fmt.Printf("\nTotal: %d registries\n", 0)
		return
	}
	for _, reg := range registries {
		namespace := reg.Namespace
		if namespace == "" {
			namespace = extractNamespace(reg.URL)
		}
		fmt.Printf("%-16s %-26s %s\n", reg.Name, reg.URL, namespace)
	}

	if detail {
		fmt.Println()
		for i, reg := range registries {
			namespace := reg.Namespace
			if namespace == "" {
				namespace = extractNamespace(reg.URL)
			}
			fmt.Printf("REGISTRY: %s\n", reg.Name)
			fmt.Printf("  URL:         %s\n", reg.URL)
			fmt.Printf("  Namespace:   %s\n", namespace)
			authStatus := "not configured"
			if reg.Credentials.Username.Secret() != "" || reg.Credentials.Username.Plain() != "" ||
				reg.Credentials.Password.Secret() != "" || reg.Credentials.Password.Plain() != "" {
				authStatus = "configured"
			}
			fmt.Printf("  Auth:        %s\n", authStatus)
			if i < len(registries)-1 {
				fmt.Println()
			}
		}
	}

	fmt.Printf("\nTotal: %d registries\n", len(registries))
}

func showSecrets(cfg *entity.Config, name string, detail bool) {
	var secrets []entity.Secret
	if name != "" {
		for _, s := range cfg.Secrets {
			if s.Name == name {
				secrets = append(secrets, s)
				break
			}
		}
		if len(secrets) == 0 {
			fmt.Fprintf(os.Stderr, "Error: Secret '%s' not found\n", name)
			os.Exit(1)
		}
	} else {
		secrets = cfg.Secrets
	}

	sort.Slice(secrets, func(i, j int) bool { return secrets[i].Name < secrets[j].Name })

	if len(secrets) == 0 {
		fmt.Println("No secrets found.")
		return
	}

	if detail {
		fmt.Printf("%-20s %-16s %s\n", "KEY", "SOURCE", "DESCRIPTION")
		for _, s := range secrets {
			source := "secrets.yaml"
			description := ""
			fmt.Printf("%-20s %-16s %s\n", s.Name, source, description)
		}
	} else {
		fmt.Println("KEY")
		for _, s := range secrets {
			fmt.Println(s.Name)
		}
	}

	fmt.Printf("\nTotal: %d secrets\n", len(secrets))
}

func formatISPServicesShort(services []entity.ISPService) string {
	var result []string
	for _, s := range services {
		result = append(result, string(s))
	}
	return strings.Join(result, ", ")
}

func getISPEndpoint(isp entity.ISP) string {
	if endpoint, ok := isp.Credentials["endpoint"]; ok {
		if endpoint.Plain() != "" {
			return endpoint.Plain()
		}
	}
	// Default endpoints based on type
	switch isp.Type {
	case "aliyun":
		return "dns.aliyuncs.com"
	case "cloudflare":
		return "api.cloudflare.com"
	case "tencent":
		return "dnspod.tencentcloudapi.com"
	default:
		return ""
	}
}

func extractNamespace(url string) string {
	// Extract namespace from registry URL patterns
	// e.g., mirror.litelake.com/litelake -> litelake
	// ghcr.io/litelake -> litelake
	parts := strings.Split(url, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}
