package cli

import (
	"strings"
	"testing"
)

func TestModel_RenderTree(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24

	view := m.View()

	if !strings.Contains(view, "YAMLOps") {
		t.Error("View should contain 'YAMLOps'")
	}

	if !strings.Contains(view, "DEMO") {
		t.Error("View should contain environment name 'DEMO'")
	}

	if !strings.Contains(view, "Applications") {
		t.Error("View should contain 'Applications' tab")
	}

	if !strings.Contains(view, "DNS") {
		t.Error("View should contain 'DNS' tab")
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := NewModel("demo", "../../..")

	if m.ViewMode != ViewModeApp {
		t.Error("Default view mode should be App")
	}

	m = m.handleTab()
	if m.ViewMode != ViewModeDNS {
		t.Error("View mode should switch to DNS")
	}

	m = m.handleTab()
	if m.ViewMode != ViewModeApp {
		t.Error("View mode should switch back to App")
	}
}

func TestModel_Selection(t *testing.T) {
	m := NewModel("demo", "../../..")

	initialSelected := m.countSelected()

	if len(m.TreeNodes) > 0 && len(m.TreeNodes[0].Children) > 0 {
		leaf := m.TreeNodes[0].Children[0]
		if len(leaf.Children) > 0 {
			leaf = leaf.Children[0]
		}
		leaf.Selected = true
		leaf.UpdateParentSelection()

		newSelected := m.countSelected()
		if newSelected <= initialSelected {
			t.Error("Selection count should increase after selecting a node")
		}
	}
}

func TestModel_Navigation(t *testing.T) {
	m := NewModel("demo", "../../..")

	initialCursor := m.CursorIndex

	m = m.handleDown()
	if m.CursorIndex <= initialCursor {
		t.Error("Cursor should move down")
	}

	m = m.handleUp()
	if m.CursorIndex >= initialCursor+1 {
		t.Error("Cursor should move up")
	}
}

func TestModel_DNSTree(t *testing.T) {
	m := NewModel("demo", "../../..")

	if len(m.DNSTreeNodes) == 0 {
		t.Error("DNS tree should be built")
	}

	for _, node := range m.DNSTreeNodes {
		if node.Type != NodeTypeDomain {
			t.Errorf("DNS tree root should be domain type, got %s", node.Type)
		}
	}
}
