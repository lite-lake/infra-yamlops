package handler

import (
	"context"
	"fmt"
	"strings"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/contract"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type DNSHandler struct{}

func NewDNSHandler() *DNSHandler {
	return &DNSHandler{}
}

func (h *DNSHandler) EntityType() string {
	return "dns_record"
}

func (h *DNSHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	record, err := h.extractDNSRecordFromChange(change)
	if err != nil {
		result.Error = err
		return result, nil
	}

	domain, ok := deps.Domain(record.Domain)
	if !ok {
		result.Error = fmt.Errorf("%w: %s", domainerr.ErrDNSDomainNotFound, record.Domain)
		return result, nil
	}

	provider, err := deps.DNSProvider(domain.DNSISP)
	if err != nil {
		result.Error = fmt.Errorf("get DNS provider: %w", err)
		return result, nil
	}

	switch change.Type() {
	case valueobject.ChangeTypeDelete:
		return h.deleteRecord(ctx, change, record, provider)
	case valueobject.ChangeTypeUpdate:
		return h.updateRecord(ctx, change, record, provider)
	default:
		return h.createRecord(ctx, change, record, provider)
	}
}

func (h *DNSHandler) extractDNSRecordFromChange(ch *valueobject.Change) (*entity.DNSRecord, error) {
	if ch.NewState() != nil {
		if record, ok := ch.NewState().(*entity.DNSRecord); ok {
			return record, nil
		}
	}
	if ch.OldState() != nil {
		if record, ok := ch.OldState().(*entity.DNSRecord); ok {
			return record, nil
		}
	}
	return nil, fmt.Errorf("cannot extract DNS record from change")
}

func (h *DNSHandler) createRecord(ctx context.Context, change *valueobject.Change, record *entity.DNSRecord, provider contract.DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	dnsRecord := &contract.DNSRecord{
		Name:  record.Name,
		Type:  string(record.Type),
		Value: record.Value,
		TTL:   record.TTL,
	}

	if err := provider.CreateRecord(ctx, record.Domain, dnsRecord); err != nil {
		result.Error = fmt.Errorf("%w: create record: %w", domainerr.ErrDNSError, err)
		return result, nil
	}

	result.Success = true
	result.Output = fmt.Sprintf("created DNS record %s.%s", record.Name, record.Domain)
	return result, nil
}

func (h *DNSHandler) updateRecord(ctx context.Context, change *valueobject.Change, record *entity.DNSRecord, provider contract.DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	existingRecords, err := provider.ListRecords(ctx, record.Domain)
	if err != nil {
		result.Error = fmt.Errorf("%w: list existing records: %w", domainerr.ErrDNSError, err)
		return result, nil
	}

	dnsRecord := &contract.DNSRecord{
		Name:  record.Name,
		Type:  string(record.Type),
		Value: record.Value,
		TTL:   record.TTL,
	}

	for _, r := range existingRecords {
		if r.Name == record.Name && strings.EqualFold(r.Type, string(record.Type)) {
			if err := provider.UpdateRecord(ctx, record.Domain, r.ID, dnsRecord); err != nil {
				result.Error = fmt.Errorf("%w: update record: %w", domainerr.ErrDNSError, err)
				return result, nil
			}
			result.Success = true
			result.Output = fmt.Sprintf("updated DNS record %s.%s", record.Name, record.Domain)
			return result, nil
		}
	}

	if err := provider.CreateRecord(ctx, record.Domain, dnsRecord); err != nil {
		result.Error = fmt.Errorf("%w: create record: %w", domainerr.ErrDNSError, err)
		return result, nil
	}

	result.Success = true
	result.Output = fmt.Sprintf("created DNS record %s.%s", record.Name, record.Domain)
	return result, nil
}

func (h *DNSHandler) deleteRecord(ctx context.Context, change *valueobject.Change, record *entity.DNSRecord, provider contract.DNSProvider) (*Result, error) {
	result := &Result{Change: change, Success: false}

	existingRecords, err := provider.ListRecords(ctx, record.Domain)
	if err != nil {
		result.Error = fmt.Errorf("%w: list existing records: %w", domainerr.ErrDNSError, err)
		return result, nil
	}

	for _, r := range existingRecords {
		if r.Name == record.Name && strings.EqualFold(r.Type, string(record.Type)) {
			if err := provider.DeleteRecord(ctx, record.Domain, r.ID); err != nil {
				result.Error = fmt.Errorf("%w: delete record: %w", domainerr.ErrDNSError, err)
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
