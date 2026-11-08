package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// InfoColumn defines a column in the info list view.
type InfoColumn struct {
	Header string
	Width  int
	Align  string // "left" or "right"
}

// InfoRow represents a single row of data.
type InfoRow struct {
	Cells []string
}

// InfoListView renders a table-style information list.
type InfoListView struct {
	Title    string
	Columns  []InfoColumn
	Rows     []InfoRow
	StatLine string // e.g. "Total: 4 services across 3 servers in 2 zones"
	Cursor   int
}

// NewInfoListView creates a new InfoListView.
func NewInfoListView(title string) *InfoListView {
	return &InfoListView{
		Title:  title,
		Cursor: 0,
	}
}

// SetColumns sets the column definitions.
func (iv *InfoListView) SetColumns(columns []InfoColumn) {
	iv.Columns = columns
}

// SetRows sets the data rows.
func (iv *InfoListView) SetRows(rows []InfoRow) {
	iv.Rows = rows
	iv.Cursor = 0
}

// AddRow adds a single row.
func (iv *InfoListView) AddRow(cells ...string) {
	iv.Rows = append(iv.Rows, InfoRow{Cells: cells})
}

// SetStatLine sets the statistics line at the bottom.
func (iv *InfoListView) SetStatLine(stat string) {
	iv.StatLine = stat
}

// RowCount returns the number of rows.
func (iv *InfoListView) RowCount() int {
	return len(iv.Rows)
}

// Render renders the info list view.
func (iv *InfoListView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	var lines []string

	// Table header
	if len(iv.Columns) > 0 {
		header := iv.renderHeader()
		lines = append(lines, header)
	}

	// Viewport for rows
	contentHeight := availableHeight - 2 // header + stat line
	if contentHeight < 3 {
		contentHeight = 3
	}

	viewport := NewComponentViewport(iv.Cursor, len(iv.Rows), contentHeight)
	viewport.EnsureCursorVisible()

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(iv.Rows); i++ {
		lines = append(lines, iv.renderRow(i))
	}

	// Scroll indicator
	if viewport.TotalRows > viewport.VisibleRows {
		lines = append(lines, "")
		lines = append(lines, viewport.RenderScrollIndicator())
	}

	// Stat line
	if iv.StatLine != "" {
		lines = append(lines, "")
		lines = append(lines, iv.StatLine)
	}

	return strings.Join(lines, "\n")
}

func (iv *InfoListView) renderHeader() string {
	var parts []string
	for _, col := range iv.Columns {
		parts = append(parts, fmt.Sprintf("%-*s", col.Width, col.Header))
	}
	return "  " + strings.Join(parts, " ")
}

func (iv *InfoListView) renderRow(index int) string {
	row := iv.Rows[index]
	var parts []string
	for i, cell := range row.Cells {
		width := 16
		if i < len(iv.Columns) {
			width = iv.Columns[i].Width
		}
		parts = append(parts, fmt.Sprintf("%-*s", width, truncate(cell, width)))
	}
	line := "  " + strings.Join(parts, " ")

	// 高亮当前 cursor 行
	if index == iv.Cursor {
		return styles.SelectedStyle.Render(line)
	}

	return line
}
