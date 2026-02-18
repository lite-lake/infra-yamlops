# YAMLOps 系统设计说明

## 目录

- [1. 系统概述](#1-系统概述)
- [2. 架构设计](#2-架构设计)
- [3. 领域模型](#3-领域模型)
- [4. 应用层设计](#4-应用层设计)
- [5. 基础设施层](#5-基础设施层)
- [6. Plan 层设计](#6-plan-层设计)
- [7. CLI 命令系统](#7-cli-命令系统)
- [8. 配置文件规范](#8-配置文件规范)
- [9. 关键规则](#9-关键规则)
- [10. 使用方法](#10-使用方法)

---

## 1. 系统概述

### 1.1 项目定位

YAMLOps 是一个基于 Go 语言开发的基础设施即代码（IaC）管理工具，通过 YAML 配置文件管理：

- **服务器**：SSH 连接、环境配置
- **服务**：Docker Compose 部署
- **网关**：infra-gate 配置管理
- **DNS**：域名和记录管理（支持 Cloudflare、阿里云、腾讯云）
- **SSL 证书**：Let's Encrypt、ZeroSSL 自动化证书管理

### 1.2 核心特性

| 特性 | 说明 |
|------|------|
| 多环境支持 | prod / staging / dev 环境隔离 |
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

---

## 2. 架构设计

### 2.1 分层架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Interface Layer                               │
│                      (interfaces/cli/)                               │
│    Cobra 命令 + BubbleTea TUI，处理用户输入输出                        │
├─────────────────────────────────────────────────────────────────────┤
│                      Application Layer                               │
│                    (application/)                                    │
│    Handler 策略模式 + Executor 编排器，协调用例执行                     │
├─────────────────────────────────────────────────────────────────────┤
│                       Plan Layer                                     │
│                         (plan/)                                      │
│    规划器 + 部署生成器，生成执行计划和部署文件                          │
├─────────────────────────────────────────────────────────────────────┤
│                        Domain Layer                                  │
│                         (domain/)                                    │
│    实体 + 值对象 + 仓储接口 + 领域服务，核心业务逻辑                     │
├─────────────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                              │
│                     (infrastructure/)                                │
│    配置加载 + DNS/SSL Provider + SSH + Compose/Gate 生成              │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 依赖规则

```
Interface → Application → Plan → Domain ← Infrastructure
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
│   ├── service/                    # 领域服务
│   └── errors.go                   # 领域错误
├── application/                    # 应用层
│   ├── handler/                    # 变更处理器（策略模式）
│   └── usecase/                    # 用例执行器
├── infrastructure/                 # 基础设施层
│   └── persistence/                # 配置加载实现
├── interfaces/                     # 接口层
│   └── cli/                        # CLI 命令
├── plan/                           # 规划协调层
├── config/                         # 配置工具
├── providers/                      # 外部服务提供者
│   ├── dns/                        # DNS 提供者
│   └── ssl/                        # SSL 提供者
├── ssh/                            # SSH 客户端
├── compose/                        # Docker Compose 工具
└── gate/                           # infra-gate 工具
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
├── InfraServices[]     # 基础设施服务
├── Gateways[]          # 网关
├── Services[]          # 业务服务
├── Domains[]           # 域名
└── Certificates[]      # SSL 证书
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
    services: [server, domain, dns] # server | domain | dns | certificate
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

```yaml
registries:
  - name: dockerhub
    url: https://registry.hub.docker.com
    credentials:
      username: {secret: docker_user}
      password: {secret: docker_pass}
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

#### 3.2.6 Gateway（网关）

```yaml
gateways:
  - name: main-gateway
    zone: cn-east
    server: prod-server-1
    image: litelake/infra-gate:latest
    ports:
      http: 80
      https: 443
    ssl:
      mode: remote           # local | remote
      endpoint: http://infra-ssl:38567
    waf:
      enabled: true
      whitelist:
        - 192.168.0.0/16
    log_level: 1
```

#### 3.2.7 InfraService（基础设施服务）

```yaml
infra_services:
  - name: infra-ssl
    type: ssl                # gateway | ssl
    server: prod-server-1
    image: litelake/infra-ssl:latest
    ssl_config:
      ports:
        http: 8080
        acme: 38567
      auth:
        username: admin
        password: {secret: ssl_admin_pwd}
```

#### 3.2.8 BizService（业务服务）

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

#### 3.2.9 Domain（域名）

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

#### 3.2.10 Certificate（证书）

```yaml
certificates:
  - name: wildcard-cert
    domains: ["*.example.com", "example.com"]
    provider: letsencrypt    # letsencrypt | zerossl
    dns_provider: cloudflare
    renew_before: 720h
```

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

```go
type ChangeType int
const (
    ChangeTypeNoop            // 无变更
    ChangeTypeCreate          // 创建
    ChangeTypeUpdate          // 更新
    ChangeTypeDelete          // 删除
)

type Change struct {
    Type     ChangeType    // 变更类型
    Entity   string        // 实体类型名称
    Name     string        // 实体名称
    OldState interface{}   // 旧状态
    NewState interface{}   // 新状态
    Actions  []string      // 操作描述列表
}
```

#### 3.3.3 Scope（作用域）

```go
type Scope struct {
    Domain  string  // 域名过滤
    Zone    string  // 区域过滤
    Server  string  // 服务器过滤
    Service string  // 服务过滤
}
```

### 3.4 实体层级关系

```
ISP (底层基础设施提供商)
  └── Zone (网络区域)
        ├── Server (物理/虚拟服务器)
        │     ├── Gateway (网关服务)
        │     ├── InfraService (基础设施服务)
        │     └── BizService (业务服务)
        │           └── ServiceGatewayRoute (网关路由)
        └── Domain (域名)
              ├── DNSRecord (DNS记录)
              └── Certificate (SSL证书)
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
│DNSHandler│  │ServiceH.│  │GatewayH.│  │NoopH.   │ ...
└─────────┘  └─────────┘  └─────────┘  └─────────┘
```

### 4.2 Handler 接口

```go
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error)
}
```

### 4.3 依赖注入结构

```go
type Deps struct {
    SSHClient   SSHClient                    // SSH 客户端
    DNSProvider DNSProvider                  // DNS 服务提供商
    DNSFactory  *dns.Factory                 // DNS 提供商工厂
    Secrets     map[string]string            // 密钥解析后的值
    Domains     map[string]*entity.Domain    // 域名配置
    ISPs        map[string]*entity.ISP       // ISP 配置
    Servers     map[string]*ServerInfo       // 服务器信息
    WorkDir     string                       // 工作目录
    Env         string                       // 环境
}
```

### 4.4 Handler 类型与职责

| Handler | Entity | 职责 |
|---------|--------|------|
| DNSHandler | dns_record | DNS 记录 CRUD |
| ServiceHandler | service | Docker Compose 服务部署 |
| GatewayHandler | gateway | 网关配置 + Compose 部署 |
| ServerHandler | server | 服务器注册（无远程操作） |
| CertificateHandler | certificate | 证书管理（标记跳过） |
| RegistryHandler | registry | 镜像仓库（标记跳过） |
| NoopHandler | isp/zone/domain | 空操作 |

### 4.5 Executor 执行器

```go
func (e *Executor) Apply() []*handler.Result {
    e.registerHandlers()  // 注册所有 Handler
    
    for _, ch := range e.plan.Changes {
        h, ok := e.registry.Get(ch.Entity)
        result := h.Apply(ctx, ch, e.buildDeps(ch))
        results = append(results, result)
    }
    
    e.sshPool.CloseAll()
    return results
}
```

---

## 5. 基础设施层

### 5.1 配置加载器

```go
type ConfigLoader struct{ baseDir string }

func (l *ConfigLoader) Load(ctx context.Context, env string) (*entity.Config, error)
```

**加载顺序**：
1. secrets.yaml
2. isps.yaml
3. zones.yaml
4. infra_services.yaml
5. gateways.yaml
6. servers.yaml
7. services.yaml
8. registries.yaml
9. dns.yaml
10. certificates.yaml

### 5.2 DNS 提供者

| Provider | 特性 |
|----------|------|
| Cloudflare | 分页查询、批量操作、Account 支持 |
| Aliyun | 按名称查询、模糊匹配 |
| Tencent | 域名 CRUD、状态控制 |

### 5.3 SSL 提供者

| Provider | 特性 |
|----------|------|
| Let's Encrypt | 无需 EAB、支持 Staging 环境 |
| ZeroSSL | 必须提供 EAB 凭据 |

### 5.4 SSH 客户端

```go
type Client struct {
    client *ssh.Client
    user   string
}

// 命令执行
func (c *Client) Run(cmd string) (stdout, stderr string, err error)

// 文件传输
func (c *Client) UploadFileSudo(localPath, remotePath string) error
func (c *Client) MkdirAllSudoWithPerm(path, perm string) error
```

### 5.5 Compose 生成器

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

### 5.6 Gate 生成器

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
    endpoint: http://infra-ssl:38567

hosts:
  - name: api.example.com
    port: 80
    ssl_port: 443
    backend:
      - http://host.docker.internal:10080
```

---

## 6. Plan 层设计

### 6.1 Planner 结构

```go
type Planner struct {
    config         *entity.Config
    plannerService *service.PlannerService
    deployGen      *deploymentGenerator
    outputDir      string
    env            string
}
```

### 6.2 Plan 工作流程

```
1. 加载配置 (ConfigLoader)
   └─→ 从 userdata/{env}/ 读取所有 YAML 文件

2. 验证配置 (Validator)
   └─→ 引用完整性检查、端口冲突检测、域名冲突检测

3. 生成计划 (PlannerService)
   ├─→ PlanISPs()
   ├─→ PlanZones()
   ├─→ PlanDomains()
   ├─→ PlanRecords()
   ├─→ PlanCertificates()
   ├─→ PlanRegistries()
   ├─→ PlanServers()
   ├─→ PlanGateways()
   └─→ PlanServices()

4. 生成部署文件 (deploymentGenerator)
   ├─→ generateServiceComposes()
   └─→ generateGatewayConfigs()
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
├── (TUI)                    # 默认，启动交互界面
├── plan [scope]             # 生成执行计划
├── apply [scope]            # 应用变更
├── validate                 # 验证配置
├── list <entity>            # 列出实体
├── show <entity> <name>     # 显示详情
├── clean                    # 清理孤立资源
├── env
│   ├── check                # 检查环境状态
│   └── sync                 # 同步环境配置
├── dns
│   ├── plan                 # DNS 变更计划
│   ├── apply                # 应用 DNS 变更
│   ├── list [resource]      # 列出域名/记录
│   ├── show <resource> <name>
│   └── pull
│       ├── domains          # 从 ISP 拉取域名
│       └── records          # 从域名拉取记录
├── server
│   ├── setup                # 完整设置（check + sync）
│   ├── check                # 检查服务器状态
│   └── sync                 # 同步服务器配置
├── config
│   ├── list [type]          # 列出配置项
│   └── show <type> <name>   # 显示配置详情
└── app
    ├── plan                 # 应用部署计划
    ├── apply                # 应用部署
    ├── list [resource]      # 列出资源
    └── show <resource> <name>
```

### 7.3 过滤标志

| 标志 | 适用命令 | 描述 |
|------|----------|------|
| --domain / -d | plan, apply, dns | 按域名过滤 |
| --zone / -z | plan, apply, app | 按区域过滤 |
| --server / -s | plan, apply, app, env | 按服务器过滤 |
| --service | plan, apply | 按服务过滤 |
| --auto-approve | apply, dns apply | 跳过确认提示 |

### 7.4 TUI 快捷键

| 按键 | 功能 |
|------|------|
| ↑/k, ↓/j | 上下移动 |
| Space | 切换选择 |
| Enter | 确认/展开 |
| a/n | 选择/取消当前项 |
| A/N | 全选/全不选 |
| p | 生成计划 |
| r | 刷新配置 |
| Esc | 返回 |
| q/Ctrl+C | 退出 |

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
│   ├── services.yaml
│   ├── gateways.yaml
│   ├── infra_services.yaml
│   ├── registries.yaml
│   ├── dns.yaml
│   └── certificates.yaml
├── staging/                 # 预发布环境
│   └── ...
└── dev/                     # 开发环境
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
- Gateway 必须引用存在的 Zone 和 Server
- BizService 必须引用存在的 Server
- Domain 必须引用存在的 DNS ISP
- Certificate 必须引用存在的 Domain

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
yamlops validate -e prod

# 2. 生成执行计划
yamlops plan -e prod

# 3. 应用变更
yamlops apply -e prod
```

### 10.2 服务器设置

```bash
# 完整设置
yamlops server setup -e prod --server prod-server-1

# 仅检查
yamlops server check -e prod --zone cn-east

# 仅同步
yamlops server sync -e prod --server prod-server-1
```

### 10.3 DNS 管理

```bash
# 从 ISP 拉取域名
yamlops dns pull domains --isp aliyun

# 从域名拉取记录
yamlops dns pull records --domain example.com

# 生成 DNS 变更计划
yamlops dns plan -e prod

# 应用 DNS 变更
yamlops dns apply -e prod --auto-approve
```

### 10.4 应用部署

```bash
# 生成部署计划
yamlops app plan -e prod --server prod-server-1

# 应用部署
yamlops app apply -e prod --server prod-server-1
```

### 10.5 清理操作

```bash
# 清理孤立资源
yamlops clean -e prod
```

### 10.6 交互模式

```bash
# 启动 TUI 界面
yamlops -e prod
```

---

## 附录

### A. 错误码

| 错误 | 说明 |
|------|------|
| ErrInvalidName | 无效名称 |
| ErrInvalidIP | 无效 IP 地址 |
| ErrInvalidPort | 无效端口 |
| ErrInvalidDomain | 无效域名 |
| ErrMissingSecret | 缺少密钥引用 |
| ErrMissingReference | 缺少引用 |
| ErrPortConflict | 端口冲突 |
| ErrDomainConflict | 域名冲突 |
| ErrHostnameConflict | 主机名冲突 |

### B. 部署状态

```go
type DeploymentState struct {
    Services   map[string]*entity.BizService
    Gateways   map[string]*entity.Gateway
    Servers    map[string]*entity.Server
    Zones      map[string]*entity.Zone
    Domains    map[string]*entity.Domain
    Records    map[string]*entity.DNSRecord
    Certs      map[string]*entity.Certificate
    Registries map[string]*entity.Registry
    ISPs       map[string]*entity.ISP
}
```

### C. 设计模式

| 模式 | 应用位置 |
|------|----------|
| 策略模式 | Handler 注册表 |
| 工厂模式 | DNS/SSL Provider 创建 |
| 适配器模式 | DNS Provider 适配 |
| 对象池模式 | SSH 连接池 |
| 依赖注入 | Handler Deps 结构 |
| 泛型编程 | planSimpleEntity 函数 |
