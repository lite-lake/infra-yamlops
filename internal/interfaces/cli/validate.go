package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/litelake/yamlops/internal/infrastructure/persistence"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configurations",
	Long:  "Validate all YAML configurations.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runValidate()
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate() {
	loader := persistence.NewConfigLoader(ConfigDir)
	cfg, err := loader.Load(nil, Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := loader.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Configuration is valid.")
}
