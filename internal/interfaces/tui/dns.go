package tui

import (
	"sort"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

func (m Model) getDNSISPs() []string {
	var isps []string
	for _, isp := range m.Config.ISPs {
		if isp.HasService(entity.ISPServiceDNS) {
			isps = append(isps, isp.Name)
		}
	}
	sort.Strings(isps)
	return isps
}

func (m Model) getDNSDomains() []string {
	var domains []string
	if m.Config == nil || m.Config.Domains == nil {
		return domains
	}
	for _, d := range m.Config.Domains {
		domains = append(domains, d.Name)
	}
	sort.Strings(domains)
	return domains
}

// getDNSDomainObjects returns all domain entities sorted by name.
func (m Model) getDNSDomainObjects() []entity.Domain {
	if m.Config == nil || m.Config.Domains == nil {
		return nil
	}
	domains := make([]entity.Domain, len(m.Config.Domains))
	copy(domains, m.Config.Domains)
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})
	return domains
}
