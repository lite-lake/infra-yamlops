package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
)

type fakeDNSProvider struct {
	records      []dns.DNSRecord
	listErr      error
	transientErr error
	failures     int
	calls        int
	domains      []string
}

func (p *fakeDNSProvider) Name() string { return "fake" }

func (p *fakeDNSProvider) ListDomains(ctx context.Context) ([]string, error) { return nil, nil }

func (p *fakeDNSProvider) ListRecords(ctx context.Context, domain string) ([]dns.DNSRecord, error) {
	p.calls++
	p.domains = append(p.domains, domain)
	if p.failures > 0 {
		p.failures--
		return nil, p.transientErr
	}
	if p.listErr != nil {
		return nil, p.listErr
	}
	return p.records, nil
}

func (p *fakeDNSProvider) CreateRecord(ctx context.Context, domain string, record *dns.DNSRecord) error {
	return nil
}

func (p *fakeDNSProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	return nil
}

func (p *fakeDNSProvider) UpdateRecord(ctx context.Context, domain string, recordID string, record *dns.DNSRecord) error {
	return nil
}

func (p *fakeDNSProvider) GetRecordsByTypes(ctx context.Context, domain, recordType string) ([]dns.DNSRecord, error) {
	return nil, nil
}

func (p *fakeDNSProvider) BatchCreateRecords(ctx context.Context, domain string, records []*dns.DNSRecord) error {
	return nil
}

func (p *fakeDNSProvider) BatchDeleteRecords(ctx context.Context, domain string, recordIDs []string) error {
	return nil
}

func (p *fakeDNSProvider) EnsureRecord(ctx context.Context, domain string, record *dns.DNSRecord) error {
	return nil
}

type fakeDNSProviderFactory struct {
	provider dns.Provider
	err      error
}

func (f *fakeDNSProviderFactory) Create(isp *entity.ISP, secrets map[string]string) (dns.Provider, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.provider, nil
}

func testConfig() *entity.Config {
	return &entity.Config{
		ISPs: []entity.ISP{
			{Name: "isp1", Type: entity.ISPTypeTencent},
		},
		Domains: []entity.Domain{
			{Name: "example.com", DNSISP: "isp1", Records: []entity.DNSRecord{
				{Type: "A", Name: "@", Value: "1.1.1.1", TTL: 600},
			}},
		},
	}
}

func TestFetchDNSRemoteState_BuildsRecords(t *testing.T) {
	provider := &fakeDNSProvider{
		records: []dns.DNSRecord{
			{Name: "example.com", Type: "A", Value: "1.1.1.1", TTL: 600},
			{Name: "www.example.com", Type: "CNAME", Value: "example.com.", TTL: 600, Line: "oversea"},
		},
	}

	state, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), nil,
		&fakeDNSProviderFactory{provider: provider})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantApex := "example.com:A:@:default:1.1.1.1"
	apex, ok := state.Records[wantApex]
	if !ok {
		t.Fatalf("missing apex record %q, got keys %v", wantApex, recordKeys(state.Records))
	}
	if apex.Name != "@" || apex.Domain != "example.com" {
		t.Errorf("apex record = %+v, want name @ domain example.com", apex)
	}

	wantSub := "example.com:CNAME:www:oversea:example.com."
	sub, ok := state.Records[wantSub]
	if !ok {
		t.Fatalf("missing sub record %q, got keys %v", wantSub, recordKeys(state.Records))
	}
	if sub.Name != "www" || sub.Line != "oversea" {
		t.Errorf("sub record = %+v, want name www line oversea", sub)
	}

	if _, ok := state.Domains["example.com"]; !ok {
		t.Errorf("state.Domains missing example.com")
	}
}

func TestFetchDNSRemoteState_ListFailureAborts(t *testing.T) {
	boom := errors.New("dnspod describe record list failed")
	provider := &fakeDNSProvider{listErr: boom}

	state, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), nil,
		&fakeDNSProviderFactory{provider: provider})
	if err == nil {
		t.Fatalf("expected error, got state with %d records", len(state.Records))
	}
	if state != nil {
		t.Errorf("expected nil state on failure, got %+v", state)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want wrapped %v", err, boom)
	}
	if !strings.Contains(err.Error(), "example.com") {
		t.Errorf("error should name the failing domain, got %v", err)
	}
	if provider.calls != 1 {
		t.Errorf("ListRecords calls = %d, want 1 for non-retryable error", provider.calls)
	}
}

func TestFetchDNSRemoteState_RetriesTransientFailure(t *testing.T) {
	provider := &fakeDNSProvider{
		records:      []dns.DNSRecord{{Name: "example.com", Type: "A", Value: "1.1.1.1", TTL: 600}},
		transientErr: errors.New("connection reset by peer"),
		failures:     1,
	}

	state, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), nil,
		&fakeDNSProviderFactory{provider: provider})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if provider.calls != 2 {
		t.Errorf("ListRecords calls = %d, want 2 (one retry)", provider.calls)
	}
	if len(state.Records) != 1 {
		t.Errorf("records = %d, want 1", len(state.Records))
	}
}

func TestFetchDNSRemoteState_ProviderCreationFailureAborts(t *testing.T) {
	createErr := errors.New("missing credential")
	_, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), nil,
		&fakeDNSProviderFactory{err: createErr})
	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v, want wrapped %v", err, createErr)
	}
}

func TestFetchDNSRemoteState_MissingISPAborts(t *testing.T) {
	cfg := &entity.Config{
		Domains: []entity.Domain{{Name: "example.com", DNSISP: "missing", Records: []entity.DNSRecord{
			{Type: "A", Name: "@", Value: "1.1.1.1", TTL: 600},
		}}},
	}
	_, err := fetchDNSRemoteStateWithFactory(context.Background(), cfg, nil,
		&fakeDNSProviderFactory{provider: &fakeDNSProvider{}})
	if !errors.Is(err, domainerr.ErrISPNotFound) {
		t.Fatalf("error = %v, want %v", err, domainerr.ErrISPNotFound)
	}
}

func TestFetchDNSRemoteState_UnknownScopedDomainAborts(t *testing.T) {
	scope := valueobject.NewScope().WithDomains([]string{"unknown.com"})
	_, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), scope,
		&fakeDNSProviderFactory{provider: &fakeDNSProvider{}})
	if !errors.Is(err, domainerr.ErrDNSDomainNotFound) {
		t.Fatalf("error = %v, want %v", err, domainerr.ErrDNSDomainNotFound)
	}
}

func TestFetchDNSRemoteState_EmptyRecordsIsValid(t *testing.T) {
	provider := &fakeDNSProvider{}
	state, err := fetchDNSRemoteStateWithFactory(context.Background(), testConfig(), nil,
		&fakeDNSProviderFactory{provider: provider})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.Records) != 0 {
		t.Errorf("records = %d, want 0", len(state.Records))
	}
}

func TestFetchDNSRemoteState_ScopeLimitsDomains(t *testing.T) {
	cfg := testConfig()
	cfg.Domains = append(cfg.Domains, entity.Domain{Name: "other.com", DNSISP: "isp1", Records: []entity.DNSRecord{
		{Type: "A", Name: "@", Value: "2.2.2.2", TTL: 600},
	}})
	provider := &fakeDNSProvider{}
	scope := valueobject.NewScope().WithDomains([]string{"other.com"})

	if _, err := fetchDNSRemoteStateWithFactory(context.Background(), cfg, scope,
		&fakeDNSProviderFactory{provider: provider}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(provider.domains) != 1 || provider.domains[0] != "other.com" {
		t.Errorf("listed domains = %v, want [other.com]", provider.domains)
	}
}

func TestFetchDNSRemoteState_SkipsDomainWithoutConfiguredRecords(t *testing.T) {
	cfg := &entity.Config{
		ISPs: []entity.ISP{
			{Name: "isp1", Type: entity.ISPTypeTencent},
		},
		Domains: []entity.Domain{
			{Name: "retired.com", DNSISP: "isp1"},
		},
	}
	provider := &fakeDNSProvider{listErr: errors.New("DNS domain not found")}

	state, err := fetchDNSRemoteStateWithFactory(context.Background(), cfg, nil,
		&fakeDNSProviderFactory{provider: provider})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != 0 {
		t.Errorf("ListRecords calls = %d, want 0 for domain without configured records", provider.calls)
	}
	if len(state.Records) != 0 {
		t.Errorf("records = %d, want 0", len(state.Records))
	}
}

func recordKeys(records map[string]*entity.DNSRecord) []string {
	keys := make([]string, 0, len(records))
	for k := range records {
		keys = append(keys, k)
	}
	return keys
}
