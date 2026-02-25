package contract

import "context"

type DNSRecord struct {
	ID    string
	Name  string
	Type  string
	Value string
	TTL   int
}

type DNSProvider interface {
	Name() string
	ListDomains(ctx context.Context) ([]string, error)
	ListRecords(ctx context.Context, domain string) ([]DNSRecord, error)
	CreateRecord(ctx context.Context, domain string, record *DNSRecord) error
	DeleteRecord(ctx context.Context, domain string, recordID string) error
	UpdateRecord(ctx context.Context, domain string, recordID string, record *DNSRecord) error
	GetRecordsByTypes(ctx context.Context, domain, recordType string) ([]DNSRecord, error)
	BatchCreateRecords(ctx context.Context, domain string, records []*DNSRecord) error
	BatchDeleteRecords(ctx context.Context, domain string, recordIDs []string) error
	EnsureRecord(ctx context.Context, domain string, record *DNSRecord) error
}
