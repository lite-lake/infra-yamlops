package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
)

func TestDNSRecord_Validate(t *testing.T) {
	tests := []struct {
		name    string
		record  DNSRecord
		wantErr error
	}{
		{
			name:    "invalid type",
			record:  DNSRecord{Type: "INVALID", Name: "www", Value: "192.168.1.1", TTL: 300},
			wantErr: domain.ErrInvalidType,
		},
		{
			name:    "missing name",
			record:  DNSRecord{Type: DNSRecordTypeA, Value: "192.168.1.1", TTL: 300},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "missing value",
			record:  DNSRecord{Type: DNSRecordTypeA, Name: "www", TTL: 300},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "negative ttl",
			record:  DNSRecord{Type: DNSRecordTypeA, Name: "www", Value: "192.168.1.1", TTL: -1},
			wantErr: domain.ErrInvalidTTL,
		},
		{
			name:    "valid type A",
			record:  DNSRecord{Type: DNSRecordTypeA, Name: "www", Value: "192.168.1.1", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type AAAA",
			record:  DNSRecord{Type: DNSRecordTypeAAAA, Name: "www", Value: "2001:db8::1", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type CNAME",
			record:  DNSRecord{Type: DNSRecordTypeCNAME, Name: "alias", Value: "www.example.com", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type MX",
			record:  DNSRecord{Type: DNSRecordTypeMX, Name: "@", Value: "mail.example.com", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type TXT",
			record:  DNSRecord{Type: DNSRecordTypeTXT, Name: "@", Value: "v=spf1 include:_spf.example.com ~all", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type NS",
			record:  DNSRecord{Type: DNSRecordTypeNS, Name: "@", Value: "ns1.example.com", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid type SRV",
			record:  DNSRecord{Type: DNSRecordTypeSRV, Name: "_sip._tcp", Value: "10 60 5060 sipserver.example.com", TTL: 300},
			wantErr: nil,
		},
		{
			name:    "valid zero ttl",
			record:  DNSRecord{Type: DNSRecordTypeA, Name: "www", Value: "192.168.1.1", TTL: 0},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}
