package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/infrastructure/persistence"
)

var listCmd = &cobra.Command{
	Use:   "list <entity>",
	Short: "List entities",
	Long:  "List all entities of the specified type.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		entity := args[0]
		runList(entity)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(entity string) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	entity = strings.ToLower(entity)
	switch entity {
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
	case "gateways":
		for _, g := range cfg.Gateways {
			fmt.Printf("- %s (server: %s, ports: %d/%d)\n", g.Name, g.Server, g.Ports.HTTP, g.Ports.HTTPS)
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
		for _, d := range cfg.Domains {
			fmt.Printf("- %s (isp: %s)\n", d.Name, d.ISP)
		}
	case "records", "dns":
		for _, r := range cfg.DNSRecords {
			fmt.Printf("- %s %s %s -> %s (ttl: %d)\n", r.Domain, r.Type, r.Name, r.Value, r.TTL)
		}
	case "certificates", "certs":
		for _, c := range cfg.Certificates {
			fmt.Printf("- %s (domains: %v, provider: %s)\n", c.Name, c.Domains, c.Provider)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entity)
		fmt.Fprintf(os.Stderr, "Valid types: secrets, isps, zones, gateways, servers, services, registries, domains, records, certificates\n")
		os.Exit(1)
	}
}
