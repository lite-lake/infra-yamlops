package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
)

func TestSecret_Validate(t *testing.T) {
	tests := []struct {
		name    string
		secret  Secret
		wantErr error
	}{
		{
			name:    "missing name",
			secret:  Secret{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "valid with empty value",
			secret:  Secret{Name: "db_pass"},
			wantErr: nil,
		},
		{
			name:    "valid with value",
			secret:  Secret{Name: "db_pass", Value: "secret123"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.secret.Validate()
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

func TestDomain_Validate(t *testing.T) {
	tests := []struct {
		name    string
		domain  Domain
		wantErr bool
	}{
		{
			name:    "missing name",
			domain:  Domain{DNSISP: "cloudflare"},
			wantErr: true,
		},
		{
			name:    "invalid domain format",
			domain:  Domain{Name: "invalid..domain", DNSISP: "cloudflare"},
			wantErr: true,
		},
		{
			name:    "missing dns_isp",
			domain:  Domain{Name: "example.com"},
			wantErr: true,
		},
		{
			name:    "invalid dns record",
			domain:  Domain{Name: "example.com", DNSISP: "cloudflare", Records: []DNSRecord{{Type: "invalid"}}},
			wantErr: true,
		},
		{
			name:    "valid simple domain",
			domain:  Domain{Name: "example.com", DNSISP: "cloudflare"},
			wantErr: false,
		},
		{
			name:    "valid wildcard domain",
			domain:  Domain{Name: "*.example.com", DNSISP: "cloudflare"},
			wantErr: false,
		},
		{
			name: "valid with records",
			domain: Domain{
				Name:    "example.com",
				DNSISP:  "cloudflare",
				Parent:  "parent-isp",
				Records: []DNSRecord{{Type: DNSRecordTypeA, Name: "www", Value: "192.168.1.1", TTL: 300}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.domain.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDomain_FlattenRecords(t *testing.T) {
	domain := Domain{
		Name: "example.com",
		Records: []DNSRecord{
			{Type: DNSRecordTypeA, Name: "www", Value: "192.168.1.1", TTL: 300},
			{Type: DNSRecordTypeA, Name: "api", Value: "192.168.1.2", TTL: 300},
		},
	}

	result := domain.FlattenRecords()

	if len(result) != 2 {
		t.Errorf("FlattenRecords() returned %d records, want 2", len(result))
		return
	}

	for _, r := range result {
		if r.Domain != "example.com" {
			t.Errorf("FlattenRecords() Domain = %s, want example.com", r.Domain)
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "empty config is valid",
			config:  Config{},
			wantErr: false,
		},
		{
			name:    "invalid secret",
			config:  Config{Secrets: []Secret{{}}},
			wantErr: true,
		},
		{
			name:    "invalid zone",
			config:  Config{Zones: []Zone{{}}},
			wantErr: true,
		},
		{
			name: "valid full config",
			config: Config{
				Secrets: []Secret{{Name: "pass", Value: "secret"}},
				Zones:   []Zone{{Name: "zone-1", Region: "us-east-1"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_GetMapsAndHelpers(t *testing.T) {
	config := Config{
		Secrets: []Secret{
			{Name: "db_pass", Value: "secret123"},
			{Name: "api_key", Value: "key456"},
		},
		ISPs: []ISP{
			{Name: "cloudflare", Services: []ISPService{ISPServiceDNS}},
		},
		Zones: []Zone{
			{Name: "zone-1", Region: "us-east-1"},
		},
		Servers: []Server{
			{Name: "server-1"},
		},
		Registries: []Registry{
			{Name: "dockerhub"},
		},
		Domains: []Domain{
			{Name: "example.com", DNSISP: "cloudflare", Records: []DNSRecord{{Type: DNSRecordTypeA, Name: "www", Value: "1.1.1.1", TTL: 300}}},
		},
		Services: []BizService{
			{Name: "api"},
		},
		InfraServices: []InfraService{
			{Name: "gateway"},
		},
	}

	t.Run("GetSecretsMap", func(t *testing.T) {
		m := config.GetSecretsMap()
		if m["db_pass"] != "secret123" {
			t.Errorf("GetSecretsMap()[db_pass] = %v, want secret123", m["db_pass"])
		}
	})

	t.Run("GetISPMap", func(t *testing.T) {
		m := config.GetISPMap()
		if m["cloudflare"] == nil {
			t.Error("GetISPMap()[cloudflare] is nil")
		}
	})

	t.Run("GetZoneMap", func(t *testing.T) {
		m := config.GetZoneMap()
		if m["zone-1"] == nil {
			t.Error("GetZoneMap()[zone-1] is nil")
		}
	})

	t.Run("GetServerMap", func(t *testing.T) {
		m := config.GetServerMap()
		if m["server-1"] == nil {
			t.Error("GetServerMap()[server-1] is nil")
		}
	})

	t.Run("GetRegistryMap", func(t *testing.T) {
		m := config.GetRegistryMap()
		if m["dockerhub"] == nil {
			t.Error("GetRegistryMap()[dockerhub] is nil")
		}
	})

	t.Run("GetDomainMap", func(t *testing.T) {
		m := config.GetDomainMap()
		if m["example.com"] == nil {
			t.Error("GetDomainMap()[example.com] is nil")
		}
	})

	t.Run("GetServiceMap", func(t *testing.T) {
		m := config.GetServiceMap()
		if m["api"] == nil {
			t.Error("GetServiceMap()[api] is nil")
		}
	})

	t.Run("GetInfraServiceMap", func(t *testing.T) {
		m := config.GetInfraServiceMap()
		if m["gateway"] == nil {
			t.Error("GetInfraServiceMap()[gateway] is nil")
		}
	})

	t.Run("GetAllDNSRecords", func(t *testing.T) {
		records := config.GetAllDNSRecords()
		if len(records) != 1 {
			t.Errorf("GetAllDNSRecords() returned %d records, want 1", len(records))
		}
		if records[0].Domain != "example.com" {
			t.Errorf("GetAllDNSRecords()[0].Domain = %s, want example.com", records[0].Domain)
		}
	})
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr error
	}{
		{"valid port", "8080", 8080, nil},
		{"invalid string", "abc", 0, domain.ErrInvalidPort},
		{"port zero", "0", 0, domain.ErrInvalidPort},
		{"port negative", "-1", 0, domain.ErrInvalidPort},
		{"port too large", "65536", 0, domain.ErrInvalidPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePort(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ParsePort() error = %v, want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("ParsePort() unexpected error = %v", err)
				}
				if got != tt.want {
					t.Errorf("ParsePort() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
