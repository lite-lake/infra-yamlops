package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

// newServerResourcesCommand builds the `server resources` read-only command.
func newServerResourcesCommand(ctx *Context) *cobra.Command {
	var serverFilter, zoneFilter, serviceFilter string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Inspect server resource usage (CPU/memory/disk/containers)",
		Long: `Inspect host and per-service resource usage by running a single SSH session
per server (pure shell + JSON parsing, no Docker SDK, no agent).

Modes:
  (no filter)             Overview: one summary row per server
  --server <name>         Detail: host + disks + docker + services + unmanaged
  --zone <zone>           Overview for all servers in a zone
  --service <name>        Filter detail rows; alone prints only that service
  --json                  Emit a pure array of ServerResourceReport objects

The output follows a three-state truthfulness contract: failed collection is
shown explicitly (n/a / degraded) and never rendered as zero. Legitimately
empty values (no containers, no dirs, CPU sampling window too short) are kept
zero but never count as degraded.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runServerResources(ctx, serverFilter, zoneFilter, serviceFilter, jsonOut)
		},
	}
	cmd.Flags().StringVar(&serverFilter, "server", "", "Server name (at most one; switches to detail view)")
	cmd.Flags().StringVar(&zoneFilter, "zone", "", "Zone filter (comma-separated)")
	cmd.Flags().StringVar(&serviceFilter, "service", "", "Service name filter for detail rows")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON array")
	return cmd
}

func runServerResources(ctx *Context, serverFilter, zoneFilter, serviceFilter string, jsonOut bool) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if strings.Contains(serverFilter, ",") {
		fmt.Fprintf(os.Stderr, "Error: --server accepts a single server name, got %q\n", serverFilter)
		os.Exit(1)
	}

	targetServers, detailMode, err := resolveResourceTargets(cfg, serverFilter, zoneFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	scanCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	reports := scanServerResourcesUnified(scanCtx, cfg, targetServers, cfg.GetSecretsMap(), ctx.Concurrency, ctx.Env)

	if jsonOut {
		out, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	switch {
	case detailMode:
		for _, r := range reports {
			renderResourcesDetail(r, serviceFilter)
		}
	case serviceFilter != "":
		renderResourcesServiceFilter(reports, serviceFilter)
	default:
		renderResourcesOverview(reports)
	}
}

func resolveResourceTargets(cfg *entity.Config, serverFilter, zoneFilter string) ([]*entity.Server, bool, error) {
	if serverFilter == "" {
		var servers []*entity.Server
		for i := range cfg.Servers {
			srv := &cfg.Servers[i]
			if zoneFilter != "" && !matchesFilter(srv.Zone, zoneFilter) {
				continue
			}
			servers = append(servers, srv)
		}
		sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
		return servers, false, nil
	}

	var found *entity.Server
	for i := range cfg.Servers {
		if cfg.Servers[i].Name == serverFilter {
			found = &cfg.Servers[i]
			break
		}
	}
	if found == nil {
		return nil, false, fmt.Errorf("server %q not found in environment", serverFilter)
	}
	if zoneFilter != "" && !matchesFilter(found.Zone, zoneFilter) {
		return nil, false, fmt.Errorf("server %q does not belong to zone %q", found.Name, zoneFilter)
	}
	return []*entity.Server{found}, true, nil
}

// serverResourceScanResult captures one report or a hard connection failure.
type serverResourceScanResult struct {
	report *environment.ServerResourceReport
}

func scanServerResourcesUnified(ctx context.Context, cfg *entity.Config, servers []*entity.Server, secrets map[string]string, concurrency int, env string) []environment.ServerResourceReport {
	if concurrency <= 0 {
		concurrency = 5
	}

	type scanResult struct {
		index  int
		report *environment.ServerResourceReport
	}

	sem := make(chan struct{}, concurrency)
	resultsCh := make(chan scanResult, len(servers))
	var wg sync.WaitGroup

	for i, srv := range servers {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, s *entity.Server) {
			defer wg.Done()
			defer func() { <-sem }()

			password, err := s.SSH.Password.Resolve(secrets)
			if err != nil {
				resultsCh <- scanResult{index: idx, report: &environment.ServerResourceReport{
					ServerName: s.Name, Zone: s.Zone, Env: env, Degraded: true,
					ScanError: fmt.Sprintf("cannot resolve password: %v", err),
					ScannedAt: time.Now().UTC().Format(time.RFC3339),
					Segments:  map[string]environment.SegmentState{},
				}}
				return
			}

			strictHostKeyChecking := true
			if !s.SSH.StrictHostKeyChecking {
				strictHostKeyChecking = false
			}
			client, err := ssh.NewClientWithConfig(s.SSH.Host, s.SSH.Port, s.SSH.User, password, &ssh.SSHConfig{StrictHostKeyChecking: strictHostKeyChecking, Timeout: constants.DefaultSSHTimeout})
			if err != nil {
				resultsCh <- scanResult{index: idx, report: &environment.ServerResourceReport{
					ServerName: s.Name, Zone: s.Zone, Env: env, Degraded: true,
					ScanError: fmt.Sprintf("connection failed: %v", err),
					ScannedAt: time.Now().UTC().Format(time.RFC3339),
					Segments:  map[string]environment.SegmentState{},
				}}
				return
			}

			inspector := environment.NewInspector(client, env)
			report, _ := inspector.Inspect(ctx, cfg, s, secrets)
			client.Close()
			if report == nil {
				report = &environment.ServerResourceReport{ServerName: s.Name, Zone: s.Zone, Degraded: true, ScanError: "inspector returned nil report"}
			}
			resultsCh <- scanResult{index: idx, report: report}
		}(i, srv)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var all []scanResult
	for r := range resultsCh {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].index < all[j].index })

	reports := make([]environment.ServerResourceReport, 0, len(all))
	for _, r := range all {
		reports = append(reports, *r.report)
	}
	return reports
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func renderResourcesOverview(reports []environment.ServerResourceReport) {
	fmt.Printf("%-20s %-8s %-8s %-20s %-20s %-10s %s\n", "SERVER", "ZONE", "CPU", "MEMORY", "DISK(/)", "DIRS", "CONTAINERS")
	degraded := 0
	for _, r := range reports {
		name := r.ServerName
		zone := r.Zone
		cpu, mem, disk, dir, cont := "-", "-", "-", "-", "-"
		if r.ScanError != "" {
			name = "⚠ " + name
			cpu, mem, disk, dir, cont = "n/a", "n/a", "n/a", "n/a", "n/a"
			degraded++
		} else {
			if r.Degraded {
				name = "⚠ " + name
				degraded++
			}
			if r.Host != nil {
				if r.Host.CPUUsagePct != nil {
					cpu = fmt.Sprintf("%.1f%%", *r.Host.CPUUsagePct)
				} else {
					cpu = "n/a"
				}
				pct := 0.0
				if r.Host.MemTotalBytes > 0 {
					pct = float64(r.Host.MemUsedBytes) / float64(r.Host.MemTotalBytes) * 100
				}
				mem = fmt.Sprintf("%s/%s %.0f%%", r.Host.MemUsedHuman, r.Host.MemTotalHuman, pct)
			}
			if r.Segments["disk"] == environment.SegFailed {
				disk = "n/a"
			} else if d := findDiskMount(r.Disks, "/"); d != nil {
				disk = fmt.Sprintf("%s/%s %.0f%%", d.UsedHuman, d.TotalHuman, d.UsePercent)
			}
			if r.Segments["dirs"] == environment.SegFailed {
				dir = "n/a"
			} else if b := r.DirTotalBytes(); b != nil {
				dir = environment.FormatBytes(*b)
			}
			if r.Segments["docker_ps"] == environment.SegFailed {
				cont = "n/a"
			} else {
				cont = fmt.Sprintf("%d/%d", r.ContainerRunning(), r.ContainerTotal())
			}
		}
		fmt.Printf("%-20s %-8s %-8s %-20s %-20s %-10s %s\n", name, zone, cpu, mem, disk, dir, cont)
	}
	if degraded > 0 {
		fmt.Printf("\n⚠ %d server(s) had collection failures (shown as n/a).\n", degraded)
	}
}

func findDiskMount(disks []environment.DiskMount, mountPoint string) *environment.DiskMount {
	for i := range disks {
		if disks[i].MountPoint == mountPoint {
			return &disks[i]
		}
	}
	return nil
}

func renderResourcesDetail(r environment.ServerResourceReport, serviceFilter string) {
	fmt.Printf("SERVER: %s   (zone: %s, env: %s)\n", r.ServerName, r.Zone, r.Env)
	fmt.Printf("ScannedAt: %s\n", r.ScannedAt)
	if r.ScanError != "" {
		fmt.Printf("SCAN ERROR: %s\n", r.ScanError)
		return
	}
	if r.Degraded {
		fmt.Println("⚠ Some data collection failed; affected fields show n/a.")
	}
	fmt.Println()

	fmt.Println("HOST:")
	if r.Host == nil {
		fmt.Println("  n/a (host_cpu/meminfo/loadavg collection failed)")
	} else {
		cpu := "n/a"
		if r.Host.CPUUsagePct != nil {
			steal := 0.0
			if r.Host.StealPct != nil {
				steal = *r.Host.StealPct
			}
			cpu = fmt.Sprintf("%.2f%% (steal %.2f%%)", *r.Host.CPUUsagePct, steal)
		}
		memPct := 0.0
		if r.Host.MemTotalBytes > 0 {
			memPct = float64(r.Host.MemUsedBytes) / float64(r.Host.MemTotalBytes) * 100
		}
		fmt.Printf("  CPU:    %s   (%d cores)\n", cpu, r.Host.CPUCores)
		fmt.Printf("  Memory: %s / %s  (%.1f%%)\n", r.Host.MemUsedHuman, r.Host.MemTotalHuman, memPct)
		fmt.Printf("  Load:   %.2f %.2f %.2f\n", r.Host.Load1_5_15[0], r.Host.Load1_5_15[1], r.Host.Load1_5_15[2])
	}
	fmt.Println()

	fmt.Println("DISK MOUNTS:")
	if len(r.Disks) == 0 {
		fmt.Println("  n/a (disk collection failed or no data)")
	} else {
		fmt.Printf("  %-24s %-14s %-14s %s\n", "MOUNT", "TOTAL", "USED", "USE%")
		for _, d := range r.Disks {
			fmt.Printf("  %-24s %-14s %-14s %.1f%%\n", d.MountPoint, d.TotalHuman, d.UsedHuman, d.UsePercent)
		}
	}
	fmt.Println()

	fmt.Println("DOCKER DISK USAGE:")
	if r.DockerDF == nil {
		fmt.Println("  n/a (docker system df failed)")
	} else {
		for _, e := range r.DockerDF.Entries {
			fmt.Printf("  %-12s size: %-14s reclaimable: %s\n", e.Type, e.Size, e.Reclaimable)
		}
		fmt.Printf("  %-12s %s\n", "Total", r.DockerDF.TotalReclaimable())
	}
	fmt.Println()

	fmt.Println("SERVICES:")
	rows := r.Services
	if serviceFilter != "" {
		var filtered []environment.ServiceResourceInfo
		for _, s := range r.Services {
			if matchesFilter(s.Name, serviceFilter) {
				filtered = append(filtered, s)
			}
		}
		rows = filtered
		if len(filtered) == 0 {
			fmt.Printf("  No matching service found (filter: %s)\n", serviceFilter)
		}
	}
	if len(rows) > 0 {
		fmt.Printf("  %-20s %-12s %-8s %-22s %-7s %-12s %-10s %s\n", "NAME", "STATUS", "CPU", "MEM", "MEM%", "IMAGE", "RW", "DIR")
		for _, s := range rows {
			fmt.Printf("  %-20s %-12s %-8s %-22s %-7s %-12s %-10s %s\n",
				truncStr(s.Name, 20), s.Status, s.CPUPercent, s.MemUsage, s.MemPercent, s.ImageSize, s.RwSize, s.DirSize)
		}
		if unmatched := r.UnmatchedImageCount(); unmatched > 0 {
			fmt.Printf("\n  %d image(s) not matched to a configured service (registry/digest normalization misses).\n", unmatched)
		}
	}
	fmt.Println()

	fmt.Println("UNMANAGED CONTAINERS:")
	if len(r.Unmanaged) == 0 {
		fmt.Println("  none")
	} else {
		fmt.Printf("  %-24s %-24s %-24s %s\n", "NAMES", "IMAGE", "STATUS", "RW")
		for _, c := range r.Unmanaged {
			fmt.Printf("  %-24s %-24s %-24s %s\n", truncStr(c.Names, 24), truncStr(c.Image, 24), truncStr(c.Status, 24), c.SizeRw)
		}
	}
	fmt.Println()
	fmt.Println("NOTE: du reports file byte size (apparent size); df reports block usage. The two may differ.")
}

func renderResourcesServiceFilter(reports []environment.ServerResourceReport, serviceFilter string) {
	any := false
	for _, r := range reports {
		if r.ScanError != "" {
			fmt.Printf("SERVER: %s ⚠ %s\n", r.ServerName, r.ScanError)
			continue
		}
		var matched []environment.ServiceResourceInfo
		for _, s := range r.Services {
			if matchesFilter(s.Name, serviceFilter) {
				matched = append(matched, s)
			}
		}
		if len(matched) == 0 {
			continue
		}
		any = true
		fmt.Printf("SERVER: %s   (%s)\n", r.ServerName, r.Zone)
		for _, s := range matched {
			fmt.Printf("  %-20s %-12s %-8s %-22s %-7s %-12s %-10s %s\n",
				truncStr(s.Name, 20), s.Status, s.CPUPercent, s.MemUsage, s.MemPercent, s.ImageSize, s.RwSize, s.DirSize)
		}
		fmt.Println()
	}
	if !any {
		fmt.Printf("No matching service found (filter: %s)\n", serviceFilter)
	}
}

// truncStr truncates a string to the given width without breaking runes.
func truncStr(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width-1]) + "…"
}
