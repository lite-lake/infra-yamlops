package cli

import (
	"fmt"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagEnv       string
	flagConfigDir string
)

var Version = version.Version

// validEnvironments 有效的环境列表
var validEnvironments = map[string]bool{
	"prod":    true,
	"staging": true,
	"dev":     true,
	"demo":    true,
}

// validateFlags 验证命令行标志
func validateFlags(cmd *cobra.Command, args []string) error {
	// 验证环境标志
	if flagEnv != "" && !validEnvironments[flagEnv] {
		return fmt.Errorf("invalid environment: %s. Must be one of: prod, staging, dev, demo", flagEnv)
	}

	// 验证配置目录标志
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
		Run: func(cmd *cobra.Command, args []string) {
			runTUI(ctx)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "dev", "Environment (prod/staging/dev/demo)")
	rootCmd.PersistentFlags().StringVarP(&flagConfigDir, "config", "c", ".", "Configuration directory")

	rootCmd.AddCommand(newPlanCommand(ctx))
	rootCmd.AddCommand(newApplyCommand(ctx))
	rootCmd.AddCommand(newValidateCommand(ctx))
	rootCmd.AddCommand(newListCommand(ctx))
	rootCmd.AddCommand(newShowCommand(ctx))
	rootCmd.AddCommand(newEnvCommand(ctx))
	rootCmd.AddCommand(newDNSCommand(ctx))
	rootCmd.AddCommand(newCleanCommand(ctx))
	rootCmd.AddCommand(newServerCommand(ctx))
	rootCmd.AddCommand(newConfigCommand(ctx))
	rootCmd.AddCommand(newAppCommand(ctx))
	rootCmd.AddCommand(newServiceCommand(ctx))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
