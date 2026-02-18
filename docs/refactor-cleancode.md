# YAMLOps 代码改造方案

## 文档信息

| 项目 | 内容 |
|------|------|
| 创建日期 | 2026-02-19 |
| 目标版本 | v3.0 |
| 依据原则 | Clean Code + SOLID |
| 状态 | 待执行 |

---

## 一、当前代码评估

### 1.1 SOLID 原则遵循评估

| 原则 | 评分 | 问题 |
|------|------|------|
| **S - 单一职责** | ⭐⭐⭐ | ConfigLoader 职责过多（加载+验证）；Executor 混合了注册、执行、连接管理 |
| **O - 开闭原则** | ⭐⭐ | DNS Provider 选择用 switch-case，新增 ISP 需修改 dns_handler.go |
| **L - 里氏替换** | ⭐⭐⭐⭐ | Provider 接口实现良好 |
| **I - 接口隔离** | ⭐⭐⭐ | Deps 结构体包含过多字段，部分 Handler 只用到其中几项 |
| **D - 依赖倒置** | ⭐⭐⭐⭐ | Repository 接口定义在 Domain 层，实现良好 |

### 1.2 Clean Code 问题清单

#### 1.2.1 代码异味

| 类型 | 位置 | 描述 |
|------|------|------|
| **Long Method** | `config_loader.go` | 验证函数分散在 480 行文件中，单个验证函数过长 |
| **Switch Statement** | `dns_handler.go:80-121` | getDNSProvider 使用 switch 违反 OCP |
| **Feature Envy** | `secret_ref.go` | Resolve 方法访问外部 map |
| **Primitive Obsession** | 多处 | 大量使用 string 而非类型别名 |
| **Duplicated Code** | `planner.go` | PlanISPs/PlanZones 等方法模式重复 |
| **Global State** | `root.go:10-19` | CLI 使用全局变量传递参数 |

#### 1.2.2 安全问题

| 问题 | 位置 | 风险等级 |
|------|------|----------|
| **InsecureIgnoreHostKey** | `ssh/client.go:23` | 🔴 高 - 可能导致中间人攻击 |

#### 1.2.3 架构问题

| 问题 | 描述 |
|------|------|
| **验证逻辑错位** | 验证逻辑在 Infrastructure 层，应在 Domain 层 |
| **Handler 职责不清** | Handler 直接处理 YAML 解析和 Provider 创建 |
| **Executor 臃肿** | 混合了注册、执行、连接池管理职责 |
| **缺少工厂模式** | Provider 创建散落在 Handler 中 |

---

## 二、改造目标

### 2.1 量化指标

| 指标 | 当前 | 目标 |
|------|------|------|
| 单文件行数 | 480 行 (max) | ≤ 200 行 |
| 函数行数 | 50+ 行 | ≤ 30 行 |
| 圈复杂度 | 15+ | ≤ 10 |
| 测试覆盖率 | ~40% | ≥ 80% |
| 依赖注入 | 部分 | 完全 |

### 2.2 设计原则

1. **SRP**: 每个文件/结构体只有一个变更理由
2. **OCP**: 通过工厂模式和策略模式支持扩展
3. **DIP**: 高层模块依赖抽象接口
4. **小函数**: 每个函数只做一件事
5. **有意义的命名**: 变量/函数名表达意图

---

## 三、改造方案详解

### 3.1 架构重构

#### 3.1.1 新增 Provider Factory

**问题**: `dns_handler.go:80-121` 使用 switch-case 创建 Provider

**方案**: 引入工厂模式

```go
// internal/providers/dns/factory.go
package dns

import (
    "github.com/litelake/yamlops/internal/domain/entity"
)

type CreatorFunc func(isp *entity.ISP, secrets map[string]string) (Provider, error)

type Factory struct {
    creators map[string]CreatorFunc
}

func NewFactory() *Factory {
    return &Factory{
        creators: map[string]CreatorFunc{
            entity.ISPTypeCloudflare: createCloudflare,
            entity.ISPTypeAliyun:     createAliyun,
            entity.ISPTypeTencent:    createTencent,
        },
    }
}

func (f *Factory) Create(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    creator, ok := f.creators[isp.Type]
    if !ok {
        return nil, fmt.Errorf("unsupported provider type: %s", isp.Type)
    }
    return creator(isp, secrets)
}

func (f *Factory) Register(providerType string, creator CreatorFunc) {
    f.creators[providerType] = creator
}

func createCloudflare(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    apiToken, err := isp.Credentials["api_token"].Resolve(secrets)
    if err != nil {
        return nil, fmt.Errorf("resolve api_token: %w", err)
    }
    return NewCloudflareProvider(apiToken, ""), nil
}

func createAliyun(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    accessKeyID, err := isp.Credentials["access_key_id"].Resolve(secrets)
    if err != nil {
        return nil, fmt.Errorf("resolve access_key_id: %w", err)
    }
    accessKeySecret, err := isp.Credentials["access_key_secret"].Resolve(secrets)
    if err != nil {
        return nil, fmt.Errorf("resolve access_key_secret: %w", err)
    }
    return NewAliyunProvider(accessKeyID, accessKeySecret), nil
}

func createTencent(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    secretID, err := isp.Credentials["secret_id"].Resolve(secrets)
    if err != nil {
        return nil, fmt.Errorf("resolve secret_id: %w", err)
    }
    secretKey, err := isp.Credentials["secret_key"].Resolve(secrets)
    if err != nil {
        return nil, fmt.Errorf("resolve secret_key: %w", err)
    }
    return NewTencentProvider(secretID, secretKey), nil
}
```

#### 3.1.2 验证逻辑迁移至 Domain 层

**问题**: 验证逻辑在 `config_loader.go`，应在 Domain 层

**方案**: 创建 `domain/service/validator.go`

```go
// internal/domain/service/validator.go
package service

import (
    "github.com/litelake/yamlops/internal/domain/entity"
)

type Validator struct {
    secrets    map[string]string
    isps       map[string]*entity.ISP
    zones      map[string]*entity.Zone
    servers    map[string]*entity.Server
    registries map[string]*entity.Registry
    domains    map[string]*entity.Domain
}

func NewValidator(cfg *entity.Config) *Validator {
    return &Validator{
        secrets:    cfg.GetSecretsMap(),
        isps:       cfg.GetISPMap(),
        zones:      cfg.GetZoneMap(),
        servers:    cfg.GetServerMap(),
        registries: cfg.GetRegistryMap(),
        domains:    cfg.GetDomainMap(),
    }
}

func (v *Validator) Validate(cfg *entity.Config) error {
    if err := cfg.Validate(); err != nil {
        return err
    }
    if err := v.validateReferences(cfg); err != nil {
        return err
    }
    if err := v.validatePortConflicts(cfg); err != nil {
        return err
    }
    if err := v.validateDomainConflicts(cfg); err != nil {
        return err
    }
    return nil
}

func (v *Validator) validateReferences(cfg *entity.Config) error {
    checks := []func(*entity.Config) error{
        v.validateISPReferences,
        v.validateZoneReferences,
        v.validateServerReferences,
        v.validateServiceReferences,
        v.validateDomainReferences,
    }
    for _, check := range checks {
        if err := check(cfg); err != nil {
            return err
        }
    }
    return nil
}

func (v *Validator) validateISPReferences(cfg *entity.Config) error {
    for _, isp := range cfg.ISPs {
        for name, ref := range isp.Credentials {
            if err := v.validateSecretRef(ref, "isp", isp.Name, name); err != nil {
                return err
            }
        }
    }
    return nil
}

func (v *Validator) validateSecretRef(ref entity.SecretRef, entityType, entityName, fieldName string) error {
    if ref.Secret == "" {
        return nil
    }
    if _, ok := v.secrets[ref.Secret]; !ok {
        return fmt.Errorf("%w: secret '%s' referenced by %s '%s' field '%s' not found",
            ErrMissingReference, ref.Secret, entityType, entityName, fieldName)
    }
    return nil
}
```

#### 3.1.3 Executor 职责分离

**问题**: Executor 混合了注册、执行、连接池管理

**方案**: 拆分为 Executor + SSHPool

```go
// internal/application/usecase/executor.go
package usecase

type Executor struct {
    plan     *valueobject.Plan
    registry *handler.Registry
    sshPool  *SSHPool
    deps     *DepsBuilder
}

func NewExecutor(plan *valueobject.Plan, env string) *Executor {
    return &Executor{
        plan:     plan,
        registry: handler.NewRegistry(),
        sshPool:  NewSSHPool(),
        deps:     NewDepsBuilder(env),
    }
}

func (e *Executor) Apply(ctx context.Context) []*handler.Result {
    e.registerHandlers()
    
    results := make([]*handler.Result, 0, len(e.plan.Changes))
    for _, change := range e.plan.Changes {
        result := e.applyChange(ctx, change)
        results = append(results, result)
    }
    
    e.sshPool.CloseAll()
    return results
}

func (e *Executor) applyChange(ctx context.Context, change *valueobject.Change) *handler.Result {
    h, ok := e.registry.Get(change.Entity)
    if !ok {
        return &handler.Result{Change: change, Error: ErrNoHandler}
    }
    
    deps := e.deps.Build(change, e.sshPool)
    return h.Apply(ctx, change, deps)
}

// internal/application/usecase/ssh_pool.go
package usecase

type SSHPool struct {
    clients map[string]*ssh.Client
    mu      sync.RWMutex
}

func NewSSHPool() *SSHPool {
    return &SSHPool{clients: make(map[string]*ssh.Client)}
}

func (p *SSHPool) Get(info *handler.ServerInfo) (handler.SSHClient, error) {
    p.mu.RLock()
    if client, ok := p.clients[info.Host]; ok {
        p.mu.RUnlock()
        return client, nil
    }
    p.mu.RUnlock()
    
    p.mu.Lock()
    defer p.mu.Unlock()
    
    client, err := ssh.NewClient(info.Host, info.Port, info.User, info.Password)
    if err != nil {
        return nil, err
    }
    p.clients[info.Host] = client
    return client, nil
}

func (p *SSHPool) CloseAll() {
    p.mu.Lock()
    defer p.mu.Unlock()
    for _, client := range p.clients {
        client.Close()
    }
    p.clients = make(map[string]*ssh.Client)
}
```

#### 3.1.4 Deps 接口隔离

**问题**: Deps 包含过多字段，部分 Handler 只需要其中几项

**方案**: 按需接口

```go
// internal/application/handler/deps.go
package handler

type Deps struct {
    secrets    map[string]string
    domains    map[string]*entity.Domain
    isps       map[string]*entity.ISP
    servers    map[string]*ServerInfo
    sshPool    SSHPool
    dnsFactory *dns.Factory
    env        string
    workDir    string
}

func (d *Deps) Secrets() map[string]string { return d.secrets }
func (d *Deps) Domains() map[string]*entity.Domain { return d.domains }
func (d *Deps) ISPs() map[string]*entity.ISP { return d.isps }
func (d *Deps) Servers() map[string]*ServerInfo { return d.servers }
func (d *Deps) SSHClient(serverName string) (SSHClient, error) {
    info, ok := d.servers[serverName]
    if !ok {
        return nil, fmt.Errorf("server %s not found", serverName)
    }
    return d.sshPool.Get(info)
}
func (d *Deps) DNSProvider(ispName string) (DNSProvider, error) {
    isp, ok := d.isps[ispName]
    if !ok {
        return nil, fmt.Errorf("ISP %s not found", ispName)
    }
    return d.dnsFactory.Create(isp, d.secrets)
}
func (d *Deps) Env() string    { return d.env }
func (d *Deps) WorkDir() string { return d.workDir }
```

### 3.2 安全修复

#### 3.2.1 SSH Host Key 验证

```go
// internal/ssh/client.go
func NewClient(host string, port int, user, password string) (*Client, error) {
    knownHosts, err := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
    if err != nil {
        return nil, fmt.Errorf("get known_hosts path: %w", err)
    }
    
    hostKeyCallback, err := knownhosts.New(knownHosts)
    if err != nil {
        return nil, fmt.Errorf("load known_hosts: %w", err)
    }
    
    config := &ssh.ClientConfig{
        User:            user,
        Auth:            []ssh.AuthMethod{ssh.Password(password)},
        HostKeyCallback: hostKeyCallback,
    }
    
    addr := fmt.Sprintf("%s:%d", host, port)
    client, err := ssh.Dial("tcp", addr, config)
    if err != nil {
        return nil, fmt.Errorf("dial: %w", err)
    }
    
    return &Client{client: client, user: user}, nil
}
```

### 3.3 CLI 层重构

#### 3.3.1 消除全局变量

```go
// internal/interfaces/cli/context.go
package cli

type Context struct {
    Env       string
    ConfigDir string
}

func NewContext(env, configDir string) *Context {
    return &Context{Env: env, ConfigDir: configDir}
}

// internal/interfaces/cli/apply.go
func newApplyCommand(ctx *Context) *cobra.Command {
    var filters Filters
    cmd := &cobra.Command{
        Use:   "apply [scope]",
        Short: "Apply changes",
        Run: func(cmd *cobra.Command, args []string) {
            scope := ""
            if len(args) > 0 {
                scope = args[0]
            }
            runApply(ctx, scope, filters)
        },
    }
    cmd.Flags().StringVar(&filters.Domain, "domain", "", "Filter by domain")
    cmd.Flags().StringVar(&filters.Zone, "zone", "", "Filter by zone")
    cmd.Flags().StringVar(&filters.Server, "server", "", "Filter by server")
    cmd.Flags().StringVar(&filters.Service, "service", "", "Filter by service")
    return cmd
}
```

### 3.4 代码简化

#### 3.4.1 Planner Service 通用化

```go
// internal/domain/service/planner.go
func (s *PlannerService) planEntities[T any](
    config []T,
    state map[string]*T,
    equals func(a, b *T) bool,
    key func(*T) string,
) []*valueobject.Change {
    var changes []*valueobject.Change
    
    configMap := sliceToMap(config, key)
    
    for name, cfg := range configMap {
        if old, exists := state[name]; exists {
            if !equals(cfg, old) {
                changes = append(changes, &valueobject.Change{
                    Type:     valueobject.ChangeTypeUpdate,
                    Entity:   entityOf(cfg),
                    Name:     name,
                    OldState: old,
                    NewState: cfg,
                })
            }
            delete(state, name)
        } else {
            changes = append(changes, &valueobject.Change{
                Type:     valueobject.ChangeTypeCreate,
                Entity:   entityOf(cfg),
                Name:     name,
                NewState: cfg,
            })
        }
    }
    
    for name, old := range state {
        changes = append(changes, &valueobject.Change{
            Type:     valueobject.ChangeTypeDelete,
            Entity:   entityOf(old),
            Name:     name,
            OldState: old,
        })
    }
    
    return changes
}
```

---

## 四、分阶段执行计划

### Phase 1: 安全修复 (P0)

**预计时间**: 1 天

| 序号 | 任务 | 文件 | 风险 |
|------|------|------|------|
| 1.1 | 修复 SSH Host Key 验证 | `ssh/client.go` | 低 |
| 1.2 | 添加单元测试 | `ssh/client_test.go` | 无 |

**验收**: 
- SSH 连接使用 known_hosts 验证
- 测试覆盖新增代码

---

### Phase 2: Factory 模式 (P0)

**预计时间**: 2 天

| 序号 | 任务 | 文件 | 风险 |
|------|------|------|------|
| 2.1 | 创建 DNS Provider Factory | `providers/dns/factory.go` | 低 |
| 2.2 | 创建 SSL Provider Factory | `providers/ssl/factory.go` | 低 |
| 2.3 | 重构 dns_handler 使用 Factory | `handler/dns_handler.go` | 中 |
| 2.4 | 添加 Factory 单元测试 | `*_test.go` | 无 |

**验收**:
- 新增 ISP 只需在 Factory 注册
- dns_handler.go 行数减少 40%

---

### Phase 3: 验证逻辑迁移 (P1)

**预计时间**: 2 天

| 序号 | 任务 | 文件 | 风险 |
|------|------|------|------|
| 3.1 | 创建 Validator 服务 | `domain/service/validator.go` | 低 |
| 3.2 | 迁移验证函数 | 从 `config_loader.go` 迁移 | 中 |
| 3.3 | 更新 ConfigLoader 调用 | `persistence/config_loader.go` | 低 |
| 3.4 | 添加验证测试 | `service/validator_test.go` | 无 |

**验收**:
- config_loader.go 行数 < 150
- 所有验证逻辑在 Domain 层

---

### Phase 4: Executor 重构 (P1)

**预计时间**: 2 天

| 序号 | 任务 | 文件 | 风险 |
|------|------|------|------|
| 4.1 | 创建 SSHPool | `usecase/ssh_pool.go` | 低 |
| 4.2 | 创建 DepsBuilder | `usecase/deps_builder.go` | 低 |
| 4.3 | 重构 Executor | `usecase/executor.go` | 中 |
| 4.4 | 添加并发安全测试 | `*_test.go` | 无 |

**验收**:
- executor.go 行数 < 100
- SSH 连接池线程安全

---

### Phase 5: CLI 层清理 (P2)

**预计时间**: 1 天

| 序号 | 任务 | 文件 | 风险 |
|------|------|------|------|
| 5.1 | 创建 Context 替代全局变量 | `cli/context.go` | 低 |
| 5.2 | 重构所有命令使用 Context | `cli/*.go` | 中 |

**验收**:
- 无全局变量
- 所有命令可独立测试

---

### Phase 6: 代码质量提升 (P2)

**预计时间**: 3 天

| 序号 | 任务 | 描述 |
|------|------|------|
| 6.1 | 函数提取 | 将长函数拆分为小函数 |
| 6.2 | 泛型重构 | 使用泛型消除重复代码 |
| 6.3 | 类型别名 | 为 string 添加类型别名 |
| 6.4 | 命名优化 | 改进变量/函数命名 |
| 6.5 | 测试覆盖 | 提高测试覆盖率至 80% |

**验收**:
- 所有文件 < 200 行
- 所有函数 < 30 行
- 测试覆盖率 ≥ 80%

---

## 五、改造前后对比

### 5.1 DNS Handler 对比

**改造前** (209 行):
```go
func (h *DNSHandler) getDNSProvider(ispName string, deps *Deps) (DNSProvider, error) {
    isp, ok := deps.ISPs[ispName]
    if !ok {
        return nil, fmt.Errorf("ISP %s not found", ispName)
    }
    
    switch isp.Type {
    case entity.ISPTypeAliyun:
        cred := isp.Credentials["access_key_id"]
        accessKeyID, err := cred.Resolve(deps.Secrets)
        // ... 50+ lines of switch cases
    }
}
```

**改造后** (~50 行):
```go
func (h *DNSHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
    record := h.extractRecord(change)
    domain := deps.Domain(record.Domain)
    provider, err := deps.DNSProvider(domain.DNSISP)
    if err != nil {
        return &Result{Change: change, Error: err}, nil
    }
    return h.applyChange(change, record, provider)
}
```

### 5.2 架构对比

**改造前**:
```
interfaces/cli
    └── 直接调用 persistence
    └── 直接操作 plan.Planner
    └── 使用全局变量
```

**改造后**:
```
interfaces/cli
    └── Context (依赖注入)
    └── application/usecase
           ├── Executor (编排)
           ├── SSHPool (资源管理)
           └── DepsBuilder (依赖构建)
    └── domain/service
           └── Validator (验证逻辑)
    └── providers/dns
           └── Factory (创建 Provider)
```

---

## 六、风险控制

### 6.1 风险矩阵

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 重构引入 Bug | 中 | 高 | 每阶段运行完整测试 |
| 功能回归 | 低 | 高 | 保持 API 兼容 |
| 进度延迟 | 中 | 中 | 按 Phase 优先级执行 |

### 6.2 回滚策略

- 每个 Phase 完成后创建 Git Tag
- 保留旧代码在 `_deprecated` 分支
- 问题出现时回滚到上一个 Tag

---

## 七、执行检查清单

### Phase 完成标准

- [ ] 所有单元测试通过
- [ ] 集成测试通过
- [ ] 代码审查通过
- [ ] 文档更新完成
- [ ] 无新增 TODO/FIXME

### 最终验收

- [ ] `go test ./...` 全部通过
- [ ] `go vet ./...` 无警告
- [ ] 测试覆盖率 ≥ 80%
- [ ] 所有文件 < 200 行
- [ ] 无安全问题

---

## 八、参考资源

- [Clean Code by Robert C. Martin](https://www.oreilly.com/library/view/clean-code-a/9780136083238/)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)
