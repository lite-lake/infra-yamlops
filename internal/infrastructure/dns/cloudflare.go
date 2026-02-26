package dns

import (
	"context"

	"github.com/cloudflare/cloudflare-go/v2"
	"github.com/cloudflare/cloudflare-go/v2/dns"
	"github.com/cloudflare/cloudflare-go/v2/option"
	"github.com/cloudflare/cloudflare-go/v2/zones"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

type CloudflareProvider struct {
	client    *cloudflare.Client
	accountID string
}

func NewCloudflareProvider(apiToken string, accountID string) (Provider, error) {
	client := cloudflare.NewClient(
		option.WithAPIToken(apiToken),
	)
	return &CloudflareProvider{client: client, accountID: accountID}, nil
}

func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

func (p *CloudflareProvider) getZoneID(ctx context.Context, domainName string) (string, error) {
	resp, err := p.client.Zones.List(ctx, zones.ZoneListParams{
		Name: cloudflare.F(domainName),
	})
	if err != nil {
		return "", domainerr.WrapOp("list zones", err)
	}
	if len(resp.Result) == 0 {
		return "", ErrDomainNotFound
	}
	return resp.Result[0].ID, nil
}

func (p *CloudflareProvider) ListRecords(ctx context.Context, domainName string) ([]DNSRecord, error) {
	logger.Debug("listing DNS records", "provider", "cloudflare", "domain", domainName)

	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		logger.Error("failed to get zone ID", "domain", domainName, "error", err)
		return nil, err
	}

	var records []DNSRecord
	pager := p.client.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
	})
	for pager.Next() {
		record := pager.Current()
		content := ""
		if str, ok := record.Content.(string); ok {
			content = str
		}
		records = append(records, DNSRecord{
			ID:    record.ID,
			Name:  record.Name,
			Type:  string(record.Type),
			Value: content,
			TTL:   int(record.TTL),
		})
	}
	if err := pager.Err(); err != nil {
		logger.Error("failed to list records", "domain", domainName, "error", err)
		return nil, domainerr.WrapOp("list records", err)
	}

	logger.Debug("listed DNS records", "provider", "cloudflare", "domain", domainName, "count", len(records))
	return records, nil
}

func (p *CloudflareProvider) CreateRecord(ctx context.Context, domainName string, record *DNSRecord) error {
	logger.Debug("creating DNS record", "provider", "cloudflare", "domain", domainName, "name", record.Name, "type", record.Type)

	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return err
	}

	ttl := record.TTL
	if ttl == 0 {
		ttl = 1
	}

	recordParam := p.buildRecordParam(record, ttl)
	params := dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Record: recordParam,
	}

	_, err = p.client.DNS.Records.New(ctx, params)
	if err != nil {
		logger.Error("failed to create DNS record", "domain", domainName, "name", record.Name, "error", err)
		return domainerr.WrapOp("create record", err)
	}

	logger.Info("DNS record created", "provider", "cloudflare", "domain", domainName, "name", record.Name, "type", record.Type)
	return nil
}

func (p *CloudflareProvider) buildRecordParam(record *DNSRecord, ttl int) dns.RecordUnionParam {
	switch record.Type {
	case "A":
		return dns.ARecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.ARecordTypeA),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	case "AAAA":
		return dns.AAAARecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.AAAARecordTypeAAAA),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	case "CNAME":
		return dns.CNAMERecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.CNAMERecordTypeCNAME),
			Content: cloudflare.F[interface{}](record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	case "TXT":
		return dns.TXTRecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.TXTRecordTypeTXT),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	case "MX":
		return dns.MXRecordParam{
			Name:     cloudflare.F(record.Name),
			Type:     cloudflare.F(dns.MXRecordTypeMX),
			Content:  cloudflare.F(record.Value),
			TTL:      cloudflare.F(dns.TTL(ttl)),
			Priority: cloudflare.F(10.0),
		}
	case "NS":
		return dns.NSRecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.NSRecordTypeNS),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	case "SRV":
		priority, weight, port, target := ParseSRVValue(record.Value)
		return dns.SRVRecordParam{
			Name: cloudflare.F(record.Name),
			Type: cloudflare.F(dns.SRVRecordTypeSRV),
			Data: cloudflare.F(dns.SRVRecordDataParam{
				Priority: cloudflare.F(priority),
				Weight:   cloudflare.F(weight),
				Port:     cloudflare.F(port),
				Target:   cloudflare.F(target),
			}),
			TTL: cloudflare.F(dns.TTL(ttl)),
		}
	default:
		return dns.ARecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.ARecordType(record.Type)),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		}
	}
}

func (p *CloudflareProvider) DeleteRecord(ctx context.Context, domainName string, recordID string) error {
	logger.Debug("deleting DNS record", "provider", "cloudflare", "domain", domainName, "record_id", recordID)

	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return err
	}

	_, err = p.client.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cloudflare.F(zoneID),
	})
	if err != nil {
		logger.Error("failed to delete DNS record", "domain", domainName, "record_id", recordID, "error", err)
		return domainerr.WrapOp("delete record", err)
	}

	logger.Info("DNS record deleted", "provider", "cloudflare", "domain", domainName, "record_id", recordID)
	return nil
}

func (p *CloudflareProvider) UpdateRecord(ctx context.Context, domainName string, recordID string, record *DNSRecord) error {
	logger.Debug("updating DNS record", "provider", "cloudflare", "domain", domainName, "record_id", recordID, "name", record.Name)

	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return err
	}

	ttl := record.TTL
	if ttl == 0 {
		ttl = 1
	}

	recordParam := p.buildRecordParam(record, ttl)
	params := dns.RecordEditParams{
		ZoneID: cloudflare.F(zoneID),
		Record: recordParam,
	}

	_, err = p.client.DNS.Records.Edit(ctx, recordID, params)
	if err != nil {
		logger.Error("failed to update DNS record", "domain", domainName, "record_id", recordID, "error", err)
		return domainerr.WrapOp("update record", err)
	}

	logger.Info("DNS record updated", "provider", "cloudflare", "domain", domainName, "record_id", recordID)
	return nil
}

func (p *CloudflareProvider) ListDomains(ctx context.Context) ([]string, error) {
	var zoneNames []string
	params := zones.ZoneListParams{}
	if p.accountID != "" {
		params.Account = cloudflare.F(zones.ZoneListParamsAccount{
			ID: cloudflare.F(p.accountID),
		})
	}
	pager := p.client.Zones.ListAutoPaging(ctx, params)
	for pager.Next() {
		zone := pager.Current()
		zoneNames = append(zoneNames, zone.Name)
	}
	if err := pager.Err(); err != nil {
		return nil, domainerr.WrapOp("list zones", err)
	}
	return zoneNames, nil
}

func (p *CloudflareProvider) GetRecordsByTypes(ctx context.Context, domainName string, recordType string) ([]DNSRecord, error) {
	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return nil, err
	}

	var records []DNSRecord
	pager := p.client.DNS.Records.ListAutoPaging(ctx, dns.RecordListParams{
		ZoneID: cloudflare.F(zoneID),
		Type:   cloudflare.F(dns.RecordListParamsType(recordType)),
	})
	for pager.Next() {
		record := pager.Current()
		content := ""
		if str, ok := record.Content.(string); ok {
			content = str
		}
		records = append(records, DNSRecord{
			ID:    record.ID,
			Name:  record.Name,
			Type:  string(record.Type),
			Value: content,
			TTL:   int(record.TTL),
		})
	}
	if err := pager.Err(); err != nil {
		return nil, domainerr.WrapOp("list records", err)
	}
	return records, nil
}

func (p *CloudflareProvider) BatchCreateRecords(ctx context.Context, domainName string, records []*DNSRecord) error {
	return BatchCreateRecordsHelper(ctx, p, domainName, records)
}

func (p *CloudflareProvider) BatchDeleteRecords(ctx context.Context, domainName string, recordIDs []string) error {
	return BatchDeleteRecordsHelper(ctx, p, domainName, recordIDs)
}

func (p *CloudflareProvider) EnsureRecord(ctx context.Context, domainName string, record *DNSRecord) error {
	return EnsureRecordHelper(ctx, p, domainName, record)
}
