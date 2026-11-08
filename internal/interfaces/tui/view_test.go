package tui

import (
	"strings"
	"testing"
)

func TestModel_RenderLoading(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.UI.Width = 80
	m.UI.Height = 24
	m.Loading.Active = true
	m.Loading.Message = "Loading..."
	m.Loading.Spinner = 0

	view := m.View()

	if !strings.Contains(view, "Loading...") {
		t.Error("Loading view should contain loading message")
	}

	if !strings.Contains(view, "YAMLOps") {
		t.Error("Loading view should contain 'YAMLOps'")
	}
}

func TestModel_RenderMainMenu(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateMainMenu
	m.Loading.Active = false

	view := m.View()

	if !strings.Contains(view, "YAMLOps") {
		t.Error("Main menu should contain 'YAMLOps'")
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.ViewState = ViewStateTreeService
	m.Loading.Active = false

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

func TestModel_RenderPlanView(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStatePlan
	m.Loading.Active = false
	m.Action.ConfirmSelected = 0

	view := m.View()

	if !strings.Contains(view, "Plan") {
		t.Error("Plan view should contain 'Plan'")
	}
}

func TestModel_RenderProgressView(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateProgress
	m.Loading.Active = false
	m.Action.ApplyProgress = 5
	m.Action.ApplyTotal = 10

	view := m.View()

	if !strings.Contains(view, "EXECUTING") {
		t.Error("Progress view should contain 'EXECUTING'")
	}
}

func TestModel_RenderCompleteView(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateComplete
	m.Loading.Active = false
	m.Action.ApplyComplete = true

	view := m.View()

	if !strings.Contains(view, "SUMMARY") && !strings.Contains(view, "Completed") {
		t.Error("Complete view should contain 'SUMMARY' or 'Completed'")
	}
}

func TestModel_HandleEscape(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.ViewState = ViewStateTreeService
	m.Loading.Active = false
	m.UI.ErrorMessage = "test error"

	newModel, _ := m.handleEscape()
	model := newModel.(Model)

	if model.UI.ErrorMessage != "" {
		t.Error("Error message should be cleared")
	}
}

func TestSpinnerFrames(t *testing.T) {
	if len(SpinnerFrames) == 0 {
		t.Error("SpinnerFrames should not be empty")
	}

	for i, frame := range SpinnerFrames {
		if frame == "" {
			t.Errorf("SpinnerFrame[%d] should not be empty", i)
		}
	}
}

func TestLoadingState(t *testing.T) {
	m := NewModel("demo", "../../..", 5)

	m.Loading.Active = true
	m.Loading.Message = "Testing"
	m.Loading.Spinner = 0

	if !m.Loading.Active {
		t.Error("Loading should be active")
	}

	if m.Loading.Message != "Testing" {
		t.Error("Loading message should be set")
	}
}

func TestModel_HandleEnter_MainMenu(t *testing.T) {
	m := NewModel("demo", "../../..", 5)
	m.ViewState = ViewStateMainMenu
	m.Loading.Active = false
	m.UI.MainMenuIndex = 0

	// Index 0 is the "Service Management" parent node (expanded by default).
	// Enter on a parent node should toggle expand/collapse, not navigate.
	newModel, _ := m.handleEnter()
	model := newModel.(Model)

	if model.ViewState != ViewStateMainMenu {
		t.Errorf("Expected ViewStateMainMenu (parent toggle), got %d", model.ViewState)
	}
	if model.UI.MenuNodes[0].Expanded {
		t.Error("Expected 'Service Management' to be collapsed after Enter on parent")
	}
}
