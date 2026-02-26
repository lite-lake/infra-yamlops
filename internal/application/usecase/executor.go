package usecase

import (
	"github.com/lite-lake/infra-yamlops/internal/application/handler"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	infra "github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
)

type RegistryInterface interface {
	Register(h handler.Handler)
	Get(entityType string) (handler.Handler, bool)
}

type SSHPoolInterface interface {
	Get(info *handler.ServerInfo) (contract.SSHClient, error)
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
	handlerRegistry *HandlerRegistry
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

	var hr *HandlerRegistry
	if cfg.Registry != nil {
		hr = NewHandlerRegistryWithRegistry(cfg.Registry)
	} else {
		hr = NewHandlerRegistry()
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

func (e *Executor) RegisterServer(name, host string, port int, user, password string) {
	e.changeExecutor.RegisterServer(name, host, port, user, password)
}

func (e *Executor) Apply() []*handler.Result {
	e.handlerRegistry.RegisterDefaults()
	return e.changeExecutor.Apply(e.handlerRegistry.Registry())
}

func (e *Executor) FilterPlanByServer(serverName string) *valueobject.Plan {
	return e.changeExecutor.FilterPlanByServer(serverName)
}

func (e *Executor) GetRegistry() RegistryInterface {
	return e.handlerRegistry.Registry()
}

func (e *Executor) GetHandlerRegistry() *HandlerRegistry {
	return e.handlerRegistry
}

func (e *Executor) GetChangeExecutor() *ChangeExecutor {
	return e.changeExecutor
}
