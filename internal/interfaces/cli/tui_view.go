package cli

import (
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (m Model) View() string {
	if m.ShowHelp {
		return m.renderHelpView()
	}
	if m.Loading.Active {
		return m.renderLoadingView()
	}
	switch m.ViewState {
	case ViewStateMainMenu:
		return m.renderMainMenu()
	case ViewStateServiceManagement:
		return m.renderServiceManagement()
	case ViewStateServerSetup:
		return m.renderServerEnvSetup()
	case ViewStateServerCheck:
		return m.renderServerEnvResults()
	case ViewStateDNSManagement:
		return m.renderDNSManagement()
	case ViewStateDNSPullDomains:
		return m.renderDNSPullDomains()
	case ViewStateDNSPullRecords:
		return m.renderDNSPullRecords()
	case ViewStateDNSPullDiff:
		return m.renderDNSPullDiff()
	case ViewStateServiceCleanup:
		return m.renderServiceCleanup()
	case ViewStateServiceCleanupConfirm:
		return m.renderServiceCleanupConfirm()
	case ViewStateServiceCleanupComplete:
		return m.renderServiceCleanupComplete()
	case ViewStateServiceStop:
		return m.renderServiceStop()
	case ViewStateServiceStopConfirm:
		return m.renderServiceStopConfirm()
	case ViewStateServiceStopComplete:
		return m.renderServiceStopComplete()
	case ViewStateServiceRestart:
		return m.renderServiceRestart()
	case ViewStateServiceRestartConfirm:
		return m.renderServiceRestartConfirm()
	case ViewStateServiceRestartComplete:
		return m.renderServiceRestartComplete()
	}
	var content strings.Builder
	content.WriteString(m.renderHeader())
	switch m.ViewState {
	case ViewStateTree:
		content.WriteString(m.renderTree())
	case ViewStatePlan:
		content.WriteString(m.renderPlan())
	case ViewStateApplyConfirm:
		content.WriteString(m.renderApplyConfirm())
	case ViewStateApplyProgress:
		content.WriteString(m.renderApplyProgress())
	case ViewStateApplyComplete:
		content.WriteString(m.renderApplyComplete())
	}
	content.WriteString(m.renderHelp())
	return BaseStyle.Render(content.String())
}

func (m Model) renderLoadingView() string {
	var content strings.Builder
	content.WriteString(TitleStyle.Render("YAMLOps"))
	content.WriteString(" ")
	content.WriteString(EnvStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(m.Environment)))))
	content.WriteString("\n\n")

	spinner := SpinnerFrames[m.Loading.Spinner]
	loadingText := fmt.Sprintf("  %s %s", spinner, m.Loading.Message)
	content.WriteString(LoadingOverlayStyle.Render(loadingText))
	content.WriteString("\n\n")
	content.WriteString(HelpStyle.Render("  Ctrl+C to cancel  ? help"))
	return BaseStyle.Render(content.String())
}

func (m Model) renderHeader() string {
	var header strings.Builder
	header.WriteString(TitleStyle.Render("YAMLOps"))
	header.WriteString(" ")
	header.WriteString(EnvStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(m.Environment)))))
	selected := m.countSelected()
	total := m.countTotal()
	header.WriteString(fmt.Sprintf("    Selected: %d/%d", selected, total))
	header.WriteString("\n")
	return header.String()
}

func (m Model) countSelected() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += node.CountSelected()
	}
	return count
}

func (m Model) countTotal() int {
	count := 0
	for _, node := range m.getCurrentTree() {
		count += node.CountTotal()
	}
	return count
}

func (m Model) renderTree() string {
	lines := renderTreeNodes(m.getCurrentTree(), m.Tree.CursorIndex, true)

	availableHeight := m.UI.Height - 8
	if availableHeight < 5 {
		availableHeight = 5
	}

	treeHeight := availableHeight - 2
	if treeHeight < 3 {
		treeHeight = 3
	}

	if m.UI.ErrorMessage != "" {
		treeHeight -= 2
		if treeHeight < 3 {
			treeHeight = 3
		}
	}

	totalNodes := len(lines)
	viewport := NewViewport(m.Tree.CursorIndex, totalNodes, treeHeight)
	viewport.EnsureCursorVisible()
	m.UI.ScrollOffset = viewport.Offset

	var content strings.Builder
	content.WriteString(m.renderTabs())
	content.WriteString("\n\n")

	if m.UI.ErrorMessage != "" {
		content.WriteString(ChangeDeleteStyle.Render("Error: " + m.UI.ErrorMessage))
		content.WriteString("\n\n")
	}

	start := viewport.VisibleStart()
	end := viewport.VisibleEnd()
	for i := start; i < end && i < len(lines); i++ {
		content.WriteString(lines[i])
		content.WriteString("\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		content.WriteString("\n")
		content.WriteString(viewport.RenderScrollIndicator())
	}

	return content.String()
}

func (m Model) renderTabs() string {
	var tabs strings.Builder
	if m.ViewMode == ViewModeApp {
		tabs.WriteString(TabActiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(TabInactiveStyle.Render("DNS"))
	} else {
		tabs.WriteString(TabInactiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(TabActiveStyle.Render("DNS"))
	}
	return tabs.String()
}

func (m Model) renderPlan() string {
	var lines []string
	lines = append(lines, TitleStyle.Render("Execution Plan"))
	lines = append(lines, "")
	if m.UI.ErrorMessage != "" {
		lines = append(lines, ChangeDeleteStyle.Render("Error: "+m.UI.ErrorMessage))
		lines = append(lines, "")
		lines = append(lines, HelpStyle.Render("Esc back  q quit"))
		return strings.Join(lines, "\n")
	}
	if m.Action.PlanResult == nil || len(m.Action.PlanResult.Changes()) == 0 {
		lines = append(lines, "No changes detected.")
	} else {
		for _, ch := range m.Action.PlanResult.Changes() {
			style := ChangeNoopStyle
			prefix := "~"
			switch ch.Type() {
			case valueobject.ChangeTypeCreate:
				style = ChangeCreateStyle
				prefix = "+"
			case valueobject.ChangeTypeUpdate:
				style = ChangeUpdateStyle
				prefix = "~"
			case valueobject.ChangeTypeDelete:
				style = ChangeDeleteStyle
				prefix = "-"
			}
			line := fmt.Sprintf("%s %s: %s", prefix, ch.Entity(), ch.Name())
			if ch.Entity() == "service" || ch.Entity() == "infra_service" {
				if ch.RemoteExists() {
					line += " [update]"
				} else {
					line += " [new]"
				}
			}
			lines = append(lines, style.Render(line))
		}
	}
	lines = append(lines, "")
	lines = append(lines, ChangeCreateStyle.Render("Press Enter to apply"))

	availableHeight := m.UI.Height - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	viewport := NewViewport(0, totalLines, availableHeight)
	m.UI.ScrollOffset = viewport.Offset

	var content strings.Builder
	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(lines); i++ {
		content.WriteString(lines[i])
		content.WriteString("\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		content.WriteString("\n")
		content.WriteString(viewport.RenderSimpleScrollIndicator())
	}

	return content.String()
}

func (m Model) renderApplyConfirm() string {
	var content strings.Builder
	content.WriteString(TitleStyle.Render("Confirm Apply"))
	content.WriteString("\n\n")
	content.WriteString("Apply the following changes?\n\n")
	if m.Action.PlanResult != nil {
		nonNoopCount := 0
		for _, ch := range m.Action.PlanResult.Changes() {
			if ch.Type() != valueobject.ChangeTypeNoop {
				nonNoopCount++
			}
		}
		content.WriteString(fmt.Sprintf("Changes: %d\n", nonNoopCount))
	}
	content.WriteString("\n")
	options := []string{"Confirm", "Cancel"}
	for i, opt := range options {
		if i == m.Action.ConfirmSelected {
			content.WriteString(SelectedStyle.Render("▸ " + opt))
		} else {
			content.WriteString("  " + opt)
		}
		content.WriteString("\n")
	}
	return content.String()
}

func (m Model) renderApplyProgress() string {
	var content strings.Builder
	content.WriteString(TitleStyle.Render("Applying..."))
	content.WriteString("\n\n")
	progress := float64(m.Action.ApplyProgress) / float64(m.Action.ApplyTotal)
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	content.WriteString(ProgressBarStyle.Render(bar))
	content.WriteString(fmt.Sprintf(" %.0f%%\n", progress*100))
	return content.String()
}

func (m Model) renderApplyComplete() string {
	var lines []string
	lines = append(lines, TitleStyle.Render("Complete"))
	lines = append(lines, "")

	if m.Action.ApplyResults != nil {
		successCount := 0
		failCount := 0
		for _, result := range m.Action.ApplyResults {
			if result.Success {
				successCount++
				lines = append(lines, ChangeCreateStyle.Render(fmt.Sprintf("✓ %s: %s", result.Change.Entity(), result.Change.Name())))
				for _, w := range result.Warnings {
					lines = append(lines, WarningStyle.Render(fmt.Sprintf("  ⚠ %s", w)))
				}
			} else {
				failCount++
				lines = append(lines, ChangeDeleteStyle.Render(fmt.Sprintf("✗ %s: %s - %v", result.Change.Entity(), result.Change.Name(), result.Error)))
			}
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Success: %d  Failed: %d", successCount, failCount))
	}
	lines = append(lines, "")
	lines = append(lines, HelpStyle.Render("Enter back  q quit"))

	availableHeight := m.UI.Height - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	viewport := NewViewport(0, totalLines, availableHeight)
	m.UI.ScrollOffset = viewport.Offset

	var content strings.Builder
	for i := viewport.VisibleStart(); i < viewport.VisibleEnd() && i < len(lines); i++ {
		content.WriteString(lines[i])
		content.WriteString("\n")
	}

	if viewport.TotalRows > viewport.VisibleRows {
		content.WriteString("\n")
		content.WriteString(viewport.RenderSimpleScrollIndicator())
	}

	return content.String()
}

func (m Model) renderHelp() string {
	if m.ViewState == ViewStateTree {
		return "\n" + HelpTree()
	}
	if m.ViewState == ViewStateApplyProgress {
		return ""
	}
	if m.ViewState == ViewStatePlan {
		return "\n" + HelpPlan()
	}
	if m.ViewState == ViewStateApplyConfirm {
		return "\n" + HelpConfirm()
	}
	if m.ViewState == ViewStateApplyComplete {
		return "\n" + HelpComplete()
	}
	return "\n" + HelpEscQuit() + "  " + HelpStyle.Render("? help")
}

func (m Model) renderHelpView() string {
	var content strings.Builder
	content.WriteString(TitleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	helpItems := []struct {
		Section string
		Items   []HelpItem
	}{
		{
			Section: "General",
			Items: []HelpItem{
				{"Ctrl+C / q", "Quit"},
				{"Esc / x", "Back / Cancel"},
				{"?", "Show help"},
			},
		},
		{
			Section: "Navigation",
			Items: []HelpItem{
				{"↑ / k", "Move up"},
				{"↓ / j", "Move down"},
				{"Enter", "Confirm / Expand"},
				{"Tab", "Switch mode"},
			},
		},
		{
			Section: "Selection",
			Items: []HelpItem{
				{"Space", "Toggle selection"},
				{"a", "Select current group"},
				{"n", "Deselect current group"},
				{"A", "Select all"},
				{"N", "Deselect all"},
			},
		},
		{
			Section: "Actions",
			Items: []HelpItem{
				{"p", "Generate plan"},
				{"r", "Refresh / Check server environment"},
				{"s", "Sync server environment"},
			},
		},
	}

	for _, section := range helpItems {
		content.WriteString(TitleStyle.Render(section.Section))
		content.WriteString("\n")
		for _, item := range section.Items {
			content.WriteString(fmt.Sprintf("  %-20s %s\n", item.Key, item.Desc))
		}
		content.WriteString("\n")
	}

	content.WriteString(HelpStyle.Render("Press any key to close"))
	return LoadingOverlayStyle.Render(content.String())
}
