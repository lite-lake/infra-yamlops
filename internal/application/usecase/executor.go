package usecase

import (
	"context"
	"fmt"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	"github.com/litelake/yamlops/internal/ssh"
)

type Executor struct {
	plan       *valueobject.Plan
	registry   *handler.Registry
	sshClients map[string]*ssh.Client
	secrets    map[string]string
	servers    map[string]*handler.ServerInfo
	env        string
	domains    map[string]*entity.Domain
	isps       map[string]*entity.ISP
	workDir    string
}

func NewExecutor(pl *valueobject.Plan, env string) *Executor {
	if env == "" {
		env = "dev"
	}
	return &Executor{
		plan:       pl,
		registry:   handler.NewRegistry(),
		sshClients: make(map[string]*ssh.Client),
		secrets:    make(map[string]string),
		servers:    make(map[string]*handler.ServerInfo),
		domains:    make(map[string]*entity.Domain),
		isps:       make(map[string]*entity.ISP),
		env:        env,
		workDir:    ".",
	}
}

func NewExecutorWithRegistry(pl *valueobject.Plan, env string, registry *handler.Registry) *Executor {
	e := NewExecutor(pl, env)
	e.registry = registry
	return e
}

func (e *Executor) SetSecrets(secrets map[string]string) {
	e.secrets = secrets
}

func (e *Executor) SetDomains(domains map[string]*entity.Domain) {
	e.domains = domains
}

func (e *Executor) SetISPs(isps map[string]*entity.ISP) {
	e.isps = isps
}

func (e *Executor) SetWorkDir(workDir string) {
	e.workDir = workDir
}

func (e *Executor) RegisterServer(name string, host string, port int, user, password string) {
	e.servers[name] = &handler.ServerInfo{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}
}

func (e *Executor) Apply() []*handler.Result {
	results := make([]*handler.Result, 0, len(e.plan.Changes))
	ctx := context.Background()

	e.registerDefaultHandlers()

	for _, ch := range e.plan.Changes {
		result := e.applyChange(ctx, ch)
		results = append(results, result)
	}

	e.closeClients()
	return results
}

func (e *Executor) registerDefaultHandlers() {
	if _, ok := e.registry.Get("dns_record"); !ok {
		e.registry.Register(handler.NewDNSHandler())
	}
	if _, ok := e.registry.Get("service"); !ok {
		e.registry.Register(handler.NewServiceHandler())
	}
	if _, ok := e.registry.Get("gateway"); !ok {
		e.registry.Register(handler.NewGatewayHandler())
	}
	if _, ok := e.registry.Get("server"); !ok {
		e.registry.Register(handler.NewServerHandler())
	}
	if _, ok := e.registry.Get("certificate"); !ok {
		e.registry.Register(handler.NewCertificateHandler())
	}
	if _, ok := e.registry.Get("registry"); !ok {
		e.registry.Register(handler.NewRegistryHandler())
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
		return &handler.Result{Change: ch, Error: fmt.Errorf("no handler registered for entity type: %s", ch.Entity)}
	}
	deps := e.buildDeps(ch)
	result, err := h.Apply(ctx, ch, deps)
	if err != nil {
		return &handler.Result{Change: ch, Error: err}
	}
	return result
}

func (e *Executor) buildDeps(ch *valueobject.Change) *handler.Deps {
	deps := &handler.Deps{
		Secrets:   e.secrets,
		Domains:   e.domains,
		ISPs:      e.isps,
		Servers:   e.servers,
		WorkDir:   e.workDir,
		Env:       e.env,
		SSHClient: nil,
	}

	serverName := e.extractServerFromChange(ch)
	if serverName != "" {
		if client, err := e.getClient(serverName); err == nil {
			deps.SSHClient = client
		}
	}

	return deps
}

func (e *Executor) extractServerFromChange(ch *valueobject.Change) string {
	return handler.ExtractServerFromChange(ch)
}

func (e *Executor) getClient(serverName string) (handler.SSHClient, error) {
	if client, ok := e.sshClients[serverName]; ok {
		return client, nil
	}

	info, ok := e.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("server %s not registered", serverName)
	}

	client, err := ssh.NewClient(info.Host, info.Port, info.User, info.Password)
	if err != nil {
		return nil, err
	}

	e.sshClients[serverName] = client
	return client, nil
}

func (e *Executor) closeClients() {
	for _, client := range e.sshClients {
		client.Close()
	}
	e.sshClients = make(map[string]*ssh.Client)
}

func (e *Executor) FilterPlanByServer(serverName string) *valueobject.Plan {
	filtered := valueobject.NewPlan()
	for _, ch := range e.plan.Changes {
		s := e.extractServerFromChange(ch)
		if s == serverName {
			filtered.AddChange(ch)
		}
	}
	return filtered
}

func (e *Executor) GetRegistry() *handler.Registry {
	return e.registry
}
