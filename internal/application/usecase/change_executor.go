package usecase

import (
	"context"
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/application/handler"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	infra "github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

type ChangeExecutorConfig struct {
	Plan       *valueobject.Plan
	SSHPool    SSHPoolInterface
	DNSFactory DNSFactoryInterface
	Env        string
}

type ChangeExecutor struct {
	plan           *valueobject.Plan
	sshPool        SSHPoolInterface
	secrets        map[string]string
	servers        map[string]*handler.ServerInfo
	serverEntities map[string]*entity.Server
	env            string
	domains        map[string]*entity.Domain
	isps           map[string]*entity.ISP
	workDir        string
	dnsFactory     DNSFactoryInterface
}

func NewChangeExecutor(cfg *ChangeExecutorConfig) *ChangeExecutor {
	if cfg == nil {
		cfg = &ChangeExecutorConfig{}
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}
	if cfg.SSHPool == nil {
		cfg.SSHPool = NewSSHPool()
	}
	if cfg.DNSFactory == nil {
		cfg.DNSFactory = infra.NewFactory()
	}

	return &ChangeExecutor{
		plan:           cfg.Plan,
		sshPool:        cfg.SSHPool,
		secrets:        make(map[string]string),
		servers:        make(map[string]*handler.ServerInfo),
		serverEntities: make(map[string]*entity.Server),
		domains:        make(map[string]*entity.Domain),
		isps:           make(map[string]*entity.ISP),
		env:            cfg.Env,
		workDir:        ".",
		dnsFactory:     cfg.DNSFactory,
	}
}

func (e *ChangeExecutor) SetSecrets(s map[string]string)                { e.secrets = s }
func (e *ChangeExecutor) SetDomains(d map[string]*entity.Domain)        { e.domains = d }
func (e *ChangeExecutor) SetISPs(i map[string]*entity.ISP)              { e.isps = i }
func (e *ChangeExecutor) SetWorkDir(w string)                           { e.workDir = w }
func (e *ChangeExecutor) SetServerEntities(s map[string]*entity.Server) { e.serverEntities = s }

func (e *ChangeExecutor) RegisterServer(name, host string, port int, user, password string) {
	e.servers[name] = &handler.ServerInfo{Host: host, Port: port, User: user, Password: password}
}

func (e *ChangeExecutor) Apply(registry handlerRegistry) []*handler.Result {
	ctx := logger.WithOperation(context.Background(), "apply")
	log := logger.FromContext(ctx)

	log.Info("starting apply", "changes", len(e.plan.Changes()))

	results := make([]*handler.Result, 0, len(e.plan.Changes()))
	for i, ch := range e.plan.Changes() {
		log.Debug("applying change",
			"index", i+1,
			"type", ch.Type(),
			"entity", ch.Entity(),
			"name", ch.Name(),
		)
		results = append(results, e.applyChange(ctx, ch, registry))
	}

	successCount := 0
	failedCount := 0
	for _, r := range results {
		if r.Error != nil {
			failedCount++
		} else {
			successCount++
		}
	}

	log.Info("apply completed",
		"total", len(results),
		"success", successCount,
		"failed", failedCount,
	)

	e.sshPool.CloseAll()
	return results
}

func (e *ChangeExecutor) applyChange(ctx context.Context, ch *valueobject.Change, registry handlerRegistry) *handler.Result {
	log := logger.FromContext(ctx)

	h, ok := registry.Get(ch.Entity())
	if !ok {
		log.Error("no handler found", "entity", ch.Entity())
		return &handler.Result{Change: ch, Error: fmt.Errorf("no handler for: %s", ch.Entity())}
	}

	result, err := h.Apply(ctx, ch, e.buildDeps(ch))
	if err != nil {
		log.Error("change failed", "entity", ch.Entity(), "name", ch.Name(), "error", err)
		return &handler.Result{Change: ch, Error: err}
	}

	log.Debug("change applied", "entity", ch.Entity(), "name", ch.Name())
	return result
}

func (e *ChangeExecutor) buildDeps(ch *valueobject.Change) *handler.BaseDeps {
	opts := []handler.BaseDepsOption{
		handler.WithSecrets(e.secrets),
		handler.WithDomains(e.domains),
		handler.WithISPs(e.isps),
		handler.WithServers(e.servers),
		handler.WithServerEntities(e.serverEntities),
		handler.WithWorkDir(e.workDir),
		handler.WithEnv(e.env),
		handler.WithDNSFactory(e.dnsFactory),
	}

	if serverName := handler.ExtractServerFromChange(ch); serverName != "" {
		if info, ok := e.servers[serverName]; ok {
			client, err := e.sshPool.Get(info)
			opts = append(opts, handler.WithSSHClient(client, err))
		}
	}
	return handler.NewBaseDeps(opts...)
}

func (e *ChangeExecutor) FilterPlanByServer(serverName string) *valueobject.Plan {
	filtered := valueobject.NewPlan()
	for _, ch := range e.plan.Changes() {
		if handler.ExtractServerFromChange(ch) == serverName {
			filtered.AddChange(ch)
		}
	}
	return filtered
}

func (e *ChangeExecutor) GetSSHPool() SSHPoolInterface {
	return e.sshPool
}
