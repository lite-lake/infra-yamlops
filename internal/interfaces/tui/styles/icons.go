package styles

// Selection icons
const (
	IconChecked     = "✓"   // Selected / Success
	IconUnchecked   = "☐"   // Not selected
	IconPartial     = "[-]" // Partially selected (parent node)
	IconAllSelected = "[+]" // All children selected (parent node)
)

// Tree navigation icons
const (
	IconExpanded  = "▼" // Expanded node
	IconCollapsed = "▶" // Collapsed node
)

// Action icons (for Plan view)
const (
	IconCreate = "+" // New resource
	IconUpdate = "~" // Updated resource
	IconDelete = "-" // Deleted resource
)

// Execution status icons
const (
	IconSuccess = "✓"      // Execution succeeded
	IconFailed  = "✗"      // Execution failed
	IconRunning = "⟳"      // Currently executing
	IconWaiting = "-"      // Waiting to execute
	IconWarning = "⚠"      // Warning
	IconSkip    = "[SKIP]" // Interrupted / skipped
)

// Spinner frames for loading state
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Progress bar characters
const (
	ProgressFilled = "█"
	ProgressEmpty  = "░"
	ProgressWidth  = 20
)

// Status text labels
const (
	StatusOK     = "[OK]"
	StatusFAIL   = "[FAIL]"
	StatusWARN   = "[WARN]"
	StatusDRYRUN = "[DRY RUN]"
	StatusINFO   = "[INFO]"
)
