package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
)

func newServerShowCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Zone   string
		Server string
	}
	var detail bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "List servers",
		Long:  "List servers by zone. Use --detail for IP, SSH, provider, and network details.",
		Run: func(cmd *cobra.Command, args []string) {
			runServerShow(ctx, filters.Zone, filters.Server, detail)
		},
	}
	cmd.Flags().StringVar(&filters.Zone, "zone", "", "Zone filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Server, "server", "", "Server filter (comma-separated)")
	cmd.Flags().BoolVar(&detail, "detail", false, "Show detailed information")
	return cmd
}

func runServerShow(ctx *Context, zoneFilter, serverFilter string, detail bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	type serverRow struct {
		Zone      string
		Server    string
		PublicIP  string
		PrivateIP string
		SSHInfo   string
		ISP       string
		Networks  string
	}

	var rows []serverRow
	for _, srv := range cfg.Servers {
		if serverFilter != "" && !matchesFilter(srv.Name, serverFilter) {
			continue
		}
		if zoneFilter != "" && !matchesFilter(srv.Zone, zoneFilter) {
			continue
		}
		sshInfo := fmt.Sprintf("%s@%s:%d", srv.SSH.User, srv.SSH.Host, srv.SSH.Port)
		var networks []string
		for _, n := range srv.Networks {
			networks = append(networks, fmt.Sprintf("%s (%s)", n.Name, n.Driver))
		}
		rows = append(rows, serverRow{
			Zone:      srv.Zone,
			Server:    srv.Name,
			PublicIP:  srv.IP.Public,
			PrivateIP: srv.IP.Private,
			SSHInfo:   sshInfo,
			ISP:       srv.ISP,
			Networks:  strings.Join(networks, ", "),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Zone != rows[j].Zone {
			return rows[i].Zone < rows[j].Zone
		}
		return rows[i].Server < rows[j].Server
	})

	if len(rows) == 0 {
		fmt.Println("No servers found.")
		return
	}

	if detail {
		fmt.Printf("%-10s %-16s\n", "ZONE", "SERVER")
		for _, r := range rows {
			fmt.Printf("%-10s %-16s\n", r.Zone, r.Server)
		}
		fmt.Println()
		for _, r := range rows {
			fmt.Printf("SERVER: %s\n", r.Server)
			fmt.Printf("  Public IP:   %s\n", r.PublicIP)
			fmt.Printf("  Private IP:  %s\n", r.PrivateIP)
			fmt.Printf("  SSH:         %s\n", r.SSHInfo)
			fmt.Printf("  Provider:    %s\n", r.ISP)
			if r.Networks != "" {
				fmt.Printf("  Networks:    %s\n", r.Networks)
			}
			fmt.Println()
		}
	} else {
		fmt.Printf("%-10s %-16s\n", "ZONE", "SERVER")
		for _, r := range rows {
			fmt.Printf("%-10s %-16s\n", r.Zone, r.Server)
		}
	}

	zoneSet := make(map[string]bool)
	for _, r := range rows {
		zoneSet[r.Zone] = true
	}
	fmt.Printf("\nTotal: %d servers in %d zones\n", len(rows), len(zoneSet))
}
