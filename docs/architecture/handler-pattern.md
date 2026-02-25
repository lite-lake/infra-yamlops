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
    Warnings []string
}
```

---

## Handler 注册表

### Registry 实现

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

```go
func (e *Executor) registerHandlers() {
    e.registry.Register(NewDNSHandler())
    e.registry.Register(NewServiceHandler())
    e.registry.Register(NewInfraServiceHandler())
    e.registry.Register(NewServerHandler())
    e.registry.Register(NewNoopHandler("isp"))
    e.registry.Register(NewNoopHandler("zone"))
    e.registry.Register(NewNoopHandler("domain"))
    e.registry.Register(NewNoopHandler("certificate"))
    e.registry.Register(NewNoopHandler("registry"))
}
```

---

## 依赖注入接口

Handler 依赖采用接口隔离原则（ISP），拆分为专用接口：

### DNS 操作依赖

```go
type DNSDeps interface {
    DNSProvider(ispName string) (DNSProvider, error)
    Domain(name string) (*entity.Domain, bool)
    ISP(name string) (*entity.ISP, bool)
}
```

### 服务操作依赖

```go
type ServiceDeps interface {
    SSHClient(server string) (SSHClient, error)
    ServerInfo(name string) (*ServerInfo, bool)
    WorkDir() string
    Env() string
}
```

### 通用依赖

```go
type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}
```

### 组合依赖接口

```go
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}
```

---

## Handler 实现

### DNSHandler

处理 DNS 记录的 CRUD 操作。

```go
type DNSHandler struct{}

func (h *DNSHandler) EntityType() string {
    return "dns_record"
}

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

func (h *DNSHandler) create(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    record := change.NewState().(*entity.DNSRecord)
    domain, _ := deps.Domain(record.Domain)
    isp, _ := deps.ISP(domain.DNSISP)
    
    provider, err := deps.DNSProvider(isp.Name)
    if err != nil {
        return &Result{Change: change, Success: false, Error: err}, nil
    }
    
    err = provider.CreateRecord(domain.Name, &dns.DNSRecord{
        Name:  record.Name,
        Type:  record.Type,
        Value: record.Value,
        TTL:   record.TTL,
    })
    
    return &Result{Change: change, Success: err == nil, Error: err}, nil
}
```

### ServiceHandler

处理业务服务的 Docker Compose 部署。

```go
type ServiceHandler struct{}

func (h *ServiceHandler) EntityType() string {
    return "service"
}

func (h *ServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    service := change.NewState().(*entity.BizService)
    serverInfo, _ := deps.ServerInfo(service.Server)
    
    client, err := deps.SSHClient(service.Server)
    if err != nil {
        return &Result{Change: change, Success: false, Error: err}, nil
    }
    
    // 1. 上传 docker-compose.yml
    // 2. 执行 docker compose up -d
    // 3. 健康检查
    
    return &Result{Change: change, Success: true}, nil
}
```

### InfraServiceHandler

处理基础设施服务（gateway/ssl）的部署。

```go
type InfraServiceHandler struct{}

func (h *InfraServiceHandler) EntityType() string {
    return "infra_service"
}

func (h *InfraServiceHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    infra := change.NewState().(*entity.InfraService)
    
    switch infra.Type {
    case "gateway":
        return h.applyGateway(ctx, change, deps, infra)
    case "ssl":
        return h.applySSL(ctx, change, deps, infra)
    default:
        return &Result{Change: change, Success: false, Error: fmt.Errorf("unknown infra type: %s", infra.Type)}, nil
    }
}
```

### ServerHandler

处理服务器环境同步（Docker 安装、Registry 登录等）。

```go
type ServerHandler struct{}

func (h *ServerHandler) EntityType() string {
    return "server"
}

func (h *ServerHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    server := change.NewState().(*entity.Server)
    
    client, err := deps.SSHClient(server.Name)
    if err != nil {
        return &Result{Change: change, Success: false, Error: err}, nil
    }
    
    // 同步服务器环境
    // 1. 检查 Docker
    // 2. 检查 Docker Compose
    // 3. 登录 Registry
    
    return &Result{Change: change, Success: true}, nil
}
```

### NoopHandler

空操作处理器，用于非部署实体（如 ISP、Zone）。

```go
type NoopHandler struct {
    entityType string
}

func NewNoopHandler(entityType string) *NoopHandler {
    return &NoopHandler{entityType: entityType}
}

func (h *NoopHandler) EntityType() string {
    return h.entityType
}

func (h *NoopHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
    return &Result{Change: change, Success: true}, nil
}
```

---

## Handler 类型与职责

| Handler | Entity | 职责 |
|---------|--------|------|
| DNSHandler | `dns_record` | DNS 记录 CRUD |
| ServiceHandler | `service` | Docker Compose 服务部署 |
| InfraServiceHandler | `infra_service` | 基础设施服务部署 (gateway/ssl) |
| ServerHandler | `server` | 服务器环境同步 |
| NoopHandler | `isp`/`zone`/`domain`/`certificate`/`registry` | 空操作（非部署实体） |

---

## Executor 执行流程

```go
func (e *Executor) Apply() []*handler.Result {
    e.registerHandlers()
    
    var results []*handler.Result
    
    for _, change := range e.plan.Changes() {
        h, ok := e.registry.Get(change.Entity())
        if !ok {
            results = append(results, &handler.Result{
                Change: change,
                Success: false,
                Error: fmt.Errorf("no handler for entity type: %s", change.Entity()),
            })
            continue
        }
        
        result := h.Apply(context.Background(), change, e.buildDeps(change))
        results = append(results, result)
    }
    
    e.sshPool.CloseAll()
    return results
}

func (e *Executor) buildDeps(change *valueobject.Change) handler.DepsProvider {
    return &BaseDeps{
        sshClient:  e.sshPool,
        dnsFactory: e.dnsFactory,
        secrets:    e.secrets,
        domains:    e.domains,
        isps:       e.isps,
        servers:    e.servers,
        workDir:    e.workDir,
        env:        e.env,
    }
}
```

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
func (e *Executor) registerHandlers() {
    // ... 现有注册
    e.registry.Register(&NewEntityHandler{})
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
