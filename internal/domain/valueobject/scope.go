package valueobject

type Scope struct {
	Domain        string
	Zone          string
	Server        string
	Service       string
	Services      []string
	InfraServices []string
	ForceDeploy   bool
	DNSOnly       bool
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
	if len(s.Services) > 0 {
		found := false
		for _, svc := range s.Services {
			if svc == service {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if s.Domain != "" && s.Domain != domain {
		return false
	}
	return true
}

func (s *Scope) MatchesInfra(zone, server, infraService string) bool {
	if s.Zone != "" && s.Zone != zone {
		return false
	}
	if s.Server != "" && s.Server != server {
		return false
	}
	if len(s.InfraServices) > 0 {
		found := false
		for _, svc := range s.InfraServices {
			if svc == infraService {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *Scope) IsEmpty() bool {
	return s.Zone == "" && s.Server == "" && s.Service == "" && s.Domain == "" && len(s.Services) == 0 && len(s.InfraServices) == 0
}

func (s *Scope) HasServicesOnly() bool {
	return len(s.Services) > 0 && len(s.InfraServices) == 0
}

func (s *Scope) HasInfraServicesOnly() bool {
	return len(s.InfraServices) > 0 && len(s.Services) == 0
}

func (s *Scope) HasAnyServiceSelection() bool {
	return len(s.Services) > 0 || len(s.InfraServices) > 0
}
