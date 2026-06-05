package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

func newListCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <entity>",
		Short: "List entities",
		Long:  "List all entities of the specified type.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			entityType := args[0]
			runList(ctx, entityType)
		},
	}

	return cmd
}

func runList(ctx *Context, entityType string) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	entityType = strings.ToLower(entityType)
	switch entityType {
	case "secrets":
		for _, s := range cfg.Secrets {
			fmt.Printf("- %s\n", s.Name)
		}
	case "isps":
		for _, i := range cfg.ISPs {
			fmt.Printf("- %s (services: %v)\n", i.Name, i.Services)
		}
	case "zones":
		for _, z := range cfg.Zones {
			fmt.Printf("- %s (isp: %s, region: %s)\n", z.Name, z.ISP, z.Region)
		}
	case "servers":
		for _, s := range cfg.Servers {
			fmt.Printf("- %s (zone: %s, ip: %s)\n", s.Name, s.Zone, s.IP.Public)
		}
	case "services":
		for _, s := range cfg.Services {
			portStr := ""
			for i, p := range s.Ports {
				if i > 0 {
					portStr += ", "
				}
				portStr += fmt.Sprintf("%d->%d", p.Host, p.Container)
			}
			fmt.Printf("- %s (server: %s, ports: %s)\n", s.Name, s.Server, portStr)
		}
	case "registries":
		for _, r := range cfg.Registries {
			fmt.Printf("- %s (%s)\n", r.Name, r.URL)
		}
	case "domains":
		domains := append([]entity.Domain(nil), cfg.Domains...)
		sort.Slice(domains, func(i, j int) bool {
			return domains[i].Name < domains[j].Name
		})
		for _, d := range domains {
			fmt.Printf("- %s (isp: %s)\n", d.Name, d.ISP)
		}
	case "records", "dns":
		records := cfg.GetAllDNSRecords()
		sort.Slice(records, func(i, j int) bool {
			if records[i].Domain != records[j].Domain {
				return records[i].Domain < records[j].Domain
			}
			if records[i].Type != records[j].Type {
				return records[i].Type < records[j].Type
			}
			return records[i].Name < records[j].Name
		})
		for _, r := range records {
			fmt.Printf("- %s %s %s -> %s (ttl: %d)\n", r.Domain, r.Type, r.Name, r.Value, r.TTL)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entityType)
		fmt.Fprintf(os.Stderr, "Valid types: secrets, isps, zones, servers, services, registries, domains, records\n")
		os.Exit(1)
	}
}
