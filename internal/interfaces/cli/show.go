package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/domain/entity"
)

func newShowCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <entity> <name>",
		Short: "Show entity details",
		Long:  "Show detailed information for the specified entity.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			entityType := args[0]
			name := args[1]
			runShow(ctx, entityType, name)
		},
	}

	return cmd
}

func runShow(ctx *Context, entityType, name string) {
	entityType = strings.ToLower(entityType)

	finder := func(cfg *entity.Config, name string) (interface{}, FindResult) {
		switch entityType {
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
		case "zone":
			if m := cfg.GetZoneMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "infra_service":
			if m := cfg.GetInfraServiceMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "server":
			if m := cfg.GetServerMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "service":
			if m := cfg.GetServiceMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "registry":
			if m := cfg.GetRegistryMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		case "domain":
			if m := cfg.GetDomainMap(); m[name] != nil {
				return m[name], FindResultFound
			}
			return nil, FindResultNotFound
		default:
			return nil, FindResultUnknownType
		}
	}

	validTypes := []string{"secret", "isp", "zone", "infra_service", "server", "service", "registry", "domain"}
	showEntity(ctx, entityType, name, finder, WithValidTypes(validTypes))
}
