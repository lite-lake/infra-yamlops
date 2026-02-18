package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	serverpkg "github.com/litelake/yamlops/internal/server"
	"github.com/litelake/yamlops/internal/ssh"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Environment operations",
	Long:  "Manage environment configurations and synchronization.",
}

var envCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check environment status",
	Long:  "Check the current status of the specified environment.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runEnvCheck(ServerFilter, ZoneFilter)
	},
}

var envSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize environment",
	Long:  "Synchronize the specified environment.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runEnvSync(ServerFilter, ZoneFilter)
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envCheckCmd)
	envCmd.AddCommand(envSyncCmd)

	envCheckCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	envCheckCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")

	envSyncCmd.Flags().StringVar(&ServerFilter, "server", "", "Filter by server")
	envSyncCmd.Flags().StringVar(&ZoneFilter, "zone", "", "Filter by zone")
}

func runEnvCheck(server, zone string) {
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
		if server != "" && srv.Name != server {
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

		checker := serverpkg.NewChecker(client, srv, registries, secrets)
		results := checker.CheckAll()
		fmt.Print(serverpkg.FormatResults(srv.Name, results))

		client.Close()
	}
}

func runEnvSync(server, zone string) {
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
		if server != "" && srv.Name != server {
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

		syncer := serverpkg.NewSyncer(client, srv, Env, registries, secrets)
		results := syncer.SyncAll()

		fmt.Printf("[%s] Sync Results\n", srv.Name)
		for _, r := range results {
			if r.Success {
				fmt.Printf("  ✅ %s: %s\n", r.Name, r.Message)
			} else {
				fmt.Printf("  ❌ %s: %s\n", r.Name, r.Message)
			}
		}

		client.Close()
	}
}
