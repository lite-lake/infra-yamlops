package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
	cfgType = strings.ToLower(cfgType)

	finder := func(cfg *entity.Config, name string) (interface{}, FindResult) {
		switch cfgType {
		case "secret":
			for _, s := range cfg.Secrets {
				if s.Name == name {
					return s, FindResultFound
				}
			}
			return nil, FindResultNotFound
		case "isp":
			if m := cfg.GetISPMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "registry":
			if m := cfg.GetRegistryMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		default:
			return nil, FindResultUnknownType
		}
	}

	validTypes := []string{"secret", "isp", "registry"}
	opts := []ShowOption{WithValidTypes(validTypes)}
	if cfgType == "secret" {
		opts = append(opts, WithWarning("WARNING: This will display sensitive values!"))
	}
	showEntity(ctx, cfgType, name, finder, opts...)
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
