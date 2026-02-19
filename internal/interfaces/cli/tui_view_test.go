package cli

import (
	"strings"
	"testing"
)

func TestModel_RenderTree(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStateTree

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
	m.ViewState = ViewStateTree

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
	m.ViewState = ViewStateTree

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

func TestModel_GeneratePlan(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateTree

	for _, node := range m.TreeNodes {
		node.SelectRecursive(false)
	}

	if len(m.TreeNodes) > 0 && len(m.TreeNodes[0].Children) > 0 {
		server := m.TreeNodes[0].Children[0]
		if len(server.Children) > 0 {
			for _, child := range server.Children {
				if child.Type == NodeTypeBiz || child.Type == NodeTypeInfra {
					child.Selected = true
					break
				}
			}
		}
	}

	m.generatePlan()

	if m.ErrorMessage != "" {
		t.Logf("Plan generation message: %s", m.ErrorMessage)
	}

	if m.PlanResult == nil {
		t.Log("Plan result is nil (expected for empty state)")
	}
}

func TestModel_RenderPlan(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStatePlan
	m.PlanResult = nil

	view := m.View()

	if !strings.Contains(view, "执行计划") {
		t.Error("Plan view should contain '执行计划'")
	}
}

func TestModel_RenderApplyConfirm(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStateApplyConfirm
	m.ConfirmSelected = 0

	view := m.View()

	if !strings.Contains(view, "确认执行") {
		t.Error("Apply confirm view should contain '确认执行'")
	}
}

func TestModel_RenderApplyProgress(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStateApplyProgress
	m.ApplyProgress = 5
	m.ApplyTotal = 10

	view := m.View()

	if !strings.Contains(view, "执行中") {
		t.Error("Apply progress view should contain '执行中'")
	}
}

func TestModel_RenderApplyComplete(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStateApplyComplete
	m.ApplyComplete = true

	view := m.View()

	if !strings.Contains(view, "执行完成") {
		t.Error("Apply complete view should contain '执行完成'")
	}
}

func TestModel_MainMenu(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.Width = 80
	m.Height = 24
	m.ViewState = ViewStateMainMenu
	m.MainMenuIndex = 0

	view := m.View()

	if !strings.Contains(view, "YAMLOps") {
		t.Error("Main menu should contain 'YAMLOps'")
	}
}

func TestModel_HandleEnter(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateMainMenu
	m.MainMenuIndex = 0

	newModel, _ := m.handleEnter()
	model := newModel.(Model)

	if model.ViewState != ViewStateServiceManagement {
		t.Errorf("Expected ViewStateServiceManagement, got %d", model.ViewState)
	}
}

func TestModel_HandleEscape(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateTree
	m.ErrorMessage = "test error"

	newModel, _ := m.handleEscape()
	model := newModel.(Model)

	if model.ViewState != ViewStateServiceManagement {
		t.Errorf("Expected ViewStateServiceManagement, got %d", model.ViewState)
	}
	if model.ErrorMessage != "" {
		t.Error("Error message should be cleared")
	}
}

func TestModel_SelectAll(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateTree

	for _, node := range m.TreeNodes {
		node.SelectRecursive(false)
	}

	m = m.handleSelectAll(true)
	selectedCount := m.countSelected()

	if selectedCount == 0 {
		t.Error("Should have selected items after select all")
	}

	m = m.handleSelectAll(false)
	selectedCount = m.countSelected()

	if selectedCount != 0 {
		t.Error("Should have no selected items after deselect all")
	}
}

func TestModel_AppTreeHasServers(t *testing.T) {
	m := NewModel("demo", "../../..")

	hasServer := false
	hasInfra := false
	hasBiz := false

	for _, zone := range m.TreeNodes {
		for _, server := range zone.Children {
			hasServer = true
			for _, svc := range server.Children {
				if svc.Type == NodeTypeInfra {
					hasInfra = true
				}
				if svc.Type == NodeTypeBiz {
					hasBiz = true
				}
			}
		}
	}

	if !hasServer {
		t.Error("App tree should have servers")
	}
	if !hasInfra {
		t.Log("Note: No infra services in tree")
	}
	if !hasBiz {
		t.Log("Note: No business services in tree")
	}
}
