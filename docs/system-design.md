# YAMLOps 系统设计说明

## 目录

- [1. 系统概述](#1-系统概述)
- [2. 架构设计](#2-架构设计)
- [3. 领域模型](#3-领域模型)
- [4. 应用层设计](#4-应用层设计)
- [5. 基础设施层](#5-基础设施层)
- [6. 规划器设计](#6-规划器设计)
- [7. CLI 命令系统](#7-cli-命令系统)
- [8. 配置文件规范](#8-配置文件规范)
- [9. 关键规则](#9-关键规则)
- [10. 使用方法](#10-使用方法)

---

## 1. 系统概述

### 1.1 项目定位

YAMLOps 是一个基于 Go 语言开发的基础设施即代码（IaC）管理工具，通过 YAML 配置文件管理：

- **服务器**：SSH 连接、环境配置、Docker 网络
- **服务**：Docker Compose 部署
- **网关**：infra-gate 配置管理
- **DNS**：域名和记录管理（支持 Cloudflare、阿里云、腾讯云）

### 1.2 核心特性

| 特性 | 说明 |
|------|------|
| 多环境支持 | prod / staging / dev / demo 环境隔离 |
| Plan/Apply 工作流 | 类似 Terraform 的预览+执行模式 |
| 密钥引用 | 支持明文和密钥引用两种方式 |
| 声明式配置 | 通过 YAML 描述期望状态 |
| TUI 界面 | 基于 BubbleTea 的交互式终端界面 |

### 1.3 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.24+ |
| CLI 框架 | Cobra |
| TUI 框架 | BubbleTea |
| YAML 解析 | gopkg.in/yaml.v3 |
| SSH | golang.org/x/crypto/ssh |
| SFTP | github.com/pkg/sftp |
| 文件锁 | github.com/gofrs/flock |

---

## 2. 架构设计

### 2.1 分层架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Interface Layer                               │
│                  (interfaces/cli/ + interfaces/tui/)                  │
│    Cobra 命令 + BubbleTea TUI，处理用户输入输出                        │
├─────────────────────────────────────────────────────────────────────┤
│                      Application Layer                               │
│                    (application/)                                    │
│    Handler 策略模式 + Executor 编排器 + Planner 规划器                 │
│    + Orchestrator 工作流 + Generator 生成器                           │
├─────────────────────────────────────────────────────────────────────┤
│                        Domain Layer                                  │
│                         (domain/)                                    │
│    实体 + 值对象 + 仓储接口 + 领域服务 + 重试机制，无外部依赖            │
├─────────────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                              │
│                     (infrastructure/)                                │
│    配置加载 + DNS Factory + 状态存储 + SSH + 生成器 + 网络 + 密钥       │
│    + 镜像仓库 + 日志                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 依赖规则

```
Interface → Application → Domain ← Infrastructure
                              ↑
                              └── 依赖倒置：Infrastructure 实现 Domain 接口
```

### 2.3 目录结构

```
cmd/yamlops/                        # CLI 入口点
internal/
├── domain/                         # 领域层（无外部依赖）
│   ├── entity/                     # 实体定义
│   ├── valueobject/                # 值对象
│   ├── repository/                 # 仓储接口
│   ├── service/                    # 领域服务（DifferService, Validator）
│   ├── contract/                   # 接口契约（DNS, SSH 等）
│   ├── retry/                      # 重试机制（Config, Option, Do, DoWithResult）
│   └── errors.go                   # 领域错误（统一定义）
├── application/                    # 应用层
│   ├── usecase/                    # 变更处理器 + 执行器 + SSH 连接池
│   │   ├── types.go                # 依赖接口定义（Handler, DNSDeps, ServiceDeps, CommonDeps, BaseDeps）
│   │   ├── dns_handler.go          # DNS 记录处理器
│   │   ├── service_handler.go      # 业务服务处理器
│   │   ├── service_common.go       # 服务公共逻辑
│   │   ├── infra_service_handler.go # 基础设施服务处理器
│   │   ├── server_handler.go       # 服务器处理器
│   │   ├── noop_handler.go         # 空操作处理器（isp/zone/domain/certificate）
│   │   ├── registry.go             # 处理器注册表
│   │   ├── executor.go             # 执行器（支持依赖注入 DIP）
│   │   ├── change_executor.go      # 变更执行器（并行执行）
│   │   └── ssh_pool.go             # SSH 连接池
│   ├── generator/                  # 部署文件生成器
│   │   ├── compose/                # Docker Compose 生成器
│   │   │   ├── generator.go
│   │   │   └── types.go
│   │   ├── gate/                   # infra-gate 配置生成器
│   │   │   ├── generator.go
│   │   │   └── types.go
│   │   └── generator.go            # 生成器主入口
│   ├── plan/                       # 规划协调层
│   │   └── planner.go              # Planner 主入口（Option 模式）
│   └── orchestrator/               # 工作流编排器
│       ├── workflow.go             # 工作流主入口
│       ├── state_fetcher.go        # 状态获取器
│       └── utils.go                # 工具函数
├── infrastructure/                 # 基础设施层
│   ├── persistence/                # 配置加载实现
│   │   └── config_loader.go        # 配置加载器（30 秒缓存）
│   ├── state/                      # 状态存储实现
│   │   └── file_store.go           # 文件状态存储（flock 文件锁 + 原子写入）
│   ├── dns/                        # DNS Provider（Cloudflare、阿里云、腾讯云）
│   │   ├── provider.go             # Provider 类型别名
│   │   ├── common.go               # DNS 公共逻辑（EnsureRecord、批量操作）
│   │   ├── factory.go              # DNS Provider 工厂
│   │   ├── cloudflare.go           # Cloudflare 实现
│   │   ├── aliyun.go               # 阿里云实现
│   │   └── tencent.go              # 腾讯云实现
│   ├── ssh/                        # SSH 客户端
│   │   ├── client.go               # SSH 客户端（Run、RunWithStdin、StreamRun、Healthy）
│   │   ├── sftp.go                 # SFTP 文件传输
│   │   └── shell_escape.go         # Shell 转义
│   ├── network/                    # Docker 网络管理
│   │   └── manager.go
│   ├── registry/                   # 镜像仓库管理
│   │   └── manager.go
│   ├── secrets/                    # 密钥解析器
│   │   └── resolver.go             # SecretResolver 实现
│   └── logger/                     # 日志基础设施
│       ├── logger.go               # 日志主入口
│       ├── context.go              # 上下文日志
│       └── metrics.go              # 指标记录
├── interfaces/                     # 接口层
│   ├── cli/                        # CLI 命令（Cobra）
│   │   ├── root.go                 # 根命令
│   │   ├── workflow.go             # CLI 工作流模块
│   │   ├── context.go              # 执行上下文
│   │   ├── plan.go                 # Plan 命令
│   │   ├── apply.go                # Apply 命令
│   │   ├── validate.go             # Validate 命令
│   │   ├── list.go                 # List 命令
│   │   ├── show.go                 # Show 命令
│   │   ├── clean.go                # Clean 命令
│   │   ├── confirm.go              # 确认对话框
│   │   ├── env.go                  # Env 命令
│   │   ├── dns.go                  # DNS 命令
│   │   ├── dns_pull.go             # DNS Pull 命令
│   │   ├── app.go                  # App 命令
│   │   ├── server_cmd.go           # Server 命令
│   │   ├── service_cmd.go          # Service 命令（deploy/stop/restart/cleanup）
│   │   └── config_cmd.go           # Config 命令
│   ├── tui/                        # BubbleTea TUI
│   │   ├── tui.go                  # TUI 主入口
│   │   ├── tui_model.go            # TUI 数据模型
│   │   ├── tui_view.go             # TUI 视图渲染
│   │   ├── tui_render.go           # TUI 渲染逻辑
│   │   ├── tui_actions.go          # TUI 操作处理
│   │   ├── tui_keys.go             # TUI 按键处理
│   │   ├── tui_styles.go           # TUI 样式定义
│   │   ├── tui_menu.go             # TUI 菜单
│   │   ├── tui_tree.go             # TUI 树形视图
│   │   ├── tui_viewport.go         # TUI 视口滚动
│   │   ├── tui_server.go           # TUI 服务器操作
│   │   ├── tui_dns.go              # TUI DNS 操作
│   │   ├── tui_service_common.go   # TUI 服务公共逻辑
│   │   ├── tui_cleanup.go          # TUI 清理操作
│   │   ├── tui_stop.go             # TUI 停止操作
│   │   └── tui_restart.go          # TUI 重启操作
│   └── shared/
│       └── styles/                 # 共享样式
├── constants/                      # 常量定义
│   └── constants.go                # 路径、格式等常量
├── environment/                    # 服务器环境管理
│   ├── checker.go                  # 环境检查器（SSH 连接、Sudo、Docker、APT 源、Registry）
│   ├── syncer.go                   # 环境同步器
│   ├── templates.go                # APT 源配置模板
│   └── types.go                    # 类型定义
├── version/                        # 版本信息
userdata/{env}/                     # 用户配置文件
deployments/                        # 生成的部署文件（git-ignored）
```

---

## 3. 领域模型

### 3.1 实体概览

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

### 3.2 实体详情

#### 3.2.1 Secret（密钥）

```yaml
secrets:
  - name: db_password
    value: "super_secret_123"
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 密钥名称 |
| value | string | 否 | 密钥值 |

#### 3.2.2 ISP（服务提供商）

```yaml
isps:
  - name: aliyun
    type: aliyun                    # aliyun | cloudflare | tencent
    services: [server, domain, dns] # server | domain | dns
    credentials:
      access_key_id: {secret: aliyun_ak}
      access_key_secret: {secret: aliyun_sk}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 提供商名称 |
| type | ISPType | 否 | 提供商类型（默认使用 name） |
| services | []ISPService | 是 | 支持的服务类型 |
| credentials | map[string]SecretRef | 是 | 凭证映射 |

#### 3.2.3 Registry（Docker 仓库）

Registry 配置定义 Docker 镜像仓库的登录凭据。服务器的 `environment.registries` 字段会引用这些配置，在服务部署前自动登录。

```yaml
registries:
  - name: dockerhub
    url: https://registry.hub.docker.com
    credentials:
      username: {secret: docker_user}
      password: {secret: docker_pass}
```

Server 配置中引用：
```yaml
servers:
  - name: srv-cn1
    environment:
      registries:
        - dockerhub
```

#### 3.2.4 Zone（网络区域）

```yaml
zones:
  - name: cn-east
    description: 华东区域
    isp: aliyun
    region: cn-shanghai
```

#### 3.2.5 Server（服务器）

```yaml
servers:
  - name: prod-server-1
    zone: cn-east
    isp: aliyun
    os: ubuntu22
    ip:
      public: 1.2.3.4
      private: 10.0.0.1
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password: {secret: server_pwd}
    environment:
      apt_source: aliyun
      registries: [dockerhub]
```

#### 3.2.6 InfraService（基础设施服务）

```yaml
infra_services:
  - name: main-gateway
    type: gateway              # gateway
    server: prod-server-1
    image: litelake/infra-gate:latest
    ports:
      http: 80
      https: 443
    ssl:
      mode: remote             # local | remote
      endpoint: https://ssl.litelake.com/cert/json
    waf:
      enabled: true
      whitelist:
        - 192.168.0.0/16
    log_level: 1
    notification:
      enabled: true
      url: https://hooks.example.com/notify
      timeout: 10
```

#### 3.2.7 BizService（业务服务）

```yaml
services:
  - name: api-server
    server: prod-server-1
    image: myapp/api:v1.0
    ports:
      - container: 8080
        host: 10080
        protocol: tcp
    env:
      DATABASE_URL: {secret: db_url}
      REDIS_URL: {secret: redis_url}
    volumes:
      - source: /data/app
        target: /app/data
    healthcheck:
      path: /health
      interval: 30s
      timeout: 10s
    resources:
      cpu: "1"
      memory: 512M
    gateways:
      - hostname: api.example.com
        container_port: 8080
        http: true
        https: true
    internal: false
```

#### 3.2.8 Domain（域名）

```yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare
    records:
      - type: A
        name: www
        value: 1.2.3.4
        ttl: 600
      - type: CNAME
        name: api
        value: www.example.com
```

**DNS 记录类型**: A | AAAA | CNAME | MX | TXT | NS | SRV



### 3.3 值对象

#### 3.3.1 SecretRef（密钥引用）

支持两种格式：

```yaml
# 明文形式
password: "plain-text"

# 引用形式
password: {secret: db_password}
```

#### 3.3.2 Change（变更）

字段为 unexported，通过 getter 方法访问：

```go
type ChangeType int
const (
    ChangeTypeNoop            // 无变更
    ChangeTypeCreate          // 创建
    ChangeTypeUpdate          // 更新
    ChangeTypeDelete          // 删除
)

type Change struct {
    changeType   ChangeType
    entity       string
    name         string
    oldState     interface{}
    newState     interface{}
    actions      []string
    remoteExists bool
}

// Getter 方法
func (c *Change) Type() ChangeType      // 变更类型
func (c *Change) Entity() string        // 实体类型名称
func (c *Change) Name() string          // 实体名称
func (c *Change) OldState() interface{} // 旧状态
func (c *Change) NewState() interface{} // 新状态
func (c *Change) Actions() []string     // 操作描述列表
func (c *Change) RemoteExists() bool    // 远程是否存在
```

#### 3.3.3 Scope（作用域）

```go
type Scope struct {
    domain        string
    zone          string
    server        string
    service       string
    services      []string
    infraServices []string
    forceDeploy   bool
    dnsOnly       bool
}

// Getter 方法
func (s *Scope) Domain() string          // 域名过滤
func (s *Scope) Zone() string            // 区域过滤
func (s *Scope) Server() string          // 服务器过滤
func (s *Scope) Service() string         // 服务过滤
func (s *Scope) Services() []string      // 服务列表过滤
func (s *Scope) InfraServices() []string // 基础设施服务列表过滤
func (s *Scope) ForceDeploy() bool       // 强制部署
func (s *Scope) DNSOnly() bool           // 仅 DNS

// 方法
func (s *Scope) Matches(zone, server, service, domain string) bool
func (s *Scope) MatchesInfra(zone, server, infraService string) bool
func (s *Scope) IsEmpty() bool
func (s *Scope) HasServicesOnly() bool
func (s *Scope) HasInfraServicesOnly() bool
func (s *Scope) HasAnyServiceSelection() bool
```

### 3.4 实体层级关系

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

## 4. 应用层设计

### 4.1 Handler 策略模式

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

### 4.2 Handler 接口

```go
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

type Result struct {
    Change   *valueobject.Change
    Success  bool
    Error    error
    Output   string
    Warnings []string
}
```

### 4.3 依赖注入接口

采用接口组合模式，支持细粒度的依赖注入：

```go
// DNS 操作依赖
type DNSDeps interface {
    DNSProvider(ispName string) (DNSProvider, error)
    Domain(name string) (*entity.Domain, bool)
    ISP(name string) (*entity.ISP, bool)
}

// 服务操作依赖
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

// 通用依赖
type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

// 组合依赖接口
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}
```

### 4.4 BaseDeps 实现结构

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
```

### 4.5 Handler 类型与职责

| Handler | Entity | 职责 |
|---------|--------|------|
| DNSHandler | dns_record | DNS 记录 CRUD |
| ServiceHandler | service | Docker Compose 服务部署 |
| InfraServiceHandler | infra_service | 基础设施服务部署 (gateway) |
| ServerHandler | server | 服务器环境同步（含 Registry 登录） |
| NoopHandler | isp/zone/domain/certificate | 空操作（非部署实体，跳过） |

### 4.6 Executor 执行器

Executor 委托给 ChangeExecutor 执行变更：

```go
type Executor struct {
    handlerRegistry *Registry
    changeExecutor  *ChangeExecutor
    plan            *valueobject.Plan
    env             string
}
```

**并行执行**：ChangeExecutor 将变更按服务器分组，不同服务器的变更在独立的 goroutine 中并行执行。DNS/ISP/Zone 类型的变更归入 `dns` 组。同一服务器组内的变更顺序执行。

```go
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

func (e *ChangeExecutor) Apply(registry handlerRegistry) []*Result {
    // 1. 按服务器分组变更
    // 2. DNS 变更排序：Delete → Update → Create
    // 3. 每组启动一个 goroutine 并行执行
    // 4. 汇总结果，关闭 SSH 连接池
}
```

### 4.7 Executor 依赖接口

```go
type RegistryInterface interface {
    Register(h Handler)
    Get(entityType string) (Handler, bool)
}

type SSHPoolInterface interface {
    Get(info *ServerInfo) (contract.SSHClient, error)
    CloseAll()
}

type DNSFactoryInterface interface {
    Create(isp *entity.ISP, secrets map[string]string) (dns.Provider, error)
}
```

---

## 5. 基础设施层

### 5.1 配置加载器

```go
type ConfigLoader struct{ baseDir string }

func NewConfigLoader(baseDir string) *ConfigLoader
func (l *ConfigLoader) Load(ctx context.Context, env string) (*entity.Config, error)
func (l *ConfigLoader) Validate(cfg *entity.Config) error
```

**加载顺序**（带 30 秒缓存）：
1. secrets.yaml
2. isps.yaml
3. zones.yaml
4. services_infra.yaml
5. servers.yaml
6. services_biz.yaml
7. registries.yaml
8. dns.yaml

### 5.2 状态存储

```go
type FileStore struct {
    path  string
    flock *flock.Flock  // github.com/gofrs/flock 文件锁
}

func NewFileStore(path string) *FileStore
func (s *FileStore) Load(ctx context.Context, env string) (*repository.DeploymentState, error)
func (s *FileStore) Save(ctx context.Context, env string, state *repository.DeploymentState) error
```

- **文件锁**：使用 `gofrs/flock` 实现进程级文件锁，防止并发读写冲突
- **原子写入**：先写入临时文件 `.state/{env}.yaml.tmp`，再 `os.Rename` 替换
- **存储路径**：`{configDir}/.state/{env}.yaml`

### 5.3 DNS 提供者

#### 5.3.1 Provider 接口

```go
type DNSProvider interface {
    Name() string
    ListDomains(ctx context.Context) ([]string, error)
    ListRecords(ctx context.Context, domain string) ([]DNSRecord, error)
    CreateRecord(ctx context.Context, domain string, record *DNSRecord) error
    DeleteRecord(ctx context.Context, domain string, recordID string) error
    UpdateRecord(ctx context.Context, domain string, recordID string, record *DNSRecord) error
    GetRecordsByTypes(ctx context.Context, domain, recordType string) ([]DNSRecord, error)
    BatchCreateRecords(ctx context.Context, domain string, records []*DNSRecord) error
    BatchDeleteRecords(ctx context.Context, domain string, recordIDs []string) error
    EnsureRecord(ctx context.Context, domain string, record *DNSRecord) error
}

type DNSRecord struct {
    ID    string
    Name  string
    Type  string
    Value string
    TTL   int
}
```

#### 5.3.2 公共逻辑 (common.go)

```go
// 确保记录存在，自动判断创建或更新（带重试）
func EnsureRecord(ctx context.Context, provider Provider, domain string, desired *DNSRecord, retryCfg *RetryConfig) error {
    records, err := provider.ListRecords(ctx, domain)
    // ...
    for _, existing := range records {
        if existing.Type == desired.Type && existing.Name == desired.Name {
            if existing.Value == desired.Value && existing.TTL == desired.TTL {
                return nil  // 无需变更
            }
            return provider.UpdateRecord(ctx, domain, existing.ID, desired)
        }
    }
    return provider.CreateRecord(ctx, domain, desired)
}

// 批量操作辅助函数
func BatchCreateRecordsHelper(ctx context.Context, provider Provider, domainName string, records []*DNSRecord) error
func BatchDeleteRecordsHelper(ctx context.Context, provider Provider, domainName string, recordIDs []string) error
```

#### 5.3.3 提供商实现

| Provider | 特性 |
|----------|------|
| Cloudflare | 分页查询、批量操作、Account 支持 |
| Aliyun | 按名称查询、模糊匹配 |
| Tencent | 域名 CRUD、状态控制 |

### 5.4 SSL 提供者

| Provider | 特性 |
|----------|------|
| Let's Encrypt | 无需 EAB、支持 Staging 环境 |
| ZeroSSL | 必须提供 EAB 凭据 |

### 5.5 SSH 客户端

```go
type Client struct {
    client           *ssh.Client
    user             string
    sftp             sftpClient
    lastHealthCheck  time.Time
    healthCheckCache bool
}

// 命令执行
func (c *Client) Run(cmd string) (stdout, stderr string, err error)
func (c *Client) RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error)
func (c *Client) StreamRun(ctx context.Context, cmd string, stdoutChan, stderrChan chan string) error

// 健康检查（30 秒缓存）
func (c *Client) Healthy() bool

// 文件传输
func (c *Client) UploadFile(localPath, remotePath string) error
func (c *Client) UploadFileSudo(localPath, remotePath string) error
func (c *Client) UploadFileSudoWithPerm(localPath, remotePath, perm string) error

// 目录操作
func (c *Client) MkdirAll(path string) error
func (c *Client) MkdirAllSudo(path string) error
func (c *Client) MkdirAllSudoWithPerm(path, perm string) error

// 文件检查
func (c *Client) FileExists(path string) (bool, error)

// 连接管理
func (c *Client) Close() error
func NewClientWithRetry(ctx context.Context, host string, port int, user, password string, cfg *SSHRetryConfig) (*Client, error)
```

### 5.6 Compose 生成器

生成 Docker Compose 文件格式：

```yaml
version: "3.8"
services:
  yo-{env}-{service-name}:
    image: {image}
    container_name: yo-{env}-{service-name}
    ports: [...]
    environment: {...}
    healthcheck: {...}
    deploy:
      resources:
        limits: {...}
    networks:
      - yamlops-{env}
    restart: unless-stopped
networks:
  yamlops-{env}: {}
```

### 5.7 Gate 生成器

生成 infra-gate 配置格式：

```yaml
server:
  port: 80
  g_zip_enabled: true
  http2_enabled: true

logger:
  level: 1

waf:
  enabled: true
  whitelist:
    ip_ranges: [...]

ssl:
  remote:
    enabled: true
    endpoint: https://ssl.litelake.com/cert/json

hosts:
  - name: api.example.com
    port: 80
    ssl_port: 443
    backend:
      - http://host.docker.internal:10080
```

---

## 6. 规划器设计

> 规划器（Planner）位于 `internal/application/plan/`，是应用层的一部分。

### 6.1 Planner 结构

```go
type Planner struct {
    config        *entity.Config
    deployGen     DeploymentGenerator      // interface
    stateRepo     repository.StateRepository // interface
    differService *service.DifferService
    outputDir     string
    env           string
}
```

### 6.2 Plan 工作流程

```
1. LoadAndValidate (加载 + 验证配置)
   └─→ 从 userdata/{env}/ 读取所有 YAML 文件

2. ResolveSecrets (解析密钥引用)
   └─→ 将 {secret: xxx} 引用替换为实际值

3. GenerateDeploymentsWithScope (生成部署文件)
   ├─→ generateServiceComposes()
   ├─→ generateInfraServiceComposes()
   └─→ generateGatewayConfigs()

4. FetchRemoteState (获取远程状态)
   └─→ 通过 SSH 获取服务器当前状态

5. Planner.Plan (生成计划)
   ├─→ PlanISPs()
   ├─→ PlanZones()
   ├─→ PlanDomains()
   ├─→ PlanRecords()
   ├─→ PlanServers()
   ├─→ PlanInfraServices()
   └─→ PlanServices()
```

### 6.3 变更检测算法

```go
func planSimpleEntity[T any](
    plan *valueobject.Plan,
    cfgMap map[string]*T,        // 配置中的实体
    stateMap map[string]*T,      // 状态中的实体
    equals func(a, b *T) bool,   // 比较函数
    entityName string,
    scopeMatcher func(name string) bool,
) {
    // 1. 检测删除：state 中有，cfg 中没有
    for name, state := range stateMap {
        if _, exists := cfgMap[name]; !exists {
            if scopeMatcher(name) {
                plan.AddChange(Delete)
            }
        }
    }
    
    // 2. 检测创建/更新
    for name, cfg := range cfgMap {
        if state, exists := stateMap[name]; exists {
            if !equals(state, cfg) {
                plan.AddChange(Update)
            }
        } else {
            plan.AddChange(Create)
        }
    }
}
```

---

## 7. CLI 命令系统

### 7.1 全局标志

| 标志 | 短标志 | 默认值 | 描述 |
|------|--------|--------|------|
| --env | -e | dev | 环境名称 |
| --config | -c | . | 配置目录路径 |
| --version | -v | false | 显示版本信息 |

### 7.2 命令层级

```
yamlops
├── tui                        # 启动交互界面（默认）
│   └── -e <env>               # 指定环境
├── cli                        # CLI 命令
│   ├── plan [scope]           # 生成执行计划
│   ├── apply [scope]          # 应用变更
│   ├── validate               # 验证配置
│   ├── list <entity>          # 列出实体
│   ├── show <entity> <name>   # 显示详情
│   ├── clean                  # 清理孤立资源
│   ├── env
│   │   ├── check              # 检查环境状态
│   │   └── sync               # 同步环境配置
│   ├── dns
│   │   ├── plan               # DNS 变更计划
│   │   ├── apply              # 应用 DNS 变更
│   │   ├── list [resource]    # 列出域名/记录
│   │   ├── show <resource> <name>
│   │   └── pull
│   │       ├── domains        # 从 ISP 拉取域名
│   │       └── records        # 从域名拉取记录
│   ├── server
│   │   ├── setup              # 完整设置（check + sync）
│   │   ├── check              # 检查服务器状态
│   │   └── sync               # 同步服务器配置
│   ├── service
│   │   ├── deploy             # 部署服务
│   │   ├── stop               # 停止服务
│   │   ├── restart            # 重启服务
│   │   └── cleanup            # 清理孤立容器和目录
│   ├── config
│   │   ├── list [type]        # 列出配置项
│   │   └── show <type> <name> # 显示配置详情
│   └── app
│       ├── plan               # 应用部署计划
│       ├── apply              # 应用部署
│       ├── list [resource]    # 列出资源
│       └── show <resource> <name>
└── api                        # API 服务（计划中）
```

### 7.3 过滤标志

| 标志 | 适用命令 | 描述 |
|------|----------|------|
| --domain | dns show/deploy/pull | 按域名过滤 |
| --zone | server/service 子命令 | 按区域过滤 |
| --server | server/service 子命令 | 按服务器过滤 |
| --type | service 子命令 | 服务类别过滤 (biz/infra) |
| --service | service 子命令 | 按服务名过滤（逗号分隔多选） |
| --dry-run | 所有变更命令 | 预览变更不执行 |
| --yes | 所有变更命令 | 跳过确认 |
| --force | service deploy/dns deploy/dns pull | 强制执行 |
| --detail | 所有 show 命令 | 显示详细信息 |

### 7.4 TUI 快捷键

| 按键 | 功能 |
|------|------|
| ↑/k, ↓/j | 上下移动 |
| Space | 切换选择 |
| Enter | 确认/展开 |
| Esc | 返回上一级/取消 |
| ? | 帮助 |
| q | 退出（仅主菜单） |
| a / n | 全选/全不选 |
| d | 切换摘要/详细视图 |
| f | 切换 force 模式 |
| / | 搜索过滤 |
| Ctrl+C | 中断执行 |
| r | 重新 Plan |

---

## 8. 配置文件规范

### 8.1 目录结构

```
userdata/
├── prod/                    # 生产环境
│   ├── secrets.yaml
│   ├── isps.yaml
│   ├── zones.yaml
│   ├── servers.yaml
│   ├── services_biz.yaml
│   ├── services_infra.yaml
│   ├── registries.yaml
│   └── dns.yaml
├── staging/                 # 预发布环境
│   └── ...
├── dev/                     # 开发环境
│   └── ...
└── demo/                    # 演示环境
    └── ...
```

### 8.2 密钥引用规则

```yaml
# 明文形式
password: "plain-text"

# 引用 secrets.yaml 中的密钥
password: {secret: db_password}
```

### 8.3 命名规范

| 元素 | 格式 | 示例 |
|------|------|------|
| 容器名 | yo-{env}-{name} | yo-prod-api-server |
| 网络名 | yamlops-{env} | yamlops-prod |
| 部署目录 | /data/yamlops/yo-{env}-{name} | /data/yamlops/yo-prod-api |

---

## 9. 关键规则

### 9.1 引用完整性

- Zone 必须引用存在的 ISP
- Server 必须引用存在的 Zone
- InfraService 必须引用存在的 Server
- BizService 必须引用存在的 Server
- Domain 必须引用存在的 DNS ISP

### 9.2 唯一性约束

- Domain 名称不能重复
- DNS 记录 (Type + Name + Value) 不能重复
- 服务网关路由的 Hostname 不能重复

### 9.3 端口规则

- 所有端口必须在 1-65535 范围
- 同一服务器上的端口不能冲突
- TTL 必须为非负整数

### 9.4 格式规则

- 健康检查路径必须以 / 开头
- WAF 白名单必须是有效的 CIDR 格式
- 网关 SSL 模式必须是 local 或 remote
- 域名支持通配符前缀 `*.`

---

## 10. 使用方法

### 10.1 标准工作流

```bash
# 1. 验证配置
yamlops cli service validate -e prod
yamlops cli server validate -e prod
yamlops cli dns validate -e prod
yamlops cli config validate -e prod

# 2. 预览变更
yamlops cli service deploy -e prod --dry-run

# 3. 应用变更
yamlops cli service deploy -e prod --yes
```

### 10.2 服务器设置

```bash
# 预览环境差异
yamlops cli server setup -e prod --server prod-server-1 --dry-run

# 同步环境
yamlops cli server setup -e prod --server prod-server-1 --yes
```

### 10.3 DNS 管理

```bash
# 从 ISP 拉取域名
yamlops cli dns pull domains -e prod --isp aliyun

# 从域名拉取记录
yamlops cli dns pull records -e prod --domain example.com

# 预览 DNS 变更
yamlops cli dns deploy -e prod --dry-run

# 应用 DNS 变更
yamlops cli dns deploy -e prod --yes
```

### 10.4 服务部署

```bash
# 预览变更
yamlops cli service deploy -e prod --server prod-server-1 --dry-run

# 应用变更
yamlops cli service deploy -e prod --server prod-server-1 --yes
```

### 10.5 服务操作

```bash
# 部署服务
yamlops cli service deploy -e prod --type biz --yes

# 停止服务
yamlops cli service stop -e prod --type biz --yes

# 重启服务
yamlops cli service restart -e prod --type biz --yes

# 清理孤立资源
yamlops cli service cleanup -e prod --yes
```

### 10.6 交互模式

```bash
# 启动 TUI 界面
yamlops tui -e prod
```

---

## 附录

### A. 错误码

Domain 层统一定义在 `internal/domain/errors.go`：

#### 验证错误

| 错误 | 说明 |
|------|------|
| ErrInvalidName | 无效名称 |
| ErrInvalidIP | 无效 IP 地址 |
| ErrInvalidPort | 无效端口 |
| ErrInvalidProtocol | 无效协议 |
| ErrInvalidDomain | 无效域名 |
| ErrInvalidCIDR | 无效 CIDR 格式 |
| ErrInvalidURL | 无效 URL |
| ErrInvalidTTL | 无效 TTL |
| ErrInvalidDuration | 无效时长 |
| ErrInvalidType | 无效类型 |
| ErrInvalidPath | 无效路径 |
| ErrInvalidFormat | 无效格式 |
| ErrEmptyValue | 空值 |
| ErrRequired | 必填字段缺失 |
| ErrMissingSecret | 缺少密钥引用 |
| ErrConfigNotLoaded | 配置未加载 |
| ErrMissingReference | 缺少引用 |
| ErrPortConflict | 端口冲突 |
| ErrDomainConflict | 域名冲突 |
| ErrHostnameConflict | 主机名冲突 |
| ErrDNSSubdomainConflict | DNS 子域名冲突 |

#### SSH 错误

| 错误 | 说明 |
|------|------|
| ErrSSHConnectFailed | SSH 连接失败 |
| ErrSSHAuthFailed | SSH 认证失败 |
| ErrSSHSessionFailed | SSH 会话创建失败 |
| ErrSSHCommandFailed | SSH 命令执行失败 |
| ErrSSHHostKeyMismatch | SSH 主机密钥不匹配 |
| ErrSSHFileTransfer | SSH 文件传输失败 |
| ErrSSHClientNotAvailable | SSH 客户端不可用 |

#### 配置错误

| 错误 | 说明 |
|------|------|
| ErrConfigReadFailed | 配置读取失败 |
| ErrConfigParseFailed | 配置解析失败 |
| ErrConfigValidateFail | 配置验证失败 |
| ErrConfigNotFound | 配置未找到 |

#### 状态错误

| 错误 | 说明 |
|------|------|
| ErrStateReadFailed | 状态读取失败 |
| ErrStateWriteFailed | 状态写入失败 |
| ErrStateSerializeFail | 状态序列化失败 |
| ErrStateNotFound | 状态未找到 |

#### DNS 错误

| 错误 | 说明 |
|------|------|
| ErrDNSError | DNS 操作失败 |
| ErrDNSRecordExists | DNS 记录已存在 |
| ErrDNSRecordNotFound | DNS 记录未找到 |
| ErrDNSDomainNotFound | DNS 域名未找到 |

#### 网络错误

| 错误 | 说明 |
|------|------|
| ErrNetworkTimeout | 网络超时 |
| ErrNetworkUnreachable | 网络不可达 |
| ErrDNSResolveFailed | DNS 解析失败 |
| ErrConnectionRefused | 连接被拒绝 |
| ErrConnectionReset | 连接重置 |

#### 文件错误

| 错误 | 说明 |
|------|------|
| ErrFileReadFailed | 文件读取失败 |
| ErrFileWriteFailed | 文件写入失败 |
| ErrFileNotFound | 文件未找到 |
| ErrTempFileFailed | 临时文件操作失败 |
| ErrDirectoryCreateFailed | 目录创建失败 |
| ErrDirectoryRemoveFailed | 目录删除失败 |

#### Compose 错误

| 错误 | 说明 |
|------|------|
| ErrComposeGenerateFailed | Compose 生成失败 |
| ErrComposeSyncFailed | Compose 同步失败 |
| ErrDockerComposeFailed | Docker Compose 执行失败 |
| ErrServiceInvalid | 服务无效 |

#### 其他错误

| 错误 | 说明 |
|------|------|
| ErrISPNotFound | ISP 未找到 |
| ErrISPNoDNSService | ISP 不提供 DNS 服务 |
| ErrServerNotRegistered | 服务器未注册 |
| ErrRegistryNotFound | Registry 未找到 |
| ErrRegistryLoginFailed | Registry 登录失败 |
| ErrUnsupportedProvider | 不支持的提供商类型 |
| ErrMissingCredential | 缺少凭据 |

### B. 部署状态

```go
type DeploymentState struct {
    Secrets       map[string]string
    Registries    map[string]*entity.Registry
    ISPs          map[string]*entity.ISP
    Zones         map[string]*entity.Zone
    Servers       map[string]*entity.Server
    InfraServices map[string]*entity.InfraService
    Services      map[string]*entity.BizService
    Domains       map[string]*entity.Domain
    Records       map[string]*entity.DNSRecord
}
```

### C. 设计模式

| 模式 | 应用位置 |
|------|----------|
| 策略模式 | Handler 注册表，不同 Handler 实现 |
| 工厂模式 | DNS Provider 创建（infrastructure/dns/factory.go） |
| 适配器模式 | DNS Provider 适配 |
| 对象池模式 | SSH 连接池（usecase/ssh_pool.go） |
| 依赖注入 (DIP) | Executor 通过 ExecutorConfig 接收依赖 |
| 接口隔离 (ISP) | DNSDeps / ServiceDeps / CommonDeps 组合为 DepsProvider |
| 注册表模式 | Handler Registry（handler/registry.go） |
| Option 模式 | Planner 配置、Retry 重试机制 |
| 泛型编程 | planSimpleEntity 函数（domain/service/differ_generic.go） |

### D. 架构改进

#### D.1 Handler 依赖接口隔离 (ISP)

Handler 依赖拆分为专注的接口，定义在 `internal/application/usecase/types.go`：

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

#### D.2 Executor 依赖注入 (DIP)

Executor 通过配置结构接收依赖，位于 `internal/application/usecase/executor.go`：

```go
type ExecutorConfig struct {
    Registry   RegistryInterface
    SSHPool    SSHPoolInterface
    DNSFactory DNSFactoryInterface
    Plan       *valueobject.Plan
    Env        string
}

func NewExecutor(cfg *ExecutorConfig) *Executor
```

#### D.3 Orchestrator 工作流模块

`internal/application/orchestrator/` 封装工作流编排逻辑：

```go
type Workflow struct {
    env          string
    configDir    string
    loader       repository.ConfigLoader
    stateFetcher *StateFetcher
}

func (w *Workflow) Env() string
func (w *Workflow) LoadConfig(ctx context.Context) (*entity.Config, error)
func (w *Workflow) LoadAndValidate(ctx context.Context) (*entity.Config, error)
func (w *Workflow) ResolveSecrets(cfg *entity.Config) error
func (w *Workflow) CreatePlanner(cfg *entity.Config, outputDir string) *plan.Planner
func (w *Workflow) Plan(ctx context.Context, outputDir string, scope *valueobject.Scope) (*valueobject.Plan, *entity.Config, error)
func (w *Workflow) GenerateDeployments(cfg *entity.Config, outputDir string) error
func (w *Workflow) GenerateDeploymentsWithScope(cfg *entity.Config, outputDir string, scope *valueobject.Scope) error
func (w *Workflow) FetchRemoteState(ctx context.Context, cfg *entity.Config) *repository.DeploymentState
func (w *Workflow) SaveState(ctx context.Context, cfg *entity.Config) error
```

#### D.4 Generator 生成器模块

`internal/application/generator/` 负责生成部署文件：

```go
type Generator struct {
    composeGen *compose.Generator
    gateGen    *gate.Generator
    outputDir  string
    env        string
}

func (g *Generator) Generate(config *entity.Config) error
func (g *Generator) GenerateWithScope(config *entity.Config, scope *valueobject.Scope) error
```

生成内容：
- Docker Compose 文件（业务服务 + 基础设施服务）
- Gateway 配置文件

#### D.5 Planner 规划器

`internal/application/plan/planner.go` 使用 Option 模式：

```go
type Planner struct {
    config        *entity.Config
    deployGen     DeploymentGenerator      // interface
    stateRepo     repository.StateRepository // interface
    differService *service.DifferService
    outputDir     string
    env           string
}

type PlannerOption func(*Planner)

func WithConfig(cfg *entity.Config) PlannerOption
func WithEnv(env string) PlannerOption
func WithState(st *DeploymentState) PlannerOption
func WithStateRepo(repo repository.StateRepository) PlannerOption
func WithGenerator(gen DeploymentGenerator) PlannerOption
func WithOutputDir(dir string) PlannerOption
```

#### D.6 Retry 重试机制

`internal/domain/retry/retry.go` 提供 Option 模式的重试机制：

```go
type Config struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay    time.Duration
    Multiplier  float64
}

type Option func(*Config)

func WithMaxAttempts(n int) Option
func WithInitialDelay(d time.Duration) Option

func Do(ctx context.Context, fn func() error, opts ...Option) error
func DoWithResult[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error)
```

#### D.7 Secrets 解析器

`internal/infrastructure/secrets/resolver.go` 负责密钥引用解析：

```go
type SecretResolver struct {
    secrets map[string]string
}

func NewSecretResolver(secrets []*entity.Secret) *SecretResolver
func (r *SecretResolver) Resolve(ref valueobject.SecretRef) (string, error)
func (r *SecretResolver) ResolveAll(cfg *entity.Config) error
```

#### D.8 Environment 管理模块

`internal/environment/` 负责服务器环境检查和同步：

| 文件 | 职责 |
|------|------|
| checker.go | 检查 SSH 连接、Sudo 免密、Docker、Docker Compose、APT 源、Registry 登录状态 |
| syncer.go | 同步服务器环境配置 |
| templates.go | APT 源配置模板 |
| types.go | 类型定义（CheckResult、CheckStatus） |

#### D.9 TUI 模块拆分

TUI 已拆分为独立包 `internal/interfaces/tui/`，与 CLI 命令分离：

| 文件 | 职责 |
|------|------|
| tui.go | 主入口、Update 循环 |
| tui_model.go | 数据模型定义 |
| tui_view.go | 主视图渲染 |
| tui_render.go | 渲染逻辑 |
| tui_server.go | 服务器操作（检查、同步） |
| tui_dns.go | DNS 操作（拉取、管理） |
| tui_service_common.go | 服务公共逻辑 |
| tui_cleanup.go | 服务清理（孤立资源） |
| tui_stop.go | 服务停止 |
| tui_restart.go | 服务重启 |

共享样式定义在 `internal/interfaces/shared/styles/`。

#### D.10 常量集中管理

应用级常量统一在 `internal/constants/constants.go` 定义：

```go
const (
    RemoteBaseDir     = "/data/yamlops"
    ServiceNameFormat = "yo-%s-%s"
    TempFilePattern   = "yamlops-*.yml"
    RemoteTempFileFmt = "/tmp/yamlops-%d"
    StateDir          = ".state"
    StateFileFormat   = "%s.yaml"
)

// 默认端口
const (
    DefaultHTTPPort  = 80
    DefaultHTTPSPort = 443
)

// Compose 配置
const (
    ComposeVersion       = "3.8"
    DefaultRestartPolicy = "unless-stopped"
)

// SSH 配置
const (
    DefaultSSHTimeoutSec           = 30
    DefaultSSHRetryAttempts        = 3
    DefaultSSHRetryInitialDelaySec = 1
    DefaultSSHRetryMaxDelaySec     = 30
)

// 文件权限
const (
    FilePermissionOwnerRW = 0600
    DirPermissionOwner    = 0700
    DefaultRemoteFilePerm = "644"
    DefaultRemoteDirPerm  = "755"
)
```

#### D.11 统一 Domain 错误

所有领域错误集中在 `internal/domain/errors.go` 定义，便于统一管理和复用：

```go
var (
    ErrInvalidName      = errors.New("invalid name")
    ErrInvalidIP        = errors.New("invalid IP address")
    // ...
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}

func WrapOp(op string, err error) error {
    return fmt.Errorf("%s: %w", op, err)
}

func WrapEntity(entity, name string, err error) error {
    return fmt.Errorf("%s[%s]: %w", entity, name, err)
}
```

#### D.12 Logger 日志模块

`internal/infrastructure/logger/` 提供日志基础设施：

| 文件 | 职责 |
|------|------|
| logger.go | 日志主入口、全局 Logger |
| context.go | 上下文日志、字段管理 |
| metrics.go | 指标记录 |
