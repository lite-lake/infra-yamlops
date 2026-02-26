package cli

import (
	"fmt"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/version"
	"github.com/spf13/cobra"
)

var (
	flagEnv         string
	flagConfigDir   string
	flagShowVersion bool
)

var Version = version.Version

func Execute() {
	ctx := NewContext()

	rootCmd := &cobra.Command{
		Use:   "yamlops",
		Short: "Infrastructure YAML operations tool",
		Long:  "Yamlops is a CLI tool for managing infrastructure through YAML configurations.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if flagShowVersion {
				fmt.Println(Version)
				os.Exit(0)
			}
			ctx.Env = flagEnv
			ctx.ConfigDir = flagConfigDir
		},
		Run: func(cmd *cobra.Command, args []string) {
			runTUI(ctx)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "dev", "Environment (prod/staging/dev/demo)")
	rootCmd.PersistentFlags().StringVarP(&flagConfigDir, "config", "c", ".", "Configuration directory")
	rootCmd.PersistentFlags().BoolVarP(&flagShowVersion, "version", "v", false, "Show version information")

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
