package dns

import (
	"context"

	"github.com/litelake/yamlops/internal/domain"
)

var (
	ErrDomainNotFound  = domain.ErrDNSDomainNotFound
	ErrRecordNotFound  = domain.ErrDNSRecordNotFound
	ErrInvalidResponse = domain.ErrDNSError
)

type DNSRecord struct {
	ID    string
	Name  string
	Type  string
	Value string
	TTL   int
}

type Provider interface {
	Name() string
	ListDomains(ctx context.Context) ([]string, error)
	ListRecords(ctx context.Context, domain string) ([]DNSRecord, error)
	CreateRecord(ctx context.Context, domain string, record *DNSRecord) error
	DeleteRecord(ctx context.Context, domain string, recordID string) error
	UpdateRecord(ctx context.Context, domain string, recordID string, record *DNSRecord) error
}
