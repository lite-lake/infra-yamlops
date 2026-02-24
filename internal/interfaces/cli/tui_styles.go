package cli

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

const (
	ColorPrimary    = "#7C3AED"
	ColorSuccess    = "#10B981"
	ColorWarning    = "#F59E0B"
	ColorError      = "#EF4444"
	ColorSecondary  = "#6B7280"
	ColorBgSelected = "#1E1B4B"
)

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	BaseStyle = lipgloss.NewStyle().Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Padding(0, 1)

	EnvStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess)).
			Bold(true)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary)).
			Bold(true)

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

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning))

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPrimary))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess))

	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Underline(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary))

	MenuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary)).
			Bold(true)

	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	MenuSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPrimary)).
				Background(lipgloss.Color(ColorBgSelected)).
				Padding(0, 2).
				Bold(true)

	ScrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary)).
				Padding(0, 1)

	LoadingOverlayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPrimary)).
				Bold(true).
				Padding(1, 2)
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
