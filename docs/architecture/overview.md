# 架构概述

YAMLOps 采用领域驱动设计（DDD）分层架构，实现关注点分离和依赖倒置。

## 分层架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Interface Layer                               │
│                  (interfaces/cli/ + interfaces/tui/)                  │
│    Cobra 命令 + BubbleTea TUI，处理用户输入输出                        │
├─────────────────────────────────────────────────────────────────────┤
│                      Application Layer                               │
│                    (application/)                                    │
│    Handler 策略模式 + ChangeExecutor 并行执行 + Executor 编排器        │
│    + Planner 规划器 + Orchestrator 工作流 + Generator 生成器           │
├─────────────────────────────────────────────────────────────────────┤
│                        Domain Layer                                  │
│                         (domain/)                                    │
│    实体 + 值对象 + 仓储接口 + 领域服务 + 重试机制，无外部依赖            │
├─────────────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                              │
│                     (infrastructure/)                                │
│    配置加载 + DNS Factory + 状态存储 + SSH + 网络 + 密钥               │
│    + 镜像仓库 + 日志                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## 依赖方向

```
Interface → Application → Domain ← Infrastructure
                              ↑
                              └── 依赖倒置：Infrastructure 实现 Domain 接口
```

**核心原则：**

1. **依赖倒置原则 (DIP)**：高层模块不依赖低层模块，两者都依赖抽象
2. **接口隔离原则 (ISP)**：使用多个专用接口而非一个通用接口
3. **单一职责原则 (SRP)**：每个模块只负责一个功能
4. **开闭原则 (OCP)**：对扩展开放，对修改关闭

---

## 各层职责

### 接口层 (Interface Layer)

**位置：** `internal/interfaces/cli/` 和 `internal/interfaces/tui/`

**职责：**
- 处理用户输入输出
- CLI 命令解析（Cobra）— `interfaces/cli/`
- TUI 交互界面（BubbleTea）— `interfaces/tui/`
- 参数验证和转换

**组件：**
- `cli/root.go` - 根命令和全局标志
- `cli/plan.go`, `cli/apply.go`, `cli/validate.go` - 核心命令
- `cli/dns.go`, `cli/server_cmd.go`, `cli/app.go`, `cli/service_cmd.go` - 领域命令
- `tui/*.go` - TUI 相关组件
- `shared/styles/` - 共享样式定义

---

### 应用层 (Application Layer)

**位置：** `internal/application/`

**职责：**
- 用例编排和执行
- Handler 策略模式
- 并行变更执行（按服务器分组）
- 部署文件生成
- 工作流编排

**组件：**

| 目录 | 职责 |
|------|------|
| `usecase/` | Handler 实现、Executor、ChangeExecutor、SSHPool、Handler Registry |
| `generator/` | Docker Compose 和 infra-gate 配置生成 |
| `plan/` | Planner 规划器 |
| `orchestrator/` | 工作流编排器 |

---

### 领域层 (Domain Layer)

**位置：** `internal/domain/`

**职责：**
- 业务实体定义
- 值对象
- 领域服务
- 仓储接口定义
- 领域错误

**特点：**
- **无外部依赖**
- 纯 Go 标准库
- 可独立测试

**组件：**

| 目录 | 职责 |
|------|------|
| `entity/` | 实体定义（Server, Zone, ISP, Service 等） |
| `valueobject/` | 值对象（SecretRef, Change, Scope, Plan） |
| `repository/` | 仓储接口（ConfigLoader, StateRepository） |
| `service/` | 领域服务（Validator, DifferService） |
| `contract/` | 接口契约（DNSProvider, SSHClient, Handler） |
| `retry/` | 重试机制（Option 模式） |
| `errors.go` | 统一领域错误定义 |

---

### 基础设施层 (Infrastructure Layer)

**位置：** `internal/infrastructure/`

**职责：**
- 实现领域层定义的接口
- 外部服务集成
- 配置文件加载
- 状态持久化

**组件：**

| 目录 | 职责 |
|------|------|
| `persistence/` | 配置加载器实现（带 30 秒缓存） |
| `state/` | 文件状态存储（带文件锁、原子写入） |
| `ssh/` | SSH 客户端、SFTP、Shell 转义 |
| `dns/` | DNS Provider 工厂（Cloudflare、阿里云、腾讯云） |
| `secrets/` | 密钥解析器 |
| `logger/` | 日志基础设施（slog、上下文传播、指标） |
| `network/` | Docker 网络管理（通过 SSH） |
| `registry/` | Docker 镜像仓库管理（通过 SSH） |

---

## 数据流

### Plan/Apply 工作流

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   CLI 命令   │ ──→ │  Workflow   │ ──→ │ ConfigLoader │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │  Validator  │
                    └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │   Planner   │
                    └─────────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │ Differ   │   │ Generator│   │StateStore│
     │ Service  │   │          │   │          │
     └──────────┘   └──────────┘   └──────────┘
            │              │              │
            └──────────────┼──────────────┘
                           ▼
                    ┌─────────────┐
                    │    Plan     │
                    └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │  Executor   │
                    └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ChangeExecutor│ (按服务器分组并行)
                    └─────────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │ Handler  │   │ Handler  │   │ Handler  │
     │  (DNS)   │   │ (Service)│   │ (Server) │
     └──────────┘   └──────────┘   └──────────┘
```

---

## 核心设计模式

### 1. 策略模式 (Strategy Pattern)

**应用：** Handler 注册表

```go
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

type Registry struct {
    handlers map[string]Handler
}

func (r *Registry) Register(h Handler) {
    r.handlers[h.EntityType()] = h
}

func (r *Registry) Get(entityType string) (Handler, bool) {
    h, ok := r.handlers[entityType]
    return h, ok
}
```

### 2. 工厂模式 (Factory Pattern)

**应用：** DNS Provider 创建

```go
func NewProvider(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    switch isp.Type {
    case "cloudflare":
        return newCloudflareProvider(isp, secrets)
    case "aliyun":
        return newAliyunProvider(isp, secrets)
    case "tencent":
        return newTencentProvider(isp, secrets)
    default:
        return nil, fmt.Errorf("unsupported ISP type: %s", isp.Type)
    }
}
```

### 3. 依赖注入 (Dependency Injection)

**应用：** Executor 和 ChangeExecutor 配置

```go
type ExecutorConfig struct {
    Registry   RegistryInterface
    SSHPool    SSHPoolInterface
    DNSFactory DNSFactoryInterface
    Plan       *valueobject.Plan
    Env        string
}

func NewExecutor(cfg *ExecutorConfig) *Executor {
    // 自动初始化默认组件，支持 nil 安全
}
```

### 4. 接口隔离 (Interface Segregation)

**应用：** Handler 依赖接口（领域层 vs 应用层）

```go
// 领域层契约（contract/handler.go）- 最小接口
type DepsProvider interface {
    DNSDeps     // DNSProvider(ispName string)
    ServiceDeps // SSHClient(server string)
    CommonDeps  // ResolveSecret(ref *SecretRef)
}

// 应用层实现（usecase/types.go）- 扩展接口
type DepsProvider interface {
    DNSDeps     // + Domain(), ISP()
    ServiceDeps // + ServerInfo(), Server(), WorkDir(), Env(), RegistryManager(), GetAllRegistries(), Secrets()
    CommonDeps
}
```

### 5. Option 模式

**应用：** Planner、Retry、BaseDeps 配置

```go
type Option func(*Config)

func WithMaxAttempts(n int) Option {
    return func(c *Config) { c.MaxAttempts = n }
}

func Do(ctx context.Context, fn func() error, opts ...Option) error {
    cfg := DefaultConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    // ... retry logic with exponential backoff
}
```

---

## 模块边界

### 领域模型边界

```
Config (聚合根)
├── Secrets[]           # 密钥
├── ISPs[]              # 服务提供商
├── Registries[]        # Docker 镜像仓库
├── Zones[]             # 网络区域
├── Servers[]           # 服务器
├── InfraServices[]     # 基础设施服务 (gateway)
├── Services[]          # 业务服务
└── Domains[]           # 域名
```

### 实体层级关系

```
ISP (底层基础设施提供商)
  └── Zone (网络区域)
        ├── Server (物理/虚拟服务器)
        │     ├── InfraService (基础设施服务: gateway)
        │     └── BizService (业务服务)
        │           └── ServiceGatewayRoute (网关路由)
        └── Domain (域名)
              └── DNSRecord (DNS记录)
```

---

## 扩展指南

### 添加新实体类型

1. **领域层：** 在 `domain/entity/` 创建实体
2. **领域层：** 在 `domain/errors.go` 添加错误
3. **应用层：** 在 `application/usecase/` 创建 Handler
4. **基础设施层：** 在配置加载器添加解析逻辑

### 添加新 DNS 提供者

1. 在 `infrastructure/dns/` 创建提供者实现
2. 实现 `contract.DNSProvider` 接口
3. 在 `infrastructure/dns/factory.go` 注册

### 添加新 CLI 命令

1. 在 `interfaces/cli/` 创建命令文件
2. 在 `root.go` 注册命令

### 添加新 TUI 功能

1. 在 `interfaces/tui/` 创建或修改组件文件
2. 共享样式放在 `interfaces/shared/styles/`
