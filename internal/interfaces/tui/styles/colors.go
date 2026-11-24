package styles

import "github.com/charmbracelet/lipgloss"

// Brand colors
const (
	ColorBrand = "#06B6D4" // Cyan - YAMLOps brand identity
)

// Environment colors
const (
	ColorEnvProd    = "#10B981" // Green
	ColorEnvStaging = "#F59E0B" // Yellow
	ColorEnvDev     = "#3B82F6" // Blue
)

// Status colors
const (
	ColorSuccess  = "#10B981" // Green - success, selected, created
	ColorError    = "#EF4444" // Red - failure, deleted
	ColorWarning  = "#F59E0B" // Yellow - warning, update, in-progress
	ColorInfo     = "#3B82F6" // Blue - informational
	ColorMuted    = "#6B7280" // Gray - waiting, inactive, secondary
	ColorBgDark   = "#1E1B4B" // Dark background for selected items
	ColorSelected = "#10B981" // Green - selected checkbox
)

// Change type colors
const (
	ColorCreate = ColorSuccess // Green +
	ColorUpdate = ColorWarning // Yellow ~
	ColorDelete = ColorError   // Red -
	ColorNoop   = ColorMuted   // Gray ~
)

// Tree node icon colors
const (
	ColorNodeExpanded  = "#FFFFFF" // White ▼
	ColorNodeCollapsed = "#FFFFFF" // White ▶
	ColorNodeChecked   = ColorSuccess
	ColorNodeUnchecked = ColorMuted
	ColorNodePartial   = ColorWarning
)

// Semantic styles (pre-built lipgloss styles)
var (
	BrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand)).
			Padding(0, 1)

	TitleBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand))

	EnvProdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorEnvProd)).
			Bold(true)

	EnvStagingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorEnvStaging)).
			Bold(true)

	EnvDevStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorEnvDev)).
			Bold(true)

	NavigationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning))

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorInfo))

	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted))

	CreateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCreate))

	UpdateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorUpdate))

	DeleteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDelete))

	NoopStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorNoop))

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorBrand))

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBrand)).
			Bold(true)

	MenuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBrand)).
			Bold(true)

	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	MenuSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorBrand)).
				Background(lipgloss.Color(ColorBgDark)).
				Padding(0, 2).
				Bold(true)

	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorBrand)).
			Underline(true)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMuted))

	LoadingOverlayStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorBrand)).
				Bold(true).
				Padding(1, 2)

	ScrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMuted)).
				Padding(0, 1)

	BasePaddingStyle = lipgloss.NewStyle().Padding(1, 2)
)

// EnvStyle returns the appropriate style for the given environment name.
func EnvStyle(env string) lipgloss.Style {
	switch env {
	case "prod":
		return EnvProdStyle
	case "staging":
		return EnvStagingStyle
	default:
		return EnvDevStyle
	}
}

// ChangeStyle returns the prefix and style for a given change type indicator.
func ChangeStyle(prefix string) (string, lipgloss.Style) {
	switch prefix {
	case "+":
		return prefix, CreateStyle
	case "~":
		return prefix, UpdateStyle
	case "-":
		return prefix, DeleteStyle
	default:
		return prefix, NoopStyle
	}
}
