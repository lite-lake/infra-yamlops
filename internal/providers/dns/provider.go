package dns

import "errors"

var (
	ErrDomainNotFound  = errors.New("domain not found")
	ErrRecordNotFound  = errors.New("record not found")
	ErrInvalidResponse = errors.New("invalid response from provider")
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
