package tui

import (
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// menuRow represents a single row in the flattened main menu.
type menuRow struct {
	isParent bool
	parent   int // index into MenuNodes (only valid if isParent)
	child    int // index into Children (only valid if !isParent)
}

// flattenMenuRows returns the visible rows of the main menu tree.
func (m Model) flattenMenuRows() []menuRow {
	var rows []menuRow
	for pi, node := range m.UI.MenuNodes {
		rows = append(rows, menuRow{isParent: true, parent: pi})
		if node.Expanded {
			for ci := range node.Children {
				rows = append(rows, menuRow{isParent: false, parent: pi, child: ci})
			}
		}
	}
	return rows
}

// menuRowCount returns the total number of visible menu rows.
func (m Model) menuRowCount() int {
	return len(m.flattenMenuRows())
}

func (m Model) renderMainMenuContent() string {
	rows := m.flattenMenuRows()
	cursor := m.UI.MainMenuIndex
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}
	if cursor < 0 {
		cursor = 0
	}

	availableHeight := m.UI.Height - styles.LayoutOverhead
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	viewport := NewViewport(cursor, len(rows), availableHeight)
	viewport.EnsureCursorVisible()

	var sb strings.Builder

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(rows); i++ {
		row := rows[i]
		if row.isParent {
			node := m.UI.MenuNodes[row.parent]
			prefix := "▶ "
			if node.Expanded {
				prefix = "▼ "
			}
			if i == cursor {
				sb.WriteString(styles.MenuSelectedStyle.Render(prefix+node.Label) + "\n")
			} else {
				sb.WriteString(styles.MenuItemStyle.Render("  "+prefix+node.Label) + "\n")
			}
		} else {
			child := m.UI.MenuNodes[row.parent].Children[row.child]
			if i == cursor {
				sb.WriteString(styles.MenuSelectedStyle.Render("    ▶ "+child.Label) + "\n")
			} else {
				sb.WriteString(styles.MenuItemStyle.Render("      "+child.Label) + "\n")
			}
		}
	}

	if viewport.TotalRows > viewport.VisibleRows {
		sb.WriteString("\n" + viewport.RenderSimpleScrollIndicator())
	}

	return sb.String()
}

func formatNodeStatus(status NodeStatus) string {
	switch status {
	case StatusRunning:
		return SuccessStyle.Render("[running]")
	case StatusStopped:
		return WarningStyle.Render("[stopped]")
	case StatusNeedsUpdate:
		return ChangeUpdateStyle.Render("[needs update]")
	case StatusError:
		return ChangeDeleteStyle.Render("[error]")
	default:
		return ""
	}
}
