package cli

import (
	"testing"
)

func TestNewModel_InitializesLoading(t *testing.T) {
	m := NewModel("demo", "../../..")

	if m.Loading == nil {
		t.Error("Loading state should be initialized")
	}

	if m.ViewState != ViewStateMainMenu {
		t.Error("Initial view state should be MainMenu")
	}
}

func TestNewModel_AsyncLoadConfig(t *testing.T) {
	m := NewModel("demo", "../../..")

	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command for async config loading")
	}
}

func TestTreeNode_Selection(t *testing.T) {
	root := &TreeNode{
		ID:   "root",
		Name: "root",
		Children: []*TreeNode{
			{ID: "child1", Name: "child1"},
			{ID: "child2", Name: "child2"},
		},
	}

	for _, child := range root.Children {
		child.Parent = root
	}

	if root.IsPartiallySelected() {
		t.Error("Root should not be partially selected when no children are selected")
	}

	root.Children[0].Selected = true

	if !root.IsPartiallySelected() {
		t.Error("Root should be partially selected when some children are selected")
	}

	root.SelectRecursive(true)

	if !root.Selected || !root.Children[0].Selected || !root.Children[1].Selected {
		t.Error("All nodes should be selected after SelectRecursive(true)")
	}

	root.SelectRecursive(false)

	if root.Selected || root.Children[0].Selected || root.Children[1].Selected {
		t.Error("All nodes should be deselected after SelectRecursive(false)")
	}
}

func TestTreeNode_Count(t *testing.T) {
	root := &TreeNode{
		ID:   "root",
		Name: "root",
		Children: []*TreeNode{
			{ID: "child1", Name: "child1", Selected: true},
			{ID: "child2", Name: "child2", Selected: false},
		},
	}

	selected := root.CountSelected()
	if selected != 1 {
		t.Errorf("CountSelected should return 1, got %d", selected)
	}

	total := root.CountTotal()
	if total != 2 {
		t.Errorf("CountTotal should return 2, got %d", total)
	}
}
