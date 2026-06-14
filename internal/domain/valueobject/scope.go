package valueobject

// Scope defines the filtering criteria for operations.
// All filter dimensions support multi-value ([]string).
// Empty slice (nil / len=0) means no filtering for that dimension.
type Scope struct {
	zones         []string // 多选：按网区筛选
	servers       []string // 多选：按服务器筛选
	bizServices   []string // 多选：按业务服务名称筛选（TUI 逐项勾选后填充）
	infraServices []string // 多选：按基础设施服务名称筛选（TUI 逐项勾选后填充）
	serviceTypes  []string // 类别筛选："biz" / "infra" / "biz"+"infra"；空=nil 表示不筛选
	domains       []string // 多选：按域名筛选
	dnsRecords    []string // 多选：按 DNS 记录筛选（格式：TYPE:NAME，如 "A:@"、"CNAME:api"）
	forceDeploy   bool
}

// NewScope creates an empty Scope with no filters.
func NewScope() *Scope {
	return &Scope{}
}

// NewScopeFull creates a Scope with all filter dimensions.
func NewScopeFull(
	zones, servers, bizServices, infraServices, serviceTypes, domains, dnsRecords []string,
	forceDeploy bool,
) *Scope {
	return &Scope{
		zones:         copySlice(zones),
		servers:       copySlice(servers),
		bizServices:   copySlice(bizServices),
		infraServices: copySlice(infraServices),
		serviceTypes:  copySlice(serviceTypes),
		domains:       copySlice(domains),
		dnsRecords:    copySlice(dnsRecords),
		forceDeploy:   forceDeploy,
	}
}

// Getters

func (s *Scope) Zones() []string         { return s.zones }
func (s *Scope) Servers() []string       { return s.servers }
func (s *Scope) BizServices() []string   { return s.bizServices }
func (s *Scope) InfraServices() []string { return s.infraServices }
func (s *Scope) ServiceTypes() []string  { return s.serviceTypes }
func (s *Scope) Domains() []string       { return s.domains }
func (s *Scope) DNSRecords() []string    { return s.dnsRecords }
func (s *Scope) ForceDeploy() bool       { return s.forceDeploy }

// Builder methods (immutable copy pattern)

func (s *Scope) WithZones(zones []string) *Scope {
	return &Scope{
		zones:         copySlice(zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithServers(servers []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithBizServices(bizServices []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithInfraServices(infraServices []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithServiceTypes(serviceTypes []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithDomains(domains []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithDNSRecords(dnsRecords []string) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

func (s *Scope) WithForceDeploy(forceDeploy bool) *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   forceDeploy,
	}
}

// WithServiceType appends a single service type to the scope.
func (s *Scope) WithServiceType(serviceType string) *Scope {
	newTypes := copySlice(s.serviceTypes)
	newTypes = append(newTypes, serviceType)
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  newTypes,
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

// Matches checks if the given parameters match the scope filters.
// All dimensions use the same logic: len == 0 means wildcard, otherwise value must be in the list.
func (s *Scope) Matches(zone, server, bizService, infraService, domain, record string) bool {
	if len(s.zones) > 0 && !contains(s.zones, zone) {
		return false
	}
	if len(s.servers) > 0 && !contains(s.servers, server) {
		return false
	}
	if len(s.bizServices) > 0 && !contains(s.bizServices, bizService) {
		return false
	}
	if len(s.infraServices) > 0 && !contains(s.infraServices, infraService) {
		return false
	}
	if len(s.domains) > 0 && !contains(s.domains, domain) {
		return false
	}
	if len(s.dnsRecords) > 0 && !contains(s.dnsRecords, record) {
		return false
	}
	return true
}

// MatchesZone checks if zone matches the scope filter.
func (s *Scope) MatchesZone(zone string) bool {
	return len(s.zones) == 0 || contains(s.zones, zone)
}

// MatchesServer checks if server matches the scope filter.
func (s *Scope) MatchesServer(server string) bool {
	return len(s.servers) == 0 || contains(s.servers, server)
}

// MatchesBizService checks if bizService matches the scope filter.
func (s *Scope) MatchesBizService(bizService string) bool {
	return len(s.bizServices) == 0 || contains(s.bizServices, bizService)
}

// MatchesInfraService checks if infraService matches the scope filter.
func (s *Scope) MatchesInfraService(infraService string) bool {
	return len(s.infraServices) == 0 || contains(s.infraServices, infraService)
}

// MatchesDomain checks if domain matches the scope filter.
func (s *Scope) MatchesDomain(domain string) bool {
	return len(s.domains) == 0 || contains(s.domains, domain)
}

// MatchesDNSRecord checks if record matches the scope filter.
func (s *Scope) MatchesDNSRecord(record string) bool {
	return len(s.dnsRecords) == 0 || contains(s.dnsRecords, record)
}

// ShouldProcessBiz returns true if the scope should process BizService.
// It checks ServiceTypes filter: empty means all, ["biz"] means only biz, ["infra"] means only infra.
func (s *Scope) ShouldProcessBiz() bool {
	if len(s.serviceTypes) == 0 {
		return true
	}
	return contains(s.serviceTypes, "biz")
}

// ShouldProcessInfra returns true if the scope should process InfraService.
// It checks ServiceTypes filter: empty means all, ["biz"] means only biz, ["infra"] means only infra.
func (s *Scope) ShouldProcessInfra() bool {
	if len(s.serviceTypes) == 0 {
		return true
	}
	return contains(s.serviceTypes, "infra")
}

// ShouldGenerateBizService checks if a biz service should be generated.
// Combines server and biz service name matching.
func (s *Scope) ShouldGenerateBizService(serviceName, serverName string) bool {
	if !s.MatchesServer(serverName) {
		return false
	}
	if !s.MatchesBizService(serviceName) {
		return false
	}
	return true
}

// ShouldGenerateInfraService checks if an infra service should be generated.
// Combines server and infra service name matching.
func (s *Scope) ShouldGenerateInfraService(serviceName, serverName string) bool {
	if !s.MatchesServer(serverName) {
		return false
	}
	if !s.MatchesInfraService(serviceName) {
		return false
	}
	return true
}

// IsEmpty returns true if all filter dimensions are empty.
func (s *Scope) IsEmpty() bool {
	return len(s.zones) == 0 &&
		len(s.servers) == 0 &&
		len(s.bizServices) == 0 &&
		len(s.infraServices) == 0 &&
		len(s.serviceTypes) == 0 &&
		len(s.domains) == 0 &&
		len(s.dnsRecords) == 0
}

// HasServices returns true if any service-related filter is set.
func (s *Scope) HasServices() bool {
	return len(s.bizServices) > 0 || len(s.infraServices) > 0 || len(s.serviceTypes) > 0
}

// HasDNSResources returns true if any DNS-related filter is set.
func (s *Scope) HasDNSResources() bool {
	return len(s.domains) > 0 || len(s.dnsRecords) > 0
}

// IsServiceOnly returns true if only service filters are set (no DNS filters).
func (s *Scope) IsServiceOnly() bool {
	return s.HasServices() && !s.HasDNSResources()
}

// IsDNSOnly returns true if only DNS filters are set (no service filters).
// This is used by Application layer to auto-detect DNS-only path.
func (s *Scope) IsDNSOnly() bool {
	return !s.HasServices() && s.HasDNSResources()
}

// Equals checks if two Scopes have the same filter values.
func (s *Scope) Equals(other *Scope) bool {
	if other == nil {
		return false
	}
	if s.forceDeploy != other.forceDeploy {
		return false
	}
	return slicesEqual(s.zones, other.zones) &&
		slicesEqual(s.servers, other.servers) &&
		slicesEqual(s.bizServices, other.bizServices) &&
		slicesEqual(s.infraServices, other.infraServices) &&
		slicesEqual(s.serviceTypes, other.serviceTypes) &&
		slicesEqual(s.domains, other.domains) &&
		slicesEqual(s.dnsRecords, other.dnsRecords)
}

// Clone creates a deep copy of the Scope.
func (s *Scope) Clone() *Scope {
	return &Scope{
		zones:         copySlice(s.zones),
		servers:       copySlice(s.servers),
		bizServices:   copySlice(s.bizServices),
		infraServices: copySlice(s.infraServices),
		serviceTypes:  copySlice(s.serviceTypes),
		domains:       copySlice(s.domains),
		dnsRecords:    copySlice(s.dnsRecords),
		forceDeploy:   s.forceDeploy,
	}
}

// Helper functions

func copySlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
