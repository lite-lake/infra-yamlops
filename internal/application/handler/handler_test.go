package handler

import (
	"context"
	"errors"
	"testing"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/interfaces"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/registry"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type mockDNSProvider struct {
	name    string
	records []dns.DNSRecord
	err     error
	deleted []string
	updated map[string]*dns.DNSRecord
	created []*dns.DNSRecord
}

func newMockDNSProvider(name string) *mockDNSProvider {
	return &mockDNSProvider{
		name:    name,
		records: []dns.DNSRecord{},
		deleted: []string{},
		updated: make(map[string]*dns.DNSRecord),
		created: []*dns.DNSRecord{},
	}
}

func (m *mockDNSProvider) Name() string { return m.name }

func (m *mockDNSProvider) ListRecords(ctx context.Context, domain string) ([]dns.DNSRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.records, nil
}

func (m *mockDNSProvider) CreateRecord(ctx context.Context, domain string, record *dns.DNSRecord) error {
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, record)
	return nil
}

func (m *mockDNSProvider) DeleteRecord(ctx context.Context, domain string, recordID string) error {
	if m.err != nil {
		return m.err
	}
	m.deleted = append(m.deleted, recordID)
	return nil
}

func (m *mockDNSProvider) UpdateRecord(ctx context.Context, domain string, recordID string, record *dns.DNSRecord) error {
	if m.err != nil {
		return m.err
	}
	m.updated[recordID] = record
	return nil
}

type mockSSHClient struct {
	runErr      error
	runStdout   string
	runStderr   string
	mkdirErr    error
	uploadErr   error
	uploaded    []string
	closed      bool
	commandsRun []string
}

func (m *mockSSHClient) Run(cmd string) (stdout, stderr string, err error) {
	m.commandsRun = append(m.commandsRun, cmd)
	return m.runStdout, m.runStderr, m.runErr
}

func (m *mockSSHClient) RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error) {
	m.commandsRun = append(m.commandsRun, cmd)
	return m.runStdout, m.runStderr, m.runErr
}

func (m *mockSSHClient) MkdirAllSudoWithPerm(path, perm string) error {
	return m.mkdirErr
}

func (m *mockSSHClient) UploadFileSudo(localPath, remotePath string) error {
	m.uploaded = append(m.uploaded, remotePath)
	return m.uploadErr
}

func (m *mockSSHClient) UploadFileSudoWithPerm(localPath, remotePath, perm string) error {
	m.uploaded = append(m.uploaded, remotePath)
	return m.uploadErr
}

func (m *mockSSHClient) Close() error {
	m.closed = true
	return nil
}

type mockDeps struct {
	dnsProvider    DNSProvider
	dnsErr         error
	sshClient      interfaces.SSHClient
	sshErr         error
	domains        map[string]*entity.Domain
	isps           map[string]*entity.ISP
	servers        map[string]*ServerInfo
	serverEntities map[string]*entity.Server
	secrets        map[string]string
	workDir        string
	env            string
}

func newMockDeps() *mockDeps {
	return &mockDeps{
		domains:        make(map[string]*entity.Domain),
		isps:           make(map[string]*entity.ISP),
		servers:        make(map[string]*ServerInfo),
		serverEntities: make(map[string]*entity.Server),
		secrets:        make(map[string]string),
	}
}

func (m *mockDeps) DNSProvider(ispName string) (DNSProvider, error) {
	if m.dnsErr != nil {
		return nil, m.dnsErr
	}
	return m.dnsProvider, nil
}

func (m *mockDeps) Domain(name string) (*entity.Domain, bool) {
	d, ok := m.domains[name]
	return d, ok
}

func (m *mockDeps) ISP(name string) (*entity.ISP, bool) {
	isp, ok := m.isps[name]
	return isp, ok
}

func (m *mockDeps) SSHClient(server string) (interfaces.SSHClient, error) {
	if _, ok := m.servers[server]; !ok {
		return nil, domainerr.ErrServerNotRegistered
	}
	if m.sshErr != nil {
		return nil, m.sshErr
	}
	if m.sshClient == nil {
		return nil, domainerr.ErrSSHClientNotAvailable
	}
	return m.sshClient, nil
}

func (m *mockDeps) ServerInfo(name string) (*ServerInfo, bool) {
	info, ok := m.servers[name]
	return info, ok
}

func (m *mockDeps) Server(name string) (*entity.Server, bool) {
	server, ok := m.serverEntities[name]
	return server, ok
}

func (m *mockDeps) WorkDir() string { return m.workDir }
func (m *mockDeps) Env() string     { return m.env }

func (m *mockDeps) ResolveSecret(ref *valueobject.SecretRef) (string, error) {
	if ref == nil {
		return "", nil
	}
	if ref.Plain() != "" {
		return ref.Plain(), nil
	}
	if val, ok := m.secrets[ref.Secret()]; ok {
		return val, nil
	}
	return "", errors.New("secret not found: " + ref.Secret())
}

func (m *mockDeps) RegistryManager(server string) (*registry.Manager, error) {
	return nil, nil
}

func (m *mockDeps) GetAllRegistries() []*entity.Registry {
	return nil
}

func (m *mockDeps) Secrets() map[string]string {
	return m.secrets
}

func (m *mockDeps) SetServers(servers map[string]*ServerInfo) {
	m.servers = servers
}

func TestDNSHandler_EntityType(t *testing.T) {
	h := NewDNSHandler()
	if h.EntityType() != "dns_record" {
		t.Errorf("expected 'dns_record', got %s", h.EntityType())
	}
}

func TestDNSHandler_Apply_CreateRecord(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	mockProvider := newMockDNSProvider("mock")
	deps := newMockDeps()
	deps.dnsProvider = mockProvider
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "dns_record", "www.example.com").
		WithNewState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if len(mockProvider.created) != 1 {
		t.Errorf("expected 1 record created, got %d", len(mockProvider.created))
	}
}

func TestDNSHandler_Apply_DeleteRecord(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	mockProvider := newMockDNSProvider("mock")
	mockProvider.records = []dns.DNSRecord{
		{ID: "rec-123", Name: "www", Type: "A", Value: "192.168.1.1", TTL: 300},
	}
	deps := newMockDeps()
	deps.dnsProvider = mockProvider
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "dns_record", "www.example.com").
		WithOldState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if len(mockProvider.deleted) != 1 || mockProvider.deleted[0] != "rec-123" {
		t.Errorf("expected record rec-123 to be deleted, got %v", mockProvider.deleted)
	}
}

func TestDNSHandler_Apply_DeleteRecord_NotFound(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	mockProvider := newMockDNSProvider("mock")
	mockProvider.records = []dns.DNSRecord{}
	deps := newMockDeps()
	deps.dnsProvider = mockProvider
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "dns_record", "www.example.com").
		WithOldState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success (no-op for missing record), got error: %v", result.Error)
	}
}

func TestDNSHandler_Apply_UpdateRecord(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	mockProvider := newMockDNSProvider("mock")
	mockProvider.records = []dns.DNSRecord{
		{ID: "rec-123", Name: "www", Type: "A", Value: "192.168.1.1", TTL: 300},
	}
	deps := newMockDeps()
	deps.dnsProvider = mockProvider
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "dns_record", "www.example.com").
		WithNewState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.2",
			TTL:    600,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if len(mockProvider.updated) != 1 {
		t.Errorf("expected 1 record updated, got %d", len(mockProvider.updated))
	}
}

func TestDNSHandler_Apply_DomainNotFound(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "dns_record", "www.nonexistent.com").
		WithNewState(&entity.DNSRecord{
			Domain: "nonexistent.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing domain")
	}
}

func TestDNSHandler_Apply_DNSProviderError(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	deps := newMockDeps()
	deps.dnsErr = errors.New("provider unavailable")
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "dns_record", "www.example.com").
		WithNewState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for DNS provider error")
	}
}

func TestDNSHandler_Apply_InvalidChange(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "dns_record", "invalid").
		WithNewState("not a dns record")

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for invalid change")
	}
}

func TestDNSHandler_Apply_CreateError(t *testing.T) {
	h := NewDNSHandler()
	ctx := context.Background()

	mockProvider := newMockDNSProvider("mock")
	mockProvider.err = errors.New("API error")
	deps := newMockDeps()
	deps.dnsProvider = mockProvider
	deps.domains["example.com"] = &entity.Domain{Name: "example.com", DNSISP: "test-isp"}

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "dns_record", "www.example.com").
		WithNewState(&entity.DNSRecord{
			Domain: "example.com",
			Type:   entity.DNSRecordTypeA,
			Name:   "www",
			Value:  "192.168.1.1",
			TTL:    300,
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for create error")
	}
}

func TestDNSHandler_ExtractDNSRecordFromChange(t *testing.T) {
	h := &DNSHandler{}

	tests := []struct {
		name      string
		change    *valueobject.Change
		wantErr   bool
		wantName  string
		wantValue string
	}{
		{
			name:      "from new state",
			change:    valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").WithNewState(&entity.DNSRecord{Name: "www", Value: "1.2.3.4"}),
			wantErr:   false,
			wantName:  "www",
			wantValue: "1.2.3.4",
		},
		{
			name:      "from old state",
			change:    valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").WithOldState(&entity.DNSRecord{Name: "api", Value: "5.6.7.8"}),
			wantErr:   false,
			wantName:  "api",
			wantValue: "5.6.7.8",
		},
		{
			name:      "prefer new state",
			change:    valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").WithOldState(&entity.DNSRecord{Name: "old", Value: "old"}).WithNewState(&entity.DNSRecord{Name: "new", Value: "new"}),
			wantErr:   false,
			wantName:  "new",
			wantValue: "new",
		},
		{
			name:    "no state",
			change:  valueobject.NewChange(valueobject.ChangeTypeNoop, "", ""),
			wantErr: true,
		},
		{
			name:    "invalid type",
			change:  valueobject.NewChange(valueobject.ChangeTypeNoop, "", "").WithNewState("not a record"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := h.extractDNSRecordFromChange(tt.change)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr %v, got err %v", tt.wantErr, err)
				return
			}
			if !tt.wantErr {
				if record.Name != tt.wantName {
					t.Errorf("expected name %s, got %s", tt.wantName, record.Name)
				}
				if record.Value != tt.wantValue {
					t.Errorf("expected value %s, got %s", tt.wantValue, record.Value)
				}
			}
		})
	}
}
