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
)

type CloudflareProvider struct {
	client *cloudflare.Client
}

func NewCloudflareProvider(apiToken string) Provider {
	client := cloudflare.NewClient(
		option.WithAPIToken(apiToken),
	)
	return &CloudflareProvider{client: client}
}

func (p *CloudflareProvider) Name() string {
	return "cloudflare"
}

func (p *CloudflareProvider) getZoneID(ctx context.Context, domain string) (string, error) {
	resp, err := p.client.Zones.List(ctx, zones.ZoneListParams{
		Name: cloudflare.F(domain),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list zones: %w", err)
	}
	if len(resp.Result) == 0 {
		return "", ErrDomainNotFound
	}
	return resp.Result[0].ID, nil
}

func (p *CloudflareProvider) ListRecords(domain string) ([]DNSRecord, error) {
	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domain)
	if err != nil {
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
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	return records, nil
}

func (p *CloudflareProvider) CreateRecord(domain string, record *DNSRecord) error {
	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domain)
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
		return fmt.Errorf("failed to create record: %w", err)
	}
	return nil
}

func (p *CloudflareProvider) DeleteRecord(domain string, recordID string) error {
	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domain)
	if err != nil {
		return err
	}

	_, err = p.client.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cloudflare.F(zoneID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	return nil
}

func (p *CloudflareProvider) UpdateRecord(domain string, recordID string, record *DNSRecord) error {
	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domain)
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
		return fmt.Errorf("failed to update record: %w", err)
	}
	return nil
}

func (p *CloudflareProvider) ListZones() ([]string, error) {
	ctx := context.Background()
	var zoneNames []string
	pager := p.client.Zones.ListAutoPaging(ctx, zones.ZoneListParams{})
	for pager.Next() {
		zone := pager.Current()
		zoneNames = append(zoneNames, zone.Name)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}
	return zoneNames, nil
}

func (p *CloudflareProvider) GetRecordsByType(domain string, recordType string) ([]DNSRecord, error) {
	ctx := context.Background()
	zoneID, err := p.getZoneID(ctx, domain)
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
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	return records, nil
}

func (p *CloudflareProvider) BatchCreateRecords(domain string, records []*DNSRecord) error {
	for _, record := range records {
		if err := p.CreateRecord(domain, record); err != nil {
			return fmt.Errorf("failed to create record %s: %w", record.Name, err)
		}
	}
	return nil
}

func (p *CloudflareProvider) BatchDeleteRecords(domain string, recordIDs []string) error {
	for _, recordID := range recordIDs {
		if err := p.DeleteRecord(domain, recordID); err != nil {
			return fmt.Errorf("failed to delete record %s: %w", recordID, err)
		}
	}
	return nil
}

func (p *CloudflareProvider) EnsureRecord(domain string, record *DNSRecord) error {
	records, err := p.ListRecords(domain)
	if err != nil {
		return err
	}

	for _, r := range records {
		if r.Name == record.Name && r.Type == record.Type {
			if r.Value == record.Value && r.TTL == record.TTL {
				return nil
			}
			return p.UpdateRecord(domain, r.ID, record)
		}
	}

	return p.CreateRecord(domain, record)
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
