package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Viewport struct {
	Offset      int
	VisibleRows int
	TotalRows   int
	CursorIndex int
}

func NewViewport(cursor, total, height int) *Viewport {
	v := &Viewport{
		Offset:      0,
		VisibleRows: max(1, height),
		TotalRows:   total,
		CursorIndex: cursor,
	}
	v.EnsureCursorVisible()
	return v
}

func (v *Viewport) EnsureCursorVisible() {
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

func (v *Viewport) IsScrolledUp() bool {
	return v.Offset > 0
}

func (v *Viewport) IsScrolledDown() bool {
	return v.Offset+v.VisibleRows < v.TotalRows
}

func (v *Viewport) VisibleStart() int {
	return v.Offset
}

func (v *Viewport) VisibleEnd() int {
	end := v.Offset + v.VisibleRows
	if end > v.TotalRows {
		end = v.TotalRows
	}
	return end
}

func (v *Viewport) RenderScrollIndicator() string {
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
	return scrollIndicatorStyle.Render(strings.Join(parts, " "))
}

func (v *Viewport) RenderSimpleScrollIndicator() string {
	if v.TotalRows <= v.VisibleRows {
		return ""
	}
	return scrollIndicatorStyle.Render(fmt.Sprintf("[%d/%d]", v.CursorIndex+1, v.TotalRows))
}

var scrollIndicatorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6B7280")).
	Padding(0, 1)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
