package cli

import (
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func (m Model) View() string {
	switch m.ViewState {
	case ViewStateMainMenu:
		return m.renderMainMenu()
	case ViewStateServerSetup:
		return m.renderServerSetup()
	case ViewStateServerCheck:
		return m.renderServerCheck()
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
	return baseStyle.Render(content.String())
}

func (m Model) renderHeader() string {
	var header strings.Builder
	header.WriteString(titleStyle.Render("YAMLOps"))
	header.WriteString(" ")
	header.WriteString(envStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(string(m.Environment)))))
	selected := m.countSelected()
	total := m.countTotal()
	header.WriteString(fmt.Sprintf("    选中: %d/%d", selected, total))
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
	var content strings.Builder
	content.WriteString(m.renderTabs())
	content.WriteString("\n\n")
	if m.ErrorMessage != "" {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.ErrorMessage))
		content.WriteString("\n\n")
	}
	idx := 0
	for _, node := range m.getCurrentTree() {
		content.WriteString(m.renderNode(node, 0, &idx))
	}
	return content.String()
}

func (m Model) renderTabs() string {
	var tabs strings.Builder
	if m.ViewMode == ViewModeApp {
		tabs.WriteString(tabActiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(tabInactiveStyle.Render("DNS"))
	} else {
		tabs.WriteString(tabInactiveStyle.Render("Applications"))
		tabs.WriteString("    ")
		tabs.WriteString(tabActiveStyle.Render("DNS"))
	}
	return tabs.String()
}

func (m Model) renderNode(node *TreeNode, depth int, idx *int) string {
	var content strings.Builder
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		prefix = indent[:len(indent)-2] + "├─"
	}
	cursor := "  "
	if *idx == m.CursorIndex {
		cursor = "> "
	}
	selectIcon := "○"
	if node.Selected {
		selectIcon = "◉"
	} else if node.IsPartiallySelected() {
		selectIcon = "◐"
	}
	expandIcon := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIcon = "▾"
		} else {
			expandIcon = "▸"
		}
	}
	typePrefix := ""
	switch node.Type {
	case NodeTypeInfra:
		typePrefix = "[infra] "
	case NodeTypeBiz:
		typePrefix = "[biz] "
	case NodeTypeDNSRecord:
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, selectIcon, expandIcon, typePrefix, node.Name)
	if node.Info != "" {
		line = fmt.Sprintf("%-50s %s", line, node.Info)
	}
	if *idx == m.CursorIndex {
		line = selectedStyle.Render(line)
	}
	content.WriteString(line)
	content.WriteString("\n")
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				content.WriteString(m.renderNodeLastChild(child, depth+1, idx))
			} else {
				content.WriteString(m.renderNode(child, depth+1, idx))
			}
		}
	}
	return content.String()
}

func (m Model) renderNodeLastChild(node *TreeNode, depth int, idx *int) string {
	var content strings.Builder
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		prefix = indent[:len(indent)-2] + "└─"
	}
	cursor := "  "
	if *idx == m.CursorIndex {
		cursor = "> "
	}
	selectIcon := "○"
	if node.Selected {
		selectIcon = "◉"
	} else if node.IsPartiallySelected() {
		selectIcon = "◐"
	}
	expandIcon := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIcon = "▾"
		} else {
			expandIcon = "▸"
		}
	}
	typePrefix := ""
	switch node.Type {
	case NodeTypeInfra:
		typePrefix = "[infra] "
	case NodeTypeBiz:
		typePrefix = "[biz] "
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, selectIcon, expandIcon, typePrefix, node.Name)
	if node.Info != "" {
		line = fmt.Sprintf("%-50s %s", line, node.Info)
	}
	if *idx == m.CursorIndex {
		line = selectedStyle.Render(line)
	}
	content.WriteString(line)
	content.WriteString("\n")
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			if i == len(node.Children)-1 {
				content.WriteString(m.renderNodeLastChild(child, depth+1, idx))
			} else {
				content.WriteString(m.renderNode(child, depth+1, idx))
			}
		}
	}
	return content.String()
}

func (m Model) renderPlan() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行计划"))
	content.WriteString("\n\n")
	if m.ErrorMessage != "" {
		content.WriteString(changeDeleteStyle.Render("Error: " + m.ErrorMessage))
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render("Press q to go back"))
		return content.String()
	}
	if m.PlanResult == nil || len(m.PlanResult.Changes) == 0 {
		content.WriteString("No changes detected.\n")
	} else {
		for _, ch := range m.PlanResult.Changes {
			style := changeNoopStyle
			prefix := "~"
			switch ch.Type {
			case valueobject.ChangeTypeCreate:
				style = changeCreateStyle
				prefix = "+"
			case valueobject.ChangeTypeUpdate:
				style = changeUpdateStyle
				prefix = "~"
			case valueobject.ChangeTypeDelete:
				style = changeDeleteStyle
				prefix = "-"
			}
			line := fmt.Sprintf("%s %s: %s", prefix, ch.Entity, ch.Name)
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}
	content.WriteString("\n")
	content.WriteString(changeCreateStyle.Render("Press Enter to apply changes"))
	content.WriteString("\n")
	return content.String()
}

func (m Model) renderApplyConfirm() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("确认执行"))
	content.WriteString("\n\n")
	content.WriteString("是否执行以下变更?\n\n")
	if m.PlanResult != nil {
		nonNoopCount := 0
		for _, ch := range m.PlanResult.Changes {
			if ch.Type != valueobject.ChangeTypeNoop {
				nonNoopCount++
			}
		}
		content.WriteString(fmt.Sprintf("变更项数: %d\n", nonNoopCount))
	}
	content.WriteString("\n")
	options := []string{"确认执行", "取消"}
	for i, opt := range options {
		if i == m.ConfirmSelected {
			content.WriteString(selectedStyle.Render("▸ " + opt))
		} else {
			content.WriteString("  " + opt)
		}
		content.WriteString("\n")
	}
	return content.String()
}

func (m Model) renderApplyProgress() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行中..."))
	content.WriteString("\n\n")
	progress := float64(m.ApplyProgress) / float64(m.ApplyTotal)
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	content.WriteString(progressBarStyle.Render(bar))
	content.WriteString(fmt.Sprintf(" %.0f%%\n", progress*100))
	return content.String()
}

func (m Model) renderApplyComplete() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("执行完成"))
	content.WriteString("\n\n")
	if m.ApplyResults != nil {
		successCount := 0
		failCount := 0
		for _, result := range m.ApplyResults {
			if result.Success {
				successCount++
				content.WriteString(changeCreateStyle.Render(fmt.Sprintf("✓ %s: %s", result.Change.Entity, result.Change.Name)))
			} else {
				failCount++
				content.WriteString(changeDeleteStyle.Render(fmt.Sprintf("✗ %s: %s - %v", result.Change.Entity, result.Change.Name, result.Error)))
			}
			content.WriteString("\n")
		}
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("成功: %d  失败: %d\n", successCount, failCount))
	}
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("Press Enter to return"))
	return content.String()
}

func (m Model) renderHelp() string {
	if m.ViewState == ViewStateTree {
		return helpStyle.Render("\n[Space] 选择  [Enter] 展开/折叠  [a] 全选当前  [n] 取消当前  [p] Plan\n[A] 全部选中  [N] 全部取消  [Tab] 切换 App/DNS  [r] 刷新  [q] 退出")
	}
	if m.ViewState == ViewStateApplyProgress {
		return ""
	}
	return helpStyle.Render("\n[q] 返回  [Esc] 返回主界面")
}
