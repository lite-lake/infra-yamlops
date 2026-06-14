package components

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// InfoField represents a single field in the detail view.
type InfoField struct {
	Label string
	Value string
	Level int // 0 = top level, 1 = nested, etc.
}

// InfoEntity represents an entity with its fields for detail display.
type InfoEntity struct {
	Title  string // e.g. "SERVICE: api-server"
	Fields []InfoField
	Lines  []string // additional multi-line content (e.g. sub-tables)
}

// InfoDetailView renders detailed information for entities.
type InfoDetailView struct {
	Title    string
	Columns  []InfoColumn // for the summary table at top
	Rows     []InfoRow    // for the summary table at top
	Entities []InfoEntity
	StatLine string
	Cursor   int
}

// NewInfoDetailView creates a new InfoDetailView.
func NewInfoDetailView(title string) *InfoDetailView {
	return &InfoDetailView{
		Title:  title,
		Cursor: 0,
	}
}

// SetSummaryTable sets the summary table at the top.
func (dv *InfoDetailView) SetSummaryTable(columns []InfoColumn, rows []InfoRow) {
	dv.Columns = columns
	dv.Rows = rows
}

// SetEntities sets the entity details.
func (dv *InfoDetailView) SetEntities(entities []InfoEntity) {
	dv.Entities = entities
}

// AddEntity adds a single entity.
func (dv *InfoDetailView) AddEntity(title string, fields []InfoField) {
	dv.Entities = append(dv.Entities, InfoEntity{Title: title, Fields: fields})
}

// SetStatLine sets the statistics line.
func (dv *InfoDetailView) SetStatLine(stat string) {
	dv.StatLine = stat
}

// Render renders the info detail view.
func (dv *InfoDetailView) Render(availableHeight int) string {
	if availableHeight < styles.MinContentHeight {
		availableHeight = styles.MinContentHeight
	}

	// Build all lines
	var allLines []string

	// Summary table header
	if len(dv.Columns) > 0 {
		allLines = append(allLines, dv.renderSummaryHeader())
	}

	// Summary table rows
	for _, row := range dv.Rows {
		allLines = append(allLines, dv.renderSummaryRow(row))
	}

	// Entity details
	for _, entity := range dv.Entities {
		allLines = append(allLines, "")
		allLines = append(allLines, styles.BrandStyle.Render(entity.Title))

		for _, field := range entity.Fields {
			indent := strings.Repeat("  ", field.Level+1)
			allLines = append(allLines, fmt.Sprintf("%s%-16s %s", indent, field.Label+":", field.Value))
		}

		for _, line := range entity.Lines {
			allLines = append(allLines, "  "+line)
		}
	}

	// Stat line
	if dv.StatLine != "" {
		allLines = append(allLines, "")
		allLines = append(allLines, dv.StatLine)
	}

	// Viewport
	viewport := NewComponentViewport(dv.Cursor, len(allLines), availableHeight)
	viewport.EnsureCursorVisible()

	var lines []string
	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(allLines); i++ {
		lines = append(lines, allLines[i])
	}

	// Scroll indicator
	if viewport.TotalRows > viewport.VisibleRows {
		lines = append(lines, "")
		lines = append(lines, viewport.RenderScrollIndicator())
	}

	return strings.Join(lines, "\n")
}

func (dv *InfoDetailView) renderSummaryHeader() string {
	var parts []string
	for _, col := range dv.Columns {
		parts = append(parts, fmt.Sprintf("%-*s", col.Width, col.Header))
	}
	return "  " + strings.Join(parts, " ")
}

func (dv *InfoDetailView) renderSummaryRow(row InfoRow) string {
	var parts []string
	for i, cell := range row.Cells {
		width := 16
		if i < len(dv.Columns) {
			width = dv.Columns[i].Width
		}
		parts = append(parts, fmt.Sprintf("%-*s", width, truncate(cell, width)))
	}
	return "  " + strings.Join(parts, " ")
}
