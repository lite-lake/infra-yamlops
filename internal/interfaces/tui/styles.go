package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

// Re-export color constants from styles package for backward compatibility.
const (
	ColorPrimary    = styles.ColorBrand
	ColorSuccess    = styles.ColorSuccess
	ColorWarning    = styles.ColorWarning
	ColorError      = styles.ColorError
	ColorSecondary  = styles.ColorMuted
	ColorBgSelected = styles.ColorBgDark
)

// Re-export spinner frames from styles package.
var SpinnerFrames = styles.SpinnerFrames

// Re-export lipgloss styles from styles package for backward compatibility.
var (
	BaseStyle = styles.BasePaddingStyle

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(styles.ColorBrand)).
			Padding(0, 1)

	EnvStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(styles.ColorSuccess)).
			Bold(true)

	SelectedStyle = styles.SelectedStyle

	HelpStyle = styles.MutedStyle

	ChangeCreateStyle = styles.CreateStyle
	ChangeUpdateStyle = styles.UpdateStyle
	ChangeDeleteStyle = styles.DeleteStyle
	ChangeNoopStyle   = styles.NoopStyle

	WarningStyle     = styles.WarningStyle
	ProgressBarStyle = styles.ProgressBarStyle
	SuccessStyle     = styles.SuccessStyle

	TabActiveStyle   = styles.TabActiveStyle
	TabInactiveStyle = styles.TabInactiveStyle

	MenuStyle         = styles.MenuStyle
	MenuItemStyle     = styles.MenuItemStyle
	MenuSelectedStyle = styles.MenuSelectedStyle

	ScrollIndicatorStyle = styles.ScrollIndicatorStyle
	LoadingOverlayStyle  = styles.LoadingOverlayStyle
)

// FormatChangeType returns the prefix and style for a given change type.
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
