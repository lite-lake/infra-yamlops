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

func NewScope() *Scope {
	return &Scope{}
}

func NewScopeWithValues(zone, server, service, domain string) *Scope {
	return &Scope{
		Zone:    zone,
		Server:  server,
		Service: service,
		Domain:  domain,
	}
}

func (s *Scope) Equals(other *Scope) bool {
	if other == nil {
		return false
	}
	if s.Zone != other.Zone || s.Server != other.Server || s.Service != other.Service || s.Domain != other.Domain {
		return false
	}
	if s.ForceDeploy != other.ForceDeploy || s.DNSOnly != other.DNSOnly {
		return false
	}
	if len(s.Services) != len(other.Services) || len(s.InfraServices) != len(other.InfraServices) {
		return false
	}
	for i, svc := range s.Services {
		if svc != other.Services[i] {
			return false
		}
	}
	for i, svc := range s.InfraServices {
		if svc != other.InfraServices[i] {
			return false
		}
	}
	return true
}

func (s *Scope) Clone() *Scope {
	services := make([]string, len(s.Services))
	copy(services, s.Services)
	infraServices := make([]string, len(s.InfraServices))
	copy(infraServices, s.InfraServices)
	return &Scope{
		Domain:        s.Domain,
		Zone:          s.Zone,
		Server:        s.Server,
		Service:       s.Service,
		Services:      services,
		InfraServices: infraServices,
		ForceDeploy:   s.ForceDeploy,
		DNSOnly:       s.DNSOnly,
	}
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
