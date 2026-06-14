package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newServiceShowCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var detail bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "List services",
		Long:  "List services by zone, server, and type. Use --detail for image, ports, endpoints, and gateway details.",
		Run: func(cmd *cobra.Command, args []string) {
			runServiceShow(ctx, *filters, detail)
		},
	}
	cmd.Flags().BoolVar(&detail, "detail", false, "Show detailed information")
	return cmd
}

func runServiceShow(ctx *Context, filters ServiceCmdFilters, detail bool) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	cfg, err := wf.LoadConfig(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	serviceTypes, err := parseServiceTypes(filters.Type)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	showBiz := len(serviceTypes) == 0 || containsStr(serviceTypes, "biz")
	showInfra := len(serviceTypes) == 0 || containsStr(serviceTypes, "infra")

	type serviceRow struct {
		Zone    string
		Server  string
		Service string
		Image   string
		IsInfra bool
	}

	var rows []serviceRow
	for _, svc := range cfg.Services {
		if !showBiz {
			continue
		}
		if filters.Server != "" && !matchesFilter(svc.Server, filters.Server) {
			continue
		}
		srv := cfg.GetServerMap()[svc.Server]
		zone := ""
		if srv != nil {
			zone = srv.Zone
		}
		if filters.Zone != "" && !matchesFilter(zone, filters.Zone) {
			continue
		}
		rows = append(rows, serviceRow{Zone: zone, Server: svc.Server, Service: svc.Name, Image: svc.Image})
	}
	for _, infra := range cfg.InfraServices {
		if !showInfra {
			continue
		}
		if filters.Server != "" && !matchesFilter(infra.Server, filters.Server) {
			continue
		}
		srv := cfg.GetServerMap()[infra.Server]
		zone := ""
		if srv != nil {
			zone = srv.Zone
		}
		if filters.Zone != "" && !matchesFilter(zone, filters.Zone) {
			continue
		}
		rows = append(rows, serviceRow{Zone: zone, Server: infra.Server, Service: infra.Name, Image: infra.Image, IsInfra: true})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Zone != rows[j].Zone {
			return rows[i].Zone < rows[j].Zone
		}
		if rows[i].Server != rows[j].Server {
			return rows[i].Server < rows[j].Server
		}
		return rows[i].Service < rows[j].Service
	})

	if len(rows) == 0 {
		fmt.Println("No services found.")
		return
	}

	if detail {
		fmt.Printf("%-10s %-12s %-16s %s\n", "ZONE", "SERVER", "SERVICE", "IMAGE")
		for _, r := range rows {
			fmt.Printf("%-10s %-12s %-16s %s\n", r.Zone, r.Server, r.Service, r.Image)
		}
		fmt.Println()
		for _, r := range rows {
			if r.IsInfra {
				for _, infra := range cfg.InfraServices {
					if infra.Name == r.Service {
						fmt.Printf("SERVICE: %s\n", infra.Name)
						fmt.Printf("  Image:      %s\n", infra.Image)
						if infra.GatewayPorts != nil && (infra.GatewayPorts.HTTP > 0 || infra.GatewayPorts.HTTPS > 0) {
							var ports []string
							if infra.GatewayPorts.HTTP > 0 {
								ports = append(ports, fmt.Sprintf("http:%d/tcp", infra.GatewayPorts.HTTP))
							}
							if infra.GatewayPorts.HTTPS > 0 {
								ports = append(ports, fmt.Sprintf("https:%d/tcp", infra.GatewayPorts.HTTPS))
							}
							fmt.Printf("  Ports:      %s\n", strings.Join(ports, ", "))
						}
						fmt.Printf("  Endpoints:  (none)\n")
						fmt.Printf("  Gateways:   (none)\n")
						fmt.Printf("  Health:     (none)\n")
						fmt.Println()
					}
				}
			} else {
				for _, svc := range cfg.Services {
					if svc.Name == r.Service {
						fmt.Printf("SERVICE: %s\n", svc.Name)
						fmt.Printf("  Image:      %s\n", svc.Image)
						if len(svc.Ports) > 0 {
							var ports []string
							for _, p := range svc.Ports {
								ports = append(ports, fmt.Sprintf("%d:%d/%s", p.Host, p.Container, p.Protocol))
							}
							fmt.Printf("  Ports:      %s\n", strings.Join(ports, ", "))
						}
						var eps []string
						var gwLines []string
						for _, g := range svc.Gateways {
							if g.HTTP {
								eps = append(eps, fmt.Sprintf("http://%s%s", g.Hostname, g.Path))
							}
							if g.HTTPS {
								eps = append(eps, fmt.Sprintf("https://%s%s", g.Hostname, g.Path))
							}
							protocols := ""
							if g.HTTP && g.HTTPS {
								protocols = "http+https"
							} else if g.HTTP {
								protocols = "http"
							} else if g.HTTPS {
								protocols = "https"
							}
							gwLines = append(gwLines, fmt.Sprintf("%s (%s)", g.Hostname, protocols))
						}
						if len(eps) > 0 {
							fmt.Printf("  Endpoints:  %s\n", eps[0])
							for _, ep := range eps[1:] {
								fmt.Printf("              %s\n", ep)
							}
						} else {
							fmt.Printf("  Endpoints:  (none)\n")
						}
						if len(gwLines) > 0 {
							fmt.Printf("  Gateways:   %s\n", gwLines[0])
							for _, gl := range gwLines[1:] {
								fmt.Printf("              %s\n", gl)
							}
						} else {
							fmt.Printf("  Gateways:   (none)\n")
						}
						if svc.Healthcheck != nil && svc.Healthcheck.Path != "" {
							fmt.Printf("  Health:     %s (interval: %s)\n", svc.Healthcheck.Path, svc.Healthcheck.Interval)
						} else {
							fmt.Printf("  Health:     (none)\n")
						}
						fmt.Println()
					}
				}
			}
		}
	} else {
		fmt.Printf("%-10s %-12s %-16s\n", "ZONE", "SERVER", "SERVICE")
		for _, r := range rows {
			fmt.Printf("%-10s %-12s %-16s\n", r.Zone, r.Server, r.Service)
		}
	}

	zoneSet := make(map[string]bool)
	serverSet := make(map[string]bool)
	for _, r := range rows {
		zoneSet[r.Zone] = true
		serverSet[r.Server] = true
	}
	fmt.Printf("\nTotal: %d services across %d servers in %d zones\n", len(rows), len(serverSet), len(zoneSet))
}
