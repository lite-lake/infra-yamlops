package valueobject

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
