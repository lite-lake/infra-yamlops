package dns

import (
	"fmt"
	"strconv"
	"strings"

	alidns "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

type AliyunProvider struct {
	client *alidns.Client
}

func NewAliyunProvider(accessKeyID, accessKeySecret string) (*AliyunProvider, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
	}
	config.Endpoint = tea.String("dns.aliyuncs.com")
	client, err := alidns.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create aliyun dns client: %w", err)
	}
	return &AliyunProvider{client: client}, nil
}

func (p *AliyunProvider) Name() string {
	return "aliyun"
}

func (p *AliyunProvider) ListRecords(domain string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domain),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    tea.StringValue(r.RecordId),
				Name:  tea.StringValue(r.RR),
				Type:  tea.StringValue(r.Type),
				Value: tea.StringValue(r.Value),
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *AliyunProvider) CreateRecord(domain string, record *DNSRecord) error {
	ttl := int64(record.TTL)
	if ttl == 0 {
		ttl = 600
	}

	req := &alidns.AddDomainRecordRequest{
		DomainName: tea.String(domain),
		RR:         tea.String(record.Name),
		Type:       tea.String(record.Type),
		Value:      tea.String(record.Value),
		TTL:        tea.Int64(ttl),
	}

	_, err := p.client.AddDomainRecord(req)
	if err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}
	return nil
}

func (p *AliyunProvider) DeleteRecord(domain string, recordID string) error {
	req := &alidns.DeleteDomainRecordRequest{
		RecordId: tea.String(recordID),
	}

	_, err := p.client.DeleteDomainRecord(req)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	return nil
}

func (p *AliyunProvider) UpdateRecord(domain string, recordID string, record *DNSRecord) error {
	ttl := int64(record.TTL)
	if ttl == 0 {
		ttl = 600
	}

	req := &alidns.UpdateDomainRecordRequest{
		RecordId: tea.String(recordID),
		RR:       tea.String(record.Name),
		Type:     tea.String(record.Type),
		Value:    tea.String(record.Value),
		TTL:      tea.Int64(ttl),
	}

	_, err := p.client.UpdateDomainRecord(req)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}
	return nil
}

func (p *AliyunProvider) ListDomains() ([]string, error) {
	req := &alidns.DescribeDomainsRequest{}
	resp, err := p.client.DescribeDomains(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	var domains []string
	if resp.Body != nil && resp.Body.Domains != nil {
		for _, d := range resp.Body.Domains.Domain {
			domains = append(domains, tea.StringValue(d.DomainName))
		}
	}
	return domains, nil
}

func (p *AliyunProvider) GetRecordsByType(domain string, recordType string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domain),
		Type:       tea.String(recordType),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    tea.StringValue(r.RecordId),
				Name:  tea.StringValue(r.RR),
				Type:  tea.StringValue(r.Type),
				Value: tea.StringValue(r.Value),
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *AliyunProvider) GetRecordsByName(domain string, name string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domain),
		RRKeyWord:  tea.String(name),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    tea.StringValue(r.RecordId),
				Name:  tea.StringValue(r.RR),
				Type:  tea.StringValue(r.Type),
				Value: tea.StringValue(r.Value),
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *AliyunProvider) BatchCreateRecords(domain string, records []*DNSRecord) error {
	for _, record := range records {
		if err := p.CreateRecord(domain, record); err != nil {
			return fmt.Errorf("failed to create record %s: %w", record.Name, err)
		}
	}
	return nil
}

func (p *AliyunProvider) BatchDeleteRecords(domain string, recordIDs []string) error {
	for _, recordID := range recordIDs {
		if err := p.DeleteRecord(domain, recordID); err != nil {
			return fmt.Errorf("failed to delete record %s: %w", recordID, err)
		}
	}
	return nil
}

func (p *AliyunProvider) EnsureRecord(domain string, record *DNSRecord) error {
	return EnsureRecord(p, domain, record)
}

func ParseAliyunTTL(ttlStr string) (int64, error) {
	ttl, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil {
		return 600, fmt.Errorf("invalid TTL: %s", ttlStr)
	}
	validTTLs := []int64{1, 5, 10, 20, 30, 60, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400}
	for _, validTTL := range validTTLs {
		if ttl <= validTTL {
			return validTTL, nil
		}
	}
	return 86400, nil
}

func GetFullDomain(subDomain, domain string) string {
	if subDomain == "@" {
		return domain
	}
	if subDomain == "" {
		return domain
	}
	return strings.Join([]string{subDomain, domain}, ".")
}

func GetSubDomain(fullDomain, domain string) string {
	if fullDomain == domain {
		return "@"
	}
	suffix := "." + domain
	if strings.HasSuffix(fullDomain, suffix) {
		return strings.TrimSuffix(fullDomain, suffix)
	}
	return fullDomain
}
