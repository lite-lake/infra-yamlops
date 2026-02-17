package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/litelake/yamlops/internal/apply"
	"github.com/litelake/yamlops/internal/cli"
	"github.com/litelake/yamlops/internal/config"
	"github.com/litelake/yamlops/internal/plan"
	"github.com/litelake/yamlops/internal/ssh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	env         string
	configDir   string
	showVersion bool

	domainFilter  string
	zoneFilter    string
	serverFilter  string
	serviceFilter string
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "yamlops",
	Short: "Infrastructure YAML operations tool",
	Long:  "Yamlops is a CLI tool for managing infrastructure through YAML configurations.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Println(version)
			os.Exit(0)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

var planCmd = &cobra.Command{
	Use:   "plan [scope]",
	Short: "Generate execution plan",
	Long:  "Generate an execution plan for the specified scope.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := ""
		if len(args) > 0 {
			scope = args[0]
		}
		runPlan(scope, domainFilter, zoneFilter, serverFilter, serviceFilter)
	},
}

var applyCmd = &cobra.Command{
	Use:   "apply [scope]",
	Short: "Apply changes",
	Long:  "Apply infrastructure changes for the specified scope.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		scope := ""
		if len(args) > 0 {
			scope = args[0]
		}
		runApply(scope, domainFilter, zoneFilter, serverFilter, serviceFilter)
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configurations",
	Long:  "Validate all YAML configurations.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runValidate()
	},
}

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
		runEnvCheck(serverFilter, zoneFilter)
	},
}

var envSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize environment",
	Long:  "Synchronize the specified environment.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runEnvSync(serverFilter, zoneFilter)
	},
}

var listCmd = &cobra.Command{
	Use:   "list <entity>",
	Short: "List entities",
	Long:  "List all entities of the specified type.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		entity := args[0]
		runList(entity)
	},
}

var showCmd = &cobra.Command{
	Use:   "show <entity> <name>",
	Short: "Show entity details",
	Long:  "Show detailed information for the specified entity.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		entity := args[0]
		name := args[1]
		runShow(entity, name)
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up resources",
	Long:  "Clean up temporary files and cached resources.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runClean()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&env, "env", "e", "dev", "Environment (prod/staging/dev)")
	rootCmd.PersistentFlags().StringVarP(&configDir, "config", "c", ".", "Configuration directory")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	planCmd.Flags().StringVar(&domainFilter, "domain", "", "Filter by domain")
	planCmd.Flags().StringVar(&zoneFilter, "zone", "", "Filter by zone")
	planCmd.Flags().StringVar(&serverFilter, "server", "", "Filter by server")
	planCmd.Flags().StringVar(&serviceFilter, "service", "", "Filter by service")

	applyCmd.Flags().StringVar(&domainFilter, "domain", "", "Filter by domain")
	applyCmd.Flags().StringVar(&zoneFilter, "zone", "", "Filter by zone")
	applyCmd.Flags().StringVar(&serverFilter, "server", "", "Filter by server")
	applyCmd.Flags().StringVar(&serviceFilter, "service", "", "Filter by service")

	envCheckCmd.Flags().StringVar(&serverFilter, "server", "", "Filter by server")
	envCheckCmd.Flags().StringVar(&zoneFilter, "zone", "", "Filter by zone")

	envSyncCmd.Flags().StringVar(&serverFilter, "server", "", "Filter by server")
	envSyncCmd.Flags().StringVar(&zoneFilter, "zone", "", "Filter by zone")

	envCmd.AddCommand(envCheckCmd)
	envCmd.AddCommand(envSyncCmd)

	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(cleanCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI() {
	if err := cli.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runPlan(scope, domain, zone, server, service string) {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	planner := plan.NewPlanner(cfg)
	planScope := &plan.Scope{
		Domain:  domain,
		Zone:    zone,
		Server:  server,
		Service: service,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes detected.")
		return
	}

	fmt.Println("Execution Plan:")
	fmt.Println("===============")
	for _, ch := range executionPlan.Changes {
		var prefix string
		switch ch.Type {
		case plan.ChangeTypeCreate:
			prefix = "+"
		case plan.ChangeTypeUpdate:
			prefix = "~"
		case plan.ChangeTypeDelete:
			prefix = "-"
		default:
			prefix = " "
		}
		fmt.Printf("%s %s: %s\n", prefix, ch.Entity, ch.Name)
		for _, action := range ch.Actions {
			fmt.Printf("    - %s\n", action)
		}
	}
}

func runApply(scope, domain, zone, server, service string) {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	planner := plan.NewPlanner(cfg)
	planScope := &plan.Scope{
		Domain:  domain,
		Zone:    zone,
		Server:  server,
		Service: service,
	}

	executionPlan, err := planner.Plan(planScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating plan: %v\n", err)
		os.Exit(1)
	}

	if !executionPlan.HasChanges() {
		fmt.Println("No changes to apply.")
		return
	}

	if err := planner.GenerateDeployments(); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating deployments: %v\n", err)
		os.Exit(1)
	}

	executor := apply.NewExecutor(executionPlan)
	executor.SetSecrets(cfg.GetSecretsMap())

	for _, srv := range cfg.Servers {
		if server != "" && srv.Name != server {
			continue
		}
		if zone != "" && srv.Zone != zone {
			continue
		}
		password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving password for server %s: %v\n", srv.Name, err)
			continue
		}
		executor.RegisterServer(srv.Name, srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
	}

	results := executor.Apply()

	hasError := false
	for _, result := range results {
		if result.Success {
			fmt.Printf("✓ %s: %s\n", result.Change.Entity, result.Change.Name)
		} else {
			fmt.Printf("✗ %s: %s - %v\n", result.Change.Entity, result.Change.Name, result.Error)
			hasError = true
		}
	}

	if hasError {
		os.Exit(1)
	}
}

func runValidate() {
	loader := config.NewLoader(env, configDir)
	_, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration is valid.")
}

func runEnvCheck(server, zone string) {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
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

		stdout, _, err := client.Run("docker ps --format '{{.Names}}'")
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
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
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

		stdout, stderr, err := client.Run("docker pull alpine:latest && docker images alpine:latest --format '{{.ID}}'")
		client.Close()

		if err != nil {
			fmt.Printf("[%s] Sync failed: %v\n%s\n", srv.Name, err, stderr)
			continue
		}

		fmt.Printf("[%s] Synced: alpine:%s\n", srv.Name, strings.TrimSpace(stdout))
	}
}

func runList(entity string) {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	entity = strings.ToLower(entity)
	switch entity {
	case "secrets":
		for _, s := range cfg.Secrets {
			fmt.Printf("- %s\n", s.Name)
		}
	case "isps":
		for _, i := range cfg.ISPs {
			fmt.Printf("- %s (services: %v)\n", i.Name, i.Services)
		}
	case "zones":
		for _, z := range cfg.Zones {
			fmt.Printf("- %s (isp: %s, region: %s)\n", z.Name, z.ISP, z.Region)
		}
	case "gateways":
		for _, g := range cfg.Gateways {
			fmt.Printf("- %s (server: %s, ports: %d/%d)\n", g.Name, g.Server, g.Ports.HTTP, g.Ports.HTTPS)
		}
	case "servers":
		for _, s := range cfg.Servers {
			fmt.Printf("- %s (zone: %s, ip: %s)\n", s.Name, s.Zone, s.IP.Public)
		}
	case "services":
		for _, s := range cfg.Services {
			fmt.Printf("- %s (server: %s, port: %d)\n", s.Name, s.Server, s.Port)
		}
	case "registries":
		for _, r := range cfg.Registries {
			fmt.Printf("- %s (%s)\n", r.Name, r.URL)
		}
	case "domains":
		for _, d := range cfg.Domains {
			fmt.Printf("- %s (isp: %s)\n", d.Name, d.ISP)
		}
	case "records", "dns":
		for _, r := range cfg.DNSRecords {
			fmt.Printf("- %s %s %s -> %s (ttl: %d)\n", r.Domain, r.Type, r.Name, r.Value, r.TTL)
		}
	case "certificates", "certs":
		for _, c := range cfg.Certificates {
			fmt.Printf("- %s (domains: %v, provider: %s)\n", c.Name, c.Domains, c.Provider)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entity)
		fmt.Fprintf(os.Stderr, "Valid types: secrets, isps, zones, gateways, servers, services, registries, domains, records, certificates\n")
		os.Exit(1)
	}
}

func runShow(entity, name string) {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	entity = strings.ToLower(entity)
	var found interface{}

	switch entity {
	case "secret":
		for _, s := range cfg.Secrets {
			if s.Name == name {
				found = s
				break
			}
		}
	case "isp":
		if m := cfg.GetISPMap(); m[name] != nil {
			found = m[name]
		}
	case "zone":
		if m := cfg.GetZoneMap(); m[name] != nil {
			found = m[name]
		}
	case "gateway":
		if m := cfg.GetGatewayMap(); m[name] != nil {
			found = m[name]
		}
	case "server":
		if m := cfg.GetServerMap(); m[name] != nil {
			found = m[name]
		}
	case "service":
		if m := cfg.GetServiceMap(); m[name] != nil {
			found = m[name]
		}
	case "registry":
		if m := cfg.GetRegistryMap(); m[name] != nil {
			found = m[name]
		}
	case "domain":
		if m := cfg.GetDomainMap(); m[name] != nil {
			found = m[name]
		}
	case "certificate", "cert":
		if m := cfg.GetCertificateMap(); m[name] != nil {
			found = m[name]
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown entity type: %s\n", entity)
		os.Exit(1)
	}

	if found == nil {
		fmt.Fprintf(os.Stderr, "%s '%s' not found\n", entity, name)
		os.Exit(1)
	}

	data, err := yaml.Marshal(found)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling entity: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", strings.Title(entity), name)
	fmt.Println(string(data))
}

func runClean() {
	loader := config.NewLoader(env, configDir)
	cfg, err := loader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	secrets := cfg.GetSecretsMap()
	serviceMap := cfg.GetServiceMap()
	knownServices := make(map[string]bool)
	for _, s := range cfg.Services {
		knownServices[s.Name] = true
	}

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

		stdout, _, err := client.Run("docker ps -a --format '{{json .}}'")
		if err != nil {
			fmt.Printf("[%s] Failed to list containers: %v\n", srv.Name, err)
			client.Close()
			continue
		}

		var orphans []string
		for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
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

			if !strings.HasPrefix(container.Name, "yo-") {
				continue
			}
			serviceName := strings.TrimPrefix(container.Name, "yo-")
			if _, exists := serviceMap[serviceName]; !exists {
				orphans = append(orphans, container.Name)
			}
		}
		client.Close()

		if len(orphans) == 0 {
			fmt.Printf("[%s] No orphan services found\n", srv.Name)
			continue
		}

		fmt.Printf("[%s] Found %d orphan service(s):\n", srv.Name, len(orphans))
		for _, name := range orphans {
			fmt.Printf("  - %s\n", name)
		}

		client2, err := ssh.NewClient(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password)
		if err != nil {
			fmt.Printf("[%s] Reconnection failed: %v\n", srv.Name, err)
			continue
		}

		for _, name := range orphans {
			fmt.Printf("[%s] Removing %s...\n", srv.Name, name)
			_, stderr, err := client2.Run(fmt.Sprintf("docker rm -f %s", name))
			if err != nil {
				fmt.Printf("[%s] Failed to remove %s: %v\n%s\n", srv.Name, name, err, stderr)
			} else {
				fmt.Printf("[%s] Removed %s\n", srv.Name, name)
			}
		}
		client2.Close()
	}
}
