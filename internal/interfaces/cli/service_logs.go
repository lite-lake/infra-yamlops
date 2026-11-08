package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

func newServiceLogsCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	var tail int
	var since, until, grep, runID string
	var timestamps, follow, previous, listRuns, jsonOut bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View service container logs (current / previous runs)",
		Long: `View container logs of services over SSH (read-only, no plan/confirm).

Modes:
  (default)   Tail the current container's logs (like docker logs --tail)
  --previous  Show the most recent archived run (logs exported before the
              previous deploy recreate / restart — the crash-era logs)
  --runs      List archived runs for each matched service
  --run <id>  Show one archived run by id (from --runs)
  --follow    Stream logs until Ctrl-C (requires a single --service)

Notes:
  - deploy (--force-recreate) destroys the old container; yamlops archives its
    logs under /data/yamlops/yo-{env}-{svc}/logs/ beforehand (last 5 kept).
  - Crash-restarts by Docker keep the same container, so those logs are always
    visible in the default mode (check "restarts:" in the status header).`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			opts := usecase.LogOptions{
				Tail:       tail,
				Since:      since,
				Until:      until,
				Timestamps: timestamps,
				Grep:       grep,
				RunID:      runID,
			}
			runServiceLogs(ctx, *filters, opts, previous, listRuns, follow, jsonOut)
		},
	}
	cmd.Flags().IntVar(&tail, "tail", constants.DefaultLogsTail, "Number of lines to show from the end (0 = all)")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since (e.g. 30m, 2h, 1d or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "Show logs until (e.g. 30m, 2h or RFC3339)")
	cmd.Flags().StringVar(&grep, "grep", "", "Server-side case-insensitive literal filter")
	cmd.Flags().StringVar(&runID, "run", "", "Show one archived run by id (see --runs)")
	cmd.Flags().BoolVar(&timestamps, "timestamps", false, "Show docker timestamps")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (Ctrl-C to stop)")
	cmd.Flags().BoolVar(&previous, "previous", false, "Show the most recent archived run per service")
	cmd.Flags().BoolVar(&listRuns, "runs", false, "List archived runs per service")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON")
	return cmd
}

func runServiceLogs(ctx *Context, filters ServiceCmdFilters, opts usecase.LogOptions, previous, listRuns, follow, jsonOut bool) {
	singleRequested := len(splitAndTrim(filters.Service, ",")) == 1
	if follow && !singleRequested {
		fmt.Fprintln(os.Stderr, "Error: --follow requires exactly one --service")
		os.Exit(1)
	}
	if follow && (listRuns || previous || opts.RunID != "") {
		fmt.Fprintln(os.Stderr, "Error: --follow cannot be combined with --runs/--previous/--run")
		os.Exit(1)
	}

	cfg, err := loadConfigFromContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	followEnvName = ctx.Env

	targets := collectLogTargets(cfg, filters)
	if len(targets) == 0 {
		DisplayInfo("No services matched for logs.")
		return
	}

	if listRuns {
		runServiceLogsRuns(ctx, cfg, targets, jsonOut)
		return
	}

	// --previous：逐服务解析最新归档 id，无归档的服务跳过
	if previous && opts.RunID == "" {
		previousByService := resolvePreviousRuns(ctx, cfg, targets)
		if len(previousByService) == 0 {
			DisplayInfo("No archived runs found. Archives are created automatically before deploy/restart.")
			return
		}
		var filtered []logTarget
		for _, t := range targets {
			if id, ok := previousByService[t.ServiceName]; ok {
				t.runIDOverride = id
				filtered = append(filtered, t)
			}
		}
		targets = filtered
	}

	scanCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if follow {
		runServiceLogsFollow(scanCtx, cfg, targets[0], opts)
		return
	}

	connect := func(serverName string) (contract.SSHClient, error) {
		return dialServer(cfg, serverName)
	}
	remoteDirOf := func(serviceName string) string {
		return fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, ctx.Env, serviceName))
	}

	ucTargets := make([]usecase.LogTarget, 0, len(targets))
	for _, t := range targets {
		ucTargets = append(ucTargets, usecase.LogTarget{ServiceName: t.ServiceName, ServerName: t.ServerName})
	}

	// 每个 target 可有自己的 runIDOverride（--previous 语义）
	if perServiceRunID(targets) {
		for _, t := range targets {
			e := collectOneServiceLog(ctx.Env, cfg, t, remoteDirOf(t.ServiceName), opts)
			if jsonOut {
				printLogEntryJSON(e)
			} else {
				printLogEntry(e)
			}
		}
		return
	}

	entries := usecase.CollectServiceLogs(scanCtx, ctx.Env, connect, ucTargets, remoteDirOf, opts, ctx.Concurrency)

	if jsonOut {
		fmt.Print("[")
		for i, e := range entries {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Print(marshalLogEntryJSON(e))
		}
		fmt.Println("]")
		return
	}

	for _, e := range entries {
		printLogEntry(e)
	}
}

func perServiceRunID(targets []logTarget) bool {
	for _, t := range targets {
		if t.runIDOverride != "" {
			return true
		}
	}
	return false
}

// collectOneServiceLog 处理带 runIDOverride 的单服务查询（--previous）。
func collectOneServiceLog(env string, cfg *entity.Config, t logTarget, remoteDir string, opts usecase.LogOptions) usecase.LogEntry {
	entry := usecase.LogEntry{
		ServiceName: t.ServiceName,
		ServerName:  t.ServerName,
		Container:   usecase.ContainerName(env, t.ServiceName),
		Source:      usecase.LogSourceArchived,
		ArchiveID:   t.runIDOverride,
	}
	client, err := dialServer(cfg, t.ServerName)
	if err != nil {
		entry.Content = fmt.Sprintf("ERROR: connect failed: %v", err)
		return entry
	}
	defer client.Close()
	content, err := usecase.FetchArchivedLogs(client, remoteDir, t.runIDOverride)
	if err != nil {
		entry.Content = fmt.Sprintf("ERROR: %v", err)
	} else {
		entry.Content = content
	}
	return entry
}

// logTarget 内部结构：服务 + 服务器 + 可选的归档 id 覆盖。
type logTarget struct {
	ServiceName   string
	ServerName    string
	runIDOverride string
}

func collectLogTargets(cfg *entity.Config, filters ServiceCmdFilters) []logTarget {
	var targets []logTarget
	serviceTypes, err := parseServiceTypes(filters.Type)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	showBiz := len(serviceTypes) == 0 || containsStr(serviceTypes, "biz")
	showInfra := len(serviceTypes) == 0 || containsStr(serviceTypes, "infra")
	serverMap := cfg.GetServerMap()

	add := func(name, server string) {
		if filters.Server != "" && !matchesFilter(server, filters.Server) {
			return
		}
		if srv := serverMap[server]; srv != nil && filters.Zone != "" && !matchesFilter(srv.Zone, filters.Zone) {
			return
		}
		if filters.Service != "" && !matchesFilter(name, filters.Service) {
			return
		}
		targets = append(targets, logTarget{ServiceName: name, ServerName: server})
	}
	if showBiz {
		for _, svc := range cfg.Services {
			if svc.Image == "" { // external_backends 服务无本地容器
				continue
			}
			add(svc.Name, svc.Server)
		}
	}
	if showInfra {
		for _, infra := range cfg.InfraServices {
			add(infra.Name, infra.Server)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].ServerName != targets[j].ServerName {
			return targets[i].ServerName < targets[j].ServerName
		}
		return targets[i].ServiceName < targets[j].ServiceName
	})
	return targets
}

func dialServer(cfg *entity.Config, serverName string) (*ssh.Client, error) {
	srv, ok := cfg.GetServerMap()[serverName]
	if !ok {
		return nil, fmt.Errorf("server %q not found in environment", serverName)
	}
	password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
	if err != nil {
		return nil, fmt.Errorf("resolve password: %w", err)
	}
	strict := true
	if !srv.SSH.StrictHostKeyChecking {
		strict = false
	}
	return ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, &ssh.SSHConfig{
		StrictHostKeyChecking: strict,
		Timeout:               constants.DefaultSSHTimeout,
	})
}

func printLogEntry(e usecase.LogEntry) {
	sep := strings.Repeat("─", 60)
	fmt.Println(sep)
	source := string(e.Source)
	if e.ArchiveID != "" {
		source += " (" + e.ArchiveID + ")"
	}
	fmt.Printf("SERVICE: %s   SERVER: %s   CONTAINER: %s   SOURCE: %s\n", e.ServiceName, e.ServerName, e.Container, source)
	if e.Status != nil && e.Status.Exists {
		fmt.Printf("STATE: %s (restarts: %d, started: %s)\n", e.Status.State, e.Status.RestartCount, e.Status.StartedAt)
	}
	fmt.Println(sep)
	if e.Content == "" {
		fmt.Println("(no logs)")
		return
	}
	fmt.Print(strings.TrimRight(e.Content, "\n"))
	fmt.Println()
}

type logEntryJSON struct {
	Service   string                   `json:"service"`
	Server    string                   `json:"server"`
	Container string                   `json:"container"`
	Source    string                   `json:"source"`
	RunID     string                   `json:"run_id,omitempty"`
	Status    *usecase.ContainerStatus `json:"status,omitempty"`
	Content   string                   `json:"content,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

func marshalLogEntryJSON(e usecase.LogEntry) string {
	je := logEntryJSON{
		Service:   e.ServiceName,
		Server:    e.ServerName,
		Container: e.Container,
		Source:    string(e.Source),
		RunID:     e.ArchiveID,
		Status:    e.Status,
		Content:   e.Content,
	}
	if strings.HasPrefix(e.Content, "ERROR: ") {
		je.Error = strings.TrimPrefix(e.Content, "ERROR: ")
		je.Content = ""
	}
	data, err := json.Marshal(je)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func printLogEntryJSON(e usecase.LogEntry) {
	fmt.Println(marshalLogEntryJSON(e))
}

// --- --runs 列表模式 ---

type serviceRunsRow struct {
	Service string   `json:"service"`
	Server  string   `json:"server"`
	Current bool     `json:"current_exists"`
	Runs    []string `json:"runs"`
	Error   string   `json:"error,omitempty"`
}

func runServiceLogsRuns(ctx *Context, cfg *entity.Config, targets []logTarget, jsonOut bool) {
	var rows []serviceRunsRow

	for _, t := range targets {
		row := serviceRunsRow{Service: t.ServiceName, Server: t.ServerName, Runs: []string{}}
		client, err := dialServer(cfg, t.ServerName)
		if err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, ctx.Env, t.ServiceName))
		container := usecase.ContainerName(ctx.Env, t.ServiceName)
		inspectOut, _, _ := client.Run(usecase.BuildContainerInspectCommand(container))
		st := usecase.ParseContainerInspectOutput(inspectOut)
		row.Current = st.Exists
		if runList, err := usecase.ListArchiveRuns(client, remoteDir); err != nil {
			row.Error = err.Error()
		} else {
			for _, r := range runList {
				row.Runs = append(row.Runs, r.ArchiveID)
			}
		}
		client.Close()
		rows = append(rows, row)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(data))
		return
	}

	for _, row := range rows {
		fmt.Printf("SERVICE: %s   SERVER: %s\n", row.Service, row.Server)
		if row.Error != "" {
			fmt.Printf("  ERROR: %s\n", row.Error)
			continue
		}
		status := "no container"
		if row.Current {
			status = "exists"
		}
		fmt.Printf("  current: %s\n", status)
		if len(row.Runs) == 0 {
			fmt.Println("  archives: none")
		} else {
			fmt.Println("  archives (newest first):")
			for _, id := range row.Runs {
				fmt.Printf("    - %s\n", id)
			}
		}
	}
}

// --- --previous 解析 ---

func resolvePreviousRuns(ctx *Context, cfg *entity.Config, targets []logTarget) map[string]string {
	result := make(map[string]string)
	for _, t := range targets {
		client, err := dialServer(cfg, t.ServerName)
		if err != nil {
			continue
		}
		remoteDir := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, ctx.Env, t.ServiceName))
		if runs, err := usecase.ListArchiveRuns(client, remoteDir); err == nil && len(runs) > 0 {
			result[t.ServiceName] = runs[0].ArchiveID
		}
		client.Close()
	}
	return result
}

// --- --follow 流式模式 ---

func runServiceLogsFollow(ctx context.Context, cfg *entity.Config, t logTarget, opts usecase.LogOptions) {
	client, err := dialServer(cfg, t.ServerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	container := usecase.ContainerName(cfgEnvName(cfg), t.ServiceName)
	fmt.Printf("Following %s on %s (Ctrl-C to stop)...\n", container, t.ServerName)

	cmd := "sudo docker logs --follow"
	if opts.Tail > 0 {
		cmd += fmt.Sprintf(" --tail %d", opts.Tail)
	}
	if opts.Timestamps {
		cmd += " --timestamps"
	}
	cmd += " " + ssh.ShellEscape(container) + " 2>&1"
	if opts.Grep != "" {
		cmd += fmt.Sprintf(" | grep -i -F %s || true", ssh.ShellEscape(opts.Grep))
	}

	stdoutCh := make(chan string, 128)
	stderrCh := make(chan string, 128)
	errCh := make(chan error, 1)
	go func() { errCh <- client.StreamRun(ctx, cmd, stdoutCh, stderrCh) }()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-stdoutCh:
			if !ok {
				return
			}
			fmt.Print(chunk)
		case <-stderrCh:
			// drained
		case err := <-errCh:
			for {
				select {
				case chunk, ok := <-stdoutCh:
					if !ok {
						return
					}
					fmt.Print(chunk)
				default:
					if err != nil && ctx.Err() == nil {
						fmt.Fprintf(os.Stderr, "\nstream ended: %v\n", err)
					}
					return
				}
			}
		}
	}
}

// cfgEnvName 从已加载配置反推环境名（logs 命令的 ctx.Env 已存在；此处用于 follow 路径）。
// 环境名实际由 runServiceLogs 闭包持有，这里通过包内变量传递以保持函数签名简洁。
var followEnvName string

func cfgEnvName(cfg *entity.Config) string {
	if followEnvName != "" {
		return followEnvName
	}
	return "dev"
}
