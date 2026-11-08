package components

import (
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// SearchFilter provides a search/filter component for tree views and list views.
type SearchFilter struct {
	Active     bool
	Query      string
	FilterFunc func(query string, items []string) []string
}

// NewSearchFilter creates a new SearchFilter.
func NewSearchFilter() *SearchFilter {
	return &SearchFilter{
		Active: false,
		Query:  "",
	}
}

// Activate enters search mode.
func (sf *SearchFilter) Activate() {
	sf.Active = true
	sf.Query = ""
}

// Deactivate exits search mode.
func (sf *SearchFilter) Deactivate() {
	sf.Active = false
	sf.Query = ""
}

// IsActive returns true if search mode is active.
func (sf *SearchFilter) IsActive() bool {
	return sf.Active
}

// AppendChar appends a character to the search query.
func (sf *SearchFilter) AppendChar(ch string) {
	if sf.Active {
		sf.Query += ch
	}
}

// Backspace removes the last character from the search query.
func (sf *SearchFilter) Backspace() {
	if sf.Active && len(sf.Query) > 0 {
		sf.Query = sf.Query[:len(sf.Query)-1]
	}
}

// Clear clears the search query.
func (sf *SearchFilter) Clear() {
	sf.Query = ""
}

// GetQuery returns the current search query.
func (sf *SearchFilter) GetQuery() string {
	return sf.Query
}

// Render renders the search bar.
func (sf *SearchFilter) Render() string {
	if !sf.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.InfoStyle.Render("/ "))
	b.WriteString(sf.Query)
	b.WriteString(styles.MutedStyle.Render("█"))
	return b.String()
}

// FilterItems filters items based on the search query (case-insensitive contains).
func FilterByQuery(query string, items []string) []string {
	if query == "" {
		return items
	}
	query = strings.ToLower(query)
	var result []string
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), query) {
			result = append(result, item)
		}
	}
	return result
}

// MatchesQuery checks if a string matches the search query.
func MatchesQuery(query, s string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(query))
}
