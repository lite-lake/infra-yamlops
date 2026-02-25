package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	infradns "github.com/litelake/yamlops/internal/infrastructure/dns"
)

func (m *Model) fetchDomainDiffs(ispName string) {
	m.DNS.DNSPullDiffs = nil
	m.UI.ErrorMessage = ""

	isp := m.Config.GetISPMap()[ispName]
	if isp == nil {
		m.UI.ErrorMessage = fmt.Sprintf("ISP '%s' not found", ispName)
		return
	}

	provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error creating DNS provider: %v", err)
		return
	}

	remoteDomains, err := provider.ListDomains(context.Background())
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error listing domains: %v", err)
		return
	}

	localDomainMap := make(map[string]*entity.Domain)
	for i := range m.Config.Domains {
		localDomainMap[m.Config.Domains[i].Name] = &m.Config.Domains[i]
	}

	for _, domainName := range remoteDomains {
		if _, exists := localDomainMap[domainName]; !exists {
			m.DNS.DNSPullDiffs = append(m.DNS.DNSPullDiffs, DomainDiff{
				Name:       domainName,
				DNSISP:     ispName,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		} else {
			delete(localDomainMap, domainName)
		}
	}

	for _, localDomain := range localDomainMap {
		if localDomain.DNSISP == ispName {
			m.DNS.DNSPullDiffs = append(m.DNS.DNSPullDiffs, DomainDiff{
				Name:       localDomain.Name,
				DNSISP:     localDomain.DNSISP,
				ISP:        localDomain.ISP,
				Parent:     localDomain.Parent,
				ChangeType: valueobject.ChangeTypeDelete,
			})
		}
	}

	sort.Slice(m.DNS.DNSPullDiffs, func(i, j int) bool {
		return m.DNS.DNSPullDiffs[i].Name < m.DNS.DNSPullDiffs[j].Name
	})
}

func (m *Model) fetchDomainDiffsAsync(ispName string) tea.Cmd {
	return func() tea.Msg {
		isp := m.Config.GetISPMap()[ispName]
		if isp == nil {
			return dnsDomainsFetchedMsg{err: fmt.Errorf("ISP '%s' not found", ispName)}
		}

		provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
		if err != nil {
			return dnsDomainsFetchedMsg{err: err}
		}

		remoteDomains, err := provider.ListDomains(context.Background())
		if err != nil {
			return dnsDomainsFetchedMsg{err: err}
		}

		localDomainMap := make(map[string]*entity.Domain)
		for i := range m.Config.Domains {
			localDomainMap[m.Config.Domains[i].Name] = &m.Config.Domains[i]
		}

		var diffs []DomainDiff
		for _, domainName := range remoteDomains {
			if _, exists := localDomainMap[domainName]; !exists {
				diffs = append(diffs, DomainDiff{
					Name:       domainName,
					DNSISP:     ispName,
					ChangeType: valueobject.ChangeTypeCreate,
				})
			} else {
				delete(localDomainMap, domainName)
			}
		}

		for _, localDomain := range localDomainMap {
			if localDomain.DNSISP == ispName {
				diffs = append(diffs, DomainDiff{
					Name:       localDomain.Name,
					DNSISP:     localDomain.DNSISP,
					ISP:        localDomain.ISP,
					Parent:     localDomain.Parent,
					ChangeType: valueobject.ChangeTypeDelete,
				})
			}
		}

		sort.Slice(diffs, func(i, j int) bool {
			return diffs[i].Name < diffs[j].Name
		})

		return dnsDomainsFetchedMsg{diffs: diffs}
	}
}

func (m *Model) fetchRecordDiffs(domainName string) {
	m.DNS.DNSRecordDiffs = nil
	m.UI.ErrorMessage = ""

	domain := m.Config.GetDomainMap()[domainName]
	if domain == nil {
		m.UI.ErrorMessage = fmt.Sprintf("Domain '%s' not found", domainName)
		return
	}

	isp := m.Config.GetISPMap()[domain.DNSISP]
	if isp == nil {
		m.UI.ErrorMessage = fmt.Sprintf("DNS ISP '%s' not found", domain.DNSISP)
		return
	}

	provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error creating DNS provider: %v", err)
		return
	}

	remoteRecords, err := provider.ListRecords(context.Background(), domainName)
	if err != nil {
		m.UI.ErrorMessage = fmt.Sprintf("Error listing records: %v", err)
		return
	}

	localRecordMap := make(map[string]*entity.DNSRecord)
	for i := range domain.Records {
		key := fmt.Sprintf("%s:%s:%s", domain.Records[i].Type, domain.Records[i].Name, domain.Records[i].Value)
		localRecordMap[key] = &domain.Records[i]
		localRecordMap[key].Domain = domain.Name
	}

	for _, remote := range remoteRecords {
		recordName := remote.Name
		if recordName == domainName || recordName == "" {
			recordName = "@"
		} else if strings.HasSuffix(remote.Name, "."+domainName) {
			recordName = strings.TrimSuffix(remote.Name, "."+domainName)
		}

		key := fmt.Sprintf("%s:%s:%s", remote.Type, recordName, remote.Value)
		if local, exists := localRecordMap[key]; exists {
			if local.TTL != remote.TTL {
				m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
					Domain:     domainName,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeUpdate,
				})
			}
			delete(localRecordMap, key)
		} else {
			m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
				Domain:     domainName,
				Type:       entity.DNSRecordType(remote.Type),
				Name:       recordName,
				Value:      remote.Value,
				TTL:        remote.TTL,
				ChangeType: valueobject.ChangeTypeCreate,
			})
		}
	}

	for _, local := range localRecordMap {
		m.DNS.DNSRecordDiffs = append(m.DNS.DNSRecordDiffs, RecordDiff{
			Domain:     local.Domain,
			Type:       local.Type,
			Name:       local.Name,
			Value:      local.Value,
			TTL:        local.TTL,
			ChangeType: valueobject.ChangeTypeDelete,
		})
	}

	sort.Slice(m.DNS.DNSRecordDiffs, func(i, j int) bool {
		if m.DNS.DNSRecordDiffs[i].Name != m.DNS.DNSRecordDiffs[j].Name {
			return m.DNS.DNSRecordDiffs[i].Name < m.DNS.DNSRecordDiffs[j].Name
		}
		return m.DNS.DNSRecordDiffs[i].Type < m.DNS.DNSRecordDiffs[j].Type
	})
}

func (m *Model) fetchRecordDiffsAsync(domainName string) tea.Cmd {
	return func() tea.Msg {
		domain := m.Config.GetDomainMap()[domainName]
		if domain == nil {
			return dnsRecordsFetchedMsg{err: fmt.Errorf("domain '%s' not found", domainName)}
		}

		isp := m.Config.GetISPMap()[domain.DNSISP]
		if isp == nil {
			return dnsRecordsFetchedMsg{err: fmt.Errorf("DNS ISP '%s' not found", domain.DNSISP)}
		}

		provider, err := createDNSProviderFromConfig(isp, m.Config.GetSecretsMap())
		if err != nil {
			return dnsRecordsFetchedMsg{err: err}
		}

		remoteRecords, err := provider.ListRecords(context.Background(), domainName)
		if err != nil {
			return dnsRecordsFetchedMsg{err: err}
		}

		localRecordMap := make(map[string]*entity.DNSRecord)
		for i := range domain.Records {
			key := fmt.Sprintf("%s:%s:%s", domain.Records[i].Type, domain.Records[i].Name, domain.Records[i].Value)
			localRecordMap[key] = &domain.Records[i]
			localRecordMap[key].Domain = domain.Name
		}

		var diffs []RecordDiff
		for _, remote := range remoteRecords {
			recordName := remote.Name
			if recordName == domainName || recordName == "" {
				recordName = "@"
			} else if strings.HasSuffix(remote.Name, "."+domainName) {
				recordName = strings.TrimSuffix(remote.Name, "."+domainName)
			}

			key := fmt.Sprintf("%s:%s:%s", remote.Type, recordName, remote.Value)
			if local, exists := localRecordMap[key]; exists {
				if local.TTL != remote.TTL {
					diffs = append(diffs, RecordDiff{
						Domain:     domainName,
						Type:       entity.DNSRecordType(remote.Type),
						Name:       recordName,
						Value:      remote.Value,
						TTL:        remote.TTL,
						ChangeType: valueobject.ChangeTypeUpdate,
					})
				}
				delete(localRecordMap, key)
			} else {
				diffs = append(diffs, RecordDiff{
					Domain:     domainName,
					Type:       entity.DNSRecordType(remote.Type),
					Name:       recordName,
					Value:      remote.Value,
					TTL:        remote.TTL,
					ChangeType: valueobject.ChangeTypeCreate,
				})
			}
		}

		for _, local := range localRecordMap {
			diffs = append(diffs, RecordDiff{
				Domain:     local.Domain,
				Type:       local.Type,
				Name:       local.Name,
				Value:      local.Value,
				TTL:        local.TTL,
				ChangeType: valueobject.ChangeTypeDelete,
			})
		}

		sort.Slice(diffs, func(i, j int) bool {
			if diffs[i].Name != diffs[j].Name {
				return diffs[i].Name < diffs[j].Name
			}
			return diffs[i].Type < diffs[j].Type
		})

		return dnsRecordsFetchedMsg{diffs: diffs}
	}
}

func (m *Model) saveSelectedDiffs() {
	if len(m.DNS.DNSPullDiffs) > 0 {
		selectedDiffs := make([]DomainDiff, 0)
		for i, diff := range m.DNS.DNSPullDiffs {
			if m.DNS.DNSPullSelected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}
		if len(selectedDiffs) > 0 {
			m.saveDomainDiffsToConfig(selectedDiffs)
		}
	}
	if len(m.DNS.DNSRecordDiffs) > 0 {
		selectedDiffs := make([]RecordDiff, 0)
		for i, diff := range m.DNS.DNSRecordDiffs {
			if m.DNS.DNSPullSelected[i] {
				selectedDiffs = append(selectedDiffs, diff)
			}
		}
		if len(selectedDiffs) > 0 {
			m.saveRecordDiffsToConfig(selectedDiffs)
		}
	}
}

func (m *Model) saveDomainDiffsToConfig(diffs []DomainDiff) {
	configDir := filepath.Join(m.ConfigDir, "userdata", string(m.Environment))
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		if diff.ChangeType == valueobject.ChangeTypeCreate {
			newDomains = append(newDomains, entity.Domain{
				Name:   diff.Name,
				DNSISP: diff.DNSISP,
			})
			domainSet[diff.Name] = true
		}
	}

	for _, d := range m.Config.Domains {
		if !domainSet[d.Name] {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Name == d.Name && diff.ChangeType == valueobject.ChangeTypeDelete {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomains = append(newDomains, d)
			}
		}
	}

	if err := saveYAMLConfig(dnsPath, "domains", newDomains); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		return
	}
	m.Config = nil
	m.loadConfig()
	m.buildTrees()
}

func (m *Model) saveRecordDiffsToConfig(diffs []RecordDiff) {
	configDir := filepath.Join(m.ConfigDir, "userdata", string(m.Environment))
	dnsPath := filepath.Join(configDir, "dns.yaml")

	newDomains := make([]entity.Domain, 0)
	domainSet := make(map[string]bool)

	for _, diff := range diffs {
		domainSet[diff.Domain] = true
	}

	for _, d := range m.Config.Domains {
		newDomain := entity.Domain{
			Name:   d.Name,
			ISP:    d.ISP,
			DNSISP: d.DNSISP,
			Parent: d.Parent,
		}
		for _, r := range d.Records {
			shouldKeep := true
			for _, diff := range diffs {
				if diff.Domain == d.Name && string(diff.Type) == string(r.Type) && diff.Name == r.Name &&
					(diff.ChangeType == valueobject.ChangeTypeDelete || diff.ChangeType == valueobject.ChangeTypeUpdate) {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				newDomain.Records = append(newDomain.Records, r)
			}
		}
		if domainSet[d.Name] {
			for _, diff := range diffs {
				if diff.Domain == d.Name && (diff.ChangeType == valueobject.ChangeTypeCreate || diff.ChangeType == valueobject.ChangeTypeUpdate) {
					newDomain.Records = append(newDomain.Records, entity.DNSRecord{
						Type:  diff.Type,
						Name:  diff.Name,
						Value: diff.Value,
						TTL:   diff.TTL,
					})
				}
			}
			delete(domainSet, d.Name)
		}
		newDomains = append(newDomains, newDomain)
	}

	if err := saveYAMLConfig(dnsPath, "domains", newDomains); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		return
	}
	m.Config = nil
	m.loadConfig()
	m.buildTrees()
}

func saveYAMLConfig(path, key string, data interface{}) error {
	yamlData := map[string]interface{}{key: data}
	content, err := yaml.Marshal(yamlData)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

func createDNSProviderFromConfig(isp *entity.ISP, secrets map[string]string) (infradns.Provider, error) {
	factory := infradns.NewFactory()
	return factory.Create(isp, secrets)
}
