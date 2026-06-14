package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// ValidateResult represents a single validation check result.
type ValidateResult struct {
	Level      string // "error", "warning"
	Message    string
	Suggestion string
}

// ValidateView renders validation results.
type ValidateView struct {
	Title     string
	Env       string
	Module    string // e.g. "service", "server", "dns", "config"
	OKCount   int
	FailCount int
	WarnCount int
	Results   []ValidateResult
	Passed    bool
	Cursor    int
}

// NewValidateView creates a new ValidateView.
func NewValidateView(module, env string) *ValidateView {
	return &ValidateView{
		Title:  fmt.Sprintf("Validating %s configuration (%s)...", module, env),
		Env:    env,
		Module: module,
		Cursor: 0,
	}
}

// SetResults sets the validation results and calculates counts.
func (vv *ValidateView) SetResults(okCount int, results []ValidateResult) {
	vv.OKCount = okCount
	vv.Results = results
	vv.FailCount = 0
	vv.WarnCount = 0
	for _, r := range results {
		switch r.Level {
		case "error":
			vv.FailCount++
		case "warning":
			vv.WarnCount++
		}
	}
	vv.Passed = vv.FailCount == 0
}

// AddError adds an error result.
func (vv *ValidateView) AddError(message, suggestion string) {
	vv.Results = append(vv.Results, ValidateResult{
		Level:      "error",
		Message:    message,
		Suggestion: suggestion,
	})
	vv.FailCount++
	vv.Passed = false
}

// AddWarning adds a warning result.
func (vv *ValidateView) AddWarning(message, suggestion string) {
	vv.Results = append(vv.Results, ValidateResult{
		Level:      "warning",
		Message:    message,
		Suggestion: suggestion,
	})
	vv.WarnCount++
}

// IncrementOK increments the OK count.
func (vv *ValidateView) IncrementOK() {
	vv.OKCount++
}

func (vv *ValidateView) countSummaryLines() int {
	count := 0
	if vv.OKCount > 0 {
		count++
	}
	if vv.FailCount > 0 {
		count++
	}
	if vv.WarnCount > 0 {
		count++
	}
	return count
}

// ResultString returns the final result string (PASSED/FAILED).
func (vv *ValidateView) ResultString() string {
	if vv.Passed {
		return "Result: PASSED"
	}
	return "Result: FAILED"
}

// Render renders the validate view with viewport-based scrolling.
func (vv *ValidateView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	var lines []string

	// Title
	lines = append(lines, vv.Title)
	lines = append(lines, "")

	// Summary counts
	if vv.OKCount > 0 {
		lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("  %s %d checks passed", styles.StatusOK, vv.OKCount)))
	}
	if vv.FailCount > 0 {
		lines = append(lines, styles.ErrorStyle.Render(fmt.Sprintf("  %s %d errors found", styles.StatusFAIL, vv.FailCount)))
	}
	if vv.WarnCount > 0 {
		lines = append(lines, styles.WarningStyle.Render(fmt.Sprintf("  %s %d warnings", styles.StatusWARN, vv.WarnCount)))
	}
	lines = append(lines, "")

	headerCount := len(lines)

	// Footer: result line
	var footerLines []string
	if vv.Passed {
		footerLines = append(footerLines, styles.SuccessStyle.Render(vv.ResultString()))
	} else {
		footerLines = append(footerLines, styles.ErrorStyle.Render(vv.ResultString()))
	}

	// Build scrollable content lines from errors/warnings
	var contentLines []string
	if vv.FailCount > 0 || vv.WarnCount > 0 {
		errorIdx := 1
		warnIdx := 1
		for _, result := range vv.Results {
			if result.Level == "error" {
				if errorIdx == 1 {
					contentLines = append(contentLines, styles.ErrorStyle.Render("  ERRORS:"))
				}
				contentLines = append(contentLines, styles.ErrorStyle.Render(fmt.Sprintf("    [%d] %s", errorIdx, result.Message)))
				if result.Suggestion != "" {
					contentLines = append(contentLines, styles.MutedStyle.Render(fmt.Sprintf("        Suggestion: %s", result.Suggestion)))
				}
				contentLines = append(contentLines, "")
				errorIdx++
			}
		}
		for _, result := range vv.Results {
			if result.Level == "warning" {
				if warnIdx == 1 {
					contentLines = append(contentLines, styles.WarningStyle.Render("  WARNINGS:"))
				}
				contentLines = append(contentLines, styles.WarningStyle.Render(fmt.Sprintf("    [%d] %s", warnIdx, result.Message)))
				if result.Suggestion != "" {
					contentLines = append(contentLines, styles.MutedStyle.Render(fmt.Sprintf("        Suggestion: %s", result.Suggestion)))
				}
				contentLines = append(contentLines, "")
				warnIdx++
			}
		}
	}

	// Calculate available height for scrollable content
	contentHeight := availableHeight - headerCount - len(footerLines)
	if contentHeight < 1 {
		contentHeight = 1
	}

	totalContentLines := len(contentLines)
	if totalContentLines > 0 {
		viewport := NewComponentViewport(vv.Cursor, totalContentLines, contentHeight)
		viewport.EnsureCursorVisible()

		start := viewport.VisibleStart()
		end := viewport.VisibleEnd()
		for i := start; i < end && i < totalContentLines; i++ {
			lines = append(lines, contentLines[i])
		}

		// Scroll indicator
		if viewport.TotalRows > viewport.VisibleRows {
			lines = append(lines, viewport.RenderScrollIndicator())
		}
	}

	// Footer
	lines = append(lines, footerLines...)

	return strings.Join(lines, "\n")
}
