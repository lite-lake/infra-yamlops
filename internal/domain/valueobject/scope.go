package valueobject

type Scope struct {
	domain        string
	zone          string
	server        string
	service       string
	services      []string
	infraServices []string
	forceDeploy   bool
	dnsOnly       bool
}

func NewScope() *Scope {
	return &Scope{}
}

func NewScopeWithValues(zone, server, service, domain string) *Scope {
	return &Scope{
		zone:    zone,
		server:  server,
		service: service,
		domain:  domain,
	}
}

func NewScopeFull(domain, zone, server, service string, services, infraServices []string, forceDeploy, dnsOnly bool) *Scope {
	newServices := make([]string, len(services))
	copy(newServices, services)
	newInfraServices := make([]string, len(infraServices))
	copy(newInfraServices, infraServices)
	return &Scope{
		domain:        domain,
		zone:          zone,
		server:        server,
		service:       service,
		services:      newServices,
		infraServices: newInfraServices,
		forceDeploy:   forceDeploy,
		dnsOnly:       dnsOnly,
	}
}

func (s *Scope) Domain() string          { return s.domain }
func (s *Scope) Zone() string            { return s.zone }
func (s *Scope) Server() string          { return s.server }
func (s *Scope) Service() string         { return s.service }
func (s *Scope) Services() []string      { return s.services }
func (s *Scope) InfraServices() []string { return s.infraServices }
func (s *Scope) ForceDeploy() bool       { return s.forceDeploy }
func (s *Scope) DNSOnly() bool           { return s.dnsOnly }

func (s *Scope) WithDomain(domain string) *Scope {
	return &Scope{
		domain:        domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithZone(zone string) *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithServer(server string) *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithService(service string) *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithServices(services []string) *Scope {
	newServices := make([]string, len(services))
	copy(newServices, services)
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      newServices,
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithInfraServices(infraServices []string) *Scope {
	newInfraServices := make([]string, len(infraServices))
	copy(newInfraServices, infraServices)
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: newInfraServices,
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithForceDeploy(forceDeploy bool) *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) WithDNSOnly(dnsOnly bool) *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       dnsOnly,
	}
}

func (s *Scope) copyServices() []string {
	if s.services == nil {
		return nil
	}
	services := make([]string, len(s.services))
	copy(services, s.services)
	return services
}

func (s *Scope) copyInfraServices() []string {
	if s.infraServices == nil {
		return nil
	}
	infraServices := make([]string, len(s.infraServices))
	copy(infraServices, s.infraServices)
	return infraServices
}

func (s *Scope) Equals(other *Scope) bool {
	if other == nil {
		return false
	}
	if s.zone != other.zone || s.server != other.server || s.service != other.service || s.domain != other.domain {
		return false
	}
	if s.forceDeploy != other.forceDeploy || s.dnsOnly != other.dnsOnly {
		return false
	}
	if len(s.services) != len(other.services) || len(s.infraServices) != len(other.infraServices) {
		return false
	}
	for i, svc := range s.services {
		if svc != other.services[i] {
			return false
		}
	}
	for i, svc := range s.infraServices {
		if svc != other.infraServices[i] {
			return false
		}
	}
	return true
}

func (s *Scope) Clone() *Scope {
	return &Scope{
		domain:        s.domain,
		zone:          s.zone,
		server:        s.server,
		service:       s.service,
		services:      s.copyServices(),
		infraServices: s.copyInfraServices(),
		forceDeploy:   s.forceDeploy,
		dnsOnly:       s.dnsOnly,
	}
}

func (s *Scope) Matches(zone, server, service, domain string) bool {
	if s.zone != "" && s.zone != zone {
		return false
	}
	if s.server != "" && s.server != server {
		return false
	}
	if s.service != "" && s.service != service {
		return false
	}
	if len(s.services) > 0 {
		found := false
		for _, svc := range s.services {
			if svc == service {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if s.domain != "" && s.domain != domain {
		return false
	}
	return true
}

func (s *Scope) MatchesInfra(zone, server, infraService string) bool {
	if s.zone != "" && s.zone != zone {
		return false
	}
	if s.server != "" && s.server != server {
		return false
	}
	if len(s.infraServices) > 0 {
		found := false
		for _, svc := range s.infraServices {
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
	return s.zone == "" && s.server == "" && s.service == "" && s.domain == "" && len(s.services) == 0 && len(s.infraServices) == 0
}

func (s *Scope) HasServicesOnly() bool {
	return len(s.services) > 0 && len(s.infraServices) == 0
}

func (s *Scope) HasInfraServicesOnly() bool {
	return len(s.infraServices) > 0 && len(s.services) == 0
}

func (s *Scope) HasAnyServiceSelection() bool {
	return len(s.services) > 0 || len(s.infraServices) > 0
}
