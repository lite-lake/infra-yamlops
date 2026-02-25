package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/domain/entity"
	serverpkg "github.com/litelake/yamlops/internal/environment"
	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

func newEnvCommand(ctx *Context) *cobra.Command {
	var filters Filters

	envCmd := &cobra.Command{
		Use:   "env",
		Short: "Environment operations",
		Long:  "Manage environment configurations and synchronization.",
	}

	envCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Check environment status",
		Long:  "Check the current status of the specified environment.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runEnvCheck(ctx, filters.Server, filters.Zone)
		},
	}

	envSyncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize environment",
		Long:  "Synchronize the specified environment.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runEnvSync(ctx, filters.Server, filters.Zone)
		},
	}

	envCheckCmd.Flags().StringVar(&filters.Server, "server", "", "Filter by server")
	envCheckCmd.Flags().StringVar(&filters.Zone, "zone", "", "Filter by zone")

	envSyncCmd.Flags().StringVar(&filters.Server, "server", "", "Filter by server")
	envSyncCmd.Flags().StringVar(&filters.Zone, "zone", "", "Filter by zone")

	envCmd.AddCommand(envCheckCmd)
	envCmd.AddCommand(envSyncCmd)

	return envCmd
}

type envOperation func(ctx *Context, client *ssh.Client, srv *entity.Server, cfg *entity.Config, secrets map[string]string)

func loadConfigAndFilterServers(ctx *Context, server, zone string) (*entity.Config, map[string]string, error) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		return nil, nil, err
	}
	return cfg, cfg.GetSecretsMap(), nil
}

func processServers(ctx *Context, cfg *entity.Config, secrets map[string]string, server, zone string, operation envOperation) {
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

		operation(ctx, client, srv, cfg, secrets)

		client.Close()
	}
}

func runEnvCheck(ctx *Context, server, zone string) {
	cfg, secrets, err := loadConfigAndFilterServers(ctx, server, zone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	processServers(ctx, cfg, secrets, server, zone, func(ctx *Context, client *ssh.Client, srv *entity.Server, cfg *entity.Config, secrets map[string]string) {
		checker := serverpkg.NewChecker(client, srv, cfg.Registries, secrets)
		results := checker.CheckAll()
		fmt.Print(serverpkg.FormatResults(srv.Name, results))
	})
}

func runEnvSync(ctx *Context, server, zone string) {
	cfg, secrets, err := loadConfigAndFilterServers(ctx, server, zone)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	processServers(ctx, cfg, secrets, server, zone, func(ctx *Context, client *ssh.Client, srv *entity.Server, cfg *entity.Config, secrets map[string]string) {
		syncer := serverpkg.NewSyncer(client, srv, ctx.Env, secrets, cfg.Registries)
		results := syncer.SyncAll()

		fmt.Printf("[%s] Sync Results\n", srv.Name)
		for _, r := range results {
			if r.Success {
				fmt.Printf("  ✅ %s: %s\n", r.Name, r.Message)
			} else {
				fmt.Printf("  ❌ %s: %s\n", r.Name, r.Message)
			}
		}
	})
}
