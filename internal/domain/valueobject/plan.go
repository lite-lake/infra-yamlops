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

func (p *Plan) Equals(other *Plan) bool {
	if other == nil {
		return false
	}
	if !p.Scope.Equals(other.Scope) {
		return false
	}
	if len(p.Changes) != len(other.Changes) {
		return false
	}
	for i, c := range p.Changes {
		if !c.Equals(other.Changes[i]) {
			return false
		}
	}
	return true
}

func (p *Plan) Clone() *Plan {
	changes := make([]*Change, len(p.Changes))
	for i, c := range p.Changes {
		changes[i] = &Change{
			Type:         c.Type,
			Entity:       c.Entity,
			Name:         c.Name,
			OldState:     c.OldState,
			NewState:     c.NewState,
			Actions:      append([]string(nil), c.Actions...),
			RemoteExists: c.RemoteExists,
		}
	}
	return &Plan{
		Changes: changes,
		Scope:   p.Scope.Clone(),
	}
}
