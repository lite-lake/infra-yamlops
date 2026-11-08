package cli

import (
	"fmt"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui"
	"github.com/lite-lake/infra-yamlops/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagEnv         string
	flagConfigDir   string
	flagConcurrency int
)

var Version = version.Version

func validateFlags(cmd *cobra.Command, args []string) error {
	// Skip -e validation for help commands
	if cmd.Name() == "help" || cmd.Flags().Changed("help") {
		return nil
	}

	// -e is required for all commands except help
	if flagEnv == "" {
		return fmt.Errorf("Environment flag is required\nSuggestion: Use -e <env> to specify environment")
	}

	if flagConfigDir != "" {
		info, err := os.Stat(flagConfigDir)
		if err != nil {
			return fmt.Errorf("config directory not found: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("config path is not a directory: %s", flagConfigDir)
		}
	}

	return nil
}

func Execute() {
	ctx := NewContext()

	rootCmd := &cobra.Command{
		Use:   "yamlops",
		Short: "Infrastructure YAML operations tool",
		Long: `YAMLOps - Infrastructure as Code management tool

Manage servers, services, DNS records, and configuration through YAML.
Supports interactive TUI and non-interactive CLI modes.`,
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFlags(cmd, args); err != nil {
				return err
			}
			ctx.Env = flagEnv
			ctx.ConfigDir = flagConfigDir
			ctx.Concurrency = flagConcurrency
			return nil
		},
	}

	rootCmd.SilenceUsage = true

	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "", "Environment (required)")
	rootCmd.PersistentFlags().StringVarP(&flagConfigDir, "config", "c", ".", "Configuration directory")
	rootCmd.PersistentFlags().IntVar(&flagConcurrency, "concurrency", 5, "Concurrency for server operations")

	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI",
		Long:  "Launch the interactive terminal user interface with menu-driven navigation.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.Run(ctx.Env, ctx.ConfigDir, ctx.Concurrency); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli",
		Short: "YAMLOps CLI - Infrastructure as Code management tool",
		Long: `Run non-interactive CLI commands for automation and scripting.

Examples:
  yamlops cli service show -e prod
  yamlops cli service show -e prod --type biz --detail
  yamlops cli service deploy -e prod --type biz --dry-run
  yamlops cli service deploy -e prod --type biz --yes
  yamlops cli dns deploy -e prod --domain example.com --dry-run
  yamlops cli dns pull domains -e prod --isp aliyun --yes
  yamlops cli server show -e prod --detail
  yamlops cli server setup -e prod --dry-run
  yamlops cli config show isps -e prod --detail
  yamlops cli config show secrets -e prod
  yamlops cli service validate -e prod`,
	}
	cliCmd.AddCommand(newDNSCommand(ctx))
	cliCmd.AddCommand(newServerCommand(ctx))
	cliCmd.AddCommand(newConfigCommand(ctx))
	cliCmd.AddCommand(newServiceCommand(ctx))

	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "HTTP API server (coming soon)",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("HTTP API mode is coming soon.")
		},
	}

	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(apiCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
