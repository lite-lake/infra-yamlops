package cli

import (
	"fmt"
	"os"

	tui "github.com/litelake/yamlops/internal/cli"
)

func runTUI() {
	if err := tui.Run(Env, ConfigDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
