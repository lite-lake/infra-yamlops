# YAMLOps 代码质量提升方案

## 目录

- [1. 评估概述](#1-评估概述)
- [2. 整洁代码评估](#2-整洁代码评估)
- [3. 整洁架构评估](#3-整洁架构评估)
- [4. SOLID 原则评估](#4-solid-原则评估)
- [5. 关键问题清单](#5-关键问题清单)
- [6. 提升方案](#6-提升方案)
- [7. 实施路线图](#7-实施路线图)

---

## 1. 评估概述

### 1.1 评估维度

| 维度 | 说明 | 评分 (1-10) |
|------|------|-------------|
| **整洁代码** | 命名、函数、注释、格式、错误处理 | 6.5 |
| **整洁架构** | 分层、依赖方向、边界分离 | 7.0 |
| **SOLID 原则** | SRP、OCP、LSP、ISP、DIP | 5.5 |
| **可测试性** | 接口设计、依赖注入、mock 支持 | 6.0 |
| **可维护性** | 代码复杂度、重复度、文档 | 5.5 |

**总体评分: 6.1/10**

### 1.2 优势总结

| 优势 | 体现位置 |
|------|----------|
| 清晰的分层架构 | Domain/Application/Infrastructure/Interface 四层分离 |
| 良好的领域模型 | Entity + ValueObject + Repository 模式 |
| 策略模式应用 | Handler 策略模式处理不同实体类型 |
| 泛型代码复用 | `planSimpleEntity[T]` 泛型函数 |
| 工厂模式 | DNS/SSL Provider 工厂创建 |
| 编译时接口检查 | `var _ Interface = (*Struct)(nil)` 模式 |

### 1.3 主要问题

| 问题类型 | 严重程度 | 影响范围 |
|----------|----------|----------|
| CLI 命令代码重复 | 高 | 7+ 文件 |
| God File/File Class | 高 | TUI 模块 |
| ISP 违反 | 高 | Handler Deps 结构 |
| DIP 违反 | 中 | CLI/Executor 依赖 |
| 错误处理不一致 | 中 | Domain 层 |
| 潜在 Bug | 高 | Validator/Provider |

---

## 2. 整洁代码评估

### 2.1 命名规范

| 评估项 | 状态 | 说明 |
|--------|------|------|
| 包名 | ✅ 良好 | 小写单词 (`config`, `plan`, `ssh`) |
| 类型名 | ✅ 良好 | PascalCase (`ConfigLoader`, `PlannerService`) |
| 接口名 | ✅ 良好 | `-er` 后缀 (`Provider`, `Loader`, `Handler`) |
| 错误变量 | ✅ 良好 | `Err` 前缀 (`ErrInvalidName`) |
| 常量 | ⚠️ 部分不一致 | 存在硬编码魔法字符串 |

**问题示例:**

```go
// service_handler.go:37 - 魔法字符串
remoteDir := fmt.Sprintf("/data/yamlops/yo-%s-%s", deps.Env, change.Name)
// 应提取为常量

// ssh/client.go:185 - 硬编码路径
tmpFile, err := os.CreateTemp("", "/tmp/yamlops-%d")
// 应提取为可配置项
```

### 2.2 函数设计

| 评估项 | 状态 | 问题位置 |
|--------|------|----------|
| 函数长度 | ⚠️ 部分过长 | `tui_menu.go` 多个函数超过 50 行 |
| 参数数量 | ⚠️ 部分过多 | `planSimpleEntity` 有 6 个参数 |
| 单一职责 | ❌ 违反 | `Executor` 承担过多职责 |
| 嵌套深度 | ⚠️ 过深 | `infra_service_handler.go` 4+ 层嵌套 |

**问题示例:**

```go
// infra_service_handler.go:55-128 - 深层嵌套
func (h *InfraServiceHandler) deployInfraService(...) {
    if deps.SSHClient == nil {
        if deps.SSHError != nil {
            // 4层嵌套...
        }
    }
}

// 应使用早返回模式重构
func (h *InfraServiceHandler) deployInfraService(...) {
    if err := validateSSHClient(deps); err != nil {
        return errorResult(err)
    }
    // 主逻辑
}
```

### 2.3 错误处理

| 评估项 | 状态 | 问题位置 |
|--------|------|----------|
| 错误包装 | ✅ 良好 | 使用 `%w` 包装错误 |
| 错误类型 | ⚠️ 不一致 | Domain 错误与 `errors.New` 混用 |
| Panic 使用 | ❌ 不当 | DNS Provider 使用 panic |

**问题示例:**

```go
// validator.go:97 - 潜在 Bug
if _, ok := v.isps[ref.Secret]; ok {  // 应检查 secrets 而非 isps
    continue
}

// aliyun.go:24-27 - Panic 不当使用
func NewAliyunProvider(...) *AliyunProvider {
    client, err := alidns.NewClient(...)
    if err != nil {
        panic(err)  // 应返回 error
    }
}

// 应改为
func NewAliyunProvider(...) (*AliyunProvider, error) {
    client, err := alidns.NewClient(...)
    if err != nil {
        return nil, fmt.Errorf("create aliyun client: %w", err)
    }
    return &AliyunProvider{client: client}, nil
}
```

### 2.4 代码重复

| 位置 | 重复类型 | 重复度 |
|------|----------|--------|
| CLI 命令 (`app.go`, `plan.go`, `apply.go`, `dns.go`) | 配置加载/验证/规划流程 | 70%+ |
| DNS Providers (`EnsureRecord`) | 记录确保逻辑 | 95% |
| Handler SSH 验证 | 客户端验证逻辑 | 100% (3处) |
| Gateway 配置生成 | 配置生成逻辑 | 60% |

---

## 3. 整洁架构评估

### 3.1 依赖规则

```
理想依赖方向:
Interface → Application → Domain ← Infrastructure

当前问题:
┌─────────────────────────────────────────────────────────────┐
│  Interface Layer (cli/)                                     │
│    ❌ 直接创建 persistence.ConfigLoader                      │
│    ❌ 直接导入 providers/dns                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Application Layer (application/)                           │
│    ❌ Executor 直接创建 Registry, SSHPool, DNSFactory        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Domain Layer (domain/)                                     │
│    ⚠️ SecretRef 包含 YAML 序列化逻辑                         │
│    ⚠️ InfraService 包含 UnmarshalYAML/MarshalYAML           │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 边界分离

| 边界 | 状态 | 说明 |
|------|------|------|
| Domain/Application | ⚠️ 部分模糊 | Domain 层包含序列化逻辑 |
| Application/Infrastructure | ❌ 违反 | Application 直接创建基础设施实例 |
| Interface/Application | ⚠️ 部分模糊 | CLI 直接访问基础设施层 |

### 3.3 层级职责

| 层级 | 期望职责 | 当前问题 |
|------|----------|----------|
| Domain | 纯业务逻辑，无外部依赖 | 包含 YAML 序列化 |
| Application | 用例编排，协调领域对象 | 直接创建基础设施实例 |
| Infrastructure | 技术实现，持久化 | 职责清晰 |
| Interface | 用户交互，请求处理 | 包含业务逻辑，直接访问 Infrastructure |

---

## 4. SOLID 原则评估

### 4.1 Single Responsibility Principle (SRP)

| 组件 | 状态 | 违反详情 |
|------|------|----------|
| `Executor` | ❌ | 承担：Handler 注册、SSH 池管理、依赖构建、变更执行 |
| `Model` (TUI) | ❌ | 45+ 字段，混合 UI 状态、树状态、服务状态 |
| `Validator` | ⚠️ | 验证实体结构 + 引用完整性 + 冲突检测 |
| `Config` (Entity) | ⚠️ | 数据持有 + 多个 GetXxxMap() 查询方法 |
| `Deps` | ❌ | 包含所有 Handler 可能需要的依赖 |

### 4.2 Open/Closed Principle (OCP)

| 组件 | 状态 | 违反详情 |
|------|------|----------|
| `InfraService.UnmarshalYAML` | ❌ | Switch 语句，添加新类型需修改 |
| `deploymentGenerator` | ❌ | 硬编码 Switch 判断服务类型 |
| `Validator` | ⚠️ | 添加新验证需修改 Validate() 方法 |

### 4.3 Liskov Substitution Principle (LSP)

| 组件 | 状态 | 说明 |
|------|------|------|
| DNS Providers | ✅ | 所有实现可互相替换 |
| SSL Providers | ✅ | 所有实现可互相替换 |
| Handlers | ✅ | 所有实现遵循 Handler 接口 |

### 4.4 Interface Segregation Principle (ISP)

| 接口/结构 | 状态 | 违反详情 |
|-----------|------|----------|
| `Deps` 结构 | ❌ | 9 个字段，每个 Handler 只需要部分 |
| `ConfigLoader` | ⚠️ | Load + Validate 混合，应分离 |

**ISP 违反示例:**

```go
// types.go:17-28 - Deps 包含所有依赖
type Deps struct {
    SSHClient   SSHClient           // DNSHandler 不需要
    SSHError    error               // DNSHandler 不需要
    DNSProvider DNSProvider         // ServiceHandler 不需要
    DNSFactory  *dns.Factory        // ServiceHandler 不需要
    Secrets     map[string]string   // 所有 Handler 需要
    Domains     map[string]*entity.Domain  // ServiceHandler 不需要
    ISPs        map[string]*entity.ISP     // ServiceHandler 不需要
    Servers     map[string]*ServerInfo     // DNSHandler 不需要
    WorkDir     string              // DNSHandler 不需要
    Env         string              // DNSHandler 不需要
}
```

### 4.5 Dependency Inversion Principle (DIP)

| 组件 | 状态 | 违反详情 |
|------|------|----------|
| CLI 命令 | ❌ | 直接创建 `persistence.NewConfigLoader` |
| `Executor` | ❌ | 直接创建 `handler.NewRegistry()`, `NewSSHPool()` |
| TUI Actions | ❌ | 直接创建 DNS Provider 实例 |

**DIP 违反示例:**

```go
// executor.go:26-42 - 直接创建依赖
func NewExecutor(pl *valueobject.Plan, env string) *Executor {
    return &Executor{
        registry:   handler.NewRegistry(),  // 应注入
        sshPool:    NewSSHPool(),           // 应注入
        dnsFactory: dns.NewFactory(),       // 应注入
    }
}

// app.go:97-99 - 直接创建基础设施实例
loader := persistence.NewConfigLoader(ctx.Env)  // 应注入
```

---

## 5. 关键问题清单

### 5.1 必须修复 (P0)

| # | 文件 | 行号 | 问题 | 影响 |
|---|------|------|------|------|
| 1 | `validator.go` | 97 | ISP 引用验证逻辑 Bug | 数据完整性 |
| 2 | `aliyun.go` | 24 | `panic` 而非返回 error | 稳定性 |
| 3 | `tencent.go` | 22 | `panic` 而非返回 error | 稳定性 |
| 4 | `infra_service.go` | 196 | 缺少 `GatewayPorts.Validate()` 调用 | 数据完整性 |

### 5.2 高优先级 (P1)

| # | 文件 | 问题 | 解决方案 |
|---|------|------|----------|
| 1 | CLI 命令 (7+文件) | 70%+ 代码重复 | 提取公共工作流函数 |
| 2 | `tui_menu.go` | 1256 行 God File | 拆分为多个文件 |
| 3 | `tui_model.go` | 45+ 字段 God Struct | 分解为专注结构体 |
| 4 | `types.go` | `Deps` 违反 ISP | 按需拆分接口 |
| 5 | `executor.go` | 构造函数创建所有依赖 | 依赖注入重构 |

### 5.3 中优先级 (P2)

| # | 文件 | 问题 | 解决方案 |
|---|------|------|----------|
| 1 | Domain 层错误 | 不一致的错误类型 | 统一使用 Domain 错误 |
| 2 | Handler 错误处理 | Error 在 Result 中 | 返回 error 而非 Result.Error |
| 3 | Provider `EnsureRecord` | 重复代码 | 提取到公共函数 |
| 4 | Gateway 配置生成 | 重复逻辑 | 统一到单一生成器 |
| 5 | `registry.go` | 缺少线程安全 | 添加 sync.RWMutex |

### 5.4 低优先级 (P3)

| # | 文件 | 问题 |
|---|------|------|
| 1 | 魔法字符串 | 提取为常量 |
| 2 | 未使用代码 | `planner_generic.go:61-63` 死代码 |
| 3 | 缺少单元测试 | Entity Validate() 方法无测试 |
| 4 | `Scope.IsEmpty()` | 复杂布尔表达式可读性 |

---

## 6. 提升方案

### 6.1 架构重构

#### 6.1.1 CLI 公共工作流提取

```go
// internal/interfaces/cli/workflow.go
type Workflow struct {
    loader   repository.ConfigLoader
    planner  *plan.Planner
    resolver *config.SecretResolver
}

func NewWorkflow(env, configDir string) *Workflow {
    return &Workflow{
        loader:   persistence.NewConfigLoader(configDir),
        resolver: config.NewSecretResolver(),
    }
}

func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error) {
    cfg, err := w.loader.Load(ctx, w.env)
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }
    if err := w.loader.Validate(cfg); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }
    return cfg, nil
}

func (w *Workflow) Plan(ctx context.Context, scope *valueobject.Scope) (*valueobject.Plan, error) {
    cfg, err := w.LoadAndValidate(ctx)
    if err != nil {
        return nil, err
    }
    return w.planner.Plan(ctx, cfg, scope), nil
}
```

#### 6.1.2 Handler Deps 接口拆分

```go
// internal/application/handler/types.go

// DNS Handler 依赖
type DNSDeps interface {
    GetDNSProvider() (DNSProvider, error)
    GetDomain(name string) (*entity.Domain, bool)
    GetISP(name string) (*entity.ISP, bool)
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

// Service Handler 依赖
type ServiceDeps interface {
    GetSSHClient(server string) (SSHClient, error)
    GetServerInfo(name string) (*ServerInfo, bool)
    WorkDir() string
    Env() string
}

// 基础实现
type BaseDeps struct {
    // 所有字段
}

func (d *BaseDeps) GetDNSProvider() (DNSProvider, error) { ... }
func (d *BaseDeps) GetSSHClient(server string) (SSHClient, error) { ... }
```

#### 6.1.3 依赖注入重构

```go
// internal/application/usecase/executor.go

type ExecutorConfig struct {
    Registry   *handler.Registry
    SSHPool    SSHPoolInterface
    DNSFactory DNSFactoryInterface
    Config     *entity.Config
    Plan       *valueobject.Plan
    Env        string
}

func NewExecutor(cfg *ExecutorConfig) *Executor {
    if cfg.Registry == nil {
        cfg.Registry = handler.NewRegistry()
    }
    if cfg.SSHPool == nil {
        cfg.SSHPool = NewSSHPool()
    }
    return &Executor{
        registry:   cfg.Registry,
        sshPool:    cfg.SSHPool,
        dnsFactory: cfg.DNSFactory,
        config:     cfg.Config,
        plan:       cfg.Plan,
        env:        cfg.Env,
    }
}
```

### 6.2 代码重构

#### 6.2.1 TUI 模块拆分

```
internal/interfaces/cli/
├── tui.go              # 主入口 (保持)
├── tui_model.go        # 模型定义 (精简)
├── tui_view.go         # 视图渲染 (保持)
├── tui_keys.go         # 按键绑定 (保持)
├── tui_styles.go       # 样式定义 (保持)
├── tui_tree.go         # 树组件 (保持)
├── tui_viewport.go     # 视口组件 (保持)
├── tui_menu.go         # 菜单逻辑 (精简，移除服务操作)
├── tui_server.go       # 新增：服务器操作
├── tui_dns.go          # 新增：DNS 操作
├── tui_cleanup.go      # 新增：清理操作
└── tui_stop.go         # 新增：停止操作
```

#### 6.2.2 Model 结构拆分

```go
// tui_model.go

type Model struct {
    // 核心状态
    env      string
    config   *entity.Config
    plan     *valueobject.Plan
    
    // 组合状态
    ui       *UIState       // UI 相关状态
    tree     *TreeState     // 树选择状态
    servers  *ServerState   // 服务器操作状态
    dns      *DNSState      // DNS 操作状态
}

type UIState struct {
    width       int
    height      int
    currentView ViewType
    ready       bool
}

type TreeState struct {
    root        *TreeNode
    selected    string
    expanded    map[string]bool
}

type ServerState struct {
    checking    map[string]bool
    syncing     map[string]bool
    results     map[string]*CheckResult
}
```

#### 6.2.3 DNS Provider 公共逻辑

```go
// internal/providers/dns/common.go

func EnsureRecord(provider Provider, domain string, desired *DNSRecord) error {
    records, err := provider.ListRecords(domain)
    if err != nil {
        return fmt.Errorf("list records: %w", err)
    }
    
    for _, existing := range records {
        if existing.Type == desired.Type && existing.Name == desired.Name {
            if existing.Value == desired.Value && existing.TTL == desired.TTL {
                return nil // 已存在且相同
            }
            return provider.UpdateRecord(domain, existing.ID, desired)
        }
    }
    return provider.CreateRecord(domain, desired)
}
```

### 6.3 Domain 层改进

#### 6.3.1 统一错误处理

```go
// internal/domain/errors.go

var (
    // 字段验证错误
    ErrInvalidName       = errors.New("invalid name")
    ErrInvalidIP         = errors.New("invalid IP address")
    ErrInvalidPort       = errors.New("invalid port")
    ErrInvalidProtocol   = errors.New("invalid protocol")
    ErrInvalidDomain     = errors.New("invalid domain")
    ErrInvalidCIDR       = errors.New("invalid CIDR")
    ErrInvalidFormat     = errors.New("invalid format")
    
    // 引用错误
    ErrMissingReference  = errors.New("missing reference")
    ErrMissingSecret     = errors.New("missing secret")
    
    // 冲突错误
    ErrPortConflict      = errors.New("port conflict")
    ErrDomainConflict    = errors.New("domain conflict")
    ErrHostnameConflict  = errors.New("hostname conflict")
    
    // 必填字段
    ErrRequiredField     = errors.New("required field missing")
)

// 使用示例
func (s *BizService) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: service name", domain.ErrRequiredField)
    }
    if s.Server == "" {
        return fmt.Errorf("%w: server reference", domain.ErrRequiredField)
    }
    return nil
}
```

#### 6.3.2 验证逻辑提取

```go
// internal/domain/service/validator.go

type Validator struct {
    cfg       *entity.Config
    rules     []ValidationRule
}

type ValidationRule interface {
    Validate(cfg *entity.Config) error
}

func (v *Validator) Validate() error {
    for _, rule := range v.rules {
        if err := rule.Validate(v.cfg); err != nil {
            return err
        }
    }
    return nil
}

// 具体验证规则
type ReferenceValidator struct{}
type ConflictValidator struct{}
type StructureValidator struct{}
```

### 6.4 测试改进

#### 6.4.1 测试覆盖率目标

| 包 | 当前 | 目标 |
|----|------|------|
| `domain/entity` | ~0% | 80% |
| `domain/service` | ~60% | 90% |
| `application/handler` | ~0% | 70% |
| `application/usecase` | ~30% | 80% |

#### 6.4.2 测试模式

```go
// 实体验证测试
func TestBizService_Validate(t *testing.T) {
    tests := []struct {
        name    string
        service entity.BizService
        wantErr error
    }{
        {
            name:    "missing name",
            service: entity.BizService{},
            wantErr: domain.ErrRequiredField,
        },
        {
            name: "valid",
            service: entity.BizService{
                Name:   "api",
                Server: "server-1",
                Image:  "app:latest",
            },
            wantErr: nil,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.service.Validate()
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 7. 实施路线图

### Phase 1: 紧急修复 (1-2 天)

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 修复 `validator.go:97` Bug | P0 | 0.5h |
| DNS Provider `panic` → `error` | P0 | 1h |
| 添加 `GatewayPorts.Validate()` 调用 | P0 | 0.5h |
| 添加 Entity 单元测试 | P2 | 4h |

### Phase 2: 代码重复消除 (3-5 天)

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 提取 CLI 公共工作流 | P1 | 4h |
| 重构 7+ CLI 命令 | P1 | 4h |
| 提取 DNS Provider 公共逻辑 | P2 | 2h |
| 提取 Handler SSH 验证 | P2 | 1h |

### Phase 3: 架构重构 (5-7 天)

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 拆分 `Deps` 为接口 | P1 | 4h |
| 重构 `Executor` 依赖注入 | P1 | 3h |
| 拆分 TUI God File | P1 | 4h |
| 拆分 TUI Model | P1 | 3h |

### Phase 4: 质量提升 (3-5 天)

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| 统一 Domain 错误处理 | P2 | 2h |
| Handler 错误返回重构 | P2 | 3h |
| 添加 `sync.RWMutex` 到 Registry | P2 | 0.5h |
| 提取魔法字符串常量 | P3 | 2h |
| 清理死代码 | P3 | 1h |

### Phase 5: 测试完善 (持续)

| 任务 | 优先级 | 预估时间 |
|------|--------|----------|
| Entity 测试达到 80% | P2 | 8h |
| Handler 测试达到 70% | P2 | 8h |
| 集成测试补充 | P2 | 4h |

---

## 附录

### A. 重构前后对比

#### CLI 命令重构前

```go
// app.go:94-150 (56 行)
func runAppPlan(cmd *cobra.Command, args []string) error {
    ctx := getContext(cmd)
    loader := persistence.NewConfigLoader(ctx.Env)
    cfg, err := loader.Load(nil, ctx.Env)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
        os.Exit(1)
    }
    if err := loader.Validate(cfg); err != nil {
        fmt.Fprintf(os.Stderr, "Error validating config: %v\n", err)
        os.Exit(1)
    }
    resolver := config.NewSecretResolver()
    resolver.ResolveAll(cfg)
    // ... 更多重复代码
}
```

#### CLI 命令重构后

```go
// app.go (简化后)
func runAppPlan(cmd *cobra.Command, args []string) error {
    ctx := getContext(cmd)
    wf := cli.NewWorkflow(ctx.Env, ctx.ConfigDir)
    
    plan, err := wf.Plan(cmd.Context(), ctx.Scope)
    if err != nil {
        return err
    }
    
    return wf.DisplayPlan(plan)
}
```

### B. 推荐阅读

- 《Clean Code》- Robert C. Martin
- 《Clean Architecture》- Robert C. Martin
- 《Domain-Driven Design》- Eric Evans
- 《Refactoring》- Martin Fowler

### C. 代码质量检查工具

```bash
# 静态分析
go vet ./...
staticcheck ./...

# 代码复杂度
gocyclo ./...

# 代码重复检测
dupl ./...

# 测试覆盖率
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```
