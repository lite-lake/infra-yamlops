package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
)

func TestCertificate_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cert    Certificate
		wantErr error
	}{
		{
			name:    "missing name",
			cert:    Certificate{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "missing domains",
			cert:    Certificate{Name: "cert-1"},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "empty domain in list",
			cert:    Certificate{Name: "cert-1", Domains: []string{""}},
			wantErr: domain.ErrEmptyValue,
		},
		{
			name:    "invalid provider",
			cert:    Certificate{Name: "cert-1", Domains: []string{"example.com"}, Provider: "invalid"},
			wantErr: domain.ErrInvalidType,
		},
		{
			name:    "missing dns_provider",
			cert:    Certificate{Name: "cert-1", Domains: []string{"example.com"}, Provider: CertificateProviderLetsEncrypt},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "valid letsencrypt",
			cert:    Certificate{Name: "cert-1", Domains: []string{"example.com", "*.example.com"}, Provider: CertificateProviderLetsEncrypt, DNSProvider: "cloudflare"},
			wantErr: nil,
		},
		{
			name:    "valid zerossl",
			cert:    Certificate{Name: "cert-1", Domains: []string{"example.com"}, Provider: CertificateProviderZeroSSL, DNSProvider: "aliyun"},
			wantErr: nil,
		},
		{
			name:    "valid with renew_before",
			cert:    Certificate{Name: "cert-1", Domains: []string{"example.com"}, Provider: CertificateProviderLetsEncrypt, DNSProvider: "cloudflare", RenewBefore: "720h"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cert.Validate()
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
