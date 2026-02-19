package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/infrastructure/persistence"
	"github.com/litelake/yamlops/internal/ssh"
)

func newCleanCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up resources",
		Long:  "Clean up temporary files and cached resources.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runClean(ctx)
		},
	}

	return cmd
}

func runClean(ctx *Context) {
	loader := persistence.NewConfigLoader(ctx.ConfigDir)
	cfg, err := loader.Load(nil, ctx.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	secrets := cfg.GetSecretsMap()
	serviceMap := cfg.GetServiceMap()
	infraServiceMap := cfg.GetInfraServiceMap()

	for _, srv := range cfg.Servers {
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

		containerStdout, _, err := client.Run("sudo docker ps -a --format '{{json .}}'")
		if err != nil {
			fmt.Printf("[%s] Failed to list containers: %v\n", srv.Name, err)
			client.Close()
			continue
		}

		dirStdout, _, err := client.Run("sudo ls -1 /data/yamlops 2>/dev/null || true")
		if err != nil {
			fmt.Printf("[%s] Failed to list directories: %v\n", srv.Name, err)
			client.Close()
			continue
		}

		orphanContainers := make(map[string]bool)
		orphanDirs := make(map[string]bool)

		for _, line := range strings.Split(strings.TrimSpace(containerStdout), "\n") {
			if line == "" {
				continue
			}
			var container struct {
				Name    string `json:"Names"`
				Image   string `json:"Image"`
				Project string `json:"Labels"`
			}
			if err := json.Unmarshal([]byte(line), &container); err != nil {
				continue
			}

			if !strings.HasPrefix(container.Name, "yo-"+ctx.Env+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(container.Name, "yo-"+ctx.Env+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				orphanContainers[container.Name] = true
			}
		}

		for _, line := range strings.Split(strings.TrimSpace(dirStdout), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "yo-"+ctx.Env+"-") {
				continue
			}
			serviceName := strings.TrimPrefix(line, "yo-"+ctx.Env+"-")
			_, isService := serviceMap[serviceName]
			_, isInfraService := infraServiceMap[serviceName]
			if !isService && !isInfraService {
				orphanDirs[line] = true
			}
		}

		client.Close()

		if len(orphanContainers) == 0 && len(orphanDirs) == 0 {
			fmt.Printf("[%s] No orphan services found\n", srv.Name)
			continue
		}

		fmt.Printf("[%s] Found %d orphan container(s) and %d orphan director(ies)\n",
			srv.Name, len(orphanContainers), len(orphanDirs))
		for name := range orphanContainers {
			fmt.Printf("  - container: %s\n", name)
		}
		for name := range orphanDirs {
			fmt.Printf("  - directory: %s\n", name)
		}

		client2, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			fmt.Printf("[%s] Reconnection failed: %v\n", srv.Name, err)
			continue
		}

		for name := range orphanContainers {
			fmt.Printf("[%s] Removing container %s...\n", srv.Name, name)
			_, stderr, err := client2.Run(fmt.Sprintf("sudo docker rm -f %s", name))
			if err != nil {
				fmt.Printf("[%s] Failed to remove container %s: %v\n%s\n", srv.Name, name, err, stderr)
			} else {
				fmt.Printf("[%s] Removed container %s\n", srv.Name, name)
			}
		}

		for name := range orphanDirs {
			remoteDir := fmt.Sprintf("/data/yamlops/%s", name)
			fmt.Printf("[%s] Removing directory %s...\n", srv.Name, remoteDir)
			_, stderr, err := client2.Run(fmt.Sprintf("sudo rm -rf %s", remoteDir))
			if err != nil {
				fmt.Printf("[%s] Failed to remove directory %s: %v\n%s\n", srv.Name, remoteDir, err, stderr)
			} else {
				fmt.Printf("[%s] Removed directory %s\n", srv.Name, remoteDir)
			}
		}
		client2.Close()
	}
}
