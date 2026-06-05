# Handler 策略模式

Handler 策略模式是 YAMLOps 应用层的核心设计，用于处理不同类型实体的变更应用。

## 模式概述

```
            ┌─────────────┐
            │   Handler   │ (Strategy Interface)
            │  Interface  │
            └──────┬──────┘
                   │
     ┌─────────────┼─────────────┬─────────────┐
     │             │             │             │
     ▼             ▼             ▼             ▼
┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
│DNSHandler│  │ServiceH.│  │InfraSvcH│  │NoopH.   │ ...
└─────────┘  └─────────┘  └─────────┘  └─────────┘
```

Handler 策略模式允许系统根据实体类型动态选择对应的处理器，实现：

- **开闭原则**：新增实体类型只需添加新 Handler，无需修改现有代码
- **单一职责**：每个 Handler 只负责一种实体类型的处理
- **依赖注入**：Handler 通过接口接收依赖，便于测试和扩展

---

## Handler 接口

### 核心接口定义

定义在 `internal/domain/contract/handler.go`：

```go
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}
```

| 方法 | 描述 |
|------|------|
| `EntityType()` | 返回处理器负责的实体类型标识 |
| `Apply()` | 应用变更，执行实际操作 |

### Result 结构

```go
type Result struct {
    Change   *valueobject.Change
    Success  bool
    Error    error
    Output   string
    Warnings []string
}
```

---

## Handler 注册表

### Registry 实现

定义在 `internal/application/usecase/registry.go`：

```go
type Registry struct {
    handlers map[string]Handler
}

func NewRegistry() *Registry {
    return &Registry{
        handlers: make(map[string]Handler),
    }
}

func (r *Registry) Register(h Handler) {
    r.handlers[h.EntityType()] = h
}

func (r *Registry) Get(entityType string) (Handler, bool) {
    h, ok := r.handlers[entityType]
    return h, ok
}
```

### 注册流程

定义在 `internal/application/usecase/executor.go`，使用 `RegisterDefaults()` 方法（幂等注册）：

```go
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
```

NoopHandler 注册的实体类型（定义在 `noop_handler.go`）：`isp`、`zone`、`domain`、`certificate`

---

## 依赖注入接口

Handler 依赖采用接口隔离原则（ISP），存在两层定义：

### 领域层契约（contract/handler.go）

最小接口，不依赖具体实现：

```go
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}

type DNSDeps interface {
    DNSProvider(ispName string) (DNSProvider, error)
}

type ServiceDeps interface {
    SSHClient(server string) (SSHClient, error)
}

type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}
```

### 应用层扩展（usecase/types.go）

扩展接口，提供完整依赖：

```go
type DNSDeps interface {
    DNSProvider(ispName string) (contract.DNSProvider, error)
    Domain(name string) (*entity.Domain, bool)
    ISP(name string) (*entity.ISP, bool)
}

type ServiceDeps interface {
    SSHClient(server string) (contract.SSHClient, error)
    ServerInfo(name string) (*ServerInfo, bool)
    Server(name string) (*entity.Server, bool)
    WorkDir() string
    Env() string
    RegistryManager(server string) (*registry.Manager, error)
    GetAllRegistries() []*entity.Registry
    Secrets() map[string]string
}

type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}
```

### BaseDeps 实现

使用 Option 模式构建依赖：

```go
type BaseDeps struct {
    sshClient      contract.SSHClient
    sshError       error
    dnsFactory     DNSFactory
    secrets        map[string]string
    domains        map[string]*entity.Domain
    isps           map[string]*entity.ISP
    servers        map[string]*ServerInfo
    serverEntities map[string]*entity.Server
    registries     map[string]*entity.Registry
    workDir        string
    env            string
}

func NewBaseDeps(opts ...BaseDepsOption) *BaseDeps { ... }
```

---

## Handler 实现

所有 Handler 定义在 `internal/application/usecase/` 目录下。

### DNSHandler

处理 DNS 记录的 CRUD 操作。实体类型：`dns_record`

```go
func (h *DNSHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    switch change.Type() {
    case valueobject.ChangeTypeCreate:
        return h.create(ctx, change, deps)
    case valueobject.ChangeTypeUpdate:
        return h.update(ctx, change, deps)
    case valueobject.ChangeTypeDelete:
        return h.delete(ctx, change, deps)
    default:
        return &Result{Change: change, Success: true}, nil
    }
}
```

### ServiceHandler

处理业务服务的 Docker Compose 部署。实体类型：`service`

```go
func (h *ServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    // 1. 获取 SSH 客户端和服务信息
    // 2. Registry 登录
    // 3. 同步 volumes 文件
    // 4. 拉取镜像
    // 5. docker compose up -d
}
```

### InfraServiceHandler

处理基础设施服务（gateway）的部署。实体类型：`infra_service`

```go
func (h *InfraServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    infra := change.NewState().(*entity.InfraService)
    switch infra.Type {
    case "gateway":
        return h.deployGatewayType(ctx, change, deps, infra)
    default:
        return &Result{Change: change, Success: false, Error: fmt.Errorf("unknown infra type: %s", infra.Type)}, nil
    }
}
```

### ServerHandler

处理服务器环境同步（Registry 登录）。实体类型：`server`

```go
func (h *ServerHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    // 1. 获取服务器配置的 registries 列表
    // 2. 对每个 registry 调用 RegistryManager.EnsureLoggedIn()
}
```

### NoopHandler

空操作处理器，用于非部署实体。

```go
var NoopEntities = []string{"isp", "zone", "domain", "certificate"}

func (h *NoopHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    return &Result{Change: change, Success: true, Output: "skipped (not a deployable entity)"}, nil
}
```

---

## Handler 类型与职责

| Handler | Entity | 文件 | 职责 |
|---------|--------|------|------|
| DNSHandler | `dns_record` | dns_handler.go | DNS 记录 CRUD |
| ServiceHandler | `service` | service_handler.go | Docker Compose 服务部署 |
| InfraServiceHandler | `infra_service` | infra_service_handler.go | 基础设施服务部署 (gateway) |
| ServerHandler | `server` | server_handler.go | 服务器 Registry 登录 |
| NoopHandler | `isp`/`zone`/`domain`/`certificate` | noop_handler.go | 空操作（非部署实体） |

---

## Executor 与 ChangeExecutor

### 架构分层

Executor 是面向外部的入口，内部委托给 ChangeExecutor 执行实际变更：

```
Executor (executor.go)
├── handlerRegistry *Registry     # Handler 注册表
├── changeExecutor  *ChangeExecutor  # 变更执行器
├── plan            *valueobject.Plan
└── env             string

ChangeExecutor (change_executor.go)
├── plan           *valueobject.Plan
├── sshPool        SSHPoolInterface
├── secrets        map[string]string
├── servers        map[string]*ServerInfo
├── domains        map[string]*entity.Domain
├── isps           map[string]*entity.ISP
├── dnsFactory     DNSFactoryInterface
└── workDir        string
```

### 并行执行流程

ChangeExecutor 按服务器分组，不同服务器的变更并发执行：

```go
func (ce *ChangeExecutor) Apply(registry handlerRegistry) []*Result {
    // 1. 按服务器名分组变更
    groups := ce.groupChangesByServer()

    // 2. DNS 变更排序：Delete → Update → Create
    ce.sortDNSChanges()

    // 3. 并行执行各服务器组
    var wg sync.WaitGroup
    for serverName, changes := range groups {
        wg.Add(1)
        go func(srv string, chs []*valueobject.Change) {
            defer wg.Done()
            // 构建依赖、逐个执行变更
        }(serverName, changes)
    }
    wg.Wait()

    // 4. 关闭 SSH 连接池
    ce.sshPool.CloseAll()
    return results
}
```

### SSHPool

SSH 连接池支持 TTL 过期和健康检查：

- **默认 TTL**：30 分钟
- **健康检查**：通过 `SSHHealthChecker.Healthy()` 接口
- **工厂方法**：`NewSSHPool()`、`NewSSHPoolWithTTL()`、`NewSSHPoolWithFactory()`

---

## 测试 Handler

### Mock 依赖

```go
type MockDepsProvider struct {
    DNSProviderFunc func(ispName string) (handler.DNSProvider, error)
    DomainFunc      func(name string) (*entity.Domain, bool)
    ISPFunc         func(name string) (*entity.ISP, bool)
    SSHClientFunc   func(server string) (handler.SSHClient, error)
    ServerInfoFunc  func(name string) (*handler.ServerInfo, bool)
    WorkDirFunc     func() string
    EnvFunc         func() string
    ResolveSecretFunc func(ref *valueobject.SecretRef) (string, error)
}

func (m *MockDepsProvider) DNSProvider(ispName string) (handler.DNSProvider, error) {
    return m.DNSProviderFunc(ispName)
}
// ... 其他方法实现
```

### 单元测试示例

```go
func TestDNSHandler_Create(t *testing.T) {
    handler := &DNSHandler{}
    change := valueobject.NewChange(
        valueobject.ChangeTypeCreate,
        "dns_record",
        "www.example.com",
    ).WithNewState(&entity.DNSRecord{
        Domain: "example.com",
        Name:   "www",
        Type:   "A",
        Value:  "1.2.3.4",
        TTL:    600,
    })

    mockDeps := &MockDepsProvider{
        DomainFunc: func(name string) (*entity.Domain, bool) {
            return &entity.Domain{Name: name, DNSISP: "cloudflare"}, true
        },
        ISPFunc: func(name string) (*entity.ISP, bool) {
            return &entity.ISP{Name: name, Type: "cloudflare"}, true
        },
        DNSProviderFunc: func(ispName string) (handler.DNSProvider, error) {
            return &MockDNSProvider{}, nil
        },
    }

    result, err := handler.Apply(context.Background(), change, mockDeps)

    assert.NoError(t, err)
    assert.True(t, result.Success)
}
```

---

## 扩展指南

### 添加新 Handler

1. **创建 Handler 结构体：**

```go
type NewEntityHandler struct{}

func (h *NewEntityHandler) EntityType() string {
    return "new_entity"
}

func (h *NewEntityHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    // 实现逻辑
}
```

2. **注册 Handler：**

```go
func (e *Executor) RegisterDefaults() {
    // ... 现有注册
    if _, ok := e.handlerRegistry.Get("new_entity"); !ok {
        e.handlerRegistry.Register(&NewEntityHandler{})
    }
}
```

3. **添加变更检测：**

在 `domain/service/differ.go` 添加对应的 `PlanNewEntity()` 方法。

### 添加新依赖接口

1. **定义接口：**

```go
type NewDeps interface {
    NewDependency() (SomeType, error)
}
```

2. **扩展 DepsProvider：**

```go
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
    NewDeps  // 新增
}
```

3. **实现 BaseDeps：**

```go
func (b *BaseDeps) NewDependency() (SomeType, error) {
    // 实现
}
```
