package dns

import (
	"fmt"
	"strconv"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

type TencentProvider struct {
	client *dnspod.Client
}

func NewTencentProvider(secretID, secretKey string) (*TencentProvider, error) {
	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	client, err := dnspod.NewClient(credential, "", cpf)
	if err != nil {
		return nil, fmt.Errorf("create tencent dns client: %w", err)
	}
	return &TencentProvider{client: client}, nil
}

func (p *TencentProvider) Name() string {
	return "tencent"
}

func (p *TencentProvider) ListRecords(domain string) ([]DNSRecord, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)

	resp, err := p.client.DescribeRecordList(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    strconv.FormatUint(*r.RecordId, 10),
				Name:  *r.Name,
				Type:  *r.Type,
				Value: *r.Value,
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *TencentProvider) CreateRecord(domain string, record *DNSRecord) error {
	ttl := uint64(record.TTL)
	if ttl == 0 {
		ttl = 600
	}

	req := dnspod.NewCreateRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.SubDomain = common.StringPtr(record.Name)
	req.RecordType = common.StringPtr(record.Type)
	req.Value = common.StringPtr(record.Value)
	req.TTL = common.Uint64Ptr(ttl)

	_, err := p.client.CreateRecord(req)
	if err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}
	return nil
}

func (p *TencentProvider) DeleteRecord(domain string, recordID string) error {
	recordIDInt, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid record ID: %w", err)
	}

	req := dnspod.NewDeleteRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(recordIDInt)

	_, err = p.client.DeleteRecord(req)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	return nil
}

func (p *TencentProvider) UpdateRecord(domain string, recordID string, record *DNSRecord) error {
	recordIDInt, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid record ID: %w", err)
	}

	ttl := uint64(record.TTL)
	if ttl == 0 {
		ttl = 600
	}

	req := dnspod.NewModifyRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(recordIDInt)
	req.SubDomain = common.StringPtr(record.Name)
	req.RecordType = common.StringPtr(record.Type)
	req.Value = common.StringPtr(record.Value)
	req.TTL = common.Uint64Ptr(ttl)

	_, err = p.client.ModifyRecord(req)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}
	return nil
}

func (p *TencentProvider) ListDomains() ([]string, error) {
	req := dnspod.NewDescribeDomainListRequest()
	resp, err := p.client.DescribeDomainList(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	var domains []string
	if resp.Response != nil && resp.Response.DomainList != nil {
		for _, d := range resp.Response.DomainList {
			domains = append(domains, *d.Name)
		}
	}
	return domains, nil
}

func (p *TencentProvider) GetRecordsByType(domain string, recordType string) ([]DNSRecord, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordType = common.StringPtr(recordType)

	resp, err := p.client.DescribeRecordList(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    strconv.FormatUint(*r.RecordId, 10),
				Name:  *r.Name,
				Type:  *r.Type,
				Value: *r.Value,
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *TencentProvider) GetRecordsBySubDomain(domain string, subDomain string) ([]DNSRecord, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)
	req.Subdomain = common.StringPtr(subDomain)

	resp, err := p.client.DescribeRecordList(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := 600
			if r.TTL != nil {
				ttl = int(*r.TTL)
			}
			records = append(records, DNSRecord{
				ID:    strconv.FormatUint(*r.RecordId, 10),
				Name:  *r.Name,
				Type:  *r.Type,
				Value: *r.Value,
				TTL:   ttl,
			})
		}
	}
	return records, nil
}

func (p *TencentProvider) BatchCreateRecords(domain string, records []*DNSRecord) error {
	for _, record := range records {
		if err := p.CreateRecord(domain, record); err != nil {
			return fmt.Errorf("failed to create record %s: %w", record.Name, err)
		}
	}
	return nil
}

func (p *TencentProvider) BatchDeleteRecords(domain string, recordIDs []string) error {
	for _, recordID := range recordIDs {
		if err := p.DeleteRecord(domain, recordID); err != nil {
			return fmt.Errorf("failed to delete record %s: %w", recordID, err)
		}
	}
	return nil
}

func (p *TencentProvider) EnsureRecord(domain string, record *DNSRecord) error {
	return EnsureRecord(p, domain, record)
}

func (p *TencentProvider) CreateDomain(domain string) error {
	req := dnspod.NewCreateDomainRequest()
	req.Domain = common.StringPtr(domain)

	_, err := p.client.CreateDomain(req)
	if err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}
	return nil
}

func (p *TencentProvider) DeleteDomain(domain string) error {
	req := dnspod.NewDeleteDomainRequest()
	req.Domain = common.StringPtr(domain)

	_, err := p.client.DeleteDomain(req)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}
	return nil
}

func (p *TencentProvider) SetDomainStatus(domain string, status string) error {
	enable := status == "enable"
	req := dnspod.NewModifyDomainStatusRequest()
	req.Domain = common.StringPtr(domain)
	req.Status = common.StringPtr(status)
	if enable {
		req.Status = common.StringPtr("enable")
	} else {
		req.Status = common.StringPtr("pause")
	}

	_, err := p.client.ModifyDomainStatus(req)
	if err != nil {
		return fmt.Errorf("failed to set domain status: %w", err)
	}
	return nil
}

func ParseTencentTTL(ttlStr string) (uint64, error) {
	ttl, err := strconv.ParseUint(ttlStr, 10, 64)
	if err != nil {
		return 600, fmt.Errorf("invalid TTL: %s", ttlStr)
	}
	validTTLs := []uint64{1, 5, 10, 20, 30, 60, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400}
	for _, validTTL := range validTTLs {
		if ttl <= validTTL {
			return validTTL, nil
		}
	}
	return 86400, nil
}
