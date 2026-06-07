package cli

import (
	"fmt"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui"
	"github.com/lite-lake/infra-yamlops/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagEnv       string
	flagConfigDir string
)

var Version = version.Version

func validateFlags(cmd *cobra.Command, args []string) error {
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
		Use:     "yamlops",
		Short:   "Infrastructure YAML operations tool",
		Long:    "Yamlops is a CLI tool for managing infrastructure through YAML configurations.",
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFlags(cmd, args); err != nil {
				return err
			}
			ctx.Env = flagEnv
			ctx.ConfigDir = flagConfigDir
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "dev", "Environment")
	rootCmd.PersistentFlags().StringVarP(&flagConfigDir, "config", "c", ".", "Configuration directory")

	tuiCmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI",
		Long:  "Launch the interactive terminal user interface.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := tui.Run(ctx.Env, ctx.ConfigDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli",
		Short: "Non-interactive CLI commands",
		Long:  "Run non-interactive CLI commands for automation and scripting.",
	}
	cliCmd.AddCommand(newPlanCommand(ctx))
	cliCmd.AddCommand(newApplyCommand(ctx))
	cliCmd.AddCommand(newValidateCommand(ctx))
	cliCmd.AddCommand(newListCommand(ctx))
	cliCmd.AddCommand(newShowCommand(ctx))
	cliCmd.AddCommand(newEnvCommand(ctx))
	cliCmd.AddCommand(newDNSCommand(ctx))
	cliCmd.AddCommand(newCleanCommand(ctx))
	cliCmd.AddCommand(newServerCommand(ctx))
	cliCmd.AddCommand(newConfigCommand(ctx))
	cliCmd.AddCommand(newAppCommand(ctx))
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
