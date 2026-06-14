package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// CompleteItem represents a completed operation result.
type CompleteItem struct {
	Action     string
	Name       string
	Server     string
	Success    bool
	Skipped    bool
	Error      string
	Suggestion string
}

// CompleteView renders execution results.
type CompleteView struct {
	Title         string
	Env           string
	Items         []CompleteItem
	Duration      string
	Cursor        int
	ScrollOffset  int
	customSummary string
}

// NewCompleteView creates a new CompleteView.
func NewCompleteView(title, env string) *CompleteView {
	return &CompleteView{
		Title: title,
		Env:   env,
	}
}

// SetItems sets the result items.
func (cv *CompleteView) SetItems(items []CompleteItem) {
	cv.Items = items
	cv.Cursor = 0
}

// SetSummary overrides the default summary with a custom string.
func (cv *CompleteView) SetSummary(text string) {
	cv.customSummary = text
}

// AddItem appends a single result item.
func (cv *CompleteView) AddItem(item CompleteItem) {
	cv.Items = append(cv.Items, item)
}

// SuccessCount returns the number of successful items.
func (cv *CompleteView) SuccessCount() int {
	count := 0
	for _, item := range cv.Items {
		if item.Success {
			count++
		}
	}
	return count
}

// FailCount returns the number of failed items.
func (cv *CompleteView) FailCount() int {
	count := 0
	for _, item := range cv.Items {
		if !item.Success && !item.Skipped {
			count++
		}
	}
	return count
}

// SkippedCount returns the number of skipped items.
func (cv *CompleteView) SkippedCount() int {
	count := 0
	for _, item := range cv.Items {
		if item.Skipped {
			count++
		}
	}
	return count
}

// HasFailures returns true if any item failed.
func (cv *CompleteView) HasFailures() bool {
	return cv.FailCount() > 0
}

// Summary returns the summary string (custom or default).
func (cv *CompleteView) Summary() string {
	if cv.customSummary != "" {
		return cv.customSummary
	}
	if cv.SkippedCount() > 0 {
		return fmt.Sprintf("SUMMARY: %d succeeded, %d failed, %d skipped", cv.SuccessCount(), cv.FailCount(), cv.SkippedCount())
	}
	return fmt.Sprintf("SUMMARY: %d succeeded, %d failed", cv.SuccessCount(), cv.FailCount())
}

// TotalLines returns the total number of rendered lines (for scroll calculations).
func (cv *CompleteView) TotalLines() int {
	count := 2 // title + blank
	for _, item := range cv.Items {
		count++ // item line
		if !item.Success && !item.Skipped {
			if item.Error != "" {
				count++ // error detail line
			}
			if item.Suggestion != "" {
				count++ // suggestion detail line
			}
		}
	}
	count += 2 // blank + summary
	if cv.customSummary != "" {
		// custom summary may contain multiple lines
		count += strings.Count(cv.customSummary, "\n")
	}
	if cv.SkippedCount() > 0 {
		count++ // interrupt info line
	}
	if cv.Duration != "" {
		count++ // duration line
	}
	return count
}

// CursorUp moves the cursor up.
func (cv *CompleteView) CursorUp() {
	if cv.Cursor > 0 {
		cv.Cursor--
	}
}

// CursorDown moves the cursor down.
func (cv *CompleteView) CursorDown() {
	if cv.Cursor < len(cv.Items)-1 {
		cv.Cursor++
	}
}

// ScrollUp scrolls the viewport up by one line.
func (cv *CompleteView) ScrollUp() {
	if cv.ScrollOffset > 0 {
		cv.ScrollOffset--
	}
}

// ScrollDown scrolls the viewport down by one line.
func (cv *CompleteView) ScrollDown(totalLines, visibleRows int) {
	maxOffset := max(0, totalLines-visibleRows)
	if cv.ScrollOffset < maxOffset {
		cv.ScrollOffset++
	}
}

// MaxScrollOffset returns the maximum scroll offset for the given dimensions.
func (cv *CompleteView) MaxScrollOffset(visibleRows int) int {
	totalLines := cv.TotalLines()
	return max(0, totalLines-visibleRows)
}

// actionStatusMap maps service deploy actions to their completion status text.
// Only create/update/delete actions are mapped; other actions (stop/restart/cleanup/sync/import)
// use the original action text.
var actionStatusMap = map[string]string{
	"create": "deployed",
	"update": "updated",
	"delete": "deleted",
}

// actionStatus returns the appropriate status text for the given action.
func actionStatus(action string) string {
	if s, ok := actionStatusMap[action]; ok {
		return s
	}
	return action
}

// Render renders the complete view.
func (cv *CompleteView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	var lines []string

	// Title
	title := cv.Title
	if title == "" {
		title = "RESULT"
	}
	lines = append(lines, styles.BrandStyle.Render(title))
	lines = append(lines, "")

	// Render items
	for _, item := range cv.Items {
		if item.Skipped {
			icon := styles.IconSkip
			status := "skipped (interrupted)"
			if item.Action != "" {
				status = item.Action + " (skipped)"
			}
			line := fmt.Sprintf("  %s %-8s %-16s %s (%s)",
				styles.MutedStyle.Render(icon),
				item.Action,
				truncate(item.Name, 16),
				status,
				item.Server,
			)
			lines = append(lines, line)
		} else if item.Success {
			icon := styles.IconSuccess
			status := "done"
			if item.Action != "" {
				status = actionStatus(item.Action)
			}
			line := fmt.Sprintf("  %s %-8s %-16s %s (%s)",
				styles.SuccessStyle.Render(icon),
				item.Action,
				truncate(item.Name, 16),
				status,
				item.Server,
			)
			lines = append(lines, line)
		} else {
			icon := styles.IconFailed
			status := "failed"
			line := fmt.Sprintf("  %s %-8s %-16s %s (%s)",
				styles.ErrorStyle.Render(icon),
				item.Action,
				truncate(item.Name, 16),
				status,
				item.Server,
			)
			lines = append(lines, line)

			// Error details
			if item.Error != "" {
				lines = append(lines, styles.ErrorStyle.Render(fmt.Sprintf("        Error: %s", item.Error)))
			}
			if item.Suggestion != "" {
				lines = append(lines, styles.WarningStyle.Render(fmt.Sprintf("        Suggestion: %s", item.Suggestion)))
			}
		}
	}

	// Summary
	lines = append(lines, "")
	lines = append(lines, cv.Summary())

	// Interrupt info
	if cv.SkippedCount() > 0 {
		lines = append(lines, styles.MutedStyle.Render("[INFO] Operation was interrupted by user. Already executed operations are not rolled back."))
	}

	// Duration
	if cv.Duration != "" {
		lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Duration: %s", cv.Duration)))
	}

	// Apply viewport scrolling
	totalLines := len(lines)
	visibleRows := availableHeight
	if visibleRows > totalLines {
		visibleRows = totalLines
	}

	// Clamp scroll offset
	maxOffset := max(0, totalLines-visibleRows)
	if cv.ScrollOffset > maxOffset {
		cv.ScrollOffset = maxOffset
	}
	if cv.ScrollOffset < 0 {
		cv.ScrollOffset = 0
	}

	start := cv.ScrollOffset
	end := start + visibleRows
	if end > totalLines {
		end = totalLines
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		sb.WriteString(lines[i])
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	// Scroll indicator
	if totalLines > visibleRows {
		sb.WriteString("\n")
		sb.WriteString(styles.ScrollIndicatorStyle.Render(fmt.Sprintf("[%d/%d]", start+visibleRows, totalLines)))
	}

	return sb.String()
}
