package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// ProgressItemStatus represents the status of a single progress item.
type ProgressItemStatus int

const (
	ProgressStatusWaiting ProgressItemStatus = iota
	ProgressStatusRunning
	ProgressStatusSuccess
	ProgressStatusFailed
	ProgressStatusSkipped
)

// ProgressItem represents a single item being executed.
type ProgressItem struct {
	Action string
	Name   string
	Server string
	Status ProgressItemStatus
	Error  string
}

// ProgressServerGroup groups progress items by server.
type ProgressServerGroup struct {
	ServerName string
	Items      []ProgressItem
}

// ProgressView renders execution progress.
type ProgressView struct {
	Title       string
	Env         string
	Groups      []ProgressServerGroup
	Progress    int
	Total       int
	Elapsed     string // e.g. "12s"
	Interrupted bool
}

// NewProgressView creates a new ProgressView.
func NewProgressView(title, env string) *ProgressView {
	return &ProgressView{
		Title: title,
		Env:   env,
	}
}

// SetGroups sets the server groups and calculates totals.
func (pv *ProgressView) SetGroups(groups []ProgressServerGroup) {
	pv.Groups = groups
	pv.Total = 0
	for _, g := range groups {
		pv.Total += len(g.Items)
	}
}

// SetProgress sets the current progress count.
func (pv *ProgressView) SetProgress(progress int) {
	pv.Progress = progress
}

// IncrementProgress increments progress by 1.
func (pv *ProgressView) IncrementProgress() {
	pv.Progress++
}

// UpdateItemStatus updates the status of a specific item.
func (pv *ProgressView) UpdateItemStatus(serverName, itemName string, status ProgressItemStatus, errMsg string) {
	for gi, g := range pv.Groups {
		if g.ServerName == serverName {
			for ii, item := range g.Items {
				if item.Name == itemName {
					pv.Groups[gi].Items[ii].Status = status
					pv.Groups[gi].Items[ii].Error = errMsg
					return
				}
			}
		}
	}
}

// MarkCurrentRunning marks the next waiting item as running.
func (pv *ProgressView) MarkCurrentRunning() {
	for gi := range pv.Groups {
		for ii := range pv.Groups[gi].Items {
			if pv.Groups[gi].Items[ii].Status == ProgressStatusWaiting {
				pv.Groups[gi].Items[ii].Status = ProgressStatusRunning
				return
			}
		}
	}
}

// SyncRunningFromTracker updates item statuses based on actual running state
// reported by the executor. Items in runningKeys are marked Running; other
// waiting items remain Waiting.
func (pv *ProgressView) SyncRunningFromTracker(runningKeys map[string]bool) {
	for gi, g := range pv.Groups {
		for ii, item := range g.Items {
			if item.Status == ProgressStatusWaiting {
				key := g.ServerName + ":" + item.Name
				if runningKeys[key] {
					pv.Groups[gi].Items[ii].Status = ProgressStatusRunning
				}
			}
		}
	}
}

// MarkCurrentSuccess marks the current running item as success.
func (pv *ProgressView) MarkCurrentSuccess() {
	for gi := range pv.Groups {
		for ii := range pv.Groups[gi].Items {
			if pv.Groups[gi].Items[ii].Status == ProgressStatusRunning {
				pv.Groups[gi].Items[ii].Status = ProgressStatusSuccess
				return
			}
		}
	}
}

// MarkCurrentFailed marks the current running item as failed.
func (pv *ProgressView) MarkCurrentFailed(errMsg string) {
	for gi := range pv.Groups {
		for ii := range pv.Groups[gi].Items {
			if pv.Groups[gi].Items[ii].Status == ProgressStatusRunning {
				pv.Groups[gi].Items[ii].Status = ProgressStatusFailed
				pv.Groups[gi].Items[ii].Error = errMsg
				return
			}
		}
	}
}

// MarkRemainingSkipped marks all waiting and running items as skipped.
func (pv *ProgressView) MarkRemainingSkipped() {
	for gi := range pv.Groups {
		for ii := range pv.Groups[gi].Items {
			if pv.Groups[gi].Items[ii].Status == ProgressStatusWaiting ||
				pv.Groups[gi].Items[ii].Status == ProgressStatusRunning {
				pv.Groups[gi].Items[ii].Status = ProgressStatusSkipped
			}
		}
	}
}

// IsComplete returns true if all items are done.
func (pv *ProgressView) IsComplete() bool {
	if pv.Total == 0 {
		return true
	}
	return pv.Progress >= pv.Total
}

// Summary returns a summary string.
func (pv *ProgressView) Summary() string {
	success := 0
	failed := 0
	skipped := 0
	for _, g := range pv.Groups {
		for _, item := range g.Items {
			switch item.Status {
			case ProgressStatusSuccess:
				success++
			case ProgressStatusFailed:
				failed++
			case ProgressStatusSkipped:
				skipped++
			}
		}
	}
	parts := []string{fmt.Sprintf("%d succeeded", success)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	return "RESULT: " + strings.Join(parts, ", ")
}

// activeLineIndex returns the line index in the scrollable items section
// corresponding to the currently running or most recently completed item.
func (pv *ProgressView) activeLineIndex() int {
	lineIdx := 0
	for _, group := range pv.Groups {
		lineIdx++ // group header line
		for _, item := range group.Items {
			if item.Status == ProgressStatusRunning {
				return lineIdx
			}
			lineIdx++
		}
	}
	// No running item — show last line
	return max(0, lineIdx-1)
}

// Render renders the progress view with viewport-based scrolling.
func (pv *ProgressView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	// Build fixed header lines
	var headerLines []string
	headerLines = append(headerLines, "")
	if pv.Interrupted {
		headerLines = append(headerLines, styles.WarningStyle.Render("INTERRUPTED"))
		headerLines = append(headerLines, "")
		headerLines = append(headerLines, pv.Summary())
	} else {
		headerLines = append(headerLines, styles.BrandStyle.Render("EXECUTING..."))
	}
	headerLines = append(headerLines, "")
	headerCount := len(headerLines)

	// Build footer lines
	var footerLines []string
	if pv.Total > 0 {
		progress := float64(pv.Progress) / float64(pv.Total)
		barWidth := styles.ProgressWidth
		filled := int(progress * float64(barWidth))
		bar := strings.Repeat(styles.ProgressFilled, filled) + strings.Repeat(styles.ProgressEmpty, barWidth-filled)
		footerLines = append(footerLines, "")
		footerLines = append(footerLines, fmt.Sprintf("  Progress: %s %.0f%%", styles.ProgressBarStyle.Render(bar), progress*100))
	}
	if pv.Elapsed != "" {
		footerLines = append(footerLines, "")
		footerLines = append(footerLines, styles.MutedStyle.Render(fmt.Sprintf("  Elapsed: %s", pv.Elapsed)))
	}
	footerCount := len(footerLines)

	// Build scrollable content lines (group headers + items)
	var contentLines []string
	itemIndex := 0
	for _, group := range pv.Groups {
		contentLines = append(contentLines, fmt.Sprintf("  %s %s", styles.IconExpanded, group.ServerName))
		for _, item := range group.Items {
			icon := pv.statusIcon(item.Status)
			iconStyle := pv.statusStyle(item.Status)
			indexStr := fmt.Sprintf("[%d/%d]", itemIndex+1, pv.Total)
			statusText := pv.statusText(item.Status, item.Action)
			line := fmt.Sprintf("    %s %s %-8s %-16s %s",
				iconStyle.Render(icon),
				indexStr,
				item.Action,
				truncate(item.Name, 16),
				statusText,
			)
			contentLines = append(contentLines, line)
			itemIndex++
		}
	}

	// Calculate content height available for scrollable items
	contentHeight := availableHeight - headerCount - footerCount
	if contentHeight < 1 {
		contentHeight = 1
	}

	totalContentLines := len(contentLines)

	// Auto-scroll to the active item
	activeIdx := pv.activeLineIndex()
	viewport := NewComponentViewport(activeIdx, totalContentLines, contentHeight)
	viewport.EnsureCursorVisible()

	// Assemble final output
	var lines []string
	lines = append(lines, headerLines...)

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < totalContentLines; i++ {
		lines = append(lines, contentLines[i])
	}

	// Scroll indicator
	if viewport.TotalRows > viewport.VisibleRows {
		lines = append(lines, viewport.RenderScrollIndicator())
	}

	lines = append(lines, footerLines...)

	return strings.Join(lines, "\n")
}

func (pv *ProgressView) statusIcon(status ProgressItemStatus) string {
	switch status {
	case ProgressStatusSuccess:
		return styles.IconSuccess
	case ProgressStatusFailed:
		return styles.IconFailed
	case ProgressStatusRunning:
		return styles.IconRunning
	case ProgressStatusSkipped:
		return styles.IconSkip
	default:
		return styles.IconWaiting
	}
}

func (pv *ProgressView) statusStyle(status ProgressItemStatus) lipgloss.Style {
	switch status {
	case ProgressStatusSuccess:
		return styles.SuccessStyle
	case ProgressStatusFailed:
		return styles.ErrorStyle
	case ProgressStatusRunning:
		return styles.WarningStyle
	case ProgressStatusSkipped:
		return styles.MutedStyle
	default:
		return styles.MutedStyle
	}
}

func (pv *ProgressView) statusText(status ProgressItemStatus, action string) string {
	switch status {
	case ProgressStatusSuccess:
		return successVerbForAction(action)
	case ProgressStatusFailed:
		return "failed"
	case ProgressStatusRunning:
		return runningTextForAction(action)
	case ProgressStatusSkipped:
		return "skipped (interrupted)"
	default:
		return "waiting"
	}
}

func successVerbForAction(action string) string {
	switch strings.ToLower(action) {
	case "create":
		return "deployed"
	case "update":
		return "updated"
	case "stop":
		return "stopped"
	case "restart":
		return "restarted"
	case "cleanup", "clean":
		return "cleaned"
	case "sync":
		return "synced"
	case "import":
		return "imported"
	default:
		return "done"
	}
}

func runningTextForAction(action string) string {
	switch strings.ToLower(action) {
	case "create":
		return "deploying..."
	case "update":
		return "updating..."
	case "stop":
		return "stopping..."
	case "restart":
		return "restarting..."
	case "cleanup", "clean":
		return "cleaning..."
	case "sync":
		return "syncing..."
	case "import":
		return "importing..."
	default:
		return "executing..."
	}
}
