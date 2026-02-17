# YAMLOps 设计方案

基于 YAML 存储的运维工具，支持交互式 CLI。

## 1. 项目概述

| 项目 | 说明 |
|------|------|
| 名称 | yamlops |
| 语言 | Go |
| CLI 框架 | BubbleTea (TUI) |
| 配置格式 | YAML (每个实体一个文件) |
| 多环境 | 独立目录 (prod/, staging/, dev/) |

### 核心理念

- **声明式配置**：在 YAML 中描述期望状态，工具负责达成
- **Plan + Apply**：先预览变更，确认后执行
- **全量对比**：不允许手动修改服务器，工具管理一切
- **网关统一入口**：所有对外服务通过网关暴露

### 两个领域

```
┌─────────────────────────────────────────────────────────────┐
│                    全局资源领域                              │
│         (跨网区共享，独立管理)                               │
│                                                             │
│   域名 ◄────────────► DNS解析 ◄────────────► SSL证书       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 引用/关联
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    基础设施领域                              │
│              (按网区组织，物理部署)                          │
│                                                             │
│   网区 ──► 网关 ──► 服务器 ──► 服务                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 目录结构

```
yamlops/
├── cmd/
│   └── yamlops/              # CLI 入口
├── internal/
│   ├── cli/                  # BubbleTea TUI 界面
│   ├── config/               # 配置加载与解析
│   ├── plan/                 # 变更计划生成
│   ├── apply/                # 变更执行
│   ├── ssh/                  # SSH 远程执行
│   ├── providers/            # 各服务商 SDK 封装
│   │   ├── dns/              # DNS 解析 (CF/阿里/腾讯)
│   │   └── ssl/              # SSL 证书 (LE/ZeroSSL)
│   ├── entities/             # 实体定义与校验
│   ├── compose/              # docker-compose 生成
│   └── gate/                 # infra-gate 配置生成
├── pkg/
│   └── schema/               # YAML Schema 定义
└── userdata/                 # 用户配置目录
    ├── prod/
    │   ├── isps.yaml
    │   ├── zones.yaml
    │   ├── gateways.yaml
    │   ├── servers.yaml
    │   ├── registries.yaml
    │   ├── services.yaml
    │   ├── domains.yaml
    │   ├── dns.yaml
    │   ├── certificates.yaml
    │   ├── secrets.yaml
    │   └── volumes/          # 配置文件目录
    │       ├── infra-gate/
    │       │   └── server.yml
    │       └── api-server/
    │           └── config.yml
    ├── staging/
    │   └── ...
    └── dev/
        └── ...
```

---

## 3. 服务器端部署规范

### 3.1 服务命名与路径

所有由 yamlops 管理的服务：

- **命名前缀**：`yo-<env>-` (便于识别环境和管理)
- **部署路径**：`/data/yamlops/yo-<env>-<服务名>/`

这样设计支持一台服务器同时运行多个环境的服务，节约服务器资源。

```
/data/yamlops/
├── yo-prod-infra-gate/          # 生产环境网关服务
│   ├── docker-compose.yml
│   └── volumes/
│       └── config/
│           └── server.yml
├── yo-prod-api-server/
│   └── docker-compose.yml
├── yo-staging-infra-gate/       # 预发环境网关服务
│   └── ...
├── yo-staging-api-server/
│   └── ...
├── yo-dev-infra-gate/           # 开发环境网关服务
│   └── ...
└── yo-dev-api-server/
    └── ...
```

### 3.2 Volume 同步机制

服务可以定义需要同步的配置文件目录，在 Apply 时自动上传到服务器。

**本地配置目录结构**：
```
userdata/
├── prod/
│   ├── services.yaml
│   └── volumes/                    # 配置文件存放位置
│       ├── api-server/
│       │   └── config.yml
│       └── infra-gate/
│           └── server.yml
```

**服务定义中引用**：
```yaml
services:
  - name: api-server
    volumes:
      - source: volumes://api-server/config.yml    # 引用本地配置
        target: /app/config.yml                     # 容器内路径
        sync: true                                  # 同步到服务器
```

**同步后服务器路径** (以 prod 环境为例)：
```
/data/yamlops/yo-prod-api-server/volumes/api-server/config.yml
```

### 3.3 网关服务

网关也是一个普通服务，名为 `infra-gate`，部署为 `yo-<env>-infra-gate`（如 `yo-prod-infra-gate`）。

与其他服务区别：
- 需要 `volumes/config/server.yml` 配置文件（通过 volume 同步）
- 需要暴露 80/443 端口
- 路由配置根据其他服务的 `gateway` 设置自动生成
- 不同环境使用不同端口（如 prod 用 80/443，staging 用 8080/8443）

### 3.4 网络互通

每个环境使用独立的 Docker 网络，实现环境隔离：

```yaml
networks:
  yamlops-prod:
    name: yamlops-prod
    external: true
```

同一环境内的服务加入同一个网络，服务间可通过**容器名**（即 `yo-<env>-服务名`）相互访问：

```
yo-prod-api-server ──► http://yo-prod-redis:6379  ✅ 同环境互通
yo-staging-api-server ──► http://yo-staging-redis:6379  ✅ 同环境互通
```

不同环境的服务网络隔离，无法直接访问。

### 3.5 孤儿服务检测

Plan 阶段会扫描服务器上的 `yo-<env>-*` 目录，对比 YAML 配置：

- 配置中存在，服务器不存在 → CREATE
- 两边都存在但配置不同 → UPDATE
- 服务器存在但配置不存在 → DELETE（孤儿服务）

---

## 4. 实体定义

### 4.1 秘钥引用规范

所有需要秘钥的地方（ISP凭证、服务器密码、服务环境变量等）支持两种写法：

**方式一：明文**
```yaml
password: "my-password-123"
```

**方式二：引用 secrets 文件**
```yaml
password:
  secret: db_password          # 引用 secrets.yaml 中定义的秘钥
```

### 4.2 秘钥

```yaml
secrets:
  - name: db_password
    value: "your-db-password"
    
  - name: jwt_secret
    value: "your-jwt-secret"
    
  - name: aliyun_access_key
    value: "AKIAXXXXXX"
    
  - name: aliyun_access_secret
    value: "xxxxxxx"
```

### 4.3 ISP 提供商

ISP 定义可用服务的来源和能力。

```yaml
isps:
  - name: aliyun
    services:
      - server        # 云服务器
      - domain        # 域名注册
      - dns           # DNS 解析
      - certificate   # SSL 证书签发
    credentials:
      access_key_id:
        secret: aliyun_access_key          # 引用 secrets
      access_key_secret:
        secret: aliyun_access_secret

  - name: cloudflare
    services:
      - dns
      - certificate
    credentials:
      api_token: "cf-api-token-xxxxx"      # 或直接明文

  - name: tencent
    services:
      - server
      - dns
    credentials:
      secret_id:
        secret: tencent_secret_id
      secret_key:
        secret: tencent_secret_key
```

### 4.4 网区

网区是网络隔离的边界，区内服务器互通。

```yaml
zones:
  - name: cn-east
    description: 华东区生产环境
    isp: aliyun
    region: cn-shanghai
    
  - name: cn-south
    description: 华南区
    isp: tencent
    region: ap-guangzhou
```

### 4.5 网关

网关是基于 [infra-gate](../infra-gate) 项目的服务，部署为 `yo-infra-gate`。

网关配置文件 (`server.yml`) 存放在 `userdata/{env}/volumes/infra-gate/` 目录，Apply 时自动同步。

```yaml
gateways:
  - name: gw-east-1
    zone: cn-east
    server: srv-east-01
    image: infra-gate:latest            # Docker 镜像
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate      # 引用本地配置目录
      sync: true                         # 同步到服务器
    ssl:
      mode: remote                      # local | remote
      endpoint: http://infra-ssl...
    waf:
      enabled: true
      whitelist:
        - 10.0.0.0/8
        - 192.168.0.0/16
    log_level: 1                        # 0=DEBUG, 1=INFO, ...

  - name: gw-east-2
    zone: cn-east
    server: srv-east-02
    image: infra-gate:latest
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate
      sync: true
    ssl:
      mode: remote
      endpoint: http://infra-ssl...
```

### 4.6 服务器

SSH 连接使用用户名密码方式。

**环境配置**：
- `apt_source`: APT 软件源配置
- `registries`: 要登录的 Docker 仓库列表

```yaml
servers:
  - name: srv-east-01
    zone: cn-east
    isp: aliyun
    os: ubuntu-22.04                    # 操作系统版本
    ip:
      public: 1.2.3.4
      private: 10.0.1.10
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password:
        secret: srv_east_01_password
    environment:
      apt_source: tuna                  # APT 源: tuna | aliyun | tencent | official
      registries:                       # 要登录的 Docker 仓库
        - registry-aliyun
        - registry-cnb

  - name: srv-east-02
    zone: cn-east
    isp: aliyun
    os: ubuntu-20.04
    ip:
      public: 1.2.3.5
      private: 10.0.1.11
    ssh:
      host: 1.2.3.5
      port: 22
      user: ubuntu                      # 非 root 用户
      password: "plain-password-here"
    environment:
      apt_source: aliyun
      registries:
        - registry-aliyun
```

### 4.7 服务

一个业务服务对应一个 docker-compose 服务。前后端分离就定义为两个服务。

**Volume 类型**：
- `volumes://xxx` - 引用本地配置目录，Apply 时同步到服务器
- `./xxx` - 相对路径，在服务器上创建
- `/xxx` 或 `xxx:xxx` - 普通 docker volume

```yaml
services:
  - name: api-server
    server: srv-east-01
    image: myapp/api:v1.2.0
    port: 3000
    env:
      DATABASE_URL: postgres://...
      REDIS_URL: redis://yo-prod-redis:6379
    secrets:
      - db_password                      # 引用 secrets，注入为环境变量
      - jwt_secret
    healthcheck:
      path: /health
      interval: 30s
      timeout: 10s
    resources:
      cpu: "0.5"
      memory: 256M
    volumes:
      - source: volumes://api-server/config
        target: /app/config
        sync: true                       # 同步配置到服务器
      - ./data:/app/data                 # 相对路径，服务器上自动创建
    gateway:
      enabled: true
      hostname: api.example.com
      path: /
      ssl: true

  - name: web-frontend
    server: srv-east-01
    image: myapp/web:v1.2.0
    port: 8080
    env:
      API_URL: https://api.example.com
    volumes:
      - ./html:/usr/share/nginx/html
    gateway:
      enabled: true
      hostname: www.example.com
      path: /
      ssl: true

  - name: redis
    server: srv-east-01
    image: redis:7-alpine
    port: 6379
    volumes:
      - redis-data:/data                 # 普通 docker volume
    internal: true                       # 不对外暴露，仅内部使用
```

### 4.8 Docker Registry

Docker 仓库配置，用于服务器登录私有仓库拉取镜像。

```yaml
registries:
  - name: registry-aliyun
    url: registry.cn-shanghai.aliyuncs.com
    credentials:
      username:
        secret: aliyun_registry_user
      password:
        secret: aliyun_registry_password

  - name: registry-cnb
    url: docker.cnb.cool
    credentials:
      username: "user@example.com"
      password:
        secret: cnb_registry_password

  - name: registry-dockerhub
    url: registry.hub.docker.com
    credentials:
      username: "mydockerhub"
      password:
        secret: dockerhub_password
```

### 4.9 域名

```yaml
domains:
  - name: example.com
    isp: aliyun
    auto_renew: true
    
  - name: api.example.com
    parent: example.com
    isp: cloudflare
```

### 4.10 DNS 解析

```yaml
records:
  - domain: example.com
    type: A
    name: www
    value: 1.2.3.4              # 指向网关公网 IP
    ttl: 300
    
  - domain: example.com
    type: A
    name: api
    value: 1.2.3.4
    ttl: 300
    
  - domain: example.com
    type: CNAME
    name: cdn
    value: cdn.example.com.cdn.dnsv1.com
    ttl: 600
```

### 4.11 SSL 证书

```yaml
certificates:
  - name: example-com
    domains:
      - example.com
      - "*.example.com"
    provider: letsencrypt
    dns_provider: cloudflare        # 用于 DNS-01 挑战
    auto_renew: true
    renew_before: 30d
    
  - name: api-example-com
    domains:
      - api.example.com
    provider: zerossl
    dns_provider: aliyun
    auto_renew: true
```

---

## 5. 生成的配置文件

### 5.1 本地生成 (plan 阶段)

Plan 时在本地生成部署文件：

```
deployments/
├── yo-prod-api-server/
│   ├── docker-compose.yml
│   └── volumes/                  # 需要同步的配置
│       └── api-server/
│           └── config.yml
├── yo-prod-infra-gate/
│   ├── docker-compose.yml
│   └── volumes/
│       └── infra-gate/
│           └── server.yml
└── ...
```

### 5.2 同步到服务器 (apply 阶段)

Apply 时将 `deployments/` 目录同步到服务器 `/data/yamlops/`：

```bash
# 本地 (prod 环境)
deployments/yo-prod-api-server/  →  服务器 /data/yamlops/yo-prod-api-server/
```

### 5.3 docker-compose.yml (普通服务)

基于 `services.yaml` 自动生成，部署到服务器的 `/data/yamlops/yo-<env>-<服务名>/docker-compose.yml`。

```yaml
# /data/yamlops/yo-prod-api-server/docker-compose.yml
services:
  yo-prod-api-server:
    container_name: yo-prod-api-server
    image: myapp/api:v1.2.0
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      DATABASE_URL: postgres://...
      REDIS_URL: redis://yo-prod-redis:6379
      DB_PASSWORD: your-db-password
      JWT_SECRET: your-jwt-secret
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    deploy:
      resources:
        limits:
          cpus: "0.5"
          memory: 256M
    networks:
      - yamlops-prod

networks:
  yamlops-prod:
    external: true
```

### 5.4 docker-compose.yml (网关服务)

```yaml
# /data/yamlops/yo-prod-infra-gate/docker-compose.yml
services:
  yo-prod-infra-gate:
    container_name: yo-prod-infra-gate
    image: infra-gate:latest
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./volumes/infra-gate:/app/configs:ro
      - ./cache:/app/cache
    networks:
      - yamlops-prod

networks:
  yamlops-prod:
    external: true
```

### 5.5 infra-gate 配置

基于网关和服务配置，自动生成到 `userdata/{env}/volumes/infra-gate/server.yml`：

```yaml
# /data/yamlops/yo-prod-infra-gate/configs/server.yml
server:
  port: 80
  g_zip_enabled: true
  http2_enabled: true

logger:
  level: 1
  enable_console: true
  enable_file: true
  log_dir: "./applogs"

waf:
  enabled: true
  whitelist:
    enabled: true
    ip_ranges:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
  crs:
    enabled: true
    version: "v4.19.0"

ssl:
  remote:
    enabled: true
    endpoint: "http://infra-ssl.dev.ops.litelake.com:38567/cert/get-as-json"
    auto_update: true
    update_check_window: "00:00-00:59"

hosts:
  - name: "*"
    port: 80
    text: "yamlops gateway"

  - name: api.example.com
    port: 80
    ssl_port: 443
    backend:
      - http://10.0.1.10:3000
    health_check: "/health"
    health_check_interval: 30s
    health_check_timeout: 10s

  - name: www.example.com
    port: 80
    ssl_port: 443
    backend:
      - http://10.0.1.10:8080
    health_check: "/"
    health_check_interval: 30s
    health_check_timeout: 10s
```

---

## 6. CLI 命令设计

### 6.1 命令模式

```bash
# 交互式 TUI（主要方式）
yamlops

# 命令行参数（脚本/自动化）
yamlops [command] [flags]

# 子命令
yamlops plan [scope]        # 预览变更
yamlops apply [scope]       # 执行变更
yamlops validate            # 校验配置
yamlops env check [scope]   # 验证服务器环境
yamlops env sync [scope]    # 同步服务器环境配置
yamlops list <entity>       # 列出实体
yamlops show <entity> <name> # 查看详情
yamlops clean [scope]       # 清理孤儿服务
```

### 6.2 服务器环境命令

```bash
# 验证所有服务器环境
yamlops env check

# 验证指定服务器
yamlops env check --server srv-east-01

# 验证指定网区的服务器
yamlops env check --zone cn-east

# 同步环境配置（APT源、Registry登录等）
yamlops env sync

# 同步指定服务器
yamlops env sync --server srv-east-01
```

**环境验证项目**：

| 检查项 | 说明 |
|--------|------|
| SSH 连接 | 验证能否通过 SSH 连接 |
| Sudo 免密 | 验证用户是否可以免密码执行 sudo |
| Docker 安装 | 验证 Docker 是否已安装 |
| Docker Compose | 验证 docker compose 命令可用 |
| APT 源 | 检查当前使用的 APT 源 |
| Registry 登录 | 检查是否已登录配置的 Registry |

**环境同步项目**：

| 同步项 | 说明 |
|--------|------|
| APT 源 | 根据配置切换 APT 源（清华/阿里/腾讯/官方） |
| Registry 登录 | 登录配置的 Docker 仓库 |
| Docker 网络 | 创建 yamlops 网络（如果不存在） |

### 6.4 作用域

```bash
# 全局操作
yamlops plan
yamlops apply

# 按领域
yamlops plan --domain infra      # 仅基础设施（网区/网关/服务器/服务）
yamlops plan --domain global     # 仅全局资源（域名/DNS/SSL）

# 按网区
yamlops plan --zone cn-east
yamlops apply --zone cn-east

# 按服务器
yamlops plan --server srv-east-01

# 按服务
yamlops plan --service api-server

# 组合
yamlops apply --zone cn-east --domain infra
```

### 6.5 交互式 TUI 界面

```
┌─────────────────────────────────────────────────────────────┐
│  YAMLOps v1.0.0                              [prod]         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  > Plan & Apply                                             │
│    Infrastructure              网区/网关/服务器/服务        │
│    Global Resources            域名/DNS/SSL                 │
│    Environment                 服务器环境验证/同步          │
│    Manage Entities             管理实体配置                 │
│    View Status                 查看部署状态                 │
│    Settings                    设置                         │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ↑/↓ Select  Enter Confirm  q Quit                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. 核心流程

### 7.1 Plan 流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Load YAML   │ ──▶ │  Validate    │ ──▶ │  Calculate   │
│  Configs     │     │  Schema      │     │  Changes     │
└──────────────┘     └──────────────┘     └──────────────┘
                                                │
                    ┌───────────────────────────┼───────────────────────────┐
                    │                           │                           │
                    ▼                           ▼                           ▼
            ┌──────────────┐           ┌──────────────┐           ┌──────────────┐
            │  SSH Fetch   │           │  API Query   │           │  Compare     │
            │  Server State│           │  DNS/SSL/etc │           │  & Diff      │
            └──────────────┘           └──────────────┘           └──────────────┘
```

### 7.2 Apply 流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Confirm     │ ──▶ │  Execute     │ ──▶ │  Report      │
│  Plan        │     │  Changes     │     │  Results     │
└──────────────┘     └──────────────┘     └──────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
 ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
 │  SSH Exec    │   │  DNS API     │   │  SSL ACME    │
 │  docker-compose│   │  Update      │   │  Challenge   │
 └──────────────┘   └──────────────┘   └──────────────┘
```

### 7.3 变更类型

| 类型 | 说明 |
|------|------|
| CREATE | 资源不存在，需要创建 |
| UPDATE | 资源存在但配置有变 |
| DELETE | 资源多余，需要删除（孤儿服务） |
| NOOP | 无变更 |

---

## 8. 服务商支持

### 8.1 DNS 解析

| 服务商 | API | DNS-01 挑战 |
|--------|-----|-------------|
| Cloudflare | ✅ | ✅ |
| 阿里云 DNS | ✅ | ✅ |
| 腾讯云 DNSPod | ✅ | ✅ |

### 8.2 SSL 证书

| CA | 方式 | 说明 |
|----|------|------|
| Let's Encrypt | ACME v2 | 免费，90天有效期 |
| ZeroSSL | ACME v2 | 免费，90天有效期 |

### 8.3 APT 软件源

| 源 | 代码 | 说明 |
|----|------|------|
| 清华大学 | `tuna` | 国内速度快，推荐 |
| 阿里云 | `aliyun` | 阿里云服务器推荐 |
| 腾讯云 | `tencent` | 腾讯云服务器推荐 |
| 官方 | `official` | Ubuntu 官方源，海外服务器推荐 |

---

## 9. 配置校验

Apply 前自动执行校验：

1. **Schema 校验**：YAML 格式和字段类型
2. **引用检查**：确保引用的实体（服务器、域名、证书、secrets、registry 等）存在
3. **冲突检查**：端口/域名/路径等无冲突
4. **可达性检查**：SSH 连接、API 凭证有效性
5. **环境检查**：Sudo 免密、Docker 可用、Registry 登录状态
6. **孤儿检测**：扫描服务器上多余的 `yo-<env>-*` 服务

---

## 10. 多环境管理

```
userdata/
├── prod/
│   └── *.yaml           # 生产环境
├── staging/
│   └── *.yaml           # 预发环境
└── dev/
    └── *.yaml           # 开发环境
```

切换环境：
```bash
yamlops --env prod
yamlops --env staging
```

---

## 11. 后续扩展

- [ ] Webhook 通知
- [ ] 配置版本控制集成
- [ ] 资源监控与告警
- [ ] Agent 模式远程执行
- [ ] 更多 DNS 服务商
- [ ] 对象存储管理

---

## 12. 开发计划

### Phase 1: 基础框架
- CLI 框架搭建 (BubbleTea)
- 配置加载与校验
- 秘钥引用解析
- Plan/Apply 基础流程

### Phase 2: 基础设施领域
- 网区/服务器管理
- SSH 远程执行 (用户名密码)
- 服务器环境验证 (SSH连接、Sudo免密、Docker)
- 服务器环境同步 (APT源、Registry登录)
- Docker Compose 生成与部署
- 孤儿服务检测与清理

### Phase 3: 网关集成
- 网关服务 (yo-infra-gate) 部署
- infra-gate 配置生成
- 服务路由配置

### Phase 4: 全局资源领域
- DNS 解析管理
- SSL 证书签发 (ACME DNS-01)

### Phase 5: 完善
- 错误处理优化
- 日志与调试
- 文档完善
