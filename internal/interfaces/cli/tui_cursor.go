package cli

type CursorController interface {
	GetCursor() int
	SetCursor(v int)
	MaxValue() int
}

type CursorControllerFunc func(m *Model) CursorController

var cursorControllerRegistry = make(map[ViewState]CursorControllerFunc)

func RegisterCursorController(state ViewState, fn CursorControllerFunc) {
	cursorControllerRegistry[state] = fn
}

func GetCursorController(state ViewState, m *Model) CursorController {
	if fn, ok := cursorControllerRegistry[state]; ok {
		return fn(m)
	}
	return nil
}

type simpleCursor struct {
	cursor   *int
	maxValue int
}

func (c *simpleCursor) GetCursor() int {
	if c.cursor == nil {
		return 0
	}
	return *c.cursor
}

func (c *simpleCursor) SetCursor(v int) {
	if c.cursor != nil {
		*c.cursor = v
	}
}

func (c *simpleCursor) MaxValue() int {
	return c.maxValue
}

type dynamicCursor struct {
	cursor  *int
	maxFunc func() int
}

func (c *dynamicCursor) GetCursor() int {
	if c.cursor == nil {
		return 0
	}
	return *c.cursor
}

func (c *dynamicCursor) SetCursor(v int) {
	if c.cursor != nil {
		*c.cursor = v
	}
}

func (c *dynamicCursor) MaxValue() int {
	if c.maxFunc != nil {
		return c.maxFunc()
	}
	return 0
}

type scrollCursor struct {
	scroll  *int
	maxFunc func() int
}

func (c *scrollCursor) GetCursor() int {
	if c.scroll == nil {
		return 0
	}
	return *c.scroll
}

func (c *scrollCursor) SetCursor(v int) {
	if c.scroll != nil {
		*c.scroll = v
	}
}

func (c *scrollCursor) MaxValue() int {
	if c.maxFunc != nil {
		return c.maxFunc()
	}
	return 0
}

func init() {
	RegisterCursorController(ViewStateMainMenu, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.UI.MainMenuIndex, maxValue: 2}
	})

	RegisterCursorController(ViewStateServiceManagement, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Server.ServiceMenuIndex, maxValue: 5}
	})

	RegisterCursorController(ViewStateServerSetup, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.ServerEnv.CursorIndex,
			maxFunc: func() int { return m.countServerEnvNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStateServerCheck, func(m *Model) CursorController {
		return &scrollCursor{
			scroll: &m.ServerEnv.ResultsScrollY,
			maxFunc: func() int {
				totalLines := m.countServerEnvResultLines()
				availableHeight := m.UI.Height - 8
				if availableHeight < 5 {
					availableHeight = 5
				}
				maxScroll := totalLines - availableHeight
				if maxScroll < 0 {
					maxScroll = 0
				}
				return maxScroll
			},
		}
	})

	RegisterCursorController(ViewStateDNSManagement, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.DNS.DNSMenuIndex, maxValue: 3}
	})

	RegisterCursorController(ViewStateDNSPullDomains, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.DNS.DNSISPIndex,
			maxFunc: func() int { return len(m.getDNSISPs()) - 1 },
		}
	})

	RegisterCursorController(ViewStateDNSPullRecords, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.DNS.DNSDomainIndex,
			maxFunc: func() int { return len(m.getDNSDomains()) - 1 },
		}
	})

	RegisterCursorController(ViewStateDNSPullDiff, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor: &m.DNS.DNSPullCursor,
			maxFunc: func() int {
				maxIdx := len(m.DNS.DNSPullDiffs) - 1
				if len(m.DNS.DNSRecordDiffs) > 0 {
					maxIdx = len(m.DNS.DNSRecordDiffs) - 1
				}
				return maxIdx
			},
		}
	})

	RegisterCursorController(ViewStateTree, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Tree.CursorIndex,
			maxFunc: func() int { return m.countVisibleNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStateApplyConfirm, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Action.ConfirmSelected, maxValue: 1}
	})

	RegisterCursorController(ViewStateServiceCleanup, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Cleanup.CleanupCursor,
			maxFunc: func() int { return m.countCleanupItems() - 1 },
		}
	})

	RegisterCursorController(ViewStateServiceCleanupConfirm, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Action.ConfirmSelected, maxValue: 1}
	})

	RegisterCursorController(ViewStateServiceStop, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Tree.CursorIndex,
			maxFunc: func() int { return m.countVisibleNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStateServiceStopConfirm, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Action.ConfirmSelected, maxValue: 1}
	})

	RegisterCursorController(ViewStateServiceRestart, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Tree.CursorIndex,
			maxFunc: func() int { return m.countVisibleNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStateServiceRestartConfirm, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Action.ConfirmSelected, maxValue: 1}
	})
}
