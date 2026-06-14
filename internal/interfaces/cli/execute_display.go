package cli

import (
	"fmt"
)

type ExecuteItem struct {
	Action  string
	Name    string
	Server  string
	Record  string
	Details string
	Status  string
	Success bool
}

type ExecuteResult struct {
	Success bool
	Error   string
}

func DisplayExecuting() {
	fmt.Println("\nEXECUTING...")
}

func DisplayExecuteStep(current, total int, item ExecuteItem) {
	if item.Server != "" && item.Server != "-" {
		if item.Record != "" {
			if item.Success {
				fmt.Printf("[%d/%d] %-8s %-16s %-8s [OK] %s (%s)\n",
					current, total, item.Action, item.Name, item.Record, item.Status, item.Server)
			} else {
				fmt.Printf("[%d/%d] %-8s %-16s %-8s [FAIL] %s (%s)\n",
					current, total, item.Action, item.Name, item.Record, item.Status, item.Server)
			}
		} else {
			if item.Success {
				fmt.Printf("[%d/%d] %-8s %-16s [OK] %s (%s)\n",
					current, total, item.Action, item.Name, item.Status, item.Server)
			} else {
				fmt.Printf("[%d/%d] %-8s %-16s [FAIL] %s (%s)\n",
					current, total, item.Action, item.Name, item.Status, item.Server)
			}
		}
	} else {
		if item.Record != "" {
			if item.Success {
				fmt.Printf("[%d/%d] %-8s %-16s %-12s [OK] %s\n",
					current, total, item.Action, item.Name, item.Record, item.Status)
			} else {
				fmt.Printf("[%d/%d] %-8s %-16s %-12s [FAIL] %s\n",
					current, total, item.Action, item.Name, item.Record, item.Status)
			}
		} else {
			if item.Success {
				fmt.Printf("[%d/%d] %-8s %-16s [OK] %s\n",
					current, total, item.Action, item.Name, item.Status)
			} else {
				fmt.Printf("[%d/%d] %-8s %-16s [FAIL] %s\n",
					current, total, item.Action, item.Name, item.Status)
			}
		}
	}
}

func DisplayExecuteStepWithError(current, total int, item ExecuteItem, errMsg, suggestion string) {
	if item.Server != "" && item.Server != "-" {
		if item.Record != "" {
			fmt.Printf("[%d/%d] %-8s %-16s %-8s [FAIL] %s (%s)\n",
				current, total, item.Action, item.Name, item.Record, item.Status, item.Server)
		} else {
			fmt.Printf("[%d/%d] %-8s %-16s [FAIL] %s (%s)\n",
				current, total, item.Action, item.Name, item.Status, item.Server)
		}
	} else {
		if item.Record != "" {
			fmt.Printf("[%d/%d] %-8s %-16s %-12s [FAIL] %s\n",
				current, total, item.Action, item.Name, item.Record, item.Status)
		} else {
			fmt.Printf("[%d/%d] %-8s %-16s [FAIL] %s\n",
				current, total, item.Action, item.Name, item.Status)
		}
	}
	fmt.Printf("        Error: %s\n", errMsg)
	if suggestion != "" {
		fmt.Printf("        Suggestion: %s\n", suggestion)
	}
}

func DisplayResult(succeeded, failed int) {
	if failed > 0 {
		fmt.Printf("\nRESULT: %d succeeded, %d failed (exit code 3)\n", succeeded, failed)
		ExitWithExecutionError(fmt.Sprintf("Execution failed: %d operation(s) failed", failed))
	}
	fmt.Printf("\nRESULT: %d succeeded, %d failed\n", succeeded, failed)
}

func DisplayResultWithSkipped(succeeded, failed, skipped int) {
	if failed > 0 {
		fmt.Printf("\nRESULT: %d succeeded, %d failed, %d skipped (interrupted) (exit code 3)\n", succeeded, failed, skipped)
		ExitWithExecutionError(fmt.Sprintf("Execution failed: %d operation(s) failed", failed))
	}
	fmt.Printf("\nRESULT: %d succeeded, %d failed, %d skipped (interrupted)\n", succeeded, failed, skipped)
}
