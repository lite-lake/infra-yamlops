package styles

// Terminal size requirements
const (
	MinTerminalWidth  = 80
	MinTerminalHeight = 24
	RecTerminalWidth  = 120
	RecTerminalHeight = 30
)

// Layout region heights (lines)
const (
	TitleBarHeight     = 1 // Brand + env + navigation
	TitleSepHeight     = 1 // Separator line after title
	HelpBarHeight      = 1 // Keyboard shortcut hints
	HelpSepHeight      = 1 // Separator line before help bar
	StatusBarHeight    = 1 // Status + statistics
	StatusBarSepHeight = 1 // Separator line before status bar

	// Fixed overhead: title + 2 separators + help + status
	LayoutOverhead = TitleBarHeight + TitleSepHeight + HelpBarHeight + HelpSepHeight + StatusBarHeight + StatusBarSepHeight

	// Minimum content area height
	MinContentHeight = 10
)

// Indentation
const (
	IndentTree    = 2 // Tree node indent per level
	IndentDetail  = 2 // Detail field indent
	IndentError   = 2 // Error detail indent
	IndentSuggest = 6 // Suggestion indent
)

// Column widths for table output
const (
	ColActionWidth  = 8
	ColNameWidth    = 16
	ColServerWidth  = 16
	ColDetailsWidth = 60 // Max width

	// Info list & detail table columns
	ColZoneWidth        = 12
	ColServiceWidth     = 20
	ColImageWidth       = 30
	ColISPWidth         = 12
	ColDomainWidth      = 20
	ColRecordsWidth     = 10
	ColRegistryWidth    = 12
	ColURLWidth         = 28
	ColNSWidth          = 12
	ColTypeWidth        = 12
	ColServicesWidth    = 16
	ColKeyWidth         = 20
	ColSourceWidth      = 16
	ColDescriptionWidth = 30
)

// Separator characters
const (
	SepVertical   = " │ " // Title bar internal separator
	SepHorizontal = "───" // Horizontal rule
	SepCornerBL   = "├"   // Bottom-left corner connector
	SepCornerBR   = "└"   // Bottom-right corner connector
)

// Title bar segments
const (
	TitleBrand     = "YAMLOps"
	TitleEnvPrefix = "Environment: "
	TitleNavSep    = " > "
)

// Help bar key format
const (
	HelpKeyPrefix = "["
	HelpKeySuffix = "]"
	HelpKeySep    = "  "
)
