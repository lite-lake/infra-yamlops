package tui

const (
	KeyQuit        = "q"
	KeyCancel      = "x"
	KeyEscape      = "esc"
	KeyUp          = "up"
	KeyUpAlt       = "k"
	KeyDown        = "down"
	KeyDownAlt     = "j"
	KeySpace       = " "
	KeyEnter       = "enter"
	KeyTab         = "tab"
	KeySelectAll   = "a"
	KeyDeselectAll = "n"
	KeyPlan        = "p"
	KeyRefresh     = "r"
	KeyDetail      = "d"
	KeySearch      = "/"
)

type HelpItem struct {
	Key  string
	Desc string
}

func BuildHelpText(items []HelpItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = item.Key + " " + item.Desc
	}
	return HelpStyle.Render("  " + joinParts(parts, "  "))
}

func joinParts(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

var (
	HelpNavUp       = HelpItem{Key: "↑/↓", Desc: "navigate"}
	HelpEnter       = HelpItem{Key: "Enter", Desc: "select"}
	HelpEsc         = HelpItem{Key: "Esc", Desc: "back"}
	HelpQuit        = HelpItem{Key: "q", Desc: "quit"}
	HelpSpace       = HelpItem{Key: "Space", Desc: "toggle"}
	HelpTab         = HelpItem{Key: "Tab", Desc: "switch"}
	HelpSelectAll   = HelpItem{Key: "a", Desc: "all"}
	HelpDeselectAll = HelpItem{Key: "n", Desc: "none"}
	HelpPlanItem    = HelpItem{Key: "p", Desc: "plan"}
	HelpRefresh     = HelpItem{Key: "r", Desc: "refresh"}
	HelpCurrent     = HelpItem{Key: "a", Desc: "current"}
	HelpCancel      = HelpItem{Key: "n", Desc: "cancel"}
	HelpSearch      = HelpItem{Key: "/", Desc: "search"}
)

func HelpMenu() string {
	return BuildHelpText([]HelpItem{HelpNavUp, HelpEnter, HelpQuit})
}

func HelpMenuWithEsc() string {
	return BuildHelpText([]HelpItem{HelpNavUp, HelpEnter, HelpEsc, HelpQuit})
}

func HelpTree() string {
	return BuildHelpText([]HelpItem{
		HelpSpace,
		{Key: "←/h", Desc: "collapse"},
		{Key: "→/l", Desc: "expand"},
		{Key: "a", Desc: "all"},
		{Key: "n", Desc: "none"},
		HelpPlanItem,
		HelpRefresh,
		HelpTab,
		HelpSearch,
		{Key: "Esc", Desc: "menu"},
	})
}

func HelpConfirm() string {
	return BuildHelpText([]HelpItem{
		HelpNavUp,
		HelpEnter,
		{Key: "Esc", Desc: "cancel"},
	})
}

func HelpSelectList() string {
	return BuildHelpText([]HelpItem{
		HelpNavUp,
		{Key: "Space", Desc: "toggle"},
		HelpSelectAll,
		HelpDeselectAll,
		HelpEnter,
		{Key: "Esc", Desc: "cancel"},
	})
}

func HelpStop() string {
	return BuildHelpText([]HelpItem{
		HelpSpace,
		{Key: "Enter", Desc: "expand"},
		{Key: "a", Desc: "all"},
		{Key: "n", Desc: "none"},
		{Key: "p", Desc: "stop"},
		HelpEsc,
	})
}

func HelpEscQuit() string {
	return BuildHelpText([]HelpItem{HelpEsc})
}
