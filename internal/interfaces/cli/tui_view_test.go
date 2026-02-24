package cli

import (
	"strings"
	"testing"
)

func TestModel_RenderLoading(t *testing.T) {
	m := NewModel("demo", "../../..")
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
	m := NewModel("demo", "../../..")
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
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateTree
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

func TestModel_RenderApplyConfirm(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateApplyConfirm
	m.Loading.Active = false
	m.Action.ConfirmSelected = 0

	view := m.View()

	if !strings.Contains(view, "Confirm Apply") {
		t.Error("Apply confirm view should contain 'Confirm Apply'")
	}
}

func TestModel_RenderApplyProgress(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateApplyProgress
	m.Loading.Active = false
	m.Action.ApplyProgress = 5
	m.Action.ApplyTotal = 10

	view := m.View()

	if !strings.Contains(view, "Applying") {
		t.Error("Apply progress view should contain 'Applying'")
	}
}

func TestModel_RenderApplyComplete(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.UI.Width = 80
	m.UI.Height = 24
	m.ViewState = ViewStateApplyComplete
	m.Loading.Active = false
	m.Action.ApplyComplete = true

	view := m.View()

	if !strings.Contains(view, "Complete") {
		t.Error("Apply complete view should contain 'Complete'")
	}
}

func TestModel_HandleEscape(t *testing.T) {
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateTree
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
	m := NewModel("demo", "../../..")

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
	m := NewModel("demo", "../../..")
	m.ViewState = ViewStateMainMenu
	m.Loading.Active = false
	m.UI.MainMenuIndex = 0

	newModel, _ := m.handleEnter()
	model := newModel.(Model)

	if model.ViewState != ViewStateServiceManagement {
		t.Errorf("Expected ViewStateServiceManagement, got %d", model.ViewState)
	}
}
