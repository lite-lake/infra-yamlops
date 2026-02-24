package dns

import "github.com/litelake/yamlops/internal/domain"

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
	ListDomains() ([]string, error)
	ListRecords(domain string) ([]DNSRecord, error)
	CreateRecord(domain string, record *DNSRecord) error
	DeleteRecord(domain string, recordID string) error
	UpdateRecord(domain string, recordID string, record *DNSRecord) error
}
