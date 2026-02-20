package usecase

import (
	"context"
	"fmt"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	infra "github.com/litelake/yamlops/internal/infrastructure/dns"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type RegistryInterface interface {
	Register(h handler.Handler)
	Get(entityType string) (handler.Handler, bool)
}

type SSHPoolInterface interface {
	Get(info *handler.ServerInfo) (handler.SSHClient, error)
	CloseAll()
}

type DNSFactoryInterface interface {
	Create(isp *entity.ISP, secrets map[string]string) (dns.Provider, error)
}

type ExecutorConfig struct {
	Registry   RegistryInterface
	SSHPool    SSHPoolInterface
	DNSFactory DNSFactoryInterface
	Plan       *valueobject.Plan
	Env        string
}

type Executor struct {
	plan       *valueobject.Plan
	registry   RegistryInterface
	sshPool    SSHPoolInterface
	secrets    map[string]string
	servers    map[string]*handler.ServerInfo
	env        string
	domains    map[string]*entity.Domain
	isps       map[string]*entity.ISP
	workDir    string
	dnsFactory DNSFactoryInterface
}

func NewExecutor(cfg *ExecutorConfig) *Executor {
	if cfg == nil {
		cfg = &ExecutorConfig{}
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}
	if cfg.Registry == nil {
		cfg.Registry = handler.NewRegistry()
	}
	if cfg.SSHPool == nil {
		cfg.SSHPool = NewSSHPool()
	}
	if cfg.DNSFactory == nil {
		cfg.DNSFactory = infra.NewFactory()
	}

	return &Executor{
		plan:       cfg.Plan,
		registry:   cfg.Registry,
		sshPool:    cfg.SSHPool,
		secrets:    make(map[string]string),
		servers:    make(map[string]*handler.ServerInfo),
		domains:    make(map[string]*entity.Domain),
		isps:       make(map[string]*entity.ISP),
		env:        cfg.Env,
		workDir:    ".",
		dnsFactory: cfg.DNSFactory,
	}
}

func (e *Executor) SetSecrets(s map[string]string)         { e.secrets = s }
func (e *Executor) SetDomains(d map[string]*entity.Domain) { e.domains = d }
func (e *Executor) SetISPs(i map[string]*entity.ISP)       { e.isps = i }
func (e *Executor) SetWorkDir(w string)                    { e.workDir = w }

func (e *Executor) RegisterServer(name, host string, port int, user, password string) {
	e.servers[name] = &handler.ServerInfo{Host: host, Port: port, User: user, Password: password}
}

func (e *Executor) Apply() []*handler.Result {
	e.registerHandlers()
	results := make([]*handler.Result, 0, len(e.plan.Changes))
	ctx := context.Background()
	for _, ch := range e.plan.Changes {
		results = append(results, e.applyChange(ctx, ch))
	}
	e.sshPool.CloseAll()
	return results
}

func (e *Executor) registerHandlers() {
	defaultHandlers := []struct {
		entity  string
		handler handler.Handler
	}{
		{"dns_record", handler.NewDNSHandler()},
		{"service", handler.NewServiceHandler()},
		{"infra_service", handler.NewInfraServiceHandler()},
		{"server", handler.NewServerHandler()},
	}
	for _, h := range defaultHandlers {
		if _, ok := e.registry.Get(h.entity); !ok {
			e.registry.Register(h.handler)
		}
	}
	handler.RegisterNoopHandlers(e.registry)
}

func (e *Executor) applyChange(ctx context.Context, ch *valueobject.Change) *handler.Result {
	h, ok := e.registry.Get(ch.Entity)
	if !ok {
		return &handler.Result{Change: ch, Error: fmt.Errorf("no handler for: %s", ch.Entity)}
	}
	result, err := h.Apply(ctx, ch, e.buildDeps(ch))
	if err != nil {
		return &handler.Result{Change: ch, Error: err}
	}
	return result
}

func (e *Executor) buildDeps(ch *valueobject.Change) *handler.BaseDeps {
	deps := handler.NewBaseDeps()
	deps.SetSecrets(e.secrets)
	deps.SetDomains(e.domains)
	deps.SetISPs(e.isps)
	deps.SetServers(e.servers)
	deps.SetWorkDir(e.workDir)
	deps.SetEnv(e.env)
	deps.SetDNSFactory(e.dnsFactory)

	if serverName := handler.ExtractServerFromChange(ch); serverName != "" {
		if info, ok := e.servers[serverName]; ok {
			client, err := e.sshPool.Get(info)
			deps.SetSSHClient(client, err)
		}
	}
	return deps
}

func (e *Executor) FilterPlanByServer(serverName string) *valueobject.Plan {
	filtered := valueobject.NewPlan()
	for _, ch := range e.plan.Changes {
		if handler.ExtractServerFromChange(ch) == serverName {
			filtered.AddChange(ch)
		}
	}
	return filtered
}

func (e *Executor) GetRegistry() RegistryInterface { return e.registry }
