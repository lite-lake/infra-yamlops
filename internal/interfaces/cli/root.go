package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Env         string
	ConfigDir   string
	ShowVersion bool

	DomainFilter  string
	ZoneFilter    string
	ServerFilter  string
	ServiceFilter string
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "yamlops",
	Short: "Infrastructure YAML operations tool",
	Long:  "Yamlops is a CLI tool for managing infrastructure through YAML configurations.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if ShowVersion {
			fmt.Println(Version)
			os.Exit(0)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&Env, "env", "e", "dev", "Environment (prod/staging/dev)")
	rootCmd.PersistentFlags().StringVarP(&ConfigDir, "config", "c", ".", "Configuration directory")
	rootCmd.PersistentFlags().BoolVarP(&ShowVersion, "version", "v", false, "Show version information")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
