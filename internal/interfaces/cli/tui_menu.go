package cli

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/version"
)

func (m Model) renderMainMenu() string {
	items := []string{
		"Service Management",
		"Domain/DNS Management",
		"Exit",
	}

	availableHeight := m.UI.Height - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	viewport := NewViewport(0, len(items), availableHeight)
	viewport.CursorIndex = m.UI.MainMenuIndex
	viewport.EnsureCursorVisible()

	var sb strings.Builder
	title := TitleStyle.Render(fmt.Sprintf("  YAMLOps [%s]", strings.ToUpper(string(m.Environment))))
	sb.WriteString(title + "\n\n")

	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(items); i++ {
		if i == m.UI.MainMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+items[i]) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+items[i]) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  q quit"))
	sb.WriteString("\n" + HelpStyle.Render(fmt.Sprintf("  v%s", version.Version)))

	return BaseStyle.Render(sb.String())
}

func (m Model) renderServiceManagement() string {
	items := []string{
		"Service Deploy",
		"Service Stop",
		"Service Restart",
		"Service Cleanup",
		"Server Environment",
		"Back to Main Menu",
	}

	var sb strings.Builder
	title := TitleStyle.Render("  Service Management")
	sb.WriteString(title + "\n\n")

	for i, item := range items {
		if i == m.Server.ServiceMenuIndex {
			sb.WriteString(MenuSelectedStyle.Render("> "+item) + "\n")
		} else {
			sb.WriteString(MenuItemStyle.Render("  "+item) + "\n")
		}
	}

	sb.WriteString("\n" + HelpStyle.Render("  ↑/↓ navigate  Enter select  Esc back  q quit"))

	return BaseStyle.Render(sb.String())
}

func formatNodeStatus(status NodeStatus) string {
	switch status {
	case StatusRunning:
		return SuccessStyle.Render("[running]")
	case StatusStopped:
		return WarningStyle.Render("[stopped]")
	case StatusNeedsUpdate:
		return ChangeUpdateStyle.Render("[needs update]")
	case StatusError:
		return ChangeDeleteStyle.Render("[error]")
	default:
		return ""
	}
}
