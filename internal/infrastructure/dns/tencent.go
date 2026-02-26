package dns

import (
	"context"
	"strconv"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

type TencentProvider struct {
	client *dnspod.Client
}

func NewTencentProvider(secretID, secretKey string) (Provider, error) {
	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	client, err := dnspod.NewClient(credential, "", cpf)
	if err != nil {
		return nil, domainerr.WrapOp("create tencent dns client", err)
	}
	return &TencentProvider{client: client}, nil
}

func (p *TencentProvider) Name() string {
	return "tencent"
}

func (p *TencentProvider) ListRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)

	resp, err := p.client.DescribeRecordList(req)
	if err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := constants.DefaultDNSRecordTTL
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

func (p *TencentProvider) CreateRecord(ctx context.Context, domain string, record *DNSRecord) error {
	ttl := uint64(record.TTL)
	if ttl == 0 {
		ttl = constants.DefaultDNSRecordTTL
	}

	req := dnspod.NewCreateRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.SubDomain = common.StringPtr(record.Name)
	req.RecordType = common.StringPtr(record.Type)
	req.Value = common.StringPtr(record.Value)
	req.TTL = common.Uint64Ptr(ttl)

	_, err := p.client.CreateRecord(req)
	if err != nil {
		return domainerr.WrapOp("create record", err)
	}
	return nil
}

func (p *TencentProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	recordIDInt, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return domainerr.WrapOp("parse record ID", err)
	}

	req := dnspod.NewDeleteRecordRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordId = common.Uint64Ptr(recordIDInt)

	_, err = p.client.DeleteRecord(req)
	if err != nil {
		return domainerr.WrapOp("delete record", err)
	}
	return nil
}

func (p *TencentProvider) UpdateRecord(ctx context.Context, domain string, recordID string, record *DNSRecord) error {
	recordIDInt, err := strconv.ParseUint(recordID, 10, 64)
	if err != nil {
		return domainerr.WrapOp("parse record ID", err)
	}

	ttl := uint64(record.TTL)
	if ttl == 0 {
		ttl = constants.DefaultDNSRecordTTL
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
		return domainerr.WrapOp("update record", err)
	}
	return nil
}

func (p *TencentProvider) ListDomains(ctx context.Context) ([]string, error) {
	req := dnspod.NewDescribeDomainListRequest()
	resp, err := p.client.DescribeDomainList(req)
	if err != nil {
		return nil, domainerr.WrapOp("list domains", err)
	}

	var domains []string
	if resp.Response != nil && resp.Response.DomainList != nil {
		for _, d := range resp.Response.DomainList {
			domains = append(domains, *d.Name)
		}
	}
	return domains, nil
}

func (p *TencentProvider) GetRecordsByTypes(ctx context.Context, domain string, recordType string) ([]DNSRecord, error) {
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domain)
	req.RecordType = common.StringPtr(recordType)

	resp, err := p.client.DescribeRecordList(req)
	if err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := constants.DefaultDNSRecordTTL
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
		return nil, domainerr.WrapOp("list records", err)
	}

	var records []DNSRecord
	if resp.Response != nil && resp.Response.RecordList != nil {
		for _, r := range resp.Response.RecordList {
			ttl := constants.DefaultDNSRecordTTL
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

func (p *TencentProvider) BatchCreateRecords(ctx context.Context, domain string, records []*DNSRecord) error {
	return BatchCreateRecordsHelper(ctx, p, domain, records)
}

func (p *TencentProvider) BatchDeleteRecords(ctx context.Context, domain string, recordIDs []string) error {
	return BatchDeleteRecordsHelper(ctx, p, domain, recordIDs)
}

func (p *TencentProvider) EnsureRecord(ctx context.Context, domain string, record *DNSRecord) error {
	return EnsureRecordHelper(ctx, p, domain, record)
}

func (p *TencentProvider) CreateDomain(domain string) error {
	req := dnspod.NewCreateDomainRequest()
	req.Domain = common.StringPtr(domain)

	_, err := p.client.CreateDomain(req)
	if err != nil {
		return domainerr.WrapOp("create domain", err)
	}
	return nil
}

func (p *TencentProvider) DeleteDomain(domain string) error {
	req := dnspod.NewDeleteDomainRequest()
	req.Domain = common.StringPtr(domain)

	_, err := p.client.DeleteDomain(req)
	if err != nil {
		return domainerr.WrapOp("delete domain", err)
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
		return domainerr.WrapOp("set domain status", err)
	}
	return nil
}
