package tui

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
		return &dynamicCursor{
			cursor:  &m.UI.MainMenuIndex,
			maxFunc: func() int { return m.menuRowCount() - 1 },
		}
	})

	RegisterCursorController(ViewStateServiceMenu, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Server.ServiceMenuIndex, maxValue: 6} // 7 items (0-6)
	})

	RegisterCursorController(ViewStateServerMenu, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.Server.ServiceMenuIndex, maxValue: 3} // 4 items (0-3)
	})

	RegisterCursorController(ViewStateDNSMenu, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.DNS.DNSMenuIndex, maxValue: 5} // 6 items (0-5)
	})

	RegisterCursorController(ViewStateConfigMenu, func(m *Model) CursorController {
		return &simpleCursor{cursor: &m.UI.ConfigMenuIndex, maxValue: 4} // 5 items (0-4)
	})

	RegisterCursorController(ViewStateTreeService, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Tree.CursorIndex,
			maxFunc: func() int { return m.countVisibleNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStateTreeDNS, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.Tree.CursorIndex,
			maxFunc: func() int { return m.countVisibleNodes() - 1 },
		}
	})

	RegisterCursorController(ViewStatePlan, func(m *Model) CursorController {
		if m.Action.PlanComponent != nil {
			return &dynamicCursor{
				cursor:  &m.Action.PlanComponent.Cursor,
				maxFunc: func() int { return m.Action.PlanComponent.MaxCursor() },
			}
		}
		return &simpleCursor{cursor: &m.Action.ConfirmSelected, maxValue: 0}
	})

	RegisterCursorController(ViewStateFilter, func(m *Model) CursorController {
		if m.Action.FilterView != nil {
			gi := m.Action.FilterView.SelectedGroup
			return &dynamicCursor{
				cursor: &m.Action.FilterView.Cursors[gi],
				maxFunc: func() int {
					if gi >= 0 && gi < len(m.Action.FilterView.Groups) {
						items := m.Action.FilterView.Groups[gi].Items
						if len(items) == 0 {
							return 0
						}
						return len(items) - 1
					}
					return 0
				},
			}
		}
		return nil
	})

	RegisterCursorController(ViewStateInfoList, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.UI.InfoListIndex,
			maxFunc: func() int { return m.infoListMaxIndex() },
		}
	})

	RegisterCursorController(ViewStateInfoDetail, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.UI.InfoDetailCursor,
			maxFunc: func() int { return m.infoDetailMaxIndex() },
		}
	})

	RegisterCursorController(ViewStateValidate, func(m *Model) CursorController {
		return &dynamicCursor{
			cursor:  &m.UI.ValidateCursor,
			maxFunc: func() int { return m.validateMaxIndex() },
		}
	})
}
