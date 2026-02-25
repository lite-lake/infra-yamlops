package dns

import (
	"context"

	alidns "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/litelake/yamlops/internal/constants"
	domainerr "github.com/litelake/yamlops/internal/domain"
)

type AliyunProvider struct {
	client *alidns.Client
}

func NewAliyunProvider(accessKeyID, accessKeySecret string) (Provider, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
	}
	config.Endpoint = tea.String("dns.aliyuncs.com")
	client, err := alidns.NewClient(config)
	if err != nil {
		return nil, domainerr.WrapOp("create aliyun dns client", err)
	}
	return &AliyunProvider{client: client}, nil
}

func (p *AliyunProvider) Name() string {
	return "aliyun"
}

func (p *AliyunProvider) ListRecords(ctx context.Context, domainName string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := constants.DefaultDNSRecordTTL
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

func (p *AliyunProvider) CreateRecord(ctx context.Context, domainName string, record *DNSRecord) error {
	ttl := int64(record.TTL)
	if ttl == 0 {
		ttl = constants.DefaultDNSRecordTTL
	}

	req := &alidns.AddDomainRecordRequest{
		DomainName: tea.String(domainName),
		RR:         tea.String(record.Name),
		Type:       tea.String(record.Type),
		Value:      tea.String(record.Value),
		TTL:        tea.Int64(ttl),
	}

	_, err := p.client.AddDomainRecord(req)
	if err != nil {
		return domainerr.WrapOp("create record", err)
	}
	return nil
}

func (p *AliyunProvider) DeleteRecord(ctx context.Context, domainName string, recordID string) error {
	req := &alidns.DeleteDomainRecordRequest{
		RecordId: tea.String(recordID),
	}

	_, err := p.client.DeleteDomainRecord(req)
	if err != nil {
		return domainerr.WrapOp("delete record", err)
	}
	return nil
}

func (p *AliyunProvider) UpdateRecord(ctx context.Context, domainName string, recordID string, record *DNSRecord) error {
	ttl := int64(record.TTL)
	if ttl == 0 {
		ttl = constants.DefaultDNSRecordTTL
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
		return domainerr.WrapOp("update record", err)
	}
	return nil
}

func (p *AliyunProvider) ListDomains(ctx context.Context) ([]string, error) {
	req := &alidns.DescribeDomainsRequest{}
	resp, err := p.client.DescribeDomains(req)
	if err != nil {
		return nil, domainerr.WrapOp("list domains", err)
	}

	var domains []string
	if resp.Body != nil && resp.Body.Domains != nil {
		for _, d := range resp.Body.Domains.Domain {
			domains = append(domains, tea.StringValue(d.DomainName))
		}
	}
	return domains, nil
}

func (p *AliyunProvider) GetRecordsByTypes(ctx context.Context, domainName string, recordType string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
		Type:       tea.String(recordType),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := constants.DefaultDNSRecordTTL
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

func (p *AliyunProvider) GetRecordsByName(domainName string, name string) ([]DNSRecord, error) {
	req := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
		RRKeyWord:  tea.String(name),
	}
	resp, err := p.client.DescribeDomainRecords(req)
	if err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Body != nil && resp.Body.DomainRecords != nil {
		for _, r := range resp.Body.DomainRecords.Record {
			ttl := constants.DefaultDNSRecordTTL
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

func (p *AliyunProvider) BatchCreateRecords(ctx context.Context, domainName string, records []*DNSRecord) error {
	return BatchCreateRecordsHelper(ctx, p, domainName, records)
}

func (p *AliyunProvider) BatchDeleteRecords(ctx context.Context, domainName string, recordIDs []string) error {
	return BatchDeleteRecordsHelper(ctx, p, domainName, recordIDs)
}

func (p *AliyunProvider) EnsureRecord(ctx context.Context, domainName string, record *DNSRecord) error {
	return EnsureRecordHelper(ctx, p, domainName, record)
}
