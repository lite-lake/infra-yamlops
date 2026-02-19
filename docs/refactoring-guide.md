# 重构示例指南

本文档提供代码质量提升的具体重构示例，供开发参考。

---

## 1. CLI 工作流重构

### 1.1 当前问题

以下命令存在 70%+ 代码重复：
- `app.go`: `runAppPlan`, `runAppApply`
- `plan.go`: `runPlan`
- `apply.go`: `runApply`
- `dns.go`: `runDNSPlan`, `runDNSApply`
- `validate.go`: `runValidate`
- `list.go`: `runList`
- `show.go`: `runShow`

### 1.2 重构方案

#### 创建工作流模块

```go
// internal/interfaces/cli/workflow.go
package cli

import (
    "context"
    "fmt"

    "github.com/litelake/yamlops/internal/config"
    "github.com/litelake/yamlops/internal/domain/entity"
    "github.com/litelake/yamlops/internal/domain/repository"
    "github.com/litelake/yamlops/internal/domain/valueobject"
    "github.com/litelake/yamlops/internal/infrastructure/persistence"
    "github.com/litelake/yamlops/internal/plan"
)

type Workflow struct {
    env      string
    configDir string
    loader   repository.ConfigLoader
    resolver *config.SecretResolver
}

func NewWorkflow(env, configDir string) *Workflow {
    return &Workflow{
        env:       env,
        configDir: configDir,
        loader:    persistence.NewConfigLoader(configDir),
        resolver:  config.NewSecretResolver(),
    }
}

func (w *Workflow) Env() string { return w.env }

func (w *Workflow) LoadConfig(ctx context.Context) (*entity.Config, error) {
    cfg, err := w.loader.Load(ctx, w.env)
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }
    return cfg, nil
}

func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error) {
    cfg, err := w.LoadConfig(ctx)
    if err != nil {
        return nil, err
    }
    if err := w.loader.Validate(cfg); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }
    return cfg, nil
}

func (w *Workflow) ResolveSecrets(cfg *entity.Config) {
    w.resolver.ResolveAll(cfg)
}

func (w *Workflow) CreatePlanner(outputDir string) *plan.Planner {
    return plan.NewPlanner(outputDir, w.env)
}

func (w *Workflow) Plan(ctx context.Context, outputDir string, scope *valueobject.Scope) (*valueobject.Plan, *entity.Config, error) {
    cfg, err := w.LoadAndValidate(ctx)
    if err != nil {
        return nil, nil, err
    }
    w.ResolveSecrets(cfg)
    
    planner := w.CreatePlanner(outputDir)
    plan := planner.Plan(ctx, cfg, scope)
    return plan, cfg, nil
}
```

#### 重构后的命令

```go
// internal/interfaces/cli/app.go
func runAppPlan(cmd *cobra.Command, args []string) error {
    ctx := getContext(cmd)
    wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
    
    plan, _, err := wf.Plan(cmd.Context(), ctx.OutputDir, ctx.Scope)
    if err != nil {
        return err
    }
    
    displayPlan(plan)
    return nil
}

func runAppApply(cmd *cobra.Command, args []string) error {
    ctx := getContext(cmd)
    wf := NewWorkflow(ctx.Env, ctx.ConfigDir)
    
    plan, cfg, err := wf.Plan(cmd.Context(), ctx.OutputDir, ctx.Scope)
    if err != nil {
        return err
    }
    
    if !ctx.AutoApprove && !confirmApply(plan) {
        return nil
    }
    
    return executeApply(plan, cfg, ctx)
}
```

---

## 2. Handler Deps 接口拆分

### 2.1 当前问题

`Deps` 结构包含 9 个字段，但每个 Handler 只需要其中部分。

### 2.2 重构方案

```go
// internal/application/handler/types.go
package handler

import (
    "context"
    
    "github.com/litelake/yamlops/internal/domain/entity"
    "github.com/litelake/yamlops/internal/domain/valueobject"
    "github.com/litelake/yamlops/internal/providers/dns"
)

// Handler 接口保持不变
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

// DNS 依赖接口
type DNSDeps interface {
    DNSProvider() (dns.Provider, error)
    Domain(name string) (*entity.Domain, bool)
    ISP(name string) (*entity.ISP, bool)
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

// Service 依赖接口
type ServiceDeps interface {
    SSHClient(server string) (SSHClient, error)
    ServerInfo(name string) (*ServerInfo, bool)
    WorkDir() string
    Env() string
}

// 通用依赖接口
type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

// 组合接口 - Handler 可按需选择
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}

// 基础实现
type BaseDeps struct {
    sshClient   SSHClient
    sshError    error
    dnsProvider dns.Provider
    dnsFactory  *dns.Factory
    secrets     map[string]string
    domains     map[string]*entity.Domain
    isps        map[string]*entity.ISP
    servers     map[string]*ServerInfo
    workDir     string
    env         string
}

func (d *BaseDeps) DNSProvider() (dns.Provider, error) {
    if d.dnsProvider == nil {
        return nil, fmt.Errorf("DNS provider not available")
    }
    return d.dnsProvider, nil
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
    if d.sshClient == nil {
        return nil, d.sshError
    }
    return d.sshClient, nil
}

func (d *BaseDeps) ServerInfo(name string) (*ServerInfo, bool) {
    info, ok := d.servers[name]
    return info, ok
}

func (d *BaseDeps) WorkDir() string { return d.workDir }
func (d *BaseDeps) Env() string     { return d.env }

func (d *BaseDeps) ResolveSecret(ref *valueobject.SecretRef) (string, error) {
    if ref == nil {
        return "", nil
    }
    if ref.Plain != "" {
        return ref.Plain, nil
    }
    if val, ok := d.secrets[ref.Secret]; ok {
        return val, nil
    }
    return "", fmt.Errorf("secret not found: %s", ref.Secret)
}
```

### 2.3 Handler 使用示例

```go
// internal/application/handler/dns_handler.go
func (h *DNSHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    // 使用接口方法，只访问需要的依赖
    provider, err := deps.DNSProvider()
    if err != nil {
        return &Result{Change: change, Success: false, Error: err}, nil
    }
    
    domain, ok := deps.Domain(change.DomainName)
    if !ok {
        return &Result{Change: change, Success: false, Error: fmt.Errorf("domain not found")}, nil
    }
    
    // ... 处理逻辑
}
```

---

## 3. Executor 依赖注入重构

### 3.1 当前问题

`NewExecutor` 直接创建所有依赖实例。

### 3.2 重构方案

```go
// internal/application/usecase/executor.go
package usecase

import (
    "context"
    
    "github.com/litelake/yamlops/internal/application/handler"
    "github.com/litelake/yamlops/internal/domain/entity"
    "github.com/litelake/yamlops/internal/domain/valueobject"
    "github.com/litelake/yamlops/internal/providers/dns"
)

// 接口定义
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

// 配置结构
type ExecutorConfig struct {
    Registry   RegistryInterface
    SSHPool    SSHPoolInterface
    DNSFactory DNSFactoryInterface
    
    // 必需参数
    Config     *entity.Config
    Plan       *valueobject.Plan
    Env        string
    WorkDir    string
}

// Executor 结构
type Executor struct {
    registry   RegistryInterface
    sshPool    SSHPoolInterface
    dnsFactory DNSFactoryInterface
    config     *entity.Config
    plan       *valueobject.Plan
    env        string
    workDir    string
}

// 构造函数 - 接受配置，使用默认值填充 nil
func NewExecutor(cfg *ExecutorConfig) *Executor {
    if cfg.Registry == nil {
        cfg.Registry = handler.NewRegistry()
    }
    if cfg.SSHPool == nil {
        cfg.SSHPool = NewSSHPool()
    }
    if cfg.DNSFactory == nil {
        cfg.DNSFactory = dns.NewFactory()
    }
    
    e := &Executor{
        registry:   cfg.Registry,
        sshPool:    cfg.SSHPool,
        dnsFactory: cfg.DNSFactory,
        config:     cfg.Config,
        plan:       cfg.Plan,
        env:        cfg.Env,
        workDir:    cfg.WorkDir,
    }
    
    e.registerDefaultHandlers()
    return e
}

func (e *Executor) RegisterHandler(h handler.Handler) {
    e.registry.Register(h)
}

func (e *Executor) Apply(ctx context.Context) []*handler.Result {
    var results []*handler.Result
    
    for _, change := range e.plan.Changes {
        result := e.applyChange(ctx, change)
        results = append(results, result)
    }
    
    e.sshPool.CloseAll()
    return results
}
```

### 3.3 使用示例

```go
// 生产环境使用
executor := usecase.NewExecutor(&usecase.ExecutorConfig{
    Config:  cfg,
    Plan:    plan,
    Env:     env,
    WorkDir: workDir,
})

// 测试环境使用 mock
executor := usecase.NewExecutor(&usecase.ExecutorConfig{
    Registry:   mockRegistry,
    SSHPool:    mockSSHPool,
    DNSFactory: mockDNSFactory,
    Config:     testConfig,
    Plan:       testPlan,
    Env:        "test",
    WorkDir:    "/tmp",
})
```

---

## 4. DNS Provider Panic 修复

### 4.1 当前问题

```go
// internal/providers/dns/aliyun.go
func NewAliyunProvider(isp *entity.ISP, secrets map[string]string) *AliyunProvider {
    client, err := alidns.NewClientWithAccessKey(
        region,
        accessKeyID,
        accessKeySecret,
    )
    if err != nil {
        panic(err)  // 不当使用 panic
    }
    return &AliyunProvider{client: client}
}
```

### 4.2 重构方案

```go
// internal/providers/dns/aliyun.go
func NewAliyunProvider(isp *entity.ISP, secrets map[string]string) (*AliyunProvider, error) {
    accessKeyID, err := getSecret(isp.Credentials["access_key_id"], secrets)
    if err != nil {
        return nil, fmt.Errorf("get access_key_id: %w", err)
    }
    
    accessKeySecret, err := getSecret(isp.Credentials["access_key_secret"], secrets)
    if err != nil {
        return nil, fmt.Errorf("get access_key_secret: %w", err)
    }
    
    client, err := alidns.NewClientWithAccessKey(
        region,
        accessKeyID,
        accessKeySecret,
    )
    if err != nil {
        return nil, fmt.Errorf("create aliyun client: %w", err)
    }
    
    return &AliyunProvider{client: client}, nil
}

// factory.go 也需要更新
func (f *Factory) Create(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    creator, ok := f.creators[isp.Type]
    if !ok {
        return nil, fmt.Errorf("unknown ISP type: %s", isp.Type)
    }
    return creator(isp, secrets)
}
```

---

## 5. TUI Model 拆分

### 5.1 当前问题

`Model` 结构有 45+ 字段，混合多种状态。

### 5.2 重构方案

```go
// internal/interfaces/cli/tui_model.go
package cli

type Model struct {
    // 核心配置
    env       string
    configDir string
    
    // 数据
    config *entity.Config
    plan   *valueobject.Plan
    
    // 组合状态
    ui      *UIState
    tree    *TreeState
    servers *ServerState
    dns     *DNSState
    actions *ActionState
}

// UI 状态
type UIState struct {
    width       int
    height      int
    ready       bool
    currentView ViewType
    focusArea   FocusArea
    message     string
    messageTTL  int
}

// 树状态
type TreeState struct {
    root         *TreeNode
    selectedPath []string
    expanded     map[string]bool
    checked      map[string]bool
}

// 服务器状态
type ServerState struct {
    checking map[string]bool
    syncing  map[string]bool
    results  map[string]*server.CheckResult
}

// DNS 状态
type DNSState struct {
    pullingRecords map[string]bool
    records        map[string][]*entity.DNSRecord
}

// 操作状态
type ActionState struct {
    planning    bool
    applying    bool
    cleaning    bool
    lastError   error
    lastSuccess string
}

// 构造函数
func NewModel(env, configDir string) Model {
    return Model{
        env:       env,
        configDir: configDir,
        ui: &UIState{
            currentView: ViewMenu,
            focusArea:   FocusTree,
        },
        tree: &TreeState{
            expanded: make(map[string]bool),
            checked:  make(map[string]bool),
        },
        servers: &ServerState{
            checking: make(map[string]bool),
            syncing:  make(map[string]bool),
            results:  make(map[string]*server.CheckResult),
        },
        dns: &DNSState{
            pullingRecords: make(map[string]bool),
            records:        make(map[string][]*entity.DNSRecord),
        },
        actions: &ActionState{},
    }
}
```

### 5.3 文件拆分建议

```
internal/interfaces/cli/
├── tui.go              # 主循环入口 (~50 行)
├── tui_model.go        # Model 定义 (~100 行)
├── tui_view.go         # 视图渲染 (~200 行)
├── tui_update.go       # 更新逻辑 (~150 行)
├── tui_keys.go         # 按键绑定 (~50 行)
├── tui_styles.go       # 样式 (~50 行)
├── tui_tree.go         # 树组件 (~150 行)
├── tui_viewport.go     # 视口 (~100 行)
├── tui_menu.go         # 菜单视图 (~200 行)
├── tui_server.go       # 服务器操作 (~300 行) [新增]
├── tui_dns.go          # DNS 操作 (~200 行) [新增]
├── tui_cleanup.go      # 清理操作 (~100 行) [新增]
└── tui_actions.go      # 操作辅助函数 (~200 行)
```

---

## 6. 错误处理统一

### 6.1 当前问题

```go
// biz_service.go - 混用错误类型
if s.Name == "" {
    return fmt.Errorf("%w: service name is required", domain.ErrInvalidName)
}
if s.Server == "" {
    return errors.New("server is required")  // 不一致
}
```

### 6.2 重构方案

```go
// internal/domain/errors.go
package domain

import "errors"

var (
    // 字段验证
    ErrInvalidName     = errors.New("invalid name")
    ErrInvalidIP       = errors.New("invalid IP address")
    ErrInvalidPort     = errors.New("invalid port")
    ErrInvalidProtocol = errors.New("invalid protocol")
    ErrInvalidDomain   = errors.New("invalid domain")
    ErrInvalidCIDR     = errors.New("invalid CIDR")
    ErrInvalidURL      = errors.New("invalid URL")
    
    // 必填字段
    ErrRequired = errors.New("required field missing")
    
    // 引用完整性
    ErrMissingReference = errors.New("missing reference")
    ErrMissingSecret    = errors.New("missing secret")
    
    // 冲突
    ErrPortConflict     = errors.New("port conflict")
    ErrDomainConflict   = errors.New("domain conflict")
    ErrHostnameConflict = errors.New("hostname conflict")
)

// 辅助函数
func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}

func InvalidField(field, reason string) error {
    return fmt.Errorf("%s: %s", field, reason)
}
```

```go
// biz_service.go - 统一使用
func (s *BizService) Validate() error {
    if s.Name == "" {
        return domain.RequiredField("name")
    }
    if s.Server == "" {
        return domain.RequiredField("server")
    }
    if s.Image == "" {
        return domain.RequiredField("image")
    }
    
    for i, port := range s.Ports {
        if err := port.Validate(); err != nil {
            return fmt.Errorf("ports[%d]: %w", i, err)
        }
    }
    
    return nil
}
```

---

## 7. 测试示例

### 7.1 Entity 测试

```go
// internal/domain/entity/biz_service_test.go
package entity

import (
    "testing"
    
    "github.com/litelake/yamlops/internal/domain"
)

func TestBizService_Validate(t *testing.T) {
    tests := []struct {
        name    string
        service BizService
        wantErr error
    }{
        {
            name:    "empty service",
            service: BizService{},
            wantErr: domain.ErrRequired,
        },
        {
            name: "missing server",
            service: BizService{
                Name:  "api",
                Image: "app:latest",
            },
            wantErr: domain.ErrRequired,
        },
        {
            name: "valid service",
            service: BizService{
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
            if tt.wantErr != nil {
                if !errors.Is(err, tt.wantErr) {
                    t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
                }
            } else {
                if err != nil {
                    t.Errorf("Validate() unexpected error = %v", err)
                }
            }
        })
    }
}
```

### 7.2 Handler 测试 (使用 Mock)

```go
// internal/application/handler/dns_handler_test.go
package handler

import (
    "context"
    "testing"
    
    "github.com/litelake/yamlops/internal/domain/entity"
    "github.com/litelake/yamlops/internal/domain/valueobject"
)

// Mock 实现
type mockDeps struct {
    dnsProvider dns.Provider
    domains     map[string]*entity.Domain
    isps        map[string]*entity.ISP
    secrets     map[string]string
}

func (m *mockDeps) DNSProvider() (dns.Provider, error) {
    if m.dnsProvider == nil {
        return nil, fmt.Errorf("no provider")
    }
    return m.dnsProvider, nil
}

func (m *mockDeps) Domain(name string) (*entity.Domain, bool) {
    d, ok := m.domains[name]
    return d, ok
}

// ... 其他方法实现

func TestDNSHandler_Apply_CreateRecord(t *testing.T) {
    mockProvider := &mockDNSProvider{
        records: []*dns.DNSRecord{},
    }
    
    deps := &mockDeps{
        dnsProvider: mockProvider,
        domains: map[string]*entity.Domain{
            "example.com": {Name: "example.com"},
        },
        secrets: map[string]string{},
    }
    
    handler := &DNSHandler{}
    change := &valueobject.Change{
        Type:     valueobject.ChangeTypeCreate,
        Entity:   "dns_record",
        Name:     "www.example.com",
        NewState: map[string]interface{}{
            "type":   "A",
            "name":   "www",
            "value":  "1.2.3.4",
            "domain": "example.com",
        },
    }
    
    result, err := handler.Apply(context.Background(), change, deps)
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !result.Success {
        t.Errorf("expected success, got error: %v", result.Error)
    }
    if len(mockProvider.created) != 1 {
        t.Errorf("expected 1 record created, got %d", len(mockProvider.created))
    }
}
```

---

## 8. 检查清单

重构完成后，使用以下清单验证：

### 代码质量

- [ ] 所有函数长度 < 50 行
- [ ] 所有嵌套深度 < 4 层
- [ ] 无魔法字符串/数字
- [ ] 错误处理使用 `%w` 包装
- [ ] 无 `panic` 用于可预期的错误

### 架构

- [ ] CLI 层不直接创建 Infrastructure 实例
- [ ] Application 层通过接口接收依赖
- [ ] Domain 层无外部包依赖（除标准库）
- [ ] 依赖方向正确：外层 → 内层

### SOLID

- [ ] 每个结构体只有一个变更原因
- [ ] 添加新类型无需修改现有代码
- [ ] 接口客户端专用，不强迫依赖不需要的方法
- [ ] 高层模块不依赖低层模块，两者依赖抽象

### 测试

- [ ] Entity 测试覆盖率 > 80%
- [ ] Handler 测试覆盖率 > 70%
- [ ] 所有测试通过
- [ ] 无竞态条件（`go test -race`）
