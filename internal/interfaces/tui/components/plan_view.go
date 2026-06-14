package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// PlanItem represents a single change item in the plan.
type PlanItem struct {
	Action      string // "create", "update", "delete", "stop", "restart", "cleanup", "sync", "import"
	Name        string
	Server      string // server name, or domain for DNS
	Details     string
	ChangeType  string // "+", "~", "-"
	Selected    bool
	DetailLines []string // extra detail lines shown only in detail view (indented)
}

// PlanView renders a plan preview with checkbox selection per item.
type PlanView struct {
	Title         string // e.g. "PLAN: service deploy"
	Env           string
	Type          string // e.g. "biz", "infra", "" for non-service
	OperationType string // "deploy", "stop", "restart", "cleanup", "server_setup", "dns_deploy", "dns_pull_domains", "dns_pull_records"
	Forced        bool
	DryRun        bool
	Items         []PlanItem
	Cursor        int
	ShowDetail    bool
	EnvWarning    bool   // show environment warning
	NoChanges     bool   // no changes detected
	ForceHint     bool   // show force hint when no changes
	SubHeader     string // optional extra header line (e.g. "ISP: aliyun")

	// Detail view scroll offset (item index)
	detailOffset int

	// Derived counts
	created  int
	updated  int
	deleted  int
	selected int
}

// NewPlanView creates a new PlanView with default state.
func NewPlanView(title, env, opType, operationType string, forced bool) *PlanView {
	return &PlanView{
		Title:         title,
		Env:           env,
		Type:          opType,
		OperationType: operationType,
		Forced:        forced,
		Items:         nil,
		Cursor:        0,
		ShowDetail:    false,
		EnvWarning:    true,
	}
}

// SetItems sets the plan items and initializes all to selected.
func (pv *PlanView) SetItems(items []PlanItem) {
	for i := range items {
		items[i].Selected = true
	}
	pv.Items = items
	pv.Cursor = 0
	pv.recalcCounts()
}

func (pv *PlanView) recalcCounts() {
	pv.created = 0
	pv.updated = 0
	pv.deleted = 0
	pv.selected = 0
	for _, item := range pv.Items {
		if item.Selected {
			pv.selected++
		}
		switch item.ChangeType {
		case "+":
			pv.created++
		case "~":
			pv.updated++
		case "-":
			pv.deleted++
		}
	}
}

// ToggleCurrent toggles the selected state of the item at cursor.
func (pv *PlanView) ToggleCurrent() {
	if pv.Cursor >= 0 && pv.Cursor < len(pv.Items) {
		pv.Items[pv.Cursor].Selected = !pv.Items[pv.Cursor].Selected
		pv.recalcCounts()
	}
}

// SelectAll selects or deselects all items.
func (pv *PlanView) SelectAll(selected bool) {
	for i := range pv.Items {
		pv.Items[i].Selected = selected
	}
	pv.recalcCounts()
}

// ToggleDetail toggles between summary and detail view.
func (pv *PlanView) ToggleDetail() {
	pv.ShowDetail = !pv.ShowDetail
}

// ToggleForce toggles force mode.
func (pv *PlanView) ToggleForce() {
	pv.Forced = !pv.Forced
}

// ToggleDryRun toggles dry-run mode.
func (pv *PlanView) ToggleDryRun() {
	pv.DryRun = !pv.DryRun
}

// CursorUp moves the cursor up.
func (pv *PlanView) CursorUp() {
	if pv.Cursor > 0 {
		pv.Cursor--
	}
}

// CursorDown moves the cursor down.
func (pv *PlanView) CursorDown() {
	if pv.Cursor < len(pv.Items)-1 {
		pv.Cursor++
	}
}

// GetCursor returns the current cursor position.
func (pv *PlanView) GetCursor() int {
	return pv.Cursor
}

// MaxCursor returns the maximum cursor value.
func (pv *PlanView) MaxCursor() int {
	if len(pv.Items) == 0 {
		return 0
	}
	return len(pv.Items) - 1
}

// SelectedCount returns the number of selected items.
func (pv *PlanView) SelectedCount() int {
	return pv.selected
}

// TotalCount returns the total number of items.
func (pv *PlanView) TotalCount() int {
	return len(pv.Items)
}

// HasSelected returns true if at least one item is selected.
func (pv *PlanView) HasSelected() bool {
	return pv.selected > 0
}

// GetSelectedItems returns the selected items.
func (pv *PlanView) GetSelectedItems() []PlanItem {
	var result []PlanItem
	for _, item := range pv.Items {
		if item.Selected {
			result = append(result, item)
		}
	}
	return result
}

// Render renders the plan view content for a given terminal height.
func (pv *PlanView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	var lines []string

	// Title section
	title := pv.Title
	if pv.Forced {
		title += " (forced)"
	}
	if pv.DryRun {
		title += " (dry-run)"
	}
	lines = append(lines, styles.BrandStyle.Render(title))
	lines = append(lines, fmt.Sprintf("ENV:  %s", pv.Env))
	if pv.Type != "" {
		lines = append(lines, fmt.Sprintf("TYPE: %s", pv.Type))
	}
	if pv.SubHeader != "" {
		lines = append(lines, pv.SubHeader)
	}

	lines = append(lines, "")

	// Environment warning
	if pv.EnvWarning {
		lines = append(lines, styles.WarningStyle.Render(fmt.Sprintf("  %s WARNING: Deploying to %s environment", styles.IconWarning, pv.Env)))
		lines = append(lines, "")
	}

	// No changes
	if pv.NoChanges || len(pv.Items) == 0 {
		if pv.Forced {
			lines = append(lines, styles.WarningStyle.Render(fmt.Sprintf("  %s WARNING: Force mode will deploy all services without configuration changes", styles.IconWarning)))
		} else {
			lines = append(lines, "No changes detected.")
			lines = append(lines, "")
			if pv.ForceHint {
				lines = append(lines, styles.MutedStyle.Render("[INFO] Press 'f' to enable force mode and deploy even without configuration changes."))
			}
		}
		return strings.Join(lines, "\n")
	}

	// Table header
	if pv.ShowDetail {
		lines = append(lines, pv.renderDetailHeader())
	} else {
		lines = append(lines, pv.renderSummaryHeader())
	}

	contentHeight := availableHeight - len(lines) - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	if pv.ShowDetail {
		// Detail view: items can span multiple lines, so we need line-based scrolling
		lines = pv.renderDetailItems(lines, contentHeight)
	} else {
		// Summary view: 1 line per item, use viewport
		viewport := NewComponentViewport(pv.Cursor, len(pv.Items), contentHeight)
		viewport.EnsureCursorVisible()

		start := viewport.VisibleStart()
		end := viewport.VisibleEnd()
		for i := start; i < end && i < len(pv.Items); i++ {
			lines = append(lines, pv.renderItem(i))
		}

		// Scroll indicator
		if viewport.TotalRows > viewport.VisibleRows {
			lines = append(lines, "")
			lines = append(lines, viewport.RenderScrollIndicator())
		}
	}

	// Summary line
	lines = append(lines, "")
	lines = append(lines, pv.renderSummaryLine())

	// Selection count
	lines = append(lines, styles.MutedStyle.Render(fmt.Sprintf("Selected: %d of %d changes", pv.selected, len(pv.Items))))

	return strings.Join(lines, "\n")
}

func (pv *PlanView) renderSummaryHeader() string {
	switch pv.OperationType {
	case "dns_deploy":
		return fmt.Sprintf("  %-2s %-8s %-16s %-12s %s", "", "ACTION", "DOMAIN", "RECORD", "DETAILS")
	case "dns_pull_domains", "dns_pull_records":
		return fmt.Sprintf("  %-2s %-8s %-16s %s", "", "ACTION", "DOMAIN", "DETAILS")
	case "server_setup":
		return fmt.Sprintf("  %-2s %-8s %-16s %s", "", "ACTION", "SERVER", "DETAILS")
	default:
		return fmt.Sprintf("  %-2s %-8s %-16s %-12s %s", "", "ACTION", "NAME", "SERVER", "DETAILS")
	}
}

func (pv *PlanView) renderDetailHeader() string {
	switch pv.OperationType {
	case "dns_deploy":
		return fmt.Sprintf("  %-2s %-8s %-16s %-12s %s", "", "ACTION", "DOMAIN", "RECORD", "DETAILS")
	case "dns_pull_domains", "dns_pull_records":
		return fmt.Sprintf("  %-2s %-8s %-16s %s", "", "ACTION", "DOMAIN", "DETAILS")
	case "server_setup":
		return fmt.Sprintf("  %-2s %-8s %-16s %s", "", "ACTION", "SERVER", "DETAILS")
	default:
		return fmt.Sprintf("  %-2s %-8s %-16s %-12s %s", "", "ACTION", "NAME", "SERVER", "DETAILS")
	}
}

func (pv *PlanView) renderItem(index int) string {
	if pv.ShowDetail {
		return pv.renderDetailItem(index)
	}
	return pv.renderSummaryItem(index)
}

func (pv *PlanView) renderSummaryItem(index int) string {
	item := pv.Items[index]

	checkbox := styles.IconUnchecked
	if item.Selected {
		checkbox = styles.IconChecked
	}

	cursor := " "
	if index == pv.Cursor {
		cursor = "▸"
	}

	actionStyle := styles.NoopStyle
	switch item.ChangeType {
	case "+":
		actionStyle = styles.CreateStyle
	case "~":
		actionStyle = styles.UpdateStyle
	case "-":
		actionStyle = styles.DeleteStyle
	}

	var line string
	switch pv.OperationType {
	case "dns_deploy":
		line = fmt.Sprintf("%s %s %-8s %-16s %-12s %s",
			cursor, checkbox,
			item.Action,
			truncate(item.Name, 16),
			truncate(item.Server, 12),
			item.Details,
		)
	case "dns_pull_domains", "dns_pull_records":
		line = fmt.Sprintf("%s %s %-8s %-16s %s",
			cursor, checkbox,
			item.Action,
			truncate(item.Name, 16),
			item.Details,
		)
	case "server_setup":
		line = fmt.Sprintf("%s %s %-8s %-16s %s",
			cursor, checkbox,
			item.Action,
			truncate(item.Server, 16),
			item.Details,
		)
	default:
		line = fmt.Sprintf("%s %s %-8s %-16s %-12s %s",
			cursor, checkbox,
			item.Action,
			truncate(item.Name, 16),
			truncate(item.Server, 12),
			item.Details,
		)
	}

	return actionStyle.Render(line)
}

func (pv *PlanView) renderDetailItem(index int) string {
	item := pv.Items[index]

	checkbox := styles.IconUnchecked
	if item.Selected {
		checkbox = styles.IconChecked
	}

	cursor := " "
	if index == pv.Cursor {
		cursor = "▸"
	}

	actionStyle := styles.NoopStyle
	switch item.ChangeType {
	case "+":
		actionStyle = styles.CreateStyle
	case "~":
		actionStyle = styles.UpdateStyle
	case "-":
		actionStyle = styles.DeleteStyle
	}

	var line string
	switch pv.OperationType {
	case "dns_deploy":
		line = fmt.Sprintf("%s %s %-8s %-16s %-12s %s",
			cursor, checkbox,
			item.Action,
			item.Name,
			item.Server,
			item.Details,
		)
	case "dns_pull_domains", "dns_pull_records":
		line = fmt.Sprintf("%s %s %-8s %-16s %s",
			cursor, checkbox,
			item.Action,
			item.Name,
			item.Details,
		)
	case "server_setup":
		line = fmt.Sprintf("%s %s %-8s %-16s %s",
			cursor, checkbox,
			item.Action,
			item.Server,
			item.Details,
		)
	default:
		line = fmt.Sprintf("%s %s %-8s %-16s %-12s %s",
			cursor, checkbox,
			item.Action,
			item.Name,
			item.Server,
			item.Details,
		)
	}

	result := actionStyle.Render(line)

	if len(item.DetailLines) > 0 {
		for _, dl := range item.DetailLines {
			result += "\n" + styles.MutedStyle.Render("    "+dl)
		}
	}

	return result
}

func (pv *PlanView) renderDetailItems(lines []string, maxHeight int) []string {
	if len(pv.Items) == 0 {
		return lines
	}

	pv.detailEnsureCursorVisible(maxHeight)

	scrollUp := pv.detailOffset > 0
	scrollDown := false

	renderedLines := 0
	for i := pv.detailOffset; i < len(pv.Items); i++ {
		itemLineCount := 1 + len(pv.Items[i].DetailLines)
		if renderedLines+itemLineCount > maxHeight {
			scrollDown = true
			break
		}
		lines = append(lines, pv.renderDetailItem(i))
		renderedLines += itemLineCount
	}

	for renderedLines < maxHeight {
		lines = append(lines, "")
		renderedLines++
	}

	if scrollUp || scrollDown {
		var parts []string
		if scrollUp {
			parts = append(parts, "↑")
		}
		parts = append(parts, fmt.Sprintf("%d/%d", pv.Cursor+1, len(pv.Items)))
		if scrollDown {
			parts = append(parts, "↓")
		}
		lines = append(lines, styles.ScrollIndicatorStyle.Render(strings.Join(parts, " ")))
	}

	return lines
}

func (pv *PlanView) detailEnsureCursorVisible(maxHeight int) {
	if pv.Cursor < pv.detailOffset {
		pv.detailOffset = pv.Cursor
		return
	}

	for {
		linesUsed := 0
		for i := pv.detailOffset; i <= pv.Cursor; i++ {
			linesUsed += 1 + len(pv.Items[i].DetailLines)
		}
		if linesUsed <= maxHeight {
			break
		}
		pv.detailOffset++
		if pv.detailOffset > pv.Cursor {
			pv.detailOffset = pv.Cursor
			break
		}
	}
}

func (pv *PlanView) renderSummaryLine() string {
	parts := []string{}

	// For non-deploy operations (stop, restart, cleanup), use operation-specific verbs
	switch pv.Type {
	case "stop":
		if pv.updated > 0 {
			parts = append(parts, fmt.Sprintf("%d stopped", pv.updated))
		}
	case "restart":
		if pv.updated > 0 {
			parts = append(parts, fmt.Sprintf("%d restarted", pv.updated))
		}
	case "cleanup":
		if pv.deleted > 0 {
			parts = append(parts, fmt.Sprintf("%d cleaned", pv.deleted))
		}
	default:
		// For deploy and other operations, use change type verbs
		if pv.created > 0 {
			parts = append(parts, fmt.Sprintf("%d created", pv.created))
		}
		if pv.updated > 0 {
			parts = append(parts, fmt.Sprintf("%d updated", pv.updated))
		}
		if pv.deleted > 0 {
			parts = append(parts, fmt.Sprintf("%d deleted", pv.deleted))
		}
	}

	if len(parts) == 0 {
		return "SUMMARY: no changes"
	}
	summary := "SUMMARY: " + strings.Join(parts, ", ")
	if pv.Forced {
		summary += " (forced)"
	}
	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
