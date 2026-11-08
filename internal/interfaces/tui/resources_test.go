package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/environment"
)

// sampleResourceReport builds a fully-successful report for TUI rendering tests.
func sampleResourceReport() *environment.ServerResourceReport {
	cpu := 12.3
	return &environment.ServerResourceReport{
		ServerName: "srv-a",
		Zone:       "node9",
		Env:        "dev",
		ScannedAt:  "2026-08-27T00:00:00Z",
		Degraded:   false,
		Segments: map[string]environment.SegmentState{
			"host_cpu": environment.SegOK, "meminfo": environment.SegOK, "loadavg": environment.SegOK,
			"disk": environment.SegOK, "docker_ps": environment.SegOK, "docker_stats": environment.SegOK,
			"images": environment.SegOK, "dirs": environment.SegOK, "docker_df": environment.SegOK,
		},
		Host: &environment.HostResources{
			CPUCores: 4, CPUUsagePct: &cpu, Load1_5_15: [3]float64{0.2, 0.3, 0.4},
			MemTotalBytes: 4 * 1024 * 1024 * 1024, MemUsedBytes: 2 * 1024 * 1024 * 1024,
			MemTotalHuman: "4.0 GiB", MemUsedHuman: "2.0 GiB",
		},
		Disks: []environment.DiskMount{
			{MountPoint: "/", TotalBytes: 100 * 1024 * 1024 * 1024, UsedBytes: 25 * 1024 * 1024 * 1024, UsePercent: 25, TotalHuman: "100.0 GiB", UsedHuman: "25.0 GiB"},
		},
		Services: []environment.ServiceResourceInfo{
			{Name: "app", Status: "running", CPUPercent: "1.25%", MemUsage: "100 MiB / 4 GiB", MemPercent: "2.44%", ImageSize: "100MB", RwSize: "4.1kB", DirSize: "5.0 KiB"},
		},
		Unmanaged: []environment.UnmanagedContainer{},
		DockerDF: &environment.DockerDiskUsage{
			Entries: []environment.DockerDiskUsageEntry{{Type: "Images", Size: "1GB", Reclaimable: "500MB (50%)"}},
		},
	}
}

func resourcesTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel("dev", ".", 5)
	m.Config = &entity.Config{
		Servers: []entity.Server{{Name: "srv-a", Zone: "node9"}},
		Services: []entity.BizService{
			{ServiceBase: entity.ServiceBase{Server: "srv-a"}, Name: "app", Image: "team/app:1.0"},
		},
	}
	m.Server.ServerList = []*entity.Server{{Name: "srv-a", Zone: "node9"}}
	return m
}

// firstModel unwraps a (tea.Model, tea.Cmd) pair into the concrete Model.
func firstModel(m tea.Model, _ tea.Cmd) Model { return m.(Model) }

func TestResourcesFlow_LaunchAndProgress(t *testing.T) {
	m := resourcesTestModel(t)

	m = firstModel(m.executeMainMenuOperation("server_resources"))
	if !m.Loading.Active {
		t.Fatal("Loading.Active = false, want true after launching resource scan")
	}
	if m.Resource == nil || m.Resource.Expected != 1 {
		t.Fatalf("Resource.Expected = %v, want 1", m.Resource.Expected)
	}
	if !strings.Contains(m.Loading.Message, "0/1") {
		t.Errorf("Loading.Message = %q, want '0/1' progress", m.Loading.Message)
	}

	// one server completes
	m = firstModel(m.handleResourceOneScannedMsg(resourceOneScannedMsg{serverName: "srv-a", report: sampleResourceReport()}))
	if len(m.Resource.Reports) != 1 || m.Resource.Done != 1 {
		t.Fatalf("after first scan: reports=%d done=%d, want 1/1", len(m.Resource.Reports), m.Resource.Done)
	}
	if !strings.Contains(m.Loading.Message, "1/1") {
		t.Errorf("Loading.Message = %q, want '1/1' progress", m.Loading.Message)
	}

	// final message transitions to the InfoList
	final, doneCmd := m.handleResourcesAllDoneMsg(resourcesAllDoneMsg{})
	m = final.(Model)
	if doneCmd != nil {
		t.Fatal("expected no cmd from resourcesAllDoneMsg")
	}
	if m.ViewState != ViewStateInfoList {
		t.Fatalf("ViewState = %v, want ViewStateInfoList", m.ViewState)
	}
	if m.Loading.Active {
		t.Fatal("Loading.Active = true after completion, want false")
	}
}

func TestResourcesFlow_OverviewAndDetailViews(t *testing.T) {
	m := resourcesTestModel(t)
	m = firstModel(m.executeMainMenuOperation("server_resources"))
	m = firstModel(m.handleResourceOneScannedMsg(resourceOneScannedMsg{serverName: "srv-a", report: sampleResourceReport()}))
	m = firstModel(m.handleResourcesAllDoneMsg(resourcesAllDoneMsg{}))

	view := m.View()
	if !strings.Contains(view, "CONTAINERS") {
		t.Errorf("overview view missing resource columns: %s", view)
	}
	if !strings.Contains(view, "srv-a") || !strings.Contains(view, "12.3%") {
		t.Errorf("overview view missing server row: %s", view)
	}

	// press "d" to open detail; enlarge the canvas so all blocks are visible
	m.UI.Height = 80
	m.UI.Width = 140
	m = firstModel(m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyDetail)}))
	if m.ViewState != ViewStateInfoDetail {
		t.Fatalf("ViewState = %v, want ViewStateInfoDetail", m.ViewState)
	}
	dv := m.View()
	for _, want := range []string{"HOST", "DISK MOUNTS", "DOCKER DISK USAGE", "SERVICES", "UNMANAGED CONTAINERS"} {
		if !strings.Contains(dv, want) {
			t.Errorf("detail view missing %q", want)
		}
	}
	if !strings.Contains(dv, "NOTE") {
		t.Errorf("detail view missing du/df note")
	}
}

func TestResourcesFlow_EscapeReturnsMainMenu(t *testing.T) {
	m := resourcesTestModel(t)
	m = firstModel(m.executeMainMenuOperation("server_resources"))
	m = firstModel(m.handleResourceOneScannedMsg(resourceOneScannedMsg{serverName: "srv-a", report: sampleResourceReport()}))
	m = firstModel(m.handleResourcesAllDoneMsg(resourcesAllDoneMsg{}))

	m = firstModel(m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyDetail)}))
	m = firstModel(m.handleEscape())
	if m.ViewState != ViewStateMainMenu {
		t.Fatalf("ViewState = %v after Esc, want ViewStateMainMenu", m.ViewState)
	}
}

func TestResourcesFlow_FailureNotBlank(t *testing.T) {
	m := resourcesTestModel(t)
	m = firstModel(m.executeMainMenuOperation("server_resources"))
	m = firstModel(m.handleResourceOneScannedMsg(resourceOneScannedMsg{
		serverName: "srv-a",
		report:     failedResourceReport(&entity.Server{Name: "srv-a", Zone: "node9"}, "dev", "connection failed: dial tcp"),
	}))
	m = firstModel(m.handleResourcesAllDoneMsg(resourcesAllDoneMsg{}))
	view := m.View()
	if !strings.Contains(view, "⚠") {
		t.Errorf("failed server should render a ⚠ marker: %s", view)
	}
}
