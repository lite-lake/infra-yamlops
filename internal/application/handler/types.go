package handler

import (
	"context"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/infrastructure/registry"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type DNSDeps interface {
	DNSProvider(ispName string) (DNSProvider, error)
	Domain(name string) (*entity.Domain, bool)
	ISP(name string) (*entity.ISP, bool)
}

type ServiceDeps interface {
	SSHClient(server string) (SSHClient, error)
	ServerInfo(name string) (*ServerInfo, bool)
	Server(name string) (*entity.Server, bool)
	WorkDir() string
	Env() string
	RegistryManager(server string) (*registry.Manager, error)
	GetAllRegistries() []*entity.Registry
	Secrets() map[string]string
}

type CommonDeps interface {
	ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

type DepsProvider interface {
	DNSDeps
	ServiceDeps
	CommonDeps
}

type DNSFactory interface {
	Create(isp *entity.ISP, secrets map[string]string) (dns.Provider, error)
}

type Handler interface {
	EntityType() string
	Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

type BaseDeps struct {
	sshClient      SSHClient
	sshError       error
	dnsFactory     DNSFactory
	secrets        map[string]string
	domains        map[string]*entity.Domain
	isps           map[string]*entity.ISP
	servers        map[string]*ServerInfo
	serverEntities map[string]*entity.Server
	registries     map[string]*entity.Registry
	workDir        string
	env            string
}

func NewBaseDeps() *BaseDeps {
	return &BaseDeps{
		secrets:        make(map[string]string),
		domains:        make(map[string]*entity.Domain),
		isps:           make(map[string]*entity.ISP),
		servers:        make(map[string]*ServerInfo),
		serverEntities: make(map[string]*entity.Server),
		registries:     make(map[string]*entity.Registry),
	}
}

func (d *BaseDeps) SetSSHClient(client SSHClient, err error) {
	d.sshClient = client
	d.sshError = err
}

func (d *BaseDeps) SetDNSFactory(f DNSFactory)     { d.dnsFactory = f }
func (d *BaseDeps) SetSecrets(s map[string]string) { d.secrets = s }
func (d *BaseDeps) SetDomains(domains map[string]*entity.Domain) {
	d.domains = domains
}
func (d *BaseDeps) SetISPs(isps map[string]*entity.ISP) { d.isps = isps }
func (d *BaseDeps) SetServers(servers map[string]*ServerInfo) {
	d.servers = servers
}
func (d *BaseDeps) SetServerEntities(servers map[string]*entity.Server) {
	d.serverEntities = servers
}
func (d *BaseDeps) SetWorkDir(w string)                         { d.workDir = w }
func (d *BaseDeps) SetEnv(e string)                             { d.env = e }
func (d *BaseDeps) SetRegistries(r map[string]*entity.Registry) { d.registries = r }

func (d *BaseDeps) GetAllRegistries() []*entity.Registry {
	var result []*entity.Registry
	for _, r := range d.registries {
		result = append(result, r)
	}
	return result
}

func (d *BaseDeps) RegistryManager(server string) (*registry.Manager, error) {
	if _, ok := d.servers[server]; !ok {
		return nil, domainerr.ErrServerNotRegistered
	}
	if d.sshClient == nil {
		if d.sshError != nil {
			return nil, d.sshError
		}
		return nil, domainerr.ErrSSHClientNotAvailable
	}

	// Convert map to slice
	var registryList []*entity.Registry
	for _, r := range d.registries {
		registryList = append(registryList, r)
	}

	return registry.NewManager(d.sshClient, registryList, d.secrets), nil
}

func (d *BaseDeps) DNSProvider(ispName string) (DNSProvider, error) {
	isp, ok := d.isps[ispName]
	if !ok {
		return nil, domainerr.ErrISPNotFound
	}
	if !isp.HasService(entity.ISPServiceDNS) {
		return nil, domainerr.ErrISPNoDNSService
	}
	provider, err := d.dnsFactory.Create(isp, d.secrets)
	if err != nil {
		return nil, err
	}
	return WrapDNSProvider(provider), nil
}

func (d *BaseDeps) Domain(name string) (*entity.Domain, bool) {
	domain, ok := d.domains[name]
	return domain, ok
}

func (d *BaseDeps) ISP(name string) (*entity.ISP, bool) {
	isp, ok := d.isps[name]
	return isp, ok
}

func (d *BaseDeps) SSHClient(server string) (SSHClient, error) {
	if _, ok := d.servers[server]; !ok {
		return nil, domainerr.ErrServerNotRegistered
	}
	if d.sshClient == nil {
		if d.sshError != nil {
			return nil, d.sshError
		}
		return nil, domainerr.ErrSSHClientNotAvailable
	}
	return d.sshClient, nil
}

func (d *BaseDeps) ServerInfo(name string) (*ServerInfo, bool) {
	info, ok := d.servers[name]
	return info, ok
}

func (d *BaseDeps) Server(name string) (*entity.Server, bool) {
	server, ok := d.serverEntities[name]
	return server, ok
}

func (d *BaseDeps) WorkDir() string { return d.workDir }
func (d *BaseDeps) Env() string     { return d.env }

func (d *BaseDeps) ResolveSecret(ref *valueobject.SecretRef) (string, error) {
	return ref.Resolve(d.secrets)
}

func (d *BaseDeps) RawSSHClient() SSHClient    { return d.sshClient }
func (d *BaseDeps) RawSSHError() error         { return d.sshError }
func (d *BaseDeps) Secrets() map[string]string { return d.secrets }

type ServerInfo struct {
	Host     string
	Port     int
	User     string
	Password string
}

type Result struct {
	Change   *valueobject.Change
	Success  bool
	Error    error
	Output   string
	Warnings []string
}

type SSHClient interface {
	Run(cmd string) (stdout, stderr string, err error)
	RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error)
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
