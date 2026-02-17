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
	Type     ChangeType
	Entity   string
	Name     string
	OldState interface{}
	NewState interface{}
	Actions  []string
}
