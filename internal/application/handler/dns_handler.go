package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type DNSHandler struct{}

func NewDNSHandler() *DNSHandler {
	return &DNSHandler{}
}

func (h *DNSHandler) EntityType() string {
	return "dns_record"
}

func (h *DNSHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
	result := &Result{Change: change, Success: false}

	record, err := h.extractDNSRecordFromChange(change)
	if err != nil {
		result.Error = err
		return result, nil
	}

	domain, ok := deps.Domains[record.Domain]
	if !ok {
		result.Error = fmt.Errorf("domain %s not found", record.Domain)
		return result, nil
	}

	provider, err := h.getDNSProvider(domain.DNSISP, deps)
	if err != nil {
		result.Error = fmt.Errorf("failed to get DNS provider: %w", err)
		return result, nil
	}

	switch change.Type {
	case valueobject.ChangeTypeDelete:
		return h.deleteRecord(change, record, provider)
	case valueobject.ChangeTypeUpdate:
		return h.updateRecord(change, record, provider)
	default:
		return h.createRecord(change, record, provider)
	}
}

func (h *DNSHandler) extractDNSRecordFromChange(ch *valueobject.Change) (*entity.DNSRecord, error) {
	if ch.NewState != nil {
		if record, ok := ch.NewState.(*entity.DNSRecord); ok {
			return record, nil
		}
	}
	if ch.OldState != nil {
		if record, ok := ch.OldState.(*entity.DNSRecord); ok {
			return record, nil
		}
	}
	return nil, fmt.Errorf("cannot extract DNS record from change")
}

func (h *DNSHandler) getDNSProvider(ispName string, deps *Deps) (DNSProvider, error) {
	isp, ok := deps.ISPs[ispName]
	if !ok {
		return nil, fmt.Errorf("ISP %s not found", ispName)
	}

	if !isp.HasService(entity.ISPServiceDNS) {
		return nil, fmt.Errorf("ISP %s does not provide DNS service", ispName)
	}

	provider, err := deps.DNSFactory.Create(isp, deps.Secrets)
	if err != nil {
		return nil, err
	}
	return WrapDNSProvider(provider), nil
}

func (h *DNSHandler) createRecord(change *valueobject.Change, record *entity.DNSRecord, provider DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	dnsRecord := &dns.DNSRecord{
		Name:  record.Name,
		Type:  string(record.Type),
		Value: record.Value,
		TTL:   record.TTL,
	}

	if err := provider.CreateRecord(record.Domain, dnsRecord); err != nil {
		result.Error = fmt.Errorf("failed to create record: %w", err)
		return result, nil
	}

	result.Success = true
	result.Output = fmt.Sprintf("created DNS record %s.%s", record.Name, record.Domain)
	return result, nil
}

func (h *DNSHandler) updateRecord(change *valueobject.Change, record *entity.DNSRecord, provider DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	existingRecords, err := provider.ListRecords(record.Domain)
	if err != nil {
		result.Error = fmt.Errorf("failed to list existing records: %w", err)
		return result, nil
	}

	dnsRecord := &dns.DNSRecord{
		Name:  record.Name,
		Type:  string(record.Type),
		Value: record.Value,
		TTL:   record.TTL,
	}

	for _, r := range existingRecords {
		if r.Name == record.Name && r.Type == string(record.Type) {
			if err := provider.UpdateRecord(record.Domain, r.ID, dnsRecord); err != nil {
				result.Error = fmt.Errorf("failed to update record: %w", err)
				return result, nil
			}
			result.Success = true
			result.Output = fmt.Sprintf("updated DNS record %s.%s", record.Name, record.Domain)
			return result, nil
		}
	}

	if err := provider.CreateRecord(record.Domain, dnsRecord); err != nil {
		result.Error = fmt.Errorf("failed to create record: %w", err)
		return result, nil
	}

	result.Success = true
	result.Output = fmt.Sprintf("created DNS record %s.%s", record.Name, record.Domain)
	return result, nil
}

func (h *DNSHandler) deleteRecord(change *valueobject.Change, record *entity.DNSRecord, provider DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	existingRecords, err := provider.ListRecords(record.Domain)
	if err != nil {
		result.Error = fmt.Errorf("failed to list existing records: %w", err)
		return result, nil
	}

	for _, r := range existingRecords {
		if r.Name == record.Name && strings.EqualFold(r.Type, string(record.Type)) {
			if err := provider.DeleteRecord(record.Domain, r.ID); err != nil {
				result.Error = fmt.Errorf("failed to delete record: %w", err)
				return result, nil
			}
			result.Success = true
			result.Output = fmt.Sprintf("deleted DNS record %s.%s", record.Name, record.Domain)
			return result, nil
		}
	}

	result.Success = true
	result.Output = fmt.Sprintf("DNS record %s.%s not found, skipping", record.Name, record.Domain)
	return result, nil
}
