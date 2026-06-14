package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/environment"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/persistence"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

func newServerCommand(ctx *Context) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Server management commands",
		Long: `Server management commands.

Commands:
  show      List servers (use --detail for detailed view)
  validate  Validate server configuration
  setup     Setup server environment (Plan -> Confirm -> Execute)

Examples:
  yamlops cli server show -e prod
  yamlops cli server show -e prod --detail
  yamlops cli server validate -e prod
  yamlops cli server setup -e prod --dry-run
  yamlops cli server setup -e prod --yes`,
	}

	serverCmd.AddCommand(newServerShowCommand(ctx))
	serverCmd.AddCommand(newServerValidateCommand(ctx))
	serverCmd.AddCommand(newServerSetupCommand(ctx))

	return serverCmd
}

func newServerSetupCommand(ctx *Context) *cobra.Command {
	var filters struct {
		Zone   string
		Server string
	}
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup server environment",
		Long:  "Check and sync server environment configuration using unified execution mode.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runServerSetupUnified(ctx, filters.Zone, filters.Server, dryRun, yes, ctx.Concurrency)
		},
	}
	cmd.Flags().StringVar(&filters.Zone, "zone", "", "Zone filter (comma-separated)")
	cmd.Flags().StringVar(&filters.Server, "server", "", "Server filter (comma-separated)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, execute all changes")
	return cmd
}

func runServerSetupUnified(ctx *Context, zoneFilter, serverFilter string, dryRun, yes bool, concurrency int) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	secrets := cfg.GetSecretsMap()

	type serverSetupRow struct {
		Server  string
		Details string
	}

	var rows []serverSetupRow
	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		if serverFilter != "" && !matchesFilter(srv.Name, serverFilter) {
			continue
		}
		if zoneFilter != "" && !matchesFilter(srv.Zone, zoneFilter) {
			continue
		}
		details := buildServerSetupDetails(srv)
		rows = append(rows, serverSetupRow{Server: srv.Name, Details: details})
	}

	if len(rows) == 0 {
		fmt.Println("No servers to setup.")
		return
	}

	DisplayPlanHeader(PlanHeader{
		Title: buildPlanTitle("server setup", dryRun, false),
		Env:   ctx.Env,
	})

	var planRows []PlanRow
	for _, r := range rows {
		planRows = append(planRows, PlanRow{Action: "sync", Name: r.Server, Details: r.Details})
	}
	DisplayPlanTable3Col("ACTION", "SERVER", "DETAILS", planRows)
	DisplaySummary(formatSummaryCount("synced", len(rows)))

	if dryRun {
		DisplayDryRun()
		return
	}

	if !yes {
		if !Confirm("Continue?", false) {
			DisplayCancelled()
			return
		}
	}

	DisplayExecuting()
	if concurrency <= 0 {
		concurrency = 5
	}

	type serverSetupResult struct {
		index   int
		success bool
		errMsg  string
	}

	total := len(rows)
	sem := make(chan struct{}, concurrency)
	resultsCh := make(chan serverSetupResult, total)
	var wg sync.WaitGroup
	succeeded := 0
	failed := 0

	for i, r := range rows {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, row serverSetupRow) {
			defer wg.Done()
			defer func() { <-sem }()

			var srv *entity.Server
			for j := range cfg.Servers {
				if cfg.Servers[j].Name == row.Server {
					srv = &cfg.Servers[j]
					break
				}
			}
			if srv == nil {
				resultsCh <- serverSetupResult{index: idx, success: false, errMsg: "server not found"}
				return
			}

			password, err := srv.SSH.Password.Resolve(secrets)
			if err != nil {
				resultsCh <- serverSetupResult{index: idx, success: false, errMsg: fmt.Sprintf("cannot resolve password: %v", err)}
				return
			}

			strictHostKeyChecking := true
			if !srv.SSH.StrictHostKeyChecking {
				strictHostKeyChecking = false
			}
			sshCfg := &ssh.SSHConfig{
				StrictHostKeyChecking: strictHostKeyChecking,
			}
			client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password, sshCfg)
			if err != nil {
				resultsCh <- serverSetupResult{index: idx, success: false, errMsg: fmt.Sprintf("connection failed: %v", err)}
				return
			}

			syncer := environment.NewSyncer(client, srv, ctx.Env, secrets, cfg.Registries)
			results := syncer.SyncAll()
			client.Close()

			success := true
			errMsg := ""
			for _, sr := range results {
				if !sr.Success {
					success = false
					errMsg = sr.Message
					break
				}
			}
			resultsCh <- serverSetupResult{index: idx, success: success, errMsg: errMsg}
		}(i, r)
	}

	wg.Wait()
	close(resultsCh)

	for r := range resultsCh {
		if r.success {
			DisplayExecuteStep(r.index+1, total, ExecuteItem{
				Action: "sync", Name: rows[r.index].Server, Server: rows[r.index].Server, Status: "synced", Success: true,
			})
			succeeded++
		} else {
			errMsg := r.errMsg
			if errMsg == "" {
				errMsg = "sync failed"
			}
			DisplayExecuteStepWithError(r.index+1, total, ExecuteItem{
				Action: "sync", Name: rows[r.index].Server, Server: rows[r.index].Server, Status: "failed", Success: false,
			}, errMsg, "Check server environment configuration")
			failed++
		}
	}

	DisplayResult(succeeded, failed)
}

func buildServerSetupDetails(srv *entity.Server) string {
	var parts []string

	if aptSource := srv.Environment.APTSource; aptSource != "" && aptSource != "official" {
		parts = append(parts, fmt.Sprintf("apt_source: %s (configured)", aptSource))
	}

	if len(srv.Networks) > 0 {
		names := make([]string, len(srv.Networks))
		for i, n := range srv.Networks {
			names[i] = n.Name
		}
		parts = append(parts, fmt.Sprintf("networks: %s (ensured)", strings.Join(names, ", ")))
	}

	if len(srv.Environment.Registries) > 0 {
		parts = append(parts, fmt.Sprintf("registries: %s (login)", strings.Join(srv.Environment.Registries, ", ")))
	}

	if len(parts) == 0 {
		return "docker (ready)"
	}
	return strings.Join(parts, ", ")
}
