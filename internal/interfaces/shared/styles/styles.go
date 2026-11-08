package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

const (
	ColorPrimary    = "#7C3AED"
	ColorSuccess    = "#10B981"
	ColorWarning    = "#F59E0B"
	ColorError      = "#EF4444"
	ColorSecondary  = "#6B7280"
	ColorBgSelected = "#1E1B4B"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondary))

	ChangeCreateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSuccess))

	ChangeUpdateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorWarning))

	ChangeDeleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorError))

	ChangeNoopStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondary))
)

func FormatChangeType(changeType valueobject.ChangeType) (prefix string, style lipgloss.Style) {
	switch changeType {
	case valueobject.ChangeTypeCreate:
		return "+", ChangeCreateStyle
	case valueobject.ChangeTypeUpdate:
		return "~", ChangeUpdateStyle
	case valueobject.ChangeTypeDelete:
		return "-", ChangeDeleteStyle
	default:
		return "~", ChangeNoopStyle
	}
}
