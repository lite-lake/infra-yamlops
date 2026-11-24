package tui

import (
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/tui/components"
)

type applyProgressMsg struct{}

func tickApply() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return applyProgressMsg{}
	})
}

// ProgressCallback is called by the executor after each change is applied.
type ProgressCallback func(change *valueobject.Change, serverName string, success bool, errMsg string)

// ProgressTracker is a thread-safe struct that the executor's callback writes to
// and the TUI tick loop reads from.
type ProgressTracker struct {
	mu          sync.Mutex
	completed   int
	failed      int
	updates     []progressItemUpdate
	runningKeys map[string]bool // key = "serverName:changeName"
}

type progressItemUpdate struct {
	ServerName string
	ChangeName string
	Success    bool
	ErrorMsg   string
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		runningKeys: make(map[string]bool),
	}
}

func (pt *ProgressTracker) OnChangeStart(change *valueobject.Change, serverName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	key := serverName + ":" + change.Name()
	pt.runningKeys[key] = true
}

func (pt *ProgressTracker) OnChangeApplied(change *valueobject.Change, serverName string, success bool, errMsg string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	key := serverName + ":" + change.Name()
	delete(pt.runningKeys, key)
	if success {
		pt.completed++
	} else {
		pt.completed++
		pt.failed++
	}
	pt.updates = append(pt.updates, progressItemUpdate{
		ServerName: serverName,
		ChangeName: change.Name(),
		Success:    success,
		ErrorMsg:   errMsg,
	})
}

// GetRunningKeys returns a snapshot of currently running item keys ("serverName:changeName").
func (pt *ProgressTracker) GetRunningKeys() map[string]bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	result := make(map[string]bool, len(pt.runningKeys))
	for k, v := range pt.runningKeys {
		result[k] = v
	}
	return result
}

// DrainUpdates returns all pending updates and clears the buffer.
func (pt *ProgressTracker) DrainUpdates() []progressItemUpdate {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	updates := pt.updates
	pt.updates = nil
	return updates
}

// Completed returns the total number of completed changes.
func (pt *ProgressTracker) Completed() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.completed
}

// Failed returns the total number of failed changes.
func (pt *ProgressTracker) Failed() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.failed
}

// initProgressView builds the ProgressView groups from the plan's changes.
func (m *Model) initProgressView() {
	if m.Action.PlanComponent == nil {
		return
	}
	selectedItems := m.Action.PlanComponent.GetSelectedItems()
	if len(selectedItems) == 0 {
		return
	}

	// Group by server
	serverItems := make(map[string][]components.ProgressItem)
	for _, item := range selectedItems {
		server := item.Server
		if server == "" {
			server = "local"
		}
		serverItems[server] = append(serverItems[server], components.ProgressItem{
			Action: item.Action,
			Name:   item.Name,
			Status: components.ProgressStatusWaiting,
		})
	}

	var groups []components.ProgressServerGroup
	for server, items := range serverItems {
		groups = append(groups, components.ProgressServerGroup{
			ServerName: server,
			Items:      items,
		})
	}

	pv := components.NewProgressView("EXECUTING...", string(m.Environment))
	if m.Action.PlanComponent != nil && m.Action.PlanComponent.DryRun {
		pv = components.NewProgressView("DRY RUN PREVIEW...", string(m.Environment))
	}
	pv.SetGroups(groups)
	m.Action.ProgressView = pv
	m.Action.ProgressStartTime = time.Now()
}

// syncProgressView applies pending tracker updates to the ProgressView.
func (m *Model) syncProgressView() {
	if m.Action.ProgressTracker == nil || m.Action.ProgressView == nil {
		return
	}
	updates := m.Action.ProgressTracker.DrainUpdates()
	for _, u := range updates {
		status := components.ProgressStatusSuccess
		if !u.Success {
			status = components.ProgressStatusFailed
		}
		m.Action.ProgressView.UpdateItemStatus(u.ServerName, u.ChangeName, status, u.ErrorMsg)
		m.Action.ProgressView.IncrementProgress()
	}
	// Sync running state from executor's start callbacks
	m.Action.ProgressView.SyncRunningFromTracker(m.Action.ProgressTracker.GetRunningKeys())
}

func (m Model) startLoading(message string) tea.Cmd {
	m.Loading.Active = true
	m.Loading.Message = message
	m.Loading.Spinner = 0
	return tickSpinner()
}

func (m *Model) stopLoading() {
	m.Loading.Active = false
	m.Loading.Message = ""
}
