package valueobject

type ChangeType int

const (
	ChangeTypeNoop ChangeType = iota
	ChangeTypeCreate
	ChangeTypeUpdate
	ChangeTypeDelete
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeNoop:
		return "NOOP"
	case ChangeTypeCreate:
		return "CREATE"
	case ChangeTypeUpdate:
		return "UPDATE"
	case ChangeTypeDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

type Change struct {
	changeType   ChangeType
	entity       string
	name         string
	oldState     interface{}
	newState     interface{}
	actions      []string
	remoteExists bool
}

func NewChange(changeType ChangeType, entity, name string) *Change {
	return &Change{
		changeType: changeType,
		entity:     entity,
		name:       name,
	}
}

func NewChangeFull(changeType ChangeType, entity, name string, oldState, newState interface{}, actions []string, remoteExists bool) *Change {
	newActions := make([]string, len(actions))
	copy(newActions, actions)
	return &Change{
		changeType:   changeType,
		entity:       entity,
		name:         name,
		oldState:     oldState,
		newState:     newState,
		actions:      newActions,
		remoteExists: remoteExists,
	}
}

func (c *Change) Type() ChangeType      { return c.changeType }
func (c *Change) Entity() string        { return c.entity }
func (c *Change) Name() string          { return c.name }
func (c *Change) OldState() interface{} { return c.oldState }
func (c *Change) NewState() interface{} { return c.newState }
func (c *Change) Actions() []string     { return c.actions }
func (c *Change) RemoteExists() bool    { return c.remoteExists }

func (c *Change) WithOldState(state interface{}) *Change {
	return &Change{
		changeType:   c.changeType,
		entity:       c.entity,
		name:         c.name,
		oldState:     state,
		newState:     c.newState,
		actions:      c.actions,
		remoteExists: c.remoteExists,
	}
}

func (c *Change) WithNewState(state interface{}) *Change {
	return &Change{
		changeType:   c.changeType,
		entity:       c.entity,
		name:         c.name,
		oldState:     c.oldState,
		newState:     state,
		actions:      c.actions,
		remoteExists: c.remoteExists,
	}
}

func (c *Change) WithActions(actions ...string) *Change {
	newActions := make([]string, len(actions))
	copy(newActions, actions)
	return &Change{
		changeType:   c.changeType,
		entity:       c.entity,
		name:         c.name,
		oldState:     c.oldState,
		newState:     c.newState,
		actions:      newActions,
		remoteExists: c.remoteExists,
	}
}

func (c *Change) WithRemoteExists(exists bool) *Change {
	return &Change{
		changeType:   c.changeType,
		entity:       c.entity,
		name:         c.name,
		oldState:     c.oldState,
		newState:     c.newState,
		actions:      c.actions,
		remoteExists: exists,
	}
}

func (c *Change) Equals(other *Change) bool {
	if other == nil {
		return false
	}
	if c.changeType != other.changeType || c.entity != other.entity || c.name != other.name {
		return false
	}
	if c.remoteExists != other.remoteExists {
		return false
	}
	if len(c.actions) != len(other.actions) {
		return false
	}
	for i, a := range c.actions {
		if a != other.actions[i] {
			return false
		}
	}
	return true
}

func (c *Change) Clone() *Change {
	newActions := make([]string, len(c.actions))
	copy(newActions, c.actions)
	return &Change{
		changeType:   c.changeType,
		entity:       c.entity,
		name:         c.name,
		oldState:     c.oldState,
		newState:     c.newState,
		actions:      newActions,
		remoteExists: c.remoteExists,
	}
}
