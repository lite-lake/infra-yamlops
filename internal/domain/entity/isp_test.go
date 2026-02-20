package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestISP_Validate(t *testing.T) {
	tests := []struct {
		name    string
		isp     ISP
		wantErr error
	}{
		{
			name:    "missing name",
			isp:     ISP{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "missing services",
			isp:     ISP{Name: "cloudflare", Type: ISPTypeCloudflare},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "missing credentials",
			isp:     ISP{Name: "cloudflare", Type: ISPTypeCloudflare, Services: []ISPService{ISPServiceDNS}},
			wantErr: domain.ErrRequired,
		},
		{
			name: "invalid credential",
			isp: ISP{
				Name:     "cloudflare",
				Type:     ISPTypeCloudflare,
				Services: []ISPService{ISPServiceDNS},
				Credentials: map[string]valueobject.SecretRef{
					"api_token": {},
				},
			},
			wantErr: domain.ErrEmptyValue,
		},
		{
			name: "valid with explicit type",
			isp: ISP{
				Name:     "cloudflare",
				Type:     ISPTypeCloudflare,
				Services: []ISPService{ISPServiceDNS, ISPServiceCertificate},
				Credentials: map[string]valueobject.SecretRef{
					"api_token": {Plain: "token"},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid with implicit type from name",
			isp: ISP{
				Name:     "aliyun",
				Services: []ISPService{ISPServiceServer, ISPServiceDNS},
				Credentials: map[string]valueobject.SecretRef{
					"access_key": {Secret: "aliyun_key"},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.isp.Validate()
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

func TestISP_HasService(t *testing.T) {
	isp := ISP{
		Name:     "cloudflare",
		Services: []ISPService{ISPServiceDNS, ISPServiceCertificate},
	}

	tests := []struct {
		service  ISPService
		expected bool
	}{
		{ISPServiceDNS, true},
		{ISPServiceCertificate, true},
		{ISPServiceServer, false},
		{ISPServiceDomain, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.service), func(t *testing.T) {
			if got := isp.HasService(tt.service); got != tt.expected {
				t.Errorf("HasService(%s) = %v, want %v", tt.service, got, tt.expected)
			}
		})
	}
}
