package usecase

import (
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	infra "github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
)

type RegistryInterface interface {
	Register(h Handler)
	Get(entityType string) (Handler, bool)
}

type SSHPoolInterface interface {
	Get(info *ServerInfo) (contract.SSHClient, error)
	CloseAll()
}

type DNSFactoryInterface interface {
	Create(isp *entity.ISP, secrets map[string]string) (infra.Provider, error)
}

type ExecutorConfig struct {
	Registry   RegistryInterface
	SSHPool    SSHPoolInterface
	DNSFactory DNSFactoryInterface
	Plan       *valueobject.Plan
	Env        string
}

type Executor struct {
	handlerRegistry *Registry
	changeExecutor  *ChangeExecutor
	plan            *valueobject.Plan
	env             string
}

func NewExecutor(cfg *ExecutorConfig) *Executor {
	if cfg == nil {
		cfg = &ExecutorConfig{}
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	var hr *Registry
	if cfg.Registry != nil {
		hr = NewRegistry()
	} else {
		hr = NewRegistry()
	}

	sshPool := cfg.SSHPool
	if sshPool == nil {
		sshPool = NewSSHPool()
	}

	dnsFactory := cfg.DNSFactory
	if dnsFactory == nil {
		dnsFactory = infra.NewFactory()
	}

	changeExecutorCfg := &ChangeExecutorConfig{
		Plan:       cfg.Plan,
		SSHPool:    sshPool,
		DNSFactory: dnsFactory,
		Env:        cfg.Env,
	}

	return &Executor{
		handlerRegistry: hr,
		changeExecutor:  NewChangeExecutor(changeExecutorCfg),
		plan:            cfg.Plan,
		env:             cfg.Env,
	}
}

func (e *Executor) SetSecrets(s map[string]string) {
	e.changeExecutor.SetSecrets(s)
}

func (e *Executor) SetDomains(d map[string]*entity.Domain) {
	e.changeExecutor.SetDomains(d)
}

func (e *Executor) SetISPs(i map[string]*entity.ISP) {
	e.changeExecutor.SetISPs(i)
}

func (e *Executor) SetWorkDir(w string) {
	e.changeExecutor.SetWorkDir(w)
}

func (e *Executor) SetServerEntities(s map[string]*entity.Server) {
	e.changeExecutor.SetServerEntities(s)
}

func (e *Executor) RegisterServer(name, host string, port int, user, password string, strictHostKeyChecking bool) {
	e.changeExecutor.RegisterServer(name, host, port, user, password, strictHostKeyChecking)
}

func (e *Executor) Apply() []*Result {
	e.RegisterDefaults()
	return e.changeExecutor.Apply(e.handlerRegistry)
}

func (e *Executor) FilterPlanByServer(serverName string) *valueobject.Plan {
	return e.changeExecutor.FilterPlanByServer(serverName)
}

func (e *Executor) GetRegistry() RegistryInterface {
	return e.handlerRegistry
}

func (e *Executor) GetHandlerRegistry() *Registry {
	return e.handlerRegistry
}

func (e *Executor) GetChangeExecutor() *ChangeExecutor {
	return e.changeExecutor
}

func (e *Executor) RegisterDefaults() {
	defaultHandlers := []Handler{
		NewDNSHandler(),
		NewServiceHandler(),
		NewInfraServiceHandler(),
		NewServerHandler(),
	}
	for _, h := range defaultHandlers {
		if _, ok := e.handlerRegistry.Get(h.EntityType()); !ok {
			e.handlerRegistry.Register(h)
		}
	}
	RegisterNoopHandlers(e.handlerRegistry)
}
