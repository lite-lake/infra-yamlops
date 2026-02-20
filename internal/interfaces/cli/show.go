package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newShowCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <entity> <name>",
		Short: "Show entity details",
		Long:  "Show detailed information for the specified entity.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			entity := args[0]
			name := args[1]
			runShow(ctx, entity, name)
		},
	}

	return cmd
}

func runShow(ctx *Context, entity, name string) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	entity = strings.ToLower(entity)
	var found interface{}

	switch entity {
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
	case "zone":
		if m := cfg.GetZoneMap(); m[name] != nil {
			found = m[name]
		}
	case "infra_service":
		if m := cfg.GetInfraServiceMap(); m[name] != nil {
			found = m[name]
		}
	case "server":
		if m := cfg.GetServerMap(); m[name] != nil {
			found = m[name]
		}
	case "service":
		if m := cfg.GetServiceMap(); m[name] != nil {
			found = m[name]
		}
	case "registry":
		if m := cfg.GetRegistryMap(); m[name] != nil {
			found = m[name]
		}
	case "domain":
		if m := cfg.GetDomainMap(); m[name] != nil {
			found = m[name]
		}
	case "certificate", "cert":
		if m := cfg.GetCertificateMap(); m[name] != nil {
			found = m[name]
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entity)
		os.Exit(1)
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "%s '%s' not found\n", entity, name)
		os.Exit(1)
	}

	data, err := yaml.Marshal(found)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling entity: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", strings.Title(entity), name)
	fmt.Println(string(data))
}
