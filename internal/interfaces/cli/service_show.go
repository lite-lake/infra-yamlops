package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
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
		if filters.Service != "" && !matchesFilter(svc.Name, filters.Service) {
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
		if filters.Service != "" && !matchesFilter(infra.Name, filters.Service) {
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
						if infra.Type == entity.InfraServiceTypeUptimeKuma && infra.UptimeKumaConfig != nil {
							for _, line := range formatUptimeKumaDetail(&infra) {
								fmt.Println(line)
							}
							fmt.Println()
							continue
						}
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

const (
	uptimeKumaHealthCheckURL      = "http://127.0.0.1:3001" // 与 compose 生成器主容器 healthcheck 保持一致
	uptimeKumaHealthCheckInterval = "10s"
)

// formatUptimeKumaDetail 生成 uptime-kuma 服务的 --detail 输出行（含 Endpoints/Gateways/Health 及
// Monitors/Notification/Sync agent/Admin/DB 块）。绝不输出任何密码明文：SecretRef 仅显示 secret 名。
func formatUptimeKumaDetail(infra *entity.InfraService) []string {
	uk := infra.UptimeKumaConfig
	if uk == nil {
		return nil
	}

	var lines []string
	var eps, gws []string
	for _, g := range uk.Gateways {
		proto := "http"
		if g.HTTPS {
			proto = "https"
		}
		eps = append(eps, fmt.Sprintf("%s://%s", proto, g.Hostname))
		gws = append(gws, fmt.Sprintf("%s (%s)", g.Hostname, proto))
	}
	if len(eps) > 0 {
		lines = append(lines, fmt.Sprintf("  Endpoints:  %s", eps[0]))
		for _, ep := range eps[1:] {
			lines = append(lines, fmt.Sprintf("              %s", ep))
		}
	} else {
		lines = append(lines, "  Endpoints:  (none)")
	}
	if len(gws) > 0 {
		lines = append(lines, fmt.Sprintf("  Gateways:   %s", gws[0]))
		for _, gl := range gws[1:] {
			lines = append(lines, fmt.Sprintf("              %s", gl))
		}
	} else {
		lines = append(lines, "  Gateways:   (none)")
	}
	lines = append(lines, fmt.Sprintf("  Health:     %s (interval: %s)", uptimeKumaHealthCheckURL, uptimeKumaHealthCheckInterval))

	if len(uk.Monitors) > 0 {
		lines = append(lines, "  Monitors:")
		for _, m := range uk.Monitors {
			lines = append(lines, fmt.Sprintf("  - 名称: %s | type: %s | %s", m.Name, m.Type, uptimeKumaMonitorTarget(m)))
		}
	} else {
		lines = append(lines, "  Monitors:   (none)")
	}

	notif := fmt.Sprintf("  Notification: enabled: %t", uk.Notification.Enabled)
	if uk.Notification.Enabled && uk.Notification.URL != "" {
		notif += fmt.Sprintf(" | url: %s", uk.Notification.URL)
	}
	lines = append(lines, notif)

	interval := uk.Sync.ReconcileInterval
	if interval == "" {
		interval = "60s"
	}
	relogin := uk.Sync.ReloginInterval
	if relogin == "" {
		relogin = "1h (agent default)"
	}
	lines = append(lines, fmt.Sprintf("  Sync agent: image: %s | reconcile_interval: %s | relogin_interval: %s", uk.Sync.Image, interval, relogin))

	admin := fmt.Sprintf("  Admin: username: %s", uk.Admin.Username)
	if secret := uk.Admin.Password.Secret(); secret != "" {
		admin += fmt.Sprintf(" | password: {secret: %s}", secret)
	}
	lines = append(lines, admin)

	lines = append(lines, fmt.Sprintf("  DB: type: %s | hostname: %s | name: %s | username: %s",
		uk.DB.Type, uk.DB.Hostname, uk.DB.Name, uk.DB.Username))

	return lines
}

// uptimeKumaMonitorTarget 生成 monitor 的目标摘要：优先 url，其次 hostname(:port)。
func uptimeKumaMonitorTarget(m entity.UptimeKumaMonitorConfig) string {
	if m.URL != "" {
		return fmt.Sprintf("url: %s", m.URL)
	}
	if m.Hostname != "" {
		if m.Port > 0 {
			return fmt.Sprintf("hostname: %s:%d", m.Hostname, m.Port)
		}
		return fmt.Sprintf("hostname: %s", m.Hostname)
	}
	return "target: (none)"
}
