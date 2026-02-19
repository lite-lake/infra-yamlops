package cli

const (
	KeyQuit             = "q"
	KeyCancel           = "x"
	KeyEscape           = "esc"
	KeyUp               = "up"
	KeyUpAlt            = "k"
	KeyDown             = "down"
	KeyDownAlt          = "j"
	KeySpace            = " "
	KeyEnter            = "enter"
	KeyTab              = "tab"
	KeySelectAll        = "a"
	KeySelectAllUpper   = "A"
	KeyDeselectAll      = "n"
	KeyDeselectAllUpper = "N"
	KeyPlan             = "p"
	KeyRefresh          = "r"
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
	HelpNavUp       = HelpItem{Key: "↑/↓", Desc: "选择"}
	HelpEnter       = HelpItem{Key: "Enter", Desc: "确认"}
	HelpEsc         = HelpItem{Key: "Esc", Desc: "返回"}
	HelpQuit        = HelpItem{Key: "q", Desc: "退出"}
	HelpSpace       = HelpItem{Key: "Space", Desc: "选择"}
	HelpTab         = HelpItem{Key: "Tab", Desc: "切换"}
	HelpSelectAll   = HelpItem{Key: "a", Desc: "全选"}
	HelpDeselectAll = HelpItem{Key: "n", Desc: "全不选"}
	HelpPlanItem    = HelpItem{Key: "p", Desc: "计划"}
	HelpRefresh     = HelpItem{Key: "r", Desc: "刷新"}
	HelpCurrent     = HelpItem{Key: "a", Desc: "当前"}
	HelpCancel      = HelpItem{Key: "n", Desc: "取消"}
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
		{Key: "Enter", Desc: "展开"},
		{Key: "a", Desc: "当前"},
		{Key: "n", Desc: "取消"},
		{Key: "A", Desc: "全选"},
		{Key: "N", Desc: "全不选"},
		HelpPlanItem,
		HelpRefresh,
		HelpTab,
		{Key: "Esc", Desc: "主菜单"},
		HelpQuit,
	})
}

func HelpPlan() string {
	return BuildHelpText([]HelpItem{
		{Key: "Enter", Desc: "执行"},
		HelpEsc,
		HelpQuit,
	})
}

func HelpConfirm() string {
	return BuildHelpText([]HelpItem{
		HelpNavUp,
		HelpEnter,
		{Key: "Esc", Desc: "取消"},
		HelpQuit,
	})
}

func HelpComplete() string {
	return BuildHelpText([]HelpItem{
		{Key: "Enter", Desc: "返回"},
		HelpQuit,
	})
}

func HelpSelectList() string {
	return BuildHelpText([]HelpItem{
		HelpNavUp,
		{Key: "Space", Desc: "切换"},
		HelpSelectAll,
		HelpDeselectAll,
		HelpEnter,
		{Key: "Esc", Desc: "取消"},
		HelpQuit,
	})
}

func HelpStop() string {
	return BuildHelpText([]HelpItem{
		HelpSpace,
		{Key: "Enter", Desc: "展开"},
		{Key: "a", Desc: "当前"},
		{Key: "n", Desc: "取消"},
		{Key: "A", Desc: "全选"},
		{Key: "N", Desc: "全不选"},
		{Key: "p", Desc: "确认停止"},
		HelpEsc,
		HelpQuit,
	})
}

func HelpEscQuit() string {
	return BuildHelpText([]HelpItem{HelpEsc, HelpQuit})
}
