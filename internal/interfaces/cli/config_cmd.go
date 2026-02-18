package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
)

func newConfigCommand(ctx *Context) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration items",
		Long:  "View and manage base configuration items like secrets, ISPs, and registries.",
	}

	configListCmd := &cobra.Command{
		Use:   "list [secrets|isps|registries]",
		Short: "List configuration items",
		Long:  "List configuration items. If no type specified, lists all.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfgType := ""
			if len(args) > 0 {
				cfgType = strings.ToLower(args[0])
			}
			runConfigList(ctx, cfgType)
		},
	}

	configShowCmd := &cobra.Command{
		Use:   "show <type> <name>",
		Short: "Show configuration details",
		Long:  "Show detailed configuration for the specified item.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfgType := strings.ToLower(args[0])
			name := args[1]
			runConfigShow(ctx, cfgType, name)
		},
	}

	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configShowCmd)

	return configCmd
}

func runConfigList(ctx *Context, cfgType string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfgType == "" || cfgType == "secrets" {
		if cfgType == "" {
			fmt.Println("Secrets:")
		}
		for _, s := range cfg.Secrets {
			secretType := "plain"
			if isVaultSecret(s.Value) {
				secretType = "vault"
			}
			fmt.Printf("  - %s (type: %s)\n", s.Name, secretType)
		}
		if cfgType == "" {
			fmt.Println()
		}
	}

	if cfgType == "" || cfgType == "isps" {
		if cfgType == "" {
			fmt.Println("ISPs:")
		}
		for _, i := range cfg.ISPs {
			services := formatISPServices(i.Services)
			fmt.Printf("  - %s (services: %s)\n", i.Name, services)
		}
		if cfgType == "" {
			fmt.Println()
		}
	}

	if cfgType == "" || cfgType == "registries" {
		if cfgType == "" {
			fmt.Println("Registries:")
		}
		for _, r := range cfg.Registries {
			fmt.Printf("  - %s (url: %s)\n", r.Name, r.URL)
		}
	}

	if cfgType != "" && cfgType != "secrets" && cfgType != "isps" && cfgType != "registries" {
		fmt.Fprintf(os.Stderr, "Unknown config type: %s\n", cfgType)
		fmt.Fprintf(os.Stderr, "Valid types: secrets, isps, registries\n")
		os.Exit(1)
	}
}

func runConfigShow(ctx *Context, cfgType, name string) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	var found interface{}

	switch cfgType {
	case "secret":
		for _, s := range cfg.Secrets {
			if s.Name == name {
				found = s
				break
			}
		}
	case "isp":
		if m := cfg.GetISPMap(); m[name] != nil {
			found = m[name]
		}
	case "registry":
		if m := cfg.GetRegistryMap(); m[name] != nil {
			found = m[name]
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown config type: %s\n", cfgType)
		fmt.Fprintf(os.Stderr, "Valid types: secret, isp, registry\n")
		os.Exit(1)
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "%s '%s' not found\n", cfgType, name)
		os.Exit(1)
	}

	if cfgType == "secret" {
		fmt.Println("WARNING: This will display sensitive values!")
		fmt.Println()
	}

	data, err := yaml.Marshal(found)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", strings.Title(cfgType), name)
	fmt.Println(string(data))
}

func isVaultSecret(value string) bool {
	return strings.HasPrefix(value, "vault:")
}

func formatISPServices(services []entity.ISPService) string {
	var result []string
	for _, s := range services {
		result = append(result, string(s))
	}
	return fmt.Sprintf("[%s]", strings.Join(result, ", "))
}
