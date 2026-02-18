package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/server"
	"github.com/litelake/yamlops/internal/ssh"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server operations",
	Long:  "Manage server environment setup and configuration.",
}

var serverSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup server environment",
	Long:  "Check and sync server environment configuration.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		checkOnly, _ := cmd.Flags().GetBool("check-only")
		syncOnly, _ := cmd.Flags().GetBool("sync-only")
		runServerSetup(ServerFilter, ZoneFilter, checkOnly, syncOnly)
	},
}

var serverCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check server environment",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runServerCheck(ServerFilter, ZoneFilter)
	},
}

var serverSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync server environment",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runServerSync(ServerFilter, ZoneFilter)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverSetupCmd)
	serverCmd.AddCommand(serverCheckCmd)
	serverCmd.AddCommand(serverSyncCmd)

	serverSetupCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	serverSetupCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")
	serverSetupCmd.Flags().Bool("check-only", false, "Only check, do not sync")
	serverSetupCmd.Flags().Bool("sync-only", false, "Only sync, do not check")

	serverCheckCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	serverCheckCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")

	serverSyncCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	serverSyncCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")
}

func runServerSetup(serverName, zone string, checkOnly, syncOnly bool) {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	secrets := cfg.GetSecretsMap()
	registries := convertRegistries(cfg.Registries)

	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		if serverName != "" && srv.Name != serverName {
			continue
		}
		if zone != "" && srv.Zone != zone {
			continue
		}

		password, err := srv.SSH.Password.Resolve(secrets)
		if err != nil {
			fmt.Printf("[%s] Cannot resolve password: %v\n", srv.Name, err)
			continue
		}

		client, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			fmt.Printf("[%s] Connection failed: %v\n", srv.Name, err)
			continue
		}

		if !syncOnly {
			checker := server.NewChecker(client, srv, registries, secrets)
			results := checker.CheckAll()
			fmt.Print(server.FormatResults(srv.Name, results))
		}

		if !checkOnly {
			syncer := server.NewSyncer(client, srv, Env, registries, secrets)
			results := syncer.SyncAll()
			printSyncResults(srv.Name, results)
		}

		client.Close()
	}
}

func convertRegistries(registries []entity.Registry) []*entity.Registry {
	result := make([]*entity.Registry, len(registries))
	for i := range registries {
		result[i] = &registries[i]
	}
	return result
}

func runServerCheck(serverName, zone string) {
	runServerSetup(serverName, zone, true, false)
}

func runServerSync(serverName, zone string) {
	runServerSetup(serverName, zone, false, true)
}

func printSyncResults(serverName string, results []server.SyncResult) {
	fmt.Printf("[%s] Sync Results\n", serverName)
	for _, r := range results {
		if r.Success {
			fmt.Printf("  ✅ %s: %s\n", r.Name, r.Message)
		} else {
			fmt.Printf("  ❌ %s: %s\n", r.Name, r.Message)
			if r.Error != nil {
				fmt.Printf("     Error: %v\n", r.Error)
			}
		}
	}
}
