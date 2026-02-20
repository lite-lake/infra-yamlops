package cli

import (
	"testing"
)

func TestNewModel_LoadsConfig(t *testing.T) {
	m := NewModel("demo", "../../..")

	if m.Config == nil {
		t.Error("Config should be loaded on NewModel")
	}

	if m.Config != nil {
		if len(m.Config.Zones) == 0 {
			t.Error("Zones should be loaded")
		}
		if len(m.Config.Servers) == 0 {
			t.Error("Servers should be loaded")
		}
		if len(m.Config.Services) == 0 {
			t.Error("Services should be loaded")
		}
		if len(m.Config.Domains) == 0 {
			t.Error("Domains should be loaded")
		}
	}
}

func TestNewModel_BuildsTrees(t *testing.T) {
	m := NewModel("demo", "../../..")

	if len(m.Tree.TreeNodes) == 0 {
		t.Error("App tree should be built")
	}

	if len(m.Tree.DNSTreeNodes) == 0 {
		t.Error("DNS tree should be built")
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
