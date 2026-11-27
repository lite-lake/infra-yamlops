package dns

import (
	"context"
	"errors"
	"testing"
)

type stubProvider struct {
	records     []DNSRecord
	createCalls int
	updateCalls int
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) ListDomains(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (s *stubProvider) ListRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	return s.records, nil
}

func (s *stubProvider) CreateRecord(ctx context.Context, domain string, record *DNSRecord) error {
	s.createCalls++
	s.records = append(s.records, *record)
	return nil
}

func (s *stubProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	return nil
}

func (s *stubProvider) UpdateRecord(ctx context.Context, domain string, recordID string, record *DNSRecord) error {
	s.updateCalls++
	return nil
}

func (s *stubProvider) GetRecordsByTypes(ctx context.Context, domain, recordType string) ([]DNSRecord, error) {
	return s.records, nil
}

func (s *stubProvider) GetRecordsByName(domain string, name string) ([]DNSRecord, error) {
	return s.records, nil
}

func (s *stubProvider) BatchCreateRecords(ctx context.Context, domain string, records []*DNSRecord) error {
	return nil
}

func (s *stubProvider) BatchDeleteRecords(ctx context.Context, domain string, recordIDs []string) error {
	return nil
}

func (s *stubProvider) EnsureRecord(ctx context.Context, domain string, record *DNSRecord) error {
	return EnsureRecord(ctx, s, domain, record, nil)
}

func TestEffectiveLine(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "default"},
		{"default", "default"},
		{"oversea", "oversea"},
		{"telecom", "telecom"},
	}
	for _, tt := range tests {
		if got := effectiveLine(tt.in); got != tt.want {
			t.Errorf("effectiveLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEnsureRecord_MatchesLine(t *testing.T) {
	ctx := context.Background()
	p := &stubProvider{
		records: []DNSRecord{
			{ID: "r1", Name: "demo", Type: "A", Value: "103.236.57.87", TTL: 600, Line: "default"},
			{ID: "r2", Name: "demo", Type: "A", Value: "199.68.217.251", TTL: 600, Line: "oversea"},
		},
	}

	err := p.EnsureRecord(ctx, "example-dns.cn", &DNSRecord{Name: "demo", Type: "A", Value: "199.68.217.251", TTL: 600, Line: "oversea"})
	if err != nil {
		t.Fatalf("EnsureRecord oversea: %v", err)
	}
	if p.createCalls != 0 || p.updateCalls != 0 {
		t.Errorf("expected no-op for oversea record, create=%d update=%d", p.createCalls, p.updateCalls)
	}

	err = p.EnsureRecord(ctx, "example-dns.cn", &DNSRecord{Name: "demo", Type: "A", Value: "199.68.217.252", TTL: 600, Line: "oversea"})
	if err != nil {
		t.Fatalf("EnsureRecord update oversea: %v", err)
	}
	if p.updateCalls != 1 {
		t.Errorf("expected 1 update for oversea value change, got %d", p.updateCalls)
	}

	err = p.EnsureRecord(ctx, "example-dns.cn", &DNSRecord{Name: "demo", Type: "A", Value: "103.236.57.87", TTL: 600})
	if err != nil {
		t.Fatalf("EnsureRecord default: %v", err)
	}
	if p.createCalls != 0 || p.updateCalls != 1 {
		t.Errorf("expected no-op for default record, create=%d update=%d", p.createCalls, p.updateCalls)
	}
}

func TestEnsureRecord_CreatesNewLine(t *testing.T) {
	ctx := context.Background()
	p := &stubProvider{
		records: []DNSRecord{
			{ID: "r1", Name: "demo", Type: "A", Value: "103.236.57.87", TTL: 600, Line: "default"},
		},
	}

	err := p.EnsureRecord(ctx, "example-dns.cn", &DNSRecord{Name: "demo", Type: "A", Value: "199.68.217.251", TTL: 600, Line: "oversea"})
	if err != nil {
		t.Fatalf("EnsureRecord create oversea: %v", err)
	}
	if p.createCalls != 1 {
		t.Errorf("expected 1 create for missing oversea line, got %d", p.createCalls)
	}
}

func TestIsRetryableDNSError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"network timeout", errors.New("dial tcp: i/o timeout"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"tencent rate limit", errors.New("TencentCloudSDKError: Code=RequestLimitExceeded, Message=..."), true},
		{"tencent internal error", errors.New("TencentCloudSDKError: Code=InternalError, Message=..."), true},
		{"tencent resource unavailable", errors.New("TencentCloudSDKError: Code=ResourceUnavailable, Message=..."), true},
		{"aliyun throttling", errors.New("SDK.ServerError Code=Throttling"), true},
		{"network eof", errors.New("ClientError.NetworkError, Message=Fail to get response: EOF"), true},
		{"auth failure is not retryable", errors.New("TencentCloudSDKError: Code=AuthFailure.SignatureFailure"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableDNSError(tt.err); got != tt.want {
				t.Errorf("IsRetryableDNSError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
