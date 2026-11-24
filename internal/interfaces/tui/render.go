package tui

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/styles"
)

func renderNodeToLines(node *TreeNode, depth int, idx *int, lines *[]string, cursorIndex int, showInfo bool, isLastChild bool) {
	indent := strings.Repeat("  ", depth)
	prefix := indent
	if depth > 0 {
		if isLastChild {
			prefix = indent[:len(indent)-2] + "└─"
		} else {
			prefix = indent[:len(indent)-2] + "├─"
		}
	}
	cursor := "  "
	if *idx == cursorIndex {
		cursor = "> "
	}
	var selectIcon string
	if len(node.Children) > 0 {
		if !node.Expanded {
			selectIcon = " "
		} else if node.IsPartiallySelected() {
			selectIcon = styles.IconPartial
		} else if node.Selected {
			selectIcon = styles.IconAllSelected
		} else {
			selectIcon = styles.IconUnchecked
		}
	} else {
		if node.Selected {
			selectIcon = styles.IconChecked
		} else {
			selectIcon = styles.IconUnchecked
		}
	}
	expandIcon := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIcon = styles.IconExpanded
		} else {
			expandIcon = styles.IconCollapsed
		}
	}
	typePrefix := ""
	switch node.Type {
	case NodeTypeInfra:
		typePrefix = "[infra] "
	case NodeTypeBiz:
		typePrefix = "[biz] "
	}
	line := fmt.Sprintf("%s%s%s %s%s%s", cursor, prefix, expandIcon, selectIcon, typePrefix, node.Name)
	if statusStr := formatNodeStatus(node.Status); statusStr != "" {
		line = fmt.Sprintf("%s %s", line, statusStr)
	}
	if showInfo && node.Info != "" {
		line = fmt.Sprintf("%s  %s", line, node.Info)
	}
	if *idx == cursorIndex {
		line = styles.SelectedStyle.Render(line)
	}
	*lines = append(*lines, line)
	*idx++
	if node.Expanded {
		for i, child := range node.Children {
			renderNodeToLines(child, depth+1, idx, lines, cursorIndex, showInfo, i == len(node.Children)-1)
		}
	}
}

func renderTreeNodes(nodes []*TreeNode, cursorIndex int, showInfo bool) []string {
	var lines []string
	idx := 0
	for _, node := range nodes {
		renderNodeToLines(node, 0, &idx, &lines, cursorIndex, showInfo, false)
	}
	return lines
}
