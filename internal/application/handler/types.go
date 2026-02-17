package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/providers/dns"
	"github.com/litelake/yamlops/internal/ssh"
)

type Handler interface {
	EntityType() string
	Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error)
}

type Deps struct {
	SSHClient   SSHClient
	DNSProvider DNSProvider
	Secrets     map[string]string
	Domains     map[string]*entity.Domain
	ISPs        map[string]*entity.ISP
	Servers     map[string]*ServerInfo
	WorkDir     string
	Env         string
}

type ServerInfo struct {
	Host     string
	Port     int
	User     string
	Password string
}

type Result struct {
	Change  *valueobject.Change
	Success bool
	Error   error
	Output  string
}

type SSHClient interface {
	Run(cmd string) (stdout, stderr string, err error)
	MkdirAllSudoWithPerm(path, perm string) error
	UploadFileSudo(localPath, remotePath string) error
	UploadFileSudoWithPerm(localPath, remotePath, perm string) error
	Close() error
}

type DNSProvider interface {
	Name() string
	ListRecords(domain string) ([]dns.DNSRecord, error)
	CreateRecord(domain string, record *dns.DNSRecord) error
	DeleteRecord(domain string, recordID string) error
	UpdateRecord(domain string, recordID string, record *dns.DNSRecord) error
}

var (
	_ SSHClient   = (*ssh.Client)(nil)
	_ DNSProvider = (*dnsAdapter)(nil)
)

type dnsAdapter struct {
	provider dns.Provider
}

func (a *dnsAdapter) Name() string {
	return a.provider.Name()
}

func (a *dnsAdapter) ListRecords(domain string) ([]dns.DNSRecord, error) {
	return a.provider.ListRecords(domain)
}

func (a *dnsAdapter) CreateRecord(domain string, record *dns.DNSRecord) error {
	return a.provider.CreateRecord(domain, record)
}

func (a *dnsAdapter) DeleteRecord(domain string, recordID string) error {
	return a.provider.DeleteRecord(domain, recordID)
}

func (a *dnsAdapter) UpdateRecord(domain string, recordID string, record *dns.DNSRecord) error {
	return a.provider.UpdateRecord(domain, recordID, record)
}

func WrapDNSProvider(p dns.Provider) DNSProvider {
	return &dnsAdapter{provider: p}
}
