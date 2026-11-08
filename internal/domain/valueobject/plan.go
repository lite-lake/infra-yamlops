package valueobject

type Plan struct {
	changes []*Change
	scope   *Scope
}

func NewPlan() *Plan {
	return &Plan{
		changes: make([]*Change, 0),
		scope:   &Scope{},
	}
}

func NewPlanWithScope(scope *Scope) *Plan {
	if scope == nil {
		scope = &Scope{}
	}
	return &Plan{
		changes: make([]*Change, 0),
		scope:   scope,
	}
}

func (p *Plan) Changes() []*Change { return p.changes }
func (p *Plan) Scope() *Scope      { return p.scope }

func (p *Plan) AddChange(ch *Change) {
	p.changes = append(p.changes, ch)
}

func (p *Plan) WithChange(ch *Change) *Plan {
	newChanges := make([]*Change, len(p.changes)+1)
	copy(newChanges, p.changes)
	newChanges[len(p.changes)] = ch
	return &Plan{
		changes: newChanges,
		scope:   p.scope,
	}
}

func (p *Plan) HasChanges() bool {
	for _, c := range p.changes {
		if c.Type() != ChangeTypeNoop {
			return true
		}
	}
	return false
}

func (p *Plan) FilterByType(changeType ChangeType) []*Change {
	var result []*Change
	for _, c := range p.changes {
		if c.Type() == changeType {
			result = append(result, c)
		}
	}
	return result
}

func (p *Plan) FilterByEntity(entity string) []*Change {
	var result []*Change
	for _, c := range p.changes {
		if c.Entity() == entity {
			result = append(result, c)
		}
	}
	return result
}

func (p *Plan) Equals(other *Plan) bool {
	if other == nil {
		return false
	}
	if !p.scope.Equals(other.scope) {
		return false
	}
	if len(p.changes) != len(other.changes) {
		return false
	}
	for i, c := range p.changes {
		if !c.Equals(other.changes[i]) {
			return false
		}
	}
	return true
}

func (p *Plan) Clone() *Plan {
	changes := make([]*Change, len(p.changes))
	for i, c := range p.changes {
		changes[i] = c.Clone()
	}
	return &Plan{
		changes: changes,
		scope:   p.scope.Clone(),
	}
}
