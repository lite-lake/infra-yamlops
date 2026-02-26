package entity

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

type DNSRecordType string

const (
	DNSRecordTypeA     DNSRecordType = "A"
	DNSRecordTypeAAAA  DNSRecordType = "AAAA"
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
	DNSRecordTypeMX    DNSRecordType = "MX"
	DNSRecordTypeTXT   DNSRecordType = "TXT"
	DNSRecordTypeNS    DNSRecordType = "NS"
	DNSRecordTypeSRV   DNSRecordType = "SRV"
)

type DNSRecord struct {
	Domain string        `yaml:"-"`
	Type   DNSRecordType `yaml:"type"`
	Name   string        `yaml:"name"`
	Value  string        `yaml:"value"`
	TTL    int           `yaml:"ttl"`
}

func (r *DNSRecord) Validate() error {
	validTypes := map[DNSRecordType]bool{
		DNSRecordTypeA:     true,
		DNSRecordTypeAAAA:  true,
		DNSRecordTypeCNAME: true,
		DNSRecordTypeMX:    true,
		DNSRecordTypeTXT:   true,
		DNSRecordTypeNS:    true,
		DNSRecordTypeSRV:   true,
	}
	if !validTypes[r.Type] {
		return fmt.Errorf("%w: dns record type %s", domain.ErrInvalidType, r.Type)
	}
	if r.Name == "" {
		return domain.RequiredField("name")
	}
	if r.Value == "" {
		return domain.RequiredField("value")
	}
	if r.TTL < 0 {
		return fmt.Errorf("%w: ttl must be non-negative", domain.ErrInvalidTTL)
	}
	return nil
}
