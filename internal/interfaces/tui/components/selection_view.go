package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// SelectionItem represents a selectable item in a group.
type SelectionItem struct {
	Label    string
	Selected bool
	Meta     string // additional info, e.g. "(dns)" for ISP, "(ISP: aliyun)" for domains
}

// SelectionGroup represents a group of selectable items.
type SelectionGroup struct {
	Title string
	Items []SelectionItem
}

// SelectionView renders a multi-group multi-select filter interface.
type SelectionView struct {
	Title         string
	Groups        []SelectionGroup
	SelectedGroup int    // index of currently active group
	Cursors       []int  // cursor position per group
	MatchedLine   string // e.g. "Matched services: 3 services across 2 servers"
}

// NewSelectionView creates a new SelectionView.
func NewSelectionView(title string) *SelectionView {
	return &SelectionView{
		Title:         title,
		SelectedGroup: 0,
	}
}

// SetGroups sets the selection groups.
func (sv *SelectionView) SetGroups(groups []SelectionGroup) {
	sv.Groups = groups
	sv.Cursors = make([]int, len(groups))
	sv.SelectedGroup = 0
}

// AddGroup adds a selection group.
func (sv *SelectionView) AddGroup(title string, items []SelectionItem) {
	sv.Groups = append(sv.Groups, SelectionGroup{Title: title, Items: items})
	sv.Cursors = append(sv.Cursors, 0)
}

// ToggleCurrent toggles the selected state of the current item.
func (sv *SelectionView) ToggleCurrent() {
	gi := sv.SelectedGroup
	ci := sv.Cursors[gi]
	if gi >= 0 && gi < len(sv.Groups) {
		group := &sv.Groups[gi]
		if ci >= 0 && ci < len(group.Items) {
			group.Items[ci].Selected = !group.Items[ci].Selected
		}
	}
}

// SelectAll selects or deselects all items in all groups.
func (sv *SelectionView) SelectAll(selected bool) {
	for gi := range sv.Groups {
		for ii := range sv.Groups[gi].Items {
			sv.Groups[gi].Items[ii].Selected = selected
		}
	}
}

// SelectGroupAll selects or deselects all items in the current group.
func (sv *SelectionView) SelectGroupAll(selected bool) {
	gi := sv.SelectedGroup
	if gi >= 0 && gi < len(sv.Groups) {
		for ii := range sv.Groups[gi].Items {
			sv.Groups[gi].Items[ii].Selected = selected
		}
	}
}

// NextGroup switches to the next group.
func (sv *SelectionView) NextGroup() {
	if sv.SelectedGroup < len(sv.Groups)-1 {
		sv.SelectedGroup++
	}
}

// PrevGroup switches to the previous group.
func (sv *SelectionView) PrevGroup() {
	if sv.SelectedGroup > 0 {
		sv.SelectedGroup--
	}
}

// CursorUp moves the cursor up in the current group.
func (sv *SelectionView) CursorUp() {
	gi := sv.SelectedGroup
	if sv.Cursors[gi] > 0 {
		sv.Cursors[gi]--
	}
}

// CursorDown moves the cursor down in the current group.
func (sv *SelectionView) CursorDown() {
	gi := sv.SelectedGroup
	if sv.Cursors[gi] < len(sv.Groups[gi].Items)-1 {
		sv.Cursors[gi]++
	}
}

// GetCursor returns the cursor for the current group.
func (sv *SelectionView) GetCursor() int {
	return sv.Cursors[sv.SelectedGroup]
}

// MaxCursor returns the max cursor for the current group.
func (sv *SelectionView) MaxCursor() int {
	gi := sv.SelectedGroup
	if len(sv.Groups[gi].Items) == 0 {
		return 0
	}
	return len(sv.Groups[gi].Items) - 1
}

// SelectedCount returns the total number of selected items across all groups.
func (sv *SelectionView) SelectedCount() int {
	count := 0
	for _, group := range sv.Groups {
		for _, item := range group.Items {
			if item.Selected {
				count++
			}
		}
	}
	return count
}

// HasSelected returns true if at least one item is selected.
func (sv *SelectionView) HasSelected() bool {
	return sv.SelectedCount() > 0
}

// GetSelectedLabels returns labels of selected items in a specific group.
func (sv *SelectionView) GetSelectedLabels(groupIndex int) []string {
	var labels []string
	if groupIndex >= 0 && groupIndex < len(sv.Groups) {
		for _, item := range sv.Groups[groupIndex].Items {
			if item.Selected {
				labels = append(labels, item.Label)
			}
		}
	}
	return labels
}

// activeLineIndex returns the line index in the scrollable content section
// corresponding to the current cursor position.
func (sv *SelectionView) activeLineIndex() int {
	lineIdx := 0
	for gi, group := range sv.Groups {
		lineIdx++ // group header
		if gi == sv.SelectedGroup {
			lineIdx += sv.Cursors[gi]
			return lineIdx
		}
		lineIdx += len(group.Items)
		lineIdx++ // blank line
	}
	return max(0, lineIdx-1)
}

// Render renders the selection view with viewport-based scrolling.
func (sv *SelectionView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	// Fixed header: title + blank
	var headerLines []string
	headerLines = append(headerLines, sv.Title)
	headerLines = append(headerLines, "")
	headerCount := len(headerLines)

	// Fixed footer: matched line
	var footerLines []string
	if sv.MatchedLine != "" {
		footerLines = append(footerLines, sv.MatchedLine)
	}
	footerCount := len(footerLines)

	// Build scrollable content lines: group headers + items + blank separators
	var contentLines []string
	for gi, group := range sv.Groups {
		groupIndicator := " "
		if gi == sv.SelectedGroup {
			groupIndicator = "▸"
		}
		contentLines = append(contentLines, fmt.Sprintf("  %s %s:", groupIndicator, group.Title))

		for ii, item := range group.Items {
			cursor := " "
			if gi == sv.SelectedGroup && ii == sv.Cursors[gi] {
				cursor = "▸"
			}

			checkbox := styles.IconUnchecked
			if item.Selected {
				checkbox = styles.IconChecked
			}

			meta := ""
			if item.Meta != "" {
				meta = " " + styles.MutedStyle.Render(item.Meta)
			}

			line := fmt.Sprintf("    %s %s %s%s", cursor, checkbox, item.Label, meta)
			if gi == sv.SelectedGroup && ii == sv.Cursors[gi] {
				contentLines = append(contentLines, styles.SelectedStyle.Render(line))
			} else {
				contentLines = append(contentLines, line)
			}
		}
		contentLines = append(contentLines, "")
	}

	// Calculate content height
	contentHeight := availableHeight - headerCount - footerCount
	if contentHeight < 1 {
		contentHeight = 1
	}

	totalContentLines := len(contentLines)

	// Auto-scroll to the active cursor position
	activeIdx := sv.activeLineIndex()
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
