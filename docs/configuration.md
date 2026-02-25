# 配置指南

YAMLOps 配置文件完整指南，包括目录结构、配置文件说明和环境隔离。

## 目录结构

```
userdata/
├── prod/                    # 生产环境
│   ├── secrets.yaml         # 密钥配置
│   ├── isps.yaml            # 服务提供商配置
│   ├── zones.yaml           # 网络区域配置
│   ├── servers.yaml         # 服务器配置
│   ├── services_infra.yaml  # 基础设施服务配置
│   ├── services_biz.yaml    # 业务服务配置
│   ├── registries.yaml      # Docker 仓库配置
│   └── dns.yaml             # DNS 配置
├── staging/                 # 预发布环境
│   └── ...
├── dev/                     # 开发环境
│   └── ...
└── demo/                    # 演示环境
    └── ...
```

## 环境隔离

YAMLOps 支持多环境隔离，每个环境有独立的配置目录：

| 环境 | 目录 | 用途 |
|------|------|------|
| `prod` | `userdata/prod/` | 生产环境 |
| `staging` | `userdata/staging/` | 预发布环境 |
| `dev` | `userdata/dev/` | 开发环境 |
| `demo` | `userdata/demo/` | 演示环境 |

使用 `-e` 标志指定环境：

```bash
yamlops plan -e prod
yamlops apply -e staging
yamlops validate -e dev
```

---

## 配置文件说明

### 1. secrets.yaml

存储敏感信息，如密码、API 密钥等。

```yaml
secrets:
  - name: db_password
    value: "your_secure_password"

  - name: aliyun_access_key
    value: "LTAI5tXXXXXXXXXXXXXX"

  - name: cloudflare_api_token
    value: "your_cloudflare_token"
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 密钥名称，用于引用 |
| `value` | string | 是 | 密钥值 |

---

### 2. isps.yaml

定义服务提供商（云服务商）配置。

```yaml
isps:
  - name: aliyun
    type: aliyun                    # aliyun | cloudflare | tencent
    services:
      - domain                      # domain | dns | server
      - dns
    credentials:
      access_key_id:
        secret: aliyun_access_key   # 引用 secrets.yaml 中的密钥
      access_key_secret:
        secret: aliyun_access_secret
    remark: 阿里云-主账号

  - name: cloudflare
    type: cloudflare
    services:
      - dns
    credentials:
      api_token:
        secret: cloudflare_api_token
      account_id:
        secret: cloudflare_account_id
    remark: Cloudflare DNS
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 提供商名称 |
| `type` | string | 否 | 提供商类型（默认使用 name） |
| `services` | []string | 是 | 支持的服务类型 |
| `credentials` | map | 是 | 凭证映射 |
| `remark` | string | 否 | 备注说明 |

**支持的提供商类型：**

- `aliyun` - 阿里云
- `cloudflare` - Cloudflare
- `tencent` - 腾讯云

**支持的服务类型：**

- `server` - 服务器管理
- `domain` - 域名管理
- `dns` - DNS 管理

---

### 3. zones.yaml

定义网络区域，关联 ISP 和地理位置。

```yaml
zones:
  - name: cn-east
    description: 华东区域
    isp: aliyun
    region: cn-shanghai

  - name: us-west
    description: 美西区域
    isp: cloudflare
    region: us-california
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 区域名称 |
| `description` | string | 否 | 区域描述 |
| `isp` | string | 是 | 关联的 ISP 名称 |
| `region` | string | 否 | 地理区域标识 |

---

### 4. servers.yaml

定义服务器配置。

```yaml
servers:
  - name: prod-server-1
    zone: cn-east                    # 引用 zones.yaml
    isp: aliyun                      # 引用 isps.yaml
    os: ubuntu22
    ip:
      public: 1.2.3.4
      private: 10.0.0.1
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password:
        secret: server_password      # 引用 secrets.yaml
    networks:
      - name: yamlops-prod
        type: bridge
        driver: bridge
    environment:
      apt_source: aliyun
      registries:
        - dockerhub                  # 引用 registries.yaml
    remark: 主应用服务器
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 服务器名称 |
| `zone` | string | 是 | 所属区域 |
| `isp` | string | 否 | 服务提供商 |
| `os` | string | 否 | 操作系统类型 |
| `ip.public` | string | 是 | 公网 IP |
| `ip.private` | string | 否 | 内网 IP |
| `ssh.host` | string | 是 | SSH 主机地址 |
| `ssh.port` | int | 否 | SSH 端口（默认 22） |
| `ssh.user` | string | 是 | SSH 用户名 |
| `ssh.password` | SecretRef | 是 | SSH 密码 |
| `networks` | []Network | 否 | Docker 网络配置 |
| `environment.registries` | []string | 否 | Registry 引用列表 |
| `environment.apt_source` | string | 否 | APT 源 |
| `remark` | string | 否 | 备注说明 |

---

### 5. registries.yaml

定义 Docker 镜像仓库配置。

```yaml
registries:
  - name: dockerhub
    url: https://registry.hub.docker.com
    credentials:
      username: myuser
      password:
        secret: dockerhub_password
    remark: Docker Hub

  - name: aliyun-registry
    url: registry.cn-shanghai.aliyuncs.com
    credentials:
      username:
        secret: aliyun_registry_user
      password:
        secret: aliyun_registry_pass
    remark: 阿里云镜像仓库
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 仓库名称 |
| `url` | string | 是 | 仓库地址 |
| `credentials.username` | string/SecretRef | 是 | 用户名 |
| `credentials.password` | SecretRef | 是 | 密码 |
| `remark` | string | 否 | 备注说明 |

---

### 6. services_infra.yaml

定义基础设施服务（网关、SSL 等）。

```yaml
infra_services:
  - name: main-gateway
    type: gateway                    # gateway | ssl
    server: prod-server-1            # 引用 servers.yaml
    image: litelake/infra-gate:latest
    gatewayPorts:
      http: 80
      https: 443
    gatewayConfig:
      source: volumes://infra-gate
      sync: true
    gatewaySSL:
      mode: remote                   # local | remote
      endpoint: http://infra-ssl:38567
    gatewayWAF:
      enabled: true
      whitelist:
        - 192.168.0.0/16
        - 10.0.0.0/8
    gatewayLogLevel: 1
    networks:
      - yamlops-prod

  - name: infra-ssl
    type: ssl
    server: prod-server-1
    image: litelake/infra-ssl:latest
    gatewayPorts:
      api: 38567
    gatewayConfig:
      auth:
        enabled: true
        apikey:
          secret: ssl_api_key
      storage:
        type: local
        path: /data/certs
      defaults:
        issue_provider: letsencrypt_prod
        storage_provider: local_default
    networks:
      - yamlops-prod
```

**Gateway 类型字段：**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 服务名称 |
| `type` | string | 是 | 必须为 `gateway` |
| `server` | string | 是 | 部署服务器 |
| `image` | string | 是 | Docker 镜像 |
| `gatewayPorts.http` | int | 是 | HTTP 端口 |
| `gatewayPorts.https` | int | 否 | HTTPS 端口 |
| `gatewayConfig.source` | string | 否 | 配置源路径 |
| `gatewayConfig.sync` | bool | 否 | 是否同步配置 |
| `gatewaySSL.mode` | string | 否 | SSL 模式（local/remote） |
| `gatewaySSL.endpoint` | string | 条件 | SSL 服务端点（remote 模式必填） |
| `gatewayWAF.enabled` | bool | 否 | 启用 WAF |
| `gatewayWAF.whitelist` | []string | 否 | IP 白名单（CIDR 格式） |
| `gatewayLogLevel` | int | 否 | 日志级别 |
| `networks` | []string | 否 | 网络列表 |

**SSL 类型字段：**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 服务名称 |
| `type` | string | 是 | 必须为 `ssl` |
| `server` | string | 是 | 部署服务器 |
| `image` | string | 是 | Docker 镜像 |
| `gatewayPorts.api` | int | 是 | API 端口 |
| `gatewayConfig.auth.enabled` | bool | 否 | 启用认证 |
| `gatewayConfig.auth.apikey` | SecretRef | 条件 | API 密钥 |
| `gatewayConfig.storage.type` | string | 否 | 存储类型 |
| `gatewayConfig.storage.path` | string | 否 | 存储路径 |
| `networks` | []string | 否 | 网络列表 |

---

### 7. services_biz.yaml

定义业务服务。

```yaml
services:
  - name: api-server
    server: prod-server-1            # 引用 servers.yaml
    image: myapp/api:v1.0
    ports:
      - container: 8080
        host: 10080
        protocol: tcp
    env:
      DATABASE_URL:
        secret: db_url
      REDIS_URL:
        secret: redis_url
      LOG_LEVEL: info
    secrets:
      - db_password
      - redis_password
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
    networks:
      - yamlops-prod
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 服务名称 |
| `server` | string | 是 | 部署服务器 |
| `image` | string | 是 | Docker 镜像 |
| `ports` | []Port | 否 | 端口映射列表 |
| `env` | map | 否 | 环境变量 |
| `secrets` | []string | 否 | 需要的密钥列表 |
| `volumes` | []Volume | 否 | 卷挂载 |
| `healthcheck.path` | string | 否 | 健康检查路径 |
| `healthcheck.interval` | string | 否 | 检查间隔 |
| `healthcheck.timeout` | string | 否 | 超时时间 |
| `resources.cpu` | string | 否 | CPU 限制 |
| `resources.memory` | string | 否 | 内存限制 |
| `gateways` | []Gateway | 否 | 网关路由配置 |
| `internal` | bool | 否 | 是否仅内部访问 |
| `networks` | []string | 否 | 网络列表 |

**Port 字段：**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `container` | int | 是 | 容器端口 |
| `host` | int | 是 | 主机端口 |
| `protocol` | string | 否 | 协议（tcp/udp，默认 tcp） |

**Gateway 字段：**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `hostname` | string | 是 | 域名 |
| `container_port` | int | 是 | 容器端口 |
| `path` | string | 否 | 路径前缀 |
| `http` | bool | 否 | 启用 HTTP |
| `https` | bool | 否 | 启用 HTTPS |

---

### 8. dns.yaml

定义域名和 DNS 记录。

```yaml
domains:
  - name: example.com
    isp: aliyun                      # 域名注册商
    dns_isp: cloudflare              # DNS 服务商
    records:
      - type: A
        name: www
        value: 1.2.3.4
        ttl: 600
      - type: CNAME
        name: api
        value: www.example.com
        ttl: 600
      - type: MX
        name: '@'
        value: mail.example.com
        ttl: 600
        priority: 10
      - type: TXT
        name: '@'
        value: '"v=spf1 include:_spf.google.com ~all"'
        ttl: 3600

  - name: sub.example.com
    isp: aliyun
    dns_isp: aliyun
    parent: example.com              # 父域名（可选）
    records:
      - type: A
        name: '@'
        value: 5.6.7.8
        ttl: 600
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `name` | string | 是 | 域名 |
| `isp` | string | 是 | 域名注册商 ISP |
| `dns_isp` | string | 是 | DNS 服务商 ISP |
| `parent` | string | 否 | 父域名 |
| `records` | []Record | 否 | DNS 记录列表 |

**DNS 记录字段：**

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `type` | string | 是 | 记录类型（A/AAAA/CNAME/MX/TXT/NS/SRV） |
| `name` | string | 是 | 记录名称（`@` 表示根域名） |
| `value` | string | 是 | 记录值 |
| `ttl` | int | 是 | TTL 值（秒） |
| `priority` | int | 条件 | 优先级（MX/SRV 必填） |

**支持的记录类型：**

- `A` - IPv4 地址
- `AAAA` - IPv6 地址
- `CNAME` - 别名
- `MX` - 邮件交换
- `TXT` - 文本记录
- `NS` - 名称服务器
- `SRV` - 服务记录

---

### 6. services_infra.yaml

YAMLOps 支持两种密钥格式：

### 明文形式

```yaml
password: "plain-text-value"
username: myuser
```

### 引用形式

```yaml
password:
  secret: db_password       # 引用 secrets.yaml 中名为 db_password 的密钥
```

### 示例对比

```yaml
# 明文
servers:
  - name: srv-1
    ssh:
      password: "my_password_123"

# 引用（推荐）
servers:
  - name: srv-1
    ssh:
      password:
        secret: server_ssh_password
```

---

## 命名规范

| 元素 | 格式 | 示例 |
|------|------|------|
| 容器名 | `yo-{env}-{name}` | `yo-prod-api-server` |
| 网络名 | `yamlops-{env}` | `yamlops-prod` |
| 部署目录 | `/data/yamlops/yo-{env}-{name}` | `/data/yamlops/yo-prod-api` |

---

## 配置加载顺序

YAMLOps 按以下顺序加载配置文件：

1. `secrets.yaml` - 密钥
2. `isps.yaml` - 服务提供商
3. `zones.yaml` - 网络区域
4. `registries.yaml` - Docker 仓库
5. `servers.yaml` - 服务器
6. `services_infra.yaml` - 基础设施服务
7. `services_biz.yaml` - 业务服务
8. `dns.yaml` - DNS 配置

---

## 完整示例

### 生产环境配置

**secrets.yaml:**

```yaml
secrets:
  - name: db_password
    value: "secure_password_123"
  - name: redis_password
    value: "redis_secure_456"
  - name: aliyun_ak
    value: "LTAI5tXXXXXXXXXXXXXX"
  - name: aliyun_sk
    value: "XXXXXXXXXXXXXXXXXXXX"
```

**isps.yaml:**

```yaml
isps:
  - name: aliyun
    type: aliyun
    services:
      - domain
      - dns
    credentials:
      access_key_id:
        secret: aliyun_ak
      access_key_secret:
        secret: aliyun_sk
```

**zones.yaml:**

```yaml
zones:
  - name: cn-east
    description: 华东区域
    isp: aliyun
    region: cn-shanghai
```

**servers.yaml:**

```yaml
servers:
  - name: srv-prod-1
    zone: cn-east
    isp: aliyun
    os: ubuntu22
    ip:
      public: 1.2.3.4
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password:
        secret: server_password
    environment:
      registries:
        - dockerhub
```

**services_biz.yaml:**

```yaml
services:
  - name: api
    server: srv-prod-1
    image: myapp/api:v1.0
    ports:
      - container: 8080
        host: 10080
    env:
      DB_PASSWORD:
        secret: db_password
      REDIS_PASSWORD:
        secret: redis_password
    gateways:
      - hostname: api.example.com
        container_port: 8080
        http: true
        https: true
```

**dns.yaml:**

```yaml
domains:
  - name: example.com
    isp: aliyun
    dns_isp: aliyun
    records:
      - type: A
        name: api
        value: 1.2.3.4
        ttl: 600
```
