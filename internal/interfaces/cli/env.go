package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/infrastructure/persistence"
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

	for _, srv := range cfg.Servers {
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

		stdout, _, err := client.Run("sudo docker ps --format '{{.Names}}'")
		client.Close()

		if err != nil {
			fmt.Printf("[%s] Check failed: %v\n", srv.Name, err)
			continue
		}

		fmt.Printf("[%s] Status: running\n", srv.Name)
		containers := strings.TrimSpace(stdout)
		if containers != "" {
			for _, c := range strings.Split(containers, "\n") {
				fmt.Printf("  - %s\n", c)
			}
		}
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

	for _, srv := range cfg.Servers {
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

		_, stderr, err := client.Run("sudo docker network create yamlops-" + Env + " 2>/dev/null || true")
		client.Close()

		if err != nil {
			fmt.Printf("[%s] Sync failed: %v\n%s\n", srv.Name, err, stderr)
			continue
		}

		fmt.Printf("[%s] Network yamlops-%s ready\n", srv.Name, Env)
	}
}
