package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// ComponentViewport is a viewport helper for components that need scrolling.
type ComponentViewport struct {
	Offset      int
	VisibleRows int
	TotalRows   int
	CursorIndex int
}

// NewComponentViewport creates a new viewport for a component.
func NewComponentViewport(cursor, total, height int) *ComponentViewport {
	v := &ComponentViewport{
		Offset:      0,
		VisibleRows: max(1, height),
		TotalRows:   total,
		CursorIndex: cursor,
	}
	v.EnsureCursorVisible()
	return v
}

// EnsureCursorVisible adjusts the offset so the cursor is visible.
func (v *ComponentViewport) EnsureCursorVisible() {
	if v.CursorIndex < v.Offset {
		v.Offset = v.CursorIndex
	}
	if v.CursorIndex >= v.Offset+v.VisibleRows {
		v.Offset = v.CursorIndex - v.VisibleRows + 1
	}
	if v.Offset < 0 {
		v.Offset = 0
	}
	maxOffset := max(0, v.TotalRows-v.VisibleRows)
	if v.Offset > maxOffset {
		v.Offset = maxOffset
	}
}

// VisibleStart returns the start index of visible rows.
func (v *ComponentViewport) VisibleStart() int {
	return v.Offset
}

// VisibleEnd returns the end index of visible rows.
func (v *ComponentViewport) VisibleEnd() int {
	end := v.Offset + v.VisibleRows
	if end > v.TotalRows {
		end = v.TotalRows
	}
	return end
}

// IsScrolledUp returns true if scrolled up from the top.
func (v *ComponentViewport) IsScrolledUp() bool {
	return v.Offset > 0
}

// IsScrolledDown returns true if there are more rows below.
func (v *ComponentViewport) IsScrolledDown() bool {
	return v.Offset+v.VisibleRows < v.TotalRows
}

// RenderScrollIndicator renders a scroll position indicator.
func (v *ComponentViewport) RenderScrollIndicator() string {
	if v.TotalRows <= v.VisibleRows {
		return ""
	}

	var parts []string
	if v.IsScrolledUp() {
		parts = append(parts, "↑")
	}
	parts = append(parts, fmt.Sprintf("%d/%d", v.CursorIndex+1, v.TotalRows))
	if v.IsScrolledDown() {
		parts = append(parts, "↓")
	}
	return styles.ScrollIndicatorStyle.Render(strings.Join(parts, " "))
}
