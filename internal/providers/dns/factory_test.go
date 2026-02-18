package dns

import (
	"testing"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestFactory_Create(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name        string
		isp         *entity.ISP
		secrets     map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "unsupported provider type",
			isp: &entity.ISP{
				Name: "unknown",
				Type: "unknown",
			},
			secrets:     map[string]string{},
			wantErr:     true,
			errContains: "unsupported provider type",
		},
		{
			name: "missing api_token for cloudflare",
			isp: &entity.ISP{
				Name: "cf",
				Type: entity.ISPTypeCloudflare,
				Credentials: map[string]valueobject.SecretRef{
					"api_token": {Secret: "missing_token"},
				},
			},
			secrets:     map[string]string{},
			wantErr:     true,
			errContains: "resolve api_token",
		},
		{
			name: "missing access_key_id for aliyun",
			isp: &entity.ISP{
				Name: "ali",
				Type: entity.ISPTypeAliyun,
				Credentials: map[string]valueobject.SecretRef{
					"access_key_id": {Secret: "missing_key"},
				},
			},
			secrets:     map[string]string{},
			wantErr:     true,
			errContains: "resolve access_key_id",
		},
		{
			name: "missing secret_id for tencent",
			isp: &entity.ISP{
				Name: "tencent",
				Type: entity.ISPTypeTencent,
				Credentials: map[string]valueobject.SecretRef{
					"secret_id": {Secret: "missing_id"},
				},
			},
			secrets:     map[string]string{},
			wantErr:     true,
			errContains: "resolve secret_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.Create(tt.isp, tt.secrets)
			if (err != nil) != tt.wantErr {
				t.Errorf("Factory.Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !containsString(err.Error(), tt.errContains) {
					t.Errorf("Factory.Create() error = %v, want error containing %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestFactory_Register(t *testing.T) {
	factory := NewFactory()

	customCreator := func(isp *entity.ISP, secrets map[string]string) (Provider, error) {
		return &mockProvider{name: "custom"}, nil
	}

	factory.Register("custom", customCreator)

	isp := &entity.ISP{
		Name:        "custom",
		Type:        "custom",
		Credentials: map[string]valueobject.SecretRef{},
	}

	provider, err := factory.Create(isp, map[string]string{})
	if err != nil {
		t.Fatalf("Factory.Create() error = %v", err)
	}

	if provider.Name() != "custom" {
		t.Errorf("provider.Name() = %v, want %v", provider.Name(), "custom")
	}
}

func TestFactory_DefaultProviders(t *testing.T) {
	factory := NewFactory()

	expectedTypes := []string{
		string(entity.ISPTypeCloudflare),
		string(entity.ISPTypeAliyun),
		string(entity.ISPTypeTencent),
	}

	for _, providerType := range expectedTypes {
		t.Run(providerType, func(t *testing.T) {
			_, ok := factory.creators[providerType]
			if !ok {
				t.Errorf("Factory missing default provider: %s", providerType)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                                        { return m.name }
func (m *mockProvider) ListDomains() ([]string, error)                      { return nil, nil }
func (m *mockProvider) ListRecords(domain string) ([]DNSRecord, error)      { return nil, nil }
func (m *mockProvider) CreateRecord(domain string, record *DNSRecord) error { return nil }
func (m *mockProvider) DeleteRecord(domain string, recordID string) error   { return nil }
func (m *mockProvider) UpdateRecord(domain string, recordID string, record *DNSRecord) error {
	return nil
}
