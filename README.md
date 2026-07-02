# YAMLOps

基于 YAML 的基础设施即代码（IaC）管理工具，支持多环境、Plan/Apply 工作流和交互式 TUI。

## 特性

- **多环境支持**：prod / staging / dev / demo 环境隔离
- **统一执行模式**：Plan → Confirm → Execute 三阶段工作流
- **声明式配置**：通过 YAML 描述期望状态
- **密钥管理**：支持明文和密钥引用两种方式
- **DNS 管理**：支持 Cloudflare、阿里云、腾讯云
- **Docker Compose**：自动生成和部署
- **交互式 TUI**：基于 BubbleTea 的终端界面，四区布局，统一执行流程
- **服务器环境管理**：APT 源配置、Docker 网络、Registry 登录
- **网关管理**：infra-gate 配置自动生成
- **清理功能**：自动识别并清理孤立资源

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [架构](#架构)
- [配置目录结构](#配置目录结构)
- [CLI 命令](#cli-命令)
- [实体配置](#实体配置)
- [工作流程](#工作流程)
- [服务器部署规范](#服务器部署规范)
- [故障排查](#故障排查)
- [迁移指南](docs/MIGRATION.md)

## 快速开始

```bash
# 构建
go build -o yamlops ./cmd/yamlops

# 验证配置
./yamlops cli service validate -e prod
./yamlops cli server validate -e prod
./yamlops cli dns validate -e prod
./yamlops cli config validate -e prod

# 查看变更计划（dry-run）
./yamlops cli service deploy -e prod --dry-run

# 应用变更
./yamlops cli service deploy -e prod --yes

# 启动交互式 TUI
./yamlops tui -e prod
```

## 安装

### 从源码构建

```bash
git clone <repository-url>
cd infra-yamlops
go mod tidy
go build -o yamlops ./cmd/yamlops
```

### 依赖

- Go 1.24+
- SSH 访问权限（用于服务器管理）

## 架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Interface Layer (CLI/TUI)                     │
├─────────────────────────────────────────────────────────────────────┤
│                      Application Layer (Handler + Executor)          │
├─────────────────────────────────────────────────────────────────────┤
│                       Plan Layer (Planner + Generator)               │
├─────────────────────────────────────────────────────────────────────┤
│                        Domain Layer (Entity + Service)               │
├─────────────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer (Provider + SSH)             │
└─────────────────────────────────────────────────────────────────────┘
```

详细设计文档：[docs/system-design.md](docs/system-design.md)

## 配置目录结构

```
.
├── yamlops                  # 可执行文件
└── userdata/                # 用户配置目录（当前仓库无示例数据，请自行创建）
    ├── prod/                # 生产环境
    │   ├── secrets.yaml     # 密钥
    │   ├── isps.yaml        # 服务商
    │   ├── zones.yaml       # 网区
    │   ├── servers.yaml     # 服务器
    │   ├── services_biz.yaml    # 业务服务
    │   ├── services_infra.yaml  # 基础设施服务 (gateway/ssl)
    │   ├── registries.yaml  # Docker Registry
    │   ├── dns.yaml         # 域名和 DNS 记录
    │   └── volumes/         # 配置文件
    │       ├── infra-gate/
    │       └── api-server/
    ├── staging/             # 预发环境
    ├── dev/                 # 开发环境
    └── demo/                # 演示环境
```

## CLI 命令

### 全局参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--env` | `-e` | 必填 | 环境名称 (prod/staging/dev/demo) |
| `--config` | `-c` | `.` | 配置目录路径 |
| `--concurrency` | - | `5` | 服务器操作并发数 |

> **注意**：`-e` 为必填参数（`--help` 除外）。

### 命令总览

```
yamlops
├── tui                        # 启动交互界面
│   └── -e <env>               # 指定环境（必填）
├── cli                        # CLI 命令
│   ├── config                 # 配置管理
│   │   ├── show <entity>      # 显示配置详情（isps/registries/secrets）
│   │   └── validate           # 验证配置完整性
│   ├── dns                    # DNS 管理
│   │   ├── show               # 列出域名和记录（--detail 显示详情）
│   │   ├── validate           # 验证 DNS 配置
│   │   ├── deploy             # 部署 DNS 变更（统一执行模式）
│   │   └── pull               # 从 ISP 拉取数据
│   │       ├── domains        # 拉取域名列表
│   │       └── records        # 拉取 DNS 记录
│   ├── server                 # 服务器管理
│   │   ├── show               # 列出服务器（--detail 显示详情）
│   │   ├── validate           # 验证服务器配置
│   │   └── setup              # 设置服务器环境（统一执行模式）
│   └── service                # 服务管理
│       ├── show               # 列出服务（--detail 显示详情）
│       ├── validate           # 验证服务配置
│       ├── deploy             # 部署服务（统一执行模式）
│       ├── stop               # 停止服务（统一执行模式）
│       ├── restart            # 重启服务（统一执行模式）
│       └── cleanup            # 清理孤儿资源（统一执行模式）
└── api                        # API 服务（计划中）
```

### 统一执行模式

所有变更命令（`service deploy/stop/restart/cleanup`、`server setup`、`dns deploy/pull`）遵循统一的三阶段执行模式：

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│   Plan   │ ──→ │ Confirm  │ ──→ │ Execute  │
│ (计划)    │     │ (确认)    │     │ (执行)    │
└──────────┘     └──────────┘     └──────────┘
  ↑ 干燥运行                     ↑ 跳过确认(--yes)
  (--dry-run)
```

| 参数 | 说明 |
|------|------|
| `--dry-run` | 只执行 Plan 阶段，显示变更计划但不执行 |
| `--yes` | 跳过 Confirm 阶段，直接执行所有变更 |
| `--force` | 即使配置无变更也生成部署计划（仅 service deploy/dns deploy/dns pull） |

### config - 配置管理

```bash
# 显示 ISP 列表
./yamlops cli config show isps -e prod
./yamlops cli config show isps -e prod --detail

# 显示 Registry 列表
./yamlops cli config show registries -e prod
./yamlops cli config show registries -e prod --detail

# 显示 Secret 列表（只显示 key，不显示 value）
./yamlops cli config show secrets -e prod
./yamlops cli config show secrets -e prod --detail

# 验证配置完整性
./yamlops cli config validate -e prod
```

**安全约束**：
- `config show secrets` 只显示 key 列表，不显示 value
- `config show isps` 不显示 API Key/Secret 等凭证

### dns - DNS 管理

```bash
# 显示域名列表
./yamlops cli dns show -e prod
./yamlops cli dns show -e prod --detail
./yamlops cli dns show -e prod --domain example.com

# 验证 DNS 配置
./yamlops cli dns validate -e prod

# 部署 DNS 变更
./yamlops cli dns deploy -e prod --dry-run
./yamlops cli dns deploy -e prod --domain example.com
./yamlops cli dns deploy -e prod --domain example.com --force

# 从 ISP 拉取域名
./yamlops cli dns pull domains -e prod --isp aliyun
./yamlops cli dns pull domains -e prod --isp aliyun --dry-run

# 从域名拉取 DNS 记录
./yamlops cli dns pull records -e prod --domain example.com
./yamlops cli dns pull records -e prod --domain example.com --dry-run
```

**dns 标志**：

| 标志 | 说明 |
|------|------|
| `--domain` | 域名筛选（逗号分隔多选） |
| `--record` | 记录筛选，格式 `TYPE:NAME`（逗号分隔，如 `A:@,CNAME:api`） |
| `--isp` | ISP 名称筛选（`dns pull` 专用） |
| `--detail` | 显示详细信息 |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--force` | 强制执行 |

### server - 服务器管理

```bash
# 显示服务器列表
./yamlops cli server show -e prod
./yamlops cli server show -e prod --detail
./yamlops cli server show -e prod --zone cn-east

# 验证服务器配置
./yamlops cli server validate -e prod

# 设置服务器环境
./yamlops cli server setup -e prod --dry-run
./yamlops cli server setup -e prod
./yamlops cli server setup -e prod --yes
./yamlops cli server setup -e prod --server srv-cn1 --dry-run
```

**server 标志**：

| 标志 | 说明 |
|------|------|
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--detail` | 显示详细信息 |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |

### service - 服务管理

```bash
# 显示服务列表
./yamlops cli service show -e prod
./yamlops cli service show -e prod --type biz
./yamlops cli service show -e prod --type infra
./yamlops cli service show -e prod --service my-api --detail

# 验证服务配置
./yamlops cli service validate -e prod
./yamlops cli service validate -e prod --type biz
./yamlops cli service validate -e prod --service my-api

# 部署服务
./yamlops cli service deploy -e prod --dry-run
./yamlops cli service deploy -e prod --type biz
./yamlops cli service deploy -e prod --type biz --force
./yamlops cli service deploy -e prod --type biz --yes

# 按服务名筛选
./yamlops cli service deploy -e prod --service my-api --yes
./yamlops cli service deploy -e prod --service my-api,my-worker --yes

# 停止服务
./yamlops cli service stop -e prod --type biz --dry-run
./yamlops cli service stop -e prod --type biz --yes
./yamlops cli service stop -e prod --service my-api --yes

# 重启服务
./yamlops cli service restart -e prod --type biz --yes
./yamlops cli service restart -e prod --service my-api --yes

# 清理孤儿资源
./yamlops cli service cleanup -e prod --dry-run
./yamlops cli service cleanup -e prod --yes
```

**service 标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--service` | 服务名筛选（逗号分隔多选） |
| `--detail` | 显示详细信息 |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--force` | 强制部署（仅 deploy） |
| `--concurrency` | 并发数（默认 5） |

**`--type` 参数说明**：

| 值 | 含义 |
|----|------|
| `biz` | 仅业务服务（BizService） |
| `infra` | 仅基础设施服务（InfraService） |
| `biz,infra` | 全部服务（与不传 `--type` 等效） |

### TUI 交互模式

```bash
# 启动 TUI
./yamlops tui -e prod
./yamlops tui -e prod --concurrency 10
```

**TUI 主菜单**：

```
MainMenu
├── Service Management
│   ├── Show services
│   ├── Validate services
│   ├── Deploy services
│   ├── Stop services
│   ├── Restart services
│   └── Cleanup orphan resources
├── Server Management
│   ├── Show servers
│   ├── Validate servers
│   └── Setup server environment
├── DNS Management
│   ├── Show DNS records
│   ├── Validate DNS configuration
│   ├── Deploy DNS records
│   ├── Pull domains from ISP
│   └── Pull records from ISP
└── Configuration
    ├── Show ISPs
    ├── Show Registries
    ├── Show Secrets
    └── Validate Config
```

**TUI 快捷键**：

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 上移光标 |
| `↓` / `j` | 下移光标 |
| `Enter` | 确认/进入下一阶段 |
| `Esc` | 返回上一级/取消 |
| `?` | 显示帮助 |
| `q` | 退出程序（仅主菜单） |
| `Space` | 切换选中/取消 |
| `a` / `n` | 全选/全不选 |
| `d` | 切换摘要/详细视图 |
| `f` | 切换 force 模式 |
| `/` | 搜索过滤（Tree View） |
| `Ctrl+C` | 中断执行（已执行不回滚） |
| `r` | 重新 Plan（执行完成后） |

## 实体配置

### secrets.yaml - 密钥定义

```yaml
secrets:
  - name: db_password
    value: "your-db-password"
  - name: api_key
    value: "your-api-key"
```

### isps.yaml - 服务商配置

```yaml
isps:
  - name: aliyun
    type: aliyun                    # aliyun | cloudflare | tencent
    services: [server, domain, dns]
    credentials:
      access_key_id: {secret: aliyun_access_key}
      access_key_secret: {secret: aliyun_access_secret}

  - name: cloudflare
    services: [dns]
    credentials:
      api_token: "cf-api-token"
```

### zones.yaml - 网区定义

```yaml
zones:
  - name: cn-east
    description: 华东区生产环境
    isp: aliyun
    region: cn-shanghai
```

### servers.yaml - 服务器配置

```yaml
servers:
  - name: srv-east-01
    zone: cn-east
    isp: aliyun
    os: ubuntu-22.04
    ip:
      public: 203.0.113.10
      private: 10.0.1.10
    ssh:
      host: 203.0.113.10
      port: 22
      user: root
      password: {secret: srv_east_01_password}
    environment:
      apt_source: tuna              # tuna | aliyun | tencent | official
      registries: [registry-aliyun]
    networks:
      - name: yamlops-prod
        type: bridge
        driver: bridge
```

### services_biz.yaml - 业务服务配置

指定 `image` 部署本地容器，或指定 `external_backends` 由 infra-gate 直接反向代理到外部 URL（两者互斥）。

```yaml
services:
  - name: api-server
    server: srv-east-01
    image: myapp/api:v1.0.0
    registry: registry-aliyun          # 可选，指定 Registry
    ports:
      - container: 3000
        host: 13000
        protocol: tcp
    env:
      NODE_ENV: production
      DATABASE_URL: postgres://db:5432/myapp
      REDIS_PASSWORD: {secret: redis_password}
    secrets:
      - redis_password
    networks:
      - yamlops-prod

> **自动注入的 DEPLOY_ 环境变量**：部署时 yamlops 会自动向 `.env` 文件注入 `DEPLOY_ENV_NAME`（部署环境）、`DEPLOY_ZONE_NAME`（部署网区）、`DEPLOY_SERVER_NAME`（部署主机）、`DEPLOY_SERVICE_NAME`（部署服务），无需在 `env` 字段中配置。容器内应用可直接读取，用于日志标记、链路追踪等场景。
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
        sync: true
      - ./data:/app/data
      - redis-data:/data
    gateways:
      - hostname: api.example.com
        container_port: 3000
        path: /
        http: true
        https: true
        waf:
          enabled: false          # 可选，关闭该 host 的 WAF（不设置则继承全局）
    internal: false
```

**Volume 格式**:
- `volumes://xxx` - 引用本地 volumes 目录，sync 时上传到服务器
- `./xxx` - 相对路径，在服务器上创建
- `name:/path` - Docker named volume

### services_infra.yaml - 基础设施服务

```yaml
infra_services:
  - name: gateway-east-1
    type: gateway
    server: srv-east-01
    image: infra-gate:latest
    ports:
      http: 80
      https: 443
    ssl:
      mode: remote
      endpoint: https://ssl.litelake.com/cert/json
      api_key: {secret: ssl_api_key}
    waf:
      enabled: true              # 全局总开关，关闭后所有 host 的 WAF 均关闭
      whitelist:
        - 10.0.0.0/8
        - 192.168.0.0/16
    log_level: 1
    notification:
      enabled: true
      url: https://hooks.example.com/notify
      timeout: 5s
    networks:
      - yamlops-prod
```

### registries.yaml - Docker Registry

```yaml
registries:
  - name: registry-aliyun
    url: registry.cn-shanghai.aliyuncs.com
    credentials:
      username: {secret: aliyun_registry_user}
      password: {secret: aliyun_registry_password}
```

### dns.yaml - 域名和 DNS 记录

```yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: cloudflare
    records:
      - type: A
        name: "@"
        value: 203.0.113.10
        ttl: 300
      - type: A
        name: www
        value: 203.0.113.10
        ttl: 300
      - type: CNAME
        name: cdn
        value: cdn.example.com.cdn.dnsv1.com
        ttl: 600

  - name: "*.example.com"
    parent: example.com
    dns_isp: cloudflare
```

**记录类型**: `A` | `AAAA` | `CNAME` | `MX` | `TXT` | `NS` | `SRV`

## 工作流程

### 1. 新增服务

```bash
# 1. 编辑配置
vim userdata/prod/services_biz.yaml

# 2. 验证配置
./yamlops cli service validate -e prod

# 3. 查看变更计划
./yamlops cli service deploy -e prod --dry-run

# 4. 应用变更
./yamlops cli service deploy -e prod --yes
```

### 2. 更新服务镜像

```bash
# 1. 修改 services_biz.yaml 中的 image 字段
# 2. 验证并应用
./yamlops cli service validate -e prod && ./yamlops cli service deploy -e prod --service my-api --yes
```

### 3. 新增服务器

```bash
# 1. 添加服务器配置到 servers.yaml
# 2. 添加 SSH 密码到 secrets.yaml
# 3. 验证服务器配置
./yamlops cli server validate -e prod

# 4. 预览环境差异
./yamlops cli server setup -e prod --server new-server --dry-run

# 5. 同步环境
./yamlops cli server setup -e prod --server new-server --yes
```

### 4. DNS 管理

```bash
# 从远程拉取到本地
./yamlops cli dns pull domains -e prod --isp aliyun
./yamlops cli dns pull records -e prod --domain example.com

# 预览 DNS 变更
./yamlops cli dns deploy -e prod --domain example.com --dry-run

# 应用 DNS 变更
./yamlops cli dns deploy -e prod --domain example.com --yes
```

### 5. 日常部署

```bash
# 按服务名预览变更
./yamlops cli service deploy -e prod --service my-api --dry-run

# 按服务名确认后执行
./yamlops cli service deploy -e prod --service my-api --yes

# 按网区预览变更
./yamlops cli service deploy -e prod --zone cn-east --dry-run

# 按网区确认后执行
./yamlops cli service deploy -e prod --zone cn-east --yes
```

## 服务器部署规范

### 目录结构

```
/data/yamlops/
├── yo-prod-infra-gate/      # 生产环境网关服务
│   ├── docker-compose.yml
│   └── volumes/
├── yo-prod-api-server/      # 生产环境 API 服务
│   ├── docker-compose.yml
│   └── volumes/
├── yo-staging-infra-gate/   # 预发环境网关服务
│   └── ...
└── yo-dev-api-server/       # 开发环境 API 服务
    └── ...
```

### 命名规范

- 容器名: `yo-<env>-<服务名>` (例如: `yo-prod-api-server`)
- 网络名: `yamlops-<env>` (例如: `yamlops-prod`)
- 部署目录: `/data/yamlops/yo-<env>-<服务名>`

### Docker 网络

每个环境使用独立的 Docker 网络：

```yaml
networks:
  yamlops-prod:
    external: true
```

## 变更类型

| 类型 | 符号 | 说明 |
|------|------|------|
| CREATE | + | 资源不存在，需要创建 |
| UPDATE | ~ | 资源存在但配置有变 |
| DELETE | - | 资源多余，需要删除 |
| NOOP | (空) | 无变更 |

## 服务商支持

### DNS 解析

| 服务商 | API |
|--------|-----|
| Cloudflare | ✅ |
| 阿里云 DNS | ✅ |
| 腾讯云 DNSPod | ✅ |

## 故障排查

```bash
# 验证各模块配置
./yamlops cli service validate -e prod
./yamlops cli server validate -e prod
./yamlops cli dns validate -e prod
./yamlops cli config validate -e prod
```

常见错误：
- `missing reference`: 引用的实体不存在
- `port conflict`: 端口冲突
- `hostname conflict`: 主机名冲突

### SSH 连接失败

1. 检查服务器 IP 和端口
2. 确认 secrets.yaml 中密码正确
3. 验证网络可达性

### 服务启动失败

1. 检查 Docker 镜像是否存在
2. 确认 Registry 登录状态
3. 查看容器日志: `docker logs yo-<服务名>`

## 更多文档

- [系统设计说明](docs/system-design.md)
- [CLI 命令参考](docs/cli-reference.md)
- [迁移指南](docs/MIGRATION.md)
- [变更日志](docs/CHANGELOG.md)
