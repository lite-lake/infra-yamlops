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
	Type         ChangeType
	Entity       string
	Name         string
	OldState     interface{}
	NewState     interface{}
	Actions      []string
	RemoteExists bool
}

func NewChange(changeType ChangeType, entity, name string) *Change {
	return &Change{
		Type:   changeType,
		Entity: entity,
		Name:   name,
	}
}

func (c *Change) WithOldState(state interface{}) *Change {
	c.OldState = state
	return c
}

func (c *Change) WithNewState(state interface{}) *Change {
	c.NewState = state
	return c
}

func (c *Change) WithActions(actions ...string) *Change {
	c.Actions = actions
	return c
}

func (c *Change) WithRemoteExists(exists bool) *Change {
	c.RemoteExists = exists
	return c
}

func (c *Change) Equals(other *Change) bool {
	if other == nil {
		return false
	}
	if c.Type != other.Type || c.Entity != other.Entity || c.Name != other.Name {
		return false
	}
	if c.RemoteExists != other.RemoteExists {
		return false
	}
	if len(c.Actions) != len(other.Actions) {
		return false
	}
	for i, a := range c.Actions {
		if a != other.Actions[i] {
			return false
		}
	}
	return true
}
