package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newValidateCommand(ctx *Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configurations",
		Long:  "Validate all YAML configurations.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runValidate(ctx)
		},
	}

	return cmd
}

func runValidate(ctx *Context) {
	wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
	if _, err := wf.LoadAndValidate(nil); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration is valid.")
}
