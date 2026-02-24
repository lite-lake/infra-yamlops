package dns

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/cloudflare/cloudflare-go/v2"
	"github.com/cloudflare/cloudflare-go/v2/dns"
	"github.com/cloudflare/cloudflare-go/v2/option"
	"github.com/cloudflare/cloudflare-go/v2/zones"
	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/infrastructure/logger"
)

type CloudflareProvider struct {
	client    *cloudflare.Client
	accountID string
}

func NewCloudflareProvider(apiToken string, accountID string) Provider {
	client := cloudflare.NewClient(
		option.WithAPIToken(apiToken),
	)
	return &CloudflareProvider{client: client, accountID: accountID}
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

func (p *CloudflareProvider) ListRecords(domainName string) ([]DNSRecord, error) {
	logger.Debug("listing DNS records", "provider", "cloudflare", "domain", domainName)

	ctx := context.Background()
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

func (p *CloudflareProvider) CreateRecord(domainName string, record *DNSRecord) error {
	logger.Debug("creating DNS record", "provider", "cloudflare", "domain", domainName, "name", record.Name, "type", record.Type)

	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return err
	}

	ttl := record.TTL
	if ttl == 0 {
		ttl = 1
	}

	params := dns.RecordNewParams{
		ZoneID: cloudflare.F(zoneID),
		Record: dns.ARecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.ARecordType(record.Type)),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		},
	}

	_, err = p.client.DNS.Records.New(ctx, params)
	if err != nil {
		logger.Error("failed to create DNS record", "domain", domainName, "name", record.Name, "error", err)
		return domainerr.WrapOp("create record", err)
	}

	logger.Info("DNS record created", "provider", "cloudflare", "domain", domainName, "name", record.Name, "type", record.Type)
	return nil
}

func (p *CloudflareProvider) DeleteRecord(domainName string, recordID string) error {
	logger.Debug("deleting DNS record", "provider", "cloudflare", "domain", domainName, "record_id", recordID)

	ctx := context.Background()
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

func (p *CloudflareProvider) UpdateRecord(domainName string, recordID string, record *DNSRecord) error {
	logger.Debug("updating DNS record", "provider", "cloudflare", "domain", domainName, "record_id", recordID, "name", record.Name)

	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domainName)
	if err != nil {
		return err
	}

	ttl := record.TTL
	if ttl == 0 {
		ttl = 1
	}

	params := dns.RecordEditParams{
		ZoneID: cloudflare.F(zoneID),
		Record: dns.ARecordParam{
			Name:    cloudflare.F(record.Name),
			Type:    cloudflare.F(dns.ARecordType(record.Type)),
			Content: cloudflare.F(record.Value),
			TTL:     cloudflare.F(dns.TTL(ttl)),
		},
	}

	_, err = p.client.DNS.Records.Edit(ctx, recordID, params)
	if err != nil {
		logger.Error("failed to update DNS record", "domain", domainName, "record_id", recordID, "error", err)
		return domainerr.WrapOp("update record", err)
	}

	logger.Info("DNS record updated", "provider", "cloudflare", "domain", domainName, "record_id", recordID)
	return nil
}

func (p *CloudflareProvider) ListDomains() ([]string, error) {
	ctx := context.Background()
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

func (p *CloudflareProvider) GetRecordsByType(domainName string, recordType string) ([]DNSRecord, error) {
	ctx := context.Background()
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

func (p *CloudflareProvider) BatchCreateRecords(domainName string, records []*DNSRecord) error {
	for _, record := range records {
		if err := p.CreateRecord(domainName, record); err != nil {
			return domainerr.WrapEntity("record", record.Name, err)
		}
	}
	return nil
}

func (p *CloudflareProvider) BatchDeleteRecords(domainName string, recordIDs []string) error {
	for _, recordID := range recordIDs {
		if err := p.DeleteRecord(domainName, recordID); err != nil {
			return domainerr.WrapEntity("record", recordID, err)
		}
	}
	return nil
}

func (p *CloudflareProvider) EnsureRecord(domainName string, record *DNSRecord) error {
	return EnsureRecordSimple(p, domainName, record)
}

func ParseTTL(ttlStr string) (int, error) {
	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL: %s", ttlStr)
	}
	validTTLs := []int{1, 5, 10, 20, 30, 60, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400}
	idx, _ := slices.BinarySearch(validTTLs, ttl)
	if idx < len(validTTLs) && validTTLs[idx] == ttl {
		return ttl, nil
	}
	if idx > 0 {
		return validTTLs[idx-1], nil
	}
	return 1, nil
}
