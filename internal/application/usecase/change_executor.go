package usecase

import (
	"context"
	"fmt"
	"sync"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	infra "github.com/lite-lake/infra-yamlops/internal/infrastructure/dns"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

// ChangeExecutorConfig 变更执行器配置
type ChangeExecutorConfig struct {
	Plan       *valueobject.Plan
	SSHPool    SSHPoolInterface
	DNSFactory DNSFactoryInterface
	Env        string
}

// ChangeExecutor 负责执行计划中的变更
type ChangeExecutor struct {
	plan           *valueobject.Plan
	sshPool        SSHPoolInterface
	secrets        map[string]string
	servers        map[string]*ServerInfo
	serverEntities map[string]*entity.Server
	env            string
	domains        map[string]*entity.Domain
	isps           map[string]*entity.ISP
	workDir        string
	dnsFactory     DNSFactoryInterface
}

// handlerRegistry 处理器注册表接口
type handlerRegistry interface {
	Register(h Handler)
	Get(entityType string) (Handler, bool)
}

// NewChangeExecutor 创建变更执行器
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
		servers:        make(map[string]*ServerInfo),
		serverEntities: make(map[string]*entity.Server),
		domains:        make(map[string]*entity.Domain),
		isps:           make(map[string]*entity.ISP),
		env:            cfg.Env,
		workDir:        ".",
		dnsFactory:     cfg.DNSFactory,
	}
}

// SetSecrets 设置密钥
func (e *ChangeExecutor) SetSecrets(s map[string]string) { e.secrets = s }

// SetDomains 设置域名
func (e *ChangeExecutor) SetDomains(d map[string]*entity.Domain) { e.domains = d }

// SetISPs 设置 ISP
func (e *ChangeExecutor) SetISPs(i map[string]*entity.ISP) { e.isps = i }

// SetWorkDir 设置工作目录
func (e *ChangeExecutor) SetWorkDir(w string) { e.workDir = w }

// SetServerEntities 设置服务器实体
func (e *ChangeExecutor) SetServerEntities(s map[string]*entity.Server) { e.serverEntities = s }

// RegisterServer 注册服务器
func (e *ChangeExecutor) RegisterServer(name, host string, port int, user, password string, strictHostKeyChecking bool) {
	e.servers[name] = &ServerInfo{Host: host, Port: port, User: user, Password: password, StrictHostKeyChecking: strictHostKeyChecking}
}

// Apply 执行所有变更，对独立变更进行并行执行
func (e *ChangeExecutor) Apply(registry handlerRegistry) []*Result {
	ctx := logger.WithOperation(context.Background(), "apply")
	log := logger.FromContext(ctx)

	log.Info("starting apply", "changes", len(e.plan.Changes()))

	// 按服务器分组变更，不同服务器的变更可以并行执行
	serverGroups := make(map[string][]*valueobject.Change)
	dnsChanges := make([]*valueobject.Change, 0)

	for _, ch := range e.plan.Changes() {
		serverName := ExtractServerFromChange(ch)
		if serverName != "" {
			serverGroups[serverName] = append(serverGroups[serverName], ch)
		} else if ch.Entity() == "dns" || ch.Entity() == "domain" || ch.Entity() == "record" || ch.Entity() == "dns_record" || ch.Entity() == "isp" || ch.Entity() == "zone" {
			dnsChanges = append(dnsChanges, ch)
		} else {
			// 其他类型的变更单独作为一组
			serverGroups[fmt.Sprintf("other-%d", len(serverGroups))] = []*valueobject.Change{ch}
		}
	}

	// 添加 DNS 变更组
	if len(dnsChanges) > 0 {
		serverGroups["dns"] = dnsChanges
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]*Result, 0, len(e.plan.Changes()))

	// 为每个组启动一个 goroutine
	for groupName, changes := range serverGroups {
		wg.Add(1)
		go func(name string, groupChanges []*valueobject.Change) {
			defer wg.Done()
			groupResults := e.applyChangesGroup(ctx, name, groupChanges, registry)
			mu.Lock()
			results = append(results, groupResults...)
			mu.Unlock()
		}(groupName, changes)
	}

	wg.Wait()

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

// applyChangesGroup 顺序执行一组变更
func (e *ChangeExecutor) applyChangesGroup(ctx context.Context, groupName string, changes []*valueobject.Change, registry handlerRegistry) []*Result {
	log := logger.FromContext(ctx)
	log.Debug("processing change group", "group", groupName, "changes", len(changes))

	// 对 DNS 变更组进行特殊排序：Delete → Update → Create
	sortedChanges := changes
	if groupName == "dns" {
		sortedChanges = sortDNSChanges(changes)
		log.Debug("sorted DNS changes", "count", len(sortedChanges))
	}

	results := make([]*Result, 0, len(sortedChanges))
	for i, ch := range sortedChanges {
		log.Debug("applying change",
			"group", groupName,
			"index", i+1,
			"type", ch.Type(),
			"entity", ch.Entity(),
			"name", ch.Name(),
		)
		results = append(results, e.applyChange(ctx, ch, registry))
	}
	return results
}

// sortDNSChanges 按照正确的执行顺序排序 DNS 变更：Delete → Update → Create
func sortDNSChanges(changes []*valueobject.Change) []*valueobject.Change {
	var deletes, updates, creates []*valueobject.Change
	for _, ch := range changes {
		switch ch.Type() {
		case valueobject.ChangeTypeDelete:
			deletes = append(deletes, ch)
		case valueobject.ChangeTypeUpdate:
			updates = append(updates, ch)
		case valueobject.ChangeTypeCreate:
			creates = append(creates, ch)
		default:
			// 其他类型放到最后
			creates = append(creates, ch)
		}
	}
	// 合并结果：Delete first, then Update, then Create
	result := make([]*valueobject.Change, 0, len(changes))
	result = append(result, deletes...)
	result = append(result, updates...)
	result = append(result, creates...)
	return result
}

// applyChange 执行单个变更
func (e *ChangeExecutor) applyChange(ctx context.Context, ch *valueobject.Change, registry handlerRegistry) *Result {
	log := logger.FromContext(ctx)

	h, ok := registry.Get(ch.Entity())
	if !ok {
		log.Error("no handler found", "entity", ch.Entity())
		return &Result{Change: ch, Error: fmt.Errorf("no handler for: %s", ch.Entity())}
	}

	result, err := h.Apply(ctx, ch, e.buildDeps(ch))
	if err != nil {
		log.Error("change failed", "entity", ch.Entity(), "name", ch.Name(), "error", err)
		return &Result{Change: ch, Error: err}
	}

	log.Debug("change applied", "entity", ch.Entity(), "name", ch.Name())
	return result
}

// buildDeps 构建变更处理器依赖
func (e *ChangeExecutor) buildDeps(ch *valueobject.Change) *BaseDeps {
	opts := []BaseDepsOption{
		WithSecrets(e.secrets),
		WithDomains(e.domains),
		WithISPs(e.isps),
		WithServers(e.servers),
		WithServerEntities(e.serverEntities),
		WithWorkDir(e.workDir),
		WithEnv(e.env),
		WithDNSFactory(e.dnsFactory),
	}

	if serverName := ExtractServerFromChange(ch); serverName != "" {
		if info, ok := e.servers[serverName]; ok {
			client, err := e.sshPool.Get(info)
			opts = append(opts, WithSSHClient(client, err))
		}
	}
	return NewBaseDeps(opts...)
}

// FilterPlanByServer 按服务器过滤计划
func (e *ChangeExecutor) FilterPlanByServer(serverName string) *valueobject.Plan {
	filtered := valueobject.NewPlan()
	for _, ch := range e.plan.Changes() {
		if ExtractServerFromChange(ch) == serverName {
			filtered.AddChange(ch)
		}
	}
	return filtered
}

// GetSSHPool 获取 SSH 池
func (e *ChangeExecutor) GetSSHPool() SSHPoolInterface {
	return e.sshPool
}
