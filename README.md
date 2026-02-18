# YAMLOps 使用指南

基于 YAML 的基础设施运维工具，支持交互式 CLI。

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [配置目录结构](#配置目录结构)
- [CLI 命令](#cli-命令)
- [实体配置](#实体配置)
- [秘钥管理](#秘钥管理)
- [工作流程](#工作流程)
- [服务器部署规范](#服务器部署规范)

## 快速开始

```bash
# 构建项目
go build -o yamlops ./cmd/yamlops

# 验证配置
./yamlops validate -e prod

# 查看变更计划
./yamlops plan -e prod

# 应用变更
./yamlops apply -e prod

# 启动交互式 TUI
./yamlops
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

## 配置目录结构

```
.
├── yamlops                  # 可执行文件
└── userdata/
    ├── prod/                # 生产环境
    │   ├── secrets.yaml     # 秘钥
    │   ├── isps.yaml        # 服务商
    │   ├── zones.yaml       # 网区
    │   ├── gateways.yaml    # 网关
    │   ├── servers.yaml     # 服务器
    │   ├── services.yaml    # 服务
    │   ├── registries.yaml  # Docker Registry
    │   ├── domains.yaml     # 域名
    │   ├── dns.yaml         # DNS 记录
    │   ├── certificates.yaml # SSL 证书
    │   └── volumes/         # 配置文件
    │       ├── infra-gate/
    │       └── api-server/
    ├── staging/             # 预发环境
    └── dev/                 # 开发环境
```

## CLI 命令

### 全局参数

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--env` | `-e` | dev | 环境 (prod/staging/dev) |
| `--config` | `-c` | . | 配置目录 |
| `--version` | `-v` | - | 显示版本 |
| `--help` | `-h` | - | 显示帮助 |

### 命令列表

#### validate - 验证配置

```bash
./yamlops validate -e prod
./yamlops validate -e staging -c /path/to/config
```

#### plan - 生成变更计划

```bash
# 查看所有变更
./yamlops plan -e prod

# 按作用域过滤
./yamlops plan -e prod --zone cn-east
./yamlops plan -e prod --server srv-east-01
./yamlops plan -e prod --service api-server
./yamlops plan -e prod --domain infra
```

#### apply - 应用变更

```bash
# 应用所有变更
./yamlops apply -e prod

# 按作用域应用
./yamlops apply -e prod --zone cn-east
./yamlops apply -e prod --server srv-east-01
```

#### list - 列出实体

```bash
./yamlops list servers -e prod
./yamlops list services -e prod
./yamlops list zones -e prod
./yamlops list domains -e prod
```

#### show - 查看实体详情

```bash
./yamlops show server srv-east-01 -e prod
./yamlops show service api-server -e prod
```

#### env - 环境管理

```bash
# 检查服务器环境
./yamlops env check -e prod
./yamlops env check -e prod --zone cn-east

# 同步环境配置
./yamlops env sync -e prod --server srv-east-01
```

#### dns - DNS 管理

```bash
# 查看 DNS 变更计划
./yamlops dns plan -e prod --domain example.com

# 应用 DNS 变更
./yamlops dns apply -e prod --domain example.com

# 列出 DNS 资源
./yamlops dns list domains -e prod
./yamlops dns list records -e prod

# 查看资源详情
./yamlops dns show domain example.com -e prod
./yamlops dns show record www.example.com -e prod
```

#### dns pull - 从服务商拉取 DNS 配置

**拉取域名列表**：从指定 ISP 拉取域名并同步到本地配置

```bash
# 查看可用 ISP
./yamlops dns pull domains -e prod

# 从 ISP 拉取域名（交互模式，勾选需要同步的域名）
./yamlops dns pull domains -e prod --isp aliyun

# 从 ISP 拉取域名（直接模式，自动同步所有差异）
./yamlops dns pull domains -e prod --isp aliyun --auto-approve
```

**拉取 DNS 记录**：从指定域名拉取 DNS 记录并同步到本地配置

```bash
# 查看可用域名
./yamlops dns pull records -e prod

# 从域名拉取 DNS 记录（交互模式，勾选需要同步的记录）
./yamlops dns pull records -e prod --domain example.com

# 从域名拉取 DNS 记录（直接模式，自动同步所有差异）
./yamlops dns pull records -e prod --domain example.com --auto-approve
```

**差异类型**：

| 符号 | 类型 | 说明 |
|------|------|------|
| `+` | CREATE | 远程有，本地无 |
| `~` | UPDATE | 值不同 |
| `-` | DELETE | 本地有，远程无 |

#### clean - 清理资源

```bash
./yamlops clean -e prod
```

#### TUI 交互模式

```bash
# 无参数启动进入 TUI
./yamlops -e prod
```

## 实体配置

### secrets.yaml - 秘钥定义

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
    services:
      - server
      - domain
      - dns
      - certificate
    credentials:
      access_key_id:
        secret: aliyun_access_key
      access_key_secret:
        secret: aliyun_access_secret

  - name: cloudflare
    services:
      - dns
      - certificate
    credentials:
      api_token: "cf-api-token"

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

### zones.yaml - 网区定义

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
      password:
        secret: srv_east_01_password
    environment:
      apt_source: tuna
      registries:
        - registry-aliyun
```

**APT 源选项**: `tuna` | `aliyun` | `tencent` | `official`

### gateways.yaml - 网关配置

```yaml
gateways:
  - name: gw-east-1
    zone: cn-east
    server: srv-east-01
    image: infra-gate:latest
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate
      sync: true
    ssl:
      mode: remote
      endpoint: http://infra-ssl.example.com:38567
    waf:
      enabled: true
      whitelist:
        - 10.0.0.0/8
        - 192.168.0.0/16
    log_level: 1
```

### services.yaml - 服务配置

```yaml
services:
  - name: api-server
    server: srv-east-01
    image: myapp/api:v1.0.0
    port: 3000
    env:
      NODE_ENV: production
      DATABASE_URL: postgres://db:5432/myapp
      REDIS_PASSWORD:
        secret: redis_password
    secrets:
      - db_password
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
        sync: true
      - ./data:/app/data
      - redis-data:/data
    gateway:
      enabled: true
      hostname: api.example.com
      path: /
      ssl: true
    internal: false
```

**Volume 格式**:
- `volumes://xxx` - 引用本地 volumes 目录，sync 时上传到服务器
- `./xxx` - 相对路径，在服务器上创建
- `name:/path` - Docker named volume

### registries.yaml - Docker Registry

```yaml
registries:
  - name: registry-aliyun
    url: registry.cn-shanghai.aliyuncs.com
    credentials:
      username:
        secret: aliyun_registry_user
      password:
        secret: aliyun_registry_password

  - name: registry-dockerhub
    url: registry.hub.docker.com
    credentials:
      username: "myuser"
      password:
        secret: dockerhub_password
```

### domains.yaml - 域名

```yaml
domains:
  - name: example.com
    isp: aliyun
    auto_renew: true

  - name: "*.example.com"
    parent: example.com
    isp: aliyun

  - name: api.example.com
    parent: example.com
    isp: cloudflare
```

### dns.yaml - DNS 记录

```yaml
records:
  - domain: example.com
    type: A
    name: "@"
    value: 203.0.113.10
    ttl: 300

  - domain: example.com
    type: A
    name: www
    value: 203.0.113.10
    ttl: 300

  - domain: example.com
    type: CNAME
    name: cdn
    value: cdn.example.com.cdn.dnsv1.com
    ttl: 600
```

**记录类型**: `A` | `AAAA` | `CNAME` | `MX` | `TXT` | `NS` | `SRV`

### certificates.yaml - SSL 证书

```yaml
certificates:
  - name: example-com
    domains:
      - example.com
      - "*.example.com"
    provider: letsencrypt
    dns_provider: cloudflare
    auto_renew: true
    renew_before: 30d

  - name: api-example-com
    domains:
      - api.example.com
    provider: zerossl
    dns_provider: aliyun
    auto_renew: true
```

**Provider**: `letsencrypt` | `zerossl`

## 秘钥管理

### 引用秘钥

支持两种方式引用秘钥：

```yaml
# 方式一：明文
password: "plain-text-password"

# 方式二：引用 secrets.yaml
password:
  secret: secret_name
```

### 环境变量中的秘钥

```yaml
services:
  - name: my-service
    env:
      DB_HOST: postgres.local
      DB_PASSWORD:
        secret: db_password
```

## 工作流程

### 1. 新增服务

```bash
# 1. 编辑 services.yaml 添加服务定义
vim userdata/prod/services.yaml

# 2. 验证配置
./yamlops validate -e prod

# 3. 查看变更计划
./yamlops plan -e prod --service my-service

# 4. 应用变更
./yamlops apply -e prod --service my-service
```

### 2. 更新服务镜像

```bash
# 1. 修改 services.yaml 中的 image 字段
# 2. 验证并应用
./yamlops validate -e prod && ./yamlops apply -e prod --service api-server
```

### 3. 新增服务器

```bash
# 1. 添加服务器配置
# 2. 添加 SSH 密码到 secrets.yaml
# 3. 验证环境
./yamlops env check -e prod --server new-server

# 4. 同步环境
./yamlops env sync -e prod --server new-server
```

### 4. 日常部署

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

- 所有服务容器名: `yo-<env>-<服务名>` (例如: `yo-prod-api-server`)
- 支持同一台服务器运行多个环境的服务，通过环境前缀区分
- 服务间通信使用容器名: `http://yo-<env>-<服务名>:<端口>`

### Docker 网络

每个环境使用独立的 Docker 网络 `yamlops-<env>`：

```yaml
networks:
  yamlops-prod:
    external: true
```

不同环境的服务使用各自的网络，实现环境隔离。

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

### SSL 证书

| CA | 方式 | 有效期 |
|----|------|--------|
| Let's Encrypt | ACME v2 | 90 天 |
| ZeroSSL | ACME v2 | 90 天 |

## 故障排查

### 配置验证失败

```bash
# 查看详细错误
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
