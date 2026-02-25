# YAMLOps

基于 YAML 的基础设施即代码（IaC）管理工具，支持多环境、Plan/Apply 工作流和交互式 TUI。

## 特性

- **多环境支持**：prod / staging / dev / demo 环境隔离
- **Plan/Apply 工作流**：类似 Terraform 的预览+执行模式
- **声明式配置**：通过 YAML 描述期望状态
- **密钥管理**：支持明文和密钥引用两种方式
- **DNS 管理**：支持 Cloudflare、阿里云、腾讯云
- **Docker Compose**：自动生成和部署
- **交互式 TUI**：基于 BubbleTea 的终端界面

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

## 快速开始

```bash
# 构建
go build -o yamlops ./cmd/yamlops

# 验证配置
./yamlops validate -e prod

# 查看变更计划
./yamlops plan -e prod

# 应用变更
./yamlops apply -e prod

# 启动交互式 TUI
./yamlops -e prod
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
└── userdata/
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
| `--env` | `-e` | dev | 环境 (prod/staging/dev/demo) |
| `--config` | `-c` | . | 配置目录 |
| `--version` | `-v` | - | 显示版本 |
| `--help` | `-h` | - | 显示帮助 |

### 命令总览

```
yamlops
├── plan [scope]             # 生成执行计划
├── apply [scope]            # 应用变更
├── validate                 # 验证配置
├── list <entity>            # 列出实体
├── show <entity> <name>     # 显示详情
├── clean                    # 清理孤立资源
├── env
│   ├── check                # 检查环境状态
│   └── sync                 # 同步环境配置
├── server
│   ├── setup                # 完整设置（check + sync）
│   ├── check                # 检查服务器状态
│   └── sync                 # 同步服务器配置
├── dns
│   ├── plan                 # DNS 变更计划
│   ├── apply                # 应用 DNS 变更
│   ├── list [resource]      # 列出域名/记录
│   ├── show <resource> <name>
│   └── pull
│       ├── domains          # 从 ISP 拉取域名
│       └── records          # 从域名拉取记录
├── config
│   ├── list [secrets|isps|registries]  # 列出配置项
│   └── show <type> <name>   # 显示配置详情
└── app
    ├── plan                 # 应用部署计划
    ├── apply                # 应用部署
    ├── list [resource]      # 列出资源
    └── show <resource> <name>
```

### plan - 生成变更计划

```bash
# 查看所有变更
./yamlops plan -e prod

# 按作用域过滤
./yamlops plan -e prod --zone cn-east
./yamlops plan -e prod --server srv-east-01
./yamlops plan -e prod --service api-server
./yamlops plan -e prod --domain example.com
```

### apply - 应用变更

```bash
# 应用所有变更（需确认）
./yamlops apply -e prod

# 按作用域应用
./yamlops apply -e prod --zone cn-east
./yamlops apply -e prod --server srv-east-01
```

### validate - 验证配置

```bash
./yamlops validate -e prod
./yamlops validate -e staging -c /path/to/config
```

### list / show - 实体管理

```bash
# 列出实体
./yamlops list servers -e prod
./yamlops list services -e prod
./yamlops list domains -e prod
# 查看详情
./yamlops show server srv-east-01 -e prod
./yamlops show service api-server -e prod
./yamlops show domain example.com -e prod
```

### server - 服务器管理

```bash
# 完整设置（检查 + 同步）
./yamlops server setup -e prod --server srv-east-01

# 仅检查
./yamlops server check -e prod --zone cn-east

# 仅同步
./yamlops server sync -e prod --server srv-east-01

# 组合使用
./yamlops server setup -e prod --check-only --zone cn-east
```

### dns - DNS 管理

```bash
# 查看 DNS 变更计划
./yamlops dns plan -e prod -d example.com

# 应用 DNS 变更
./yamlops dns apply -e prod -d example.com --auto-approve

# 列出 DNS 资源
./yamlops dns list domains -e prod
./yamlops dns list records -e prod

# 查看资源详情
./yamlops dns show domain example.com -e prod

# 从 ISP 拉取域名
./yamlops dns pull domains -e prod -i aliyun

# 从域名拉取 DNS 记录
./yamlops dns pull records -e prod -d example.com
```

### app - 应用部署

```bash
# 生成部署计划
./yamlops app plan -e prod -s srv-east-01

# 应用部署
./yamlops app apply -e prod --auto-approve

# 列出资源
./yamlops app list -e prod
./yamlops app list biz -e prod -s srv-east-01
```

### config - 配置管理

```bash
# 列出配置项
./yamlops config list -e prod
./yamlops config list secrets -e prod

# 查看配置详情
./yamlops config show secret db_password -e prod
./yamlops config show isp aliyun -e prod
```

### clean - 清理资源

```bash
# 清理孤立服务和目录
./yamlops clean -e prod
```

### TUI 交互模式

```bash
# 无参数启动进入 TUI
./yamlops -e prod
```

**TUI 快捷键**：

| 按键 | 功能 |
|------|------|
| ↑/k, ↓/j | 上下移动 |
| Space | 切换选择 |
| Enter | 确认/展开 |
| Tab | 切换视图 |
| a/n | 选择/取消当前项 |
| A/N | 全选/全不选 |
| p | 生成计划 |
| r | 刷新配置 |
| x | 取消操作 |
| Esc | 返回 |
| q/Ctrl+C | 退出 |

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
    services: [server, domain, dns, certificate]
    credentials:
      access_key_id: {secret: aliyun_access_key}
      access_key_secret: {secret: aliyun_access_secret}

  - name: cloudflare
    services: [dns, certificate]
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
```

### services_biz.yaml - 业务服务配置

```yaml
services:
  - name: api-server
    server: srv-east-01
    image: myapp/api:v1.0.0
    ports:
      - container: 3000
        host: 13000
        protocol: tcp
    env:
      NODE_ENV: production
      DATABASE_URL: postgres://db:5432/myapp
      REDIS_PASSWORD: {secret: redis_password}
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
    type: gateway                  # gateway | ssl
    server: srv-east-01
    image: infra-gate:latest
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate
      sync: true
    ssl:
      mode: remote                  # local | remote
      endpoint: http://infra-ssl:38567
    waf:
      enabled: true
      whitelist:
        - 10.0.0.0/8
        - 192.168.0.0/16
    log_level: 1

  - name: infra-ssl
    type: ssl
    server: srv-east-01
    image: infra-ssl:latest
    ports:
      api: 38567
    config:
      auth:
        enabled: true
        apikey: {secret: ssl_api_key}
      storage:
        type: local
        path: /data/certs
      defaults:
        issue_provider: letsencrypt_prod
        storage_provider: local_default
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
./yamlops validate -e prod

# 3. 查看变更计划
./yamlops plan -e prod --service my-service

# 4. 应用变更
./yamlops apply -e prod --service my-service
```

### 2. 更新服务镜像

```bash
# 1. 修改 services_biz.yaml 中的 image 字段
# 2. 验证并应用
./yamlops validate -e prod && ./yamlops apply -e prod --service api-server
```

### 3. 新增服务器

```bash
# 1. 添加服务器配置到 servers.yaml
# 2. 添加 SSH 密码到 secrets.yaml
# 3. 验证环境
./yamlops server check -e prod --server new-server

# 4. 同步环境
./yamlops server sync -e prod --server new-server

# 或一步完成
./yamlops server setup -e prod --server new-server
```

### 4. DNS 管理

```bash
# 从远程拉取到本地
./yamlops dns pull domains -e prod -i aliyun
./yamlops dns pull records -e prod -d example.com

# 从本地推送到远程
./yamlops dns plan -e prod
./yamlops dns apply -e prod --auto-approve
```

### 5. 日常部署

```bash
# Plan + Apply 工作流
./yamlops plan -e prod --zone cn-east
# 确认变更后
./yamlops apply -e prod --zone cn-east
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

| 服务商 | API | DNS-01 挑战 |
|--------|-----|-------------|
| Cloudflare | ✅ | ✅ |
| 阿里云 DNS | ✅ | ✅ |
| 腾讯云 DNSPod | ✅ | ✅ |

## 故障排查

### 配置验证失败

```bash
./yamlops validate -e prod
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

## 环境变量

| 变量 | 说明 |
|------|------|
| `YAMLOPS_ENV` | 默认环境 |
| `YAMLOPS_CONFIG` | 默认配置目录 |

## 更多文档

- [系统设计说明](docs/system-design.md)
