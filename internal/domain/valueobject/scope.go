package valueobject

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

func (s *Scope) IsEmpty() bool {
	return s.Zone == "" && s.Server == "" && s.Service == "" && s.Domain == ""
}
