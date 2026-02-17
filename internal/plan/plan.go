package plan

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

type Scope struct {
	Domain  string
	Zone    string
	Server  string
	Service string
}

func (s *Scope) Matches(zone, server, service, domain string) bool {
	if s.Zone != "" && s.Zone != zone {
		return false
	}
	if s.Server != "" && s.Server != server {
		return false
	}
	if s.Service != "" && s.Service != service {
		return false
	}
	if s.Domain != "" && s.Domain != domain {
		return false
	}
	return true
}

type Plan struct {
	Changes []*Change
	Scope   *Scope
}

func NewPlan() *Plan {
	return &Plan{
		Changes: make([]*Change, 0),
		Scope:   &Scope{},
	}
}

func NewPlanWithScope(scope *Scope) *Plan {
	if scope == nil {
		scope = &Scope{}
	}
	return &Plan{
		Changes: make([]*Change, 0),
		Scope:   scope,
	}
}

func (p *Plan) AddChange(ch *Change) {
	p.Changes = append(p.Changes, ch)
}

func (p *Plan) HasChanges() bool {
	for _, c := range p.Changes {
		if c.Type != ChangeTypeNoop {
			return true
		}
	}
	return false
}

func (p *Plan) FilterByType(changeType ChangeType) []*Change {
	var result []*Change
	for _, c := range p.Changes {
		if c.Type == changeType {
			result = append(result, c)
		}
	}
	return result
}

func (p *Plan) FilterByEntity(entity string) []*Change {
	var result []*Change
	for _, c := range p.Changes {
		if c.Entity == entity {
			result = append(result, c)
		}
	}
	return result
}
