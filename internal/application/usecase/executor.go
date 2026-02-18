package usecase

import (
	"context"
	"fmt"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/providers/dns"
)

type Executor struct {
	plan       *valueobject.Plan
	registry   *handler.Registry
	sshPool    *SSHPool
	secrets    map[string]string
	servers    map[string]*handler.ServerInfo
	env        string
	domains    map[string]*entity.Domain
	isps       map[string]*entity.ISP
	workDir    string
	dnsFactory *dns.Factory
}

func NewExecutor(pl *valueobject.Plan, env string) *Executor {
	if env == "" {
		env = "dev"
	}
	return &Executor{
		plan:       pl,
		registry:   handler.NewRegistry(),
		sshPool:    NewSSHPool(),
		secrets:    make(map[string]string),
		servers:    make(map[string]*handler.ServerInfo),
		domains:    make(map[string]*entity.Domain),
		isps:       make(map[string]*entity.ISP),
		env:        env,
		workDir:    ".",
		dnsFactory: dns.NewFactory(),
	}
}

func NewExecutorWithRegistry(pl *valueobject.Plan, env string, registry *handler.Registry) *Executor {
	e := NewExecutor(pl, env)
	e.registry = registry
	return e
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
		{"gateway", handler.NewGatewayHandler()},
		{"server", handler.NewServerHandler()},
		{"certificate", handler.NewCertificateHandler()},
		{"registry", handler.NewRegistryHandler()},
	}
	for _, h := range defaultHandlers {
		if _, ok := e.registry.Get(h.entity); !ok {
			e.registry.Register(h.handler)
		}
	}
	for _, et := range []string{"isp", "zone", "domain"} {
		if _, ok := e.registry.Get(et); !ok {
			e.registry.Register(handler.NewNoopHandler(et))
		}
	}
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

func (e *Executor) buildDeps(ch *valueobject.Change) *handler.Deps {
	deps := &handler.Deps{
		Secrets: e.secrets, Domains: e.domains, ISPs: e.isps,
		Servers: e.servers, WorkDir: e.workDir, Env: e.env, DNSFactory: e.dnsFactory,
	}
	if serverName := handler.ExtractServerFromChange(ch); serverName != "" {
		if info, ok := e.servers[serverName]; ok {
			if client, err := e.sshPool.Get(info); err == nil {
				deps.SSHClient = client
			}
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

func (e *Executor) GetRegistry() *handler.Registry { return e.registry }
