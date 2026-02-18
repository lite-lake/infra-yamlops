# YAMLOps 系统重新设计方案

## 1. 设计理念

### 1.1 核心原则

- **关注点分离**：基础信息（配置）、应用管理（部署）、域名管理（部署）三类职责清晰分离
- **最小惊讶**：CLI 命令与用户思维模型一致
- **精确控制**：支持从宏观（全部应用）到微观（单个服务）的操作粒度

### 1.2 实体分类

```
┌─────────────────────────────────────────────────────────────┐
│                      基础信息（Config）                       │
│  Secret / ISP / Registry                                    │
│  → 不参与部署，仅作为配置被引用                                │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌───────────────────────────┐   ┌───────────────────────────┐
│      应用管理（App）        │   │      域名管理（DNS）        │
│  Zone / Server            │   │  Domain / DNSRecord       │
│  InfraService / BizService │   │  → 可独立部署              │
│  → 可独立部署               │   └───────────────────────────┘
└───────────────────────────┘
```

---

## 2. 实体设计

### 2.1 基础信息（Config）

#### 2.1.1 Secret - 秘钥

```yaml
# userdata/{env}/secrets.yaml
secrets:
  - name: db_password
    value: "your-secret-value"
  - name: api_key
    value: { vault: production/api_key }
```

```go
type Secret struct {
    Name  string `yaml:"name"`
    Value string `yaml:"value"` // 支持明文或 vault 引用
}
```

#### 2.1.2 ISP - 服务商账号

```yaml
# userdata/{env}/isps.yaml
isps:
  - name: cloudflare
    services: [dns, domain]
    credentials:
      api_token: { secret: cf_api_token }
      
  - name: aliyun
    services: [dns, domain, server]
    credentials:
      access_key: { secret: ali_access_key }
      secret_key: { secret: ali_secret_key }
```

```go
type ISPService string

const (
    ISPServiceDNS     ISPService = "dns"
    ISPServiceDomain  ISPService = "domain"
    ISPServiceServer  ISPService = "server"
)

type ISP struct {
    Name        string                           `yaml:"name"`
    Services    []ISPService                     `yaml:"services"`
    Credentials map[string]valueobject.SecretRef `yaml:"credentials"`
}
```

#### 2.1.3 Registry - 镜像仓库

```yaml
# userdata/{env}/registries.yaml
registries:
  - name: docker-hub
    url: https://registry.hub.docker.com
    credentials:
      username: { secret: docker_user }
      password: { secret: docker_pass }
```

```go
type Registry struct {
    Name        string              `yaml:"name"`
    URL         string              `yaml:"url"`
    Credentials RegistryCredentials `yaml:"credentials"`
}
```

### 2.2 应用管理（App）

#### 2.2.1 Zone - 网区

```yaml
# userdata/{env}/zones.yaml
zones:
  - name: cn1
    description: "华东区域"
    isp: aliyun        # 可选，该区域默认 ISP
    region: cn-shanghai
```

```go
type Zone struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description,omitempty"`
    ISP         string `yaml:"isp,omitempty"`   // 可选
    Region      string `yaml:"region"`
}
```

#### 2.2.2 Server - 服务器

```yaml
# userdata/{env}/servers.yaml
servers:
  - name: cn1a
    zone: cn1
    isp: aliyun           # 可选，覆盖 zone 的 ISP
    os: ubuntu-22.04
    ip:
      public: 1.2.3.4
      private: 10.0.0.1
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password: { secret: cn1a_ssh_pass }
    environment:
      registries: [docker-hub]
```

```go
type Server struct {
    Name        string            `yaml:"name"`
    Zone        string            `yaml:"zone"`
    ISP         string            `yaml:"isp,omitempty"`   // 可选
    OS          string            `yaml:"os"`
    IP          ServerIP          `yaml:"ip"`
    SSH         ServerSSH         `yaml:"ssh"`
    Environment ServerEnvironment `yaml:"environment,omitempty"`
}
```

#### 2.2.3 InfraService - 基础服务

基础服务使用命名前缀 `infra-`，部署后为 `yo-{env}-infra-{name}`。

```yaml
# userdata/{env}/infra_services.yaml
infra_services:
  # 网关服务
  - name: gateway
    type: gateway
    server: cn1a
    image: docker.cnb.cool/lite-lake/infra-gate:prod
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate
      sync: true
    ssl:
      mode: remote
      endpoint: http://127.0.0.1:38567
    waf:
      enabled: false
      whitelist: []
      
  # SSL证书服务 (infra-ssl)
  # 项目地址: D:\Projects\lite-lake\infra-ssl
  # 镜像地址: docker.cnb.cool/lite-lake/infra-ssl:dev
  - name: ssl
    type: ssl
    server: cn1a
    image: docker.cnb.cool/lite-lake/infra-ssl:dev
    ports:
      api: 38567
    config:
      auth:
        enabled: true
        apikey: { secret: ssl_api_key }
      storage:
        type: local
        path: /data/certs
      defaults:
        issue_provider: letsencrypt_prod
        storage_provider: local_default
```

**SSL 服务 (infra-ssl) 说明：**

| 属性 | 说明 |
|------|------|
| 镜像 | `docker.cnb.cool/lite-lake/infra-ssl:dev` |
| 端口 | API 端口（默认 38567） |
| 认证 | API Key 认证 |

**API 端点：**

| 端点 | 方法 | 说明 | 认证 |
|------|------|------|------|
| `/health` | GET | 健康检查 | 否 |
| `/version` | GET | 版本信息 | 否 |
| `/cert/get-as-json?domain=xxx` | GET | 获取证书 JSON | 是 |
| `/cert/get-as-zip?domain=xxx` | GET | 获取证书 ZIP | 是 |

**证书获取流程：**
1. 网关服务调用 `/cert/get-as-json?domain=xxx`
2. 证书服务检查本地是否有有效证书（剩余 > 30 天）
3. 有则返回，无则自动签发新证书
4. 使用 DNS-01 验证（需配置对应 DNS 提供商）

**证书配置文件（挂载到容器内）：**

```yaml
# configs/domain.yml - 域名配置
domains:
  - code: domain_example_com
    domain: example.com
    dns_code: dns_cloudflare      # DNS 提供商
    issue_code: issue_letsencrypt # 签发提供商
    storage_code: storage_local   # 存储提供商

# configs/dns.yml - DNS 提供商配置
dnss:
  - code: dns_cloudflare
    type: cloudflare
    params:
      token: { secret: cf_api_token }

# configs/issue.yml - 签发提供商配置
issues:
  - code: issue_letsencrypt
    type: letsencrypt
    params:
      email: admin@example.com

# configs/storage.yml - 存储提供商配置
storages:
  - code: storage_local
    type: local
    params:
      path: /data/certs
```

```go
type InfraServiceType string

const (
    InfraServiceTypeGateway InfraServiceType = "gateway"
    InfraServiceTypeSSL     InfraServiceType = "ssl"
)

type InfraService struct {
    Name   string           `yaml:"name"`
    Type   InfraServiceType `yaml:"type"`
    Server string           `yaml:"server"`
    Image  string           `yaml:"image"`
    
    // 根据类型不同，使用不同的配置
    GatewayConfig *GatewayConfig `yaml:",inline,omitempty"`
    SSLConfig     *SSLConfig     `yaml:",inline,omitempty"`
}

type SSLConfig struct {
    Ports   SSLPorts   `yaml:"ports"`
    Auth    SSLAuth    `yaml:"auth"`
    Storage SSLStorage `yaml:"storage"`
    Defaults SSLDefaults `yaml:"defaults"`
}
```

#### 2.2.4 BizService - 业务服务

业务服务使用命名前缀 `svc-`，部署后为 `yo-{env}-svc-{name}`。

```yaml
# userdata/{env}/biz_services.yaml
biz_services:
  - name: api-server
    server: cn1a
    image: drm-hub.litelake.com/myapp/api:v1.0
    ports:
      - container: 8080
        host: 10001
    env:
      DB_HOST: postgres.internal
      DB_PASS: { secret: db_password }
    healthcheck:
      path: /health
      interval: 30s
    resources:
      cpu: "500m"
      memory: "512Mi"
    volumes:
      - /data/app:/app/data
    gateways:
      - hostname: api.example.com
        container_port: 8080
        path: /
        http: true
        https: true
```

```go
type BizService struct {
    Name        string                           `yaml:"name"`
    Server      string                           `yaml:"server"`
    Image       string                           `yaml:"image"`
    Ports       []ServicePort                    `yaml:"ports,omitempty"`
    Env         map[string]valueobject.SecretRef `yaml:"env,omitempty"`
    Healthcheck *ServiceHealthcheck              `yaml:"healthcheck,omitempty"`
    Resources   ServiceResources                 `yaml:"resources,omitempty"`
    Volumes     []ServiceVolume                  `yaml:"volumes,omitempty"`
    Gateways    []ServiceGatewayRoute            `yaml:"gateways,omitempty"`
}
```

### 2.3 域名管理（DNS）

#### 2.3.1 Domain - 域名

```yaml
# userdata/{env}/domains.yaml
domains:
  - name: example.com
    isp: aliyun           # 域名注册商（可选）
    dns_isp: cloudflare   # DNS 服务商（必填）
    auto_renew: true
    
  - name: "*.example.com"
    isp: aliyun
    dns_isp: cloudflare
    parent: example.com
```

```go
type Domain struct {
    Name      string `yaml:"name"`
    ISP       string `yaml:"isp,omitempty"`      // 域名注册商（可选）
    DNSISP    string `yaml:"dns_isp"`            // DNS 服务商（必填）
    Parent    string `yaml:"parent,omitempty"`
    AutoRenew bool   `yaml:"auto_renew,omitempty"`
}
```

#### 2.3.2 DNSRecord - DNS 记录

```yaml
# userdata/{env}/dns.yaml
dns_records:
  - domain: example.com
    type: A
    name: www
    value: 1.2.3.4
    ttl: 300
    
  - domain: example.com
    type: CNAME
    name: api
    value: api.internal.example.com
    ttl: 300
```

```go
type DNSRecordType string

const (
    DNSRecordTypeA     DNSRecordType = "A"
    DNSRecordTypeAAAA  DNSRecordType = "AAAA"
    DNSRecordTypeCNAME DNSRecordType = "CNAME"
    DNSRecordTypeMX    DNSRecordType = "MX"
    DNSRecordTypeTXT   DNSRecordType = "TXT"
    DNSRecordTypeNS    DNSRecordType = "NS"
    DNSRecordTypeSRV   DNSRecordType = "SRV"
)

type DNSRecord struct {
    Domain string        `yaml:"domain"`
    Type   DNSRecordType `yaml:"type"`
    Name   string        `yaml:"name"`
    Value  string        `yaml:"value"`
    TTL    int           `yaml:"ttl"`
}
```

---

## 3. 实体关系图

```
┌─────────┐
│ Secret  │◄─────────────────────────────────────────────┐
└────┬────┘                                              │
     │ 被引用                                            │
     ▼                                                   │
┌─────────┐     services      ┌─────────┐               │
│   ISP   │◄──────────────────│  Zone   │               │
└────┬────┘                   └────┬────┘               │
     │ dns/domain/server           │ 包含                │
     │                             ▼                     │
     │                       ┌─────────┐                 │
     │                       │ Server  │                 │
     │                       └────┬────┘                 │
     │                            │ 部署                  │
     │              ┌─────────────┼─────────────┐        │
     │              ▼             ▼             ▼        │
     │        ┌──────────┐ ┌──────────┐ ┌──────────┐     │
     │        │InfraSvc  │ │ BizSvc   │ │  ...     │     │
     │        └──────────┘ └──────────┘ └──────────┘     │
     │                                                   │
     │                   dns_isp                         │
     └──────────────────────►┌─────────┐                │
                             │ Domain  │────────────────┘
                             └────┬────┘   credentials
                                  │ 包含
                                  ▼
                            ┌───────────┐
                            │ DNSRecord │
                            └───────────┘
```

---

## 4. CLI 命令设计

### 4.1 命令结构

```
yamlops
├── [全局选项]
│   ├── -e, --env <env>        环境 (prod/staging/dev)
│   ├── -c, --config <dir>     配置目录
│   └── -v, --version          版本信息
│
├── app                        应用管理
│   ├── plan                   生成部署计划
│   ├── apply                  执行部署
│   ├── list                   列出资源
│   └── show                   查看详情
│
├── dns                        域名管理
│   ├── plan                   生成变更计划
│   ├── apply                  执行变更
│   ├── list                   列出资源
│   └── show                   查看详情
│
├── config                     基础配置（只读）
│   ├── list                   列出配置项
│   └── show                   查看配置详情
│
├── validate                   验证所有配置
│
└── [默认]                     启动 TUI 界面
```

### 4.2 应用管理命令 (app)

#### `yamlops app plan`

生成应用部署计划。

```bash
# 生成所有应用的部署计划
yamlops app plan

# 按网区筛选
yamlops app plan --zone cn1

# 按服务器筛选
yamlops app plan --server cn1a

# 按基础服务筛选
yamlops app plan --infra gateway

# 按业务服务筛选
yamlops app plan --biz api-server

# 组合筛选
yamlops app plan --zone cn1 --biz api-server
```

**选项：**
| 选项 | 说明 |
|------|------|
| `--zone, -z` | 按网区筛选 |
| `--server, -s` | 按服务器筛选 |
| `--infra, -i` | 按基础服务名筛选 |
| `--biz, -b` | 按业务服务名筛选 |

#### `yamlops app apply`

执行应用部署。

```bash
# 部署所有应用（需确认）
yamlops app apply

# 自动确认
yamlops app apply --auto-approve

# 按服务器部署
yamlops app apply --server cn1a

# 仅部署基础服务
yamlops app apply --infra gateway --infra cert

# 仅部署业务服务
yamlops app apply --biz api-server
```

**选项：**
| 选项 | 说明 |
|------|------|
| `--zone, -z` | 按网区筛选 |
| `--server, -s` | 按服务器筛选 |
| `--infra, -i` | 按基础服务名筛选（可多次指定） |
| `--biz, -b` | 按业务服务名筛选（可多次指定） |
| `--auto-approve` | 跳过确认提示 |

#### `yamlops app list`

列出应用相关资源。

```bash
# 列出所有应用资源概览
yamlops app list

# 列出特定类型
yamlops app list zones
yamlops app list servers
yamlops app list infra
yamlops app list biz
```

**输出示例：**
```
ZONES:
  cn1      (isp: aliyun, region: cn-shanghai)
  us1      (isp: aws, region: us-west-1)

SERVERS:
  cn1a     (zone: cn1, ip: 1.2.3.4)
  us1a     (zone: us1, ip: 5.6.7.8)

INFRA SERVICES:
  gateway  (server: cn1a, type: gateway)
  cert     (server: cn1a, type: cert)

BIZ SERVICES:
  api-server   (server: cn1a, ports: 10001->8080)
  web-front    (server: us1a, ports: 10001->80)
```

#### `yamlops app show`

查看应用资源详情。

```bash
yamlops app show zone cn1
yamlops app show server cn1a
yamlops app show infra gateway
yamlops app show biz api-server
```

### 4.3 域名管理命令 (dns)

#### `yamlops dns plan`

生成 DNS 变更计划。

```bash
# 生成所有 DNS 变更计划
yamlops dns plan

# 按域名筛选
yamlops dns plan --domain example.com

# 按记录筛选
yamlops dns plan --record "www.example.com"
```

**选项：**
| 选项 | 说明 |
|------|------|
| `--domain, -d` | 按域名筛选 |
| `--record, -r` | 按记录筛选（格式：name.domain） |

#### `yamlops dns apply`

执行 DNS 变更。

```bash
# 应用所有 DNS 变更
yamlops dns apply

# 仅变更指定域名
yamlops dns apply --domain example.com

# 自动确认
yamlops dns apply --auto-approve
```

#### `yamlops dns list`

列出域名相关资源。

```bash
yamlops dns list              # 列出所有
yamlops dns list domains      # 仅域名
yamlops dns list records      # 仅记录
```

**输出示例：**
```
DOMAINS:
  example.com      (dns_isp: cloudflare, isp: aliyun)
  *.example.com    (dns_isp: cloudflare, parent: example.com)

DNS RECORDS:
  example.com      A       @           -> 1.2.3.4      (ttl: 300)
  example.com      A       www         -> 1.2.3.4      (ttl: 300)
  example.com      CNAME   api         -> api.internal (ttl: 300)
```

#### `yamlops dns show`

查看域名资源详情。

```bash
yamlops dns show domain example.com
yamlops dns show record "www.example.com"
```

### 4.4 配置管理命令 (config)

#### `yamlops config list`

列出基础配置。

```bash
yamlops config list            # 列出所有
yamlops config list secrets    # 仅密钥（隐藏值）
yamlops config list isps       # 仅 ISP
yamlops config list registries # 仅镜像仓库
```

#### `yamlops config show`

查看配置详情。

```bash
yamlops config show secret db_password   # 警告：会显示值
yamlops config show isp cloudflare
yamlops config show registry docker-hub
```

### 4.5 验证命令

#### `yamlops validate`

验证所有配置文件的正确性和引用完整性。

```bash
yamlops validate
yamlops validate --strict   # 严格模式，检查所有引用
```

---

## 5. TUI 交互设计

### 5.1 设计理念

- **树形选择器**：所有可部署资源以树形结构展示
- **多选操作**：用户可精确选择要部署的项目
- **层级联动**：选中父节点自动选中所有子节点
- **即时反馈**：显示选中数量和部署预览

### 5.2 主界面布局

```
┌──────────────────────────────────────────────────────────────────────────┐
│  YAMLOps                                              prod    选中: 5/24 │
├──────────────────────────────────────────────────────────────────────────┤
│  Applications ────────────────────────────────────── DNS ────────────── │
│                                                                          │
│  ◉ cn1                                    华东区域      2 selected      │
│    │                                                                      │
│    ├─◉ cn1a                              1.2.3.4       2 selected      │
│    │   ├─◉ [infra] gateway              :80,:443      selected        │
│    │   └─○ [infra] cert                 :38567                         │
│    │                                                                      │
│    └─○ cn1b                              5.6.7.8                       │
│        ├─○ [biz] api-server             :10001                         │
│        └─○ [biz] web-front              :10002                         │
│                                                                          │
│  ○ us1                                    美西区域                      │
│    └─ [...]                                                              │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  [Space] 选择  [Enter] 展开/折叠  [a] 全选当前  [n] 取消当前  [p] Plan  │
│  [A] 全部选中  [N] 全部取消  [Tab] 切换 App/DNS  [q] 退出               │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.3 界面元素说明

#### 状态图标

| 图标 | 含义 |
|------|------|
| `◉` | 选中（包含子项） |
| `○` | 未选中 |
| `◐` | 部分选中（子项有选有未选） |
| `▸` | 折叠状态（可展开） |
| `▾` | 展开状态（可折叠） |

#### 颜色编码

| 颜色 | 含义 |
|------|------|
| 绿色 | 选中项 / 运行中 |
| 黄色 | 需要更新 |
| 红色 | 错误 / 将被删除 |
| 灰色 | 未选中 / 禁用 |

### 5.4 操作流程

#### 5.4.1 选择资源

```
1. 上下箭头导航
2. Space 切换选中状态
   - 选中 Zone → 自动选中所有 Server 和 Service
   - 选中 Server → 自动选中该 Server 下的所有 Service
   - 取消同理
3. Enter 展开/折叠节点
4. Tab 切换 Applications / DNS 视图
```

#### 5.4.2 批量选择

```
[a]     全选当前高亮节点及其所有子节点
[n]     取消选择当前高亮节点及其所有子节点
[A]     全选所有资源
[N]     取消选择所有资源
```

#### 5.4.3 执行部署

```
1. 按 [p] 生成 Plan
2. 系统显示变更预览
3. 确认后执行 Apply
```

### 5.5 Applications 视图（默认）

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Applications                                    prod    选中: 3/12     │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ◉ cn1                                    华东区域      3 selected      │
│    ▾                                                                      │
│    ├─◉ cn1a                              1.2.3.4       3 selected      │
│    │   ├─◉ [infra] gateway              :80,:443      ✓ running        │
│    │   ├─◉ [infra] cert                 :38567        ✓ running        │
│    │   └─◉ [biz] api-server             :10001        ~ needs update   │
│    │                                                                      │
│    └─○ cn1b                              5.6.7.8                       │
│        ├─○ [biz] scheduler              :10003                         │
│        └─○ [biz] worker                 :10004                         │
│                                                                          │
│  ◐ us1                                    美西区域      1/4 selected   │
│    ▾                                                                      │
│    └─◐ us1a                              9.10.11.12    1/4 selected    │
│        ├─○ [infra] gateway              :80,:443                       │
│        ├─○ [infra] cert                 :38567                         │
│        ├─○ [biz] web-front              :10001                         │
│        └─◉ [biz] api-gateway            :10002        ✓ running        │
│                                                                          │
│  ○ eu1                                    欧洲区域                      │
│    ▸ [...]                                                                │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  选中: 3 InfraService, 2 BizService    将部署到 2 台服务器              │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.6 DNS 视图（Tab 切换）

```
┌──────────────────────────────────────────────────────────────────────────┐
│  DNS & Domains                                    prod    选中: 4/10     │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ◉ example.com                           cloudflare   4 selected       │
│    ▾                                                                      │
│    ├─◉ A      @                          1.2.3.4      ✓ synced         │
│    ├─◉ A      www                        1.2.3.4      ✓ synced         │
│    ├─◉ CNAME  api                        api.internal ✓ synced         │
│    └─◉ TXT    _dmarc                     v=DMARC1...  ✓ synced         │
│                                                                          │
│  ◐ staging.example.com                   cloudflare   1/3 selected     │
│    ▾                                                                      │
│    ├─◉ A      @                          5.6.7.8      ✓ synced         │
│    ├─○ A      www                        5.6.7.8                       │
│    └─○ CNAME  api                        api.internal                  │
│                                                                          │
│  ○ internal.example.com                  cloudflare                    │
│    ▸ [...]                                                                │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  选中: 4 DNS 记录    将在 cloudflare 上更新                              │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.7 Plan 预览界面

按 `p` 后显示：

```
┌──────────────────────────────────────────────────────────────────────────┐
│  执行计划                                            5 项变更            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Applications (3 项)                                                     │
│  ────────────────────                                                    │
│                                                                          │
│    + CREATE   [infra] gateway @ cn1a                                     │
│        • 拉取镜像 docker.cnb.cool/lite-lake/infra-gate:prod             │
│        • 创建容器 yo-prod-infra-gateway                                 │
│        • 配置反向代理                                                    │
│                                                                          │
│    ~ UPDATE   [infra] cert @ cn1a                                        │
│        • 更新镜像版本                                                    │
│        • 重启容器                                                        │
│                                                                          │
│    ~ UPDATE   [biz] api-server @ cn1a                                    │
│        • 拉取镜像 drm-hub.litelake.com/myapp/api:v2.0                   │
│        • 重建容器 yo-prod-svc-api-server                                │
│        • 健康检查                                                        │
│                                                                          │
│  DNS (2 项)                                                              │
│  ────────────                                                            │
│                                                                          │
│    + CREATE   A @ example.com                                           │
│        • 1.2.3.4 (TTL: 300) [cloudflare]                                │
│                                                                          │
│    ~ UPDATE   A www.example.com                                         │
│        • 1.2.3.4 → 5.6.7.8 [cloudflare]                                 │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  [Enter] 确认执行  [Esc] 返回修改选择  [s] 仅显示选中项                   │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.8 执行进度界面

确认后显示：

### 5.8 执行进度界面

部署分为两个阶段：**拉取镜像** → **部署服务**

#### 阶段一：拉取镜像

```
┌──────────────────────────────────────────────────────────────────────────┐
│  拉取镜像...                                         3/5 完成            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  60%         │
│                                                                          │
│  ✓ [infra] gateway @ cn1a                                     完成      │
│      → docker.cnb.cool/lite-lake/infra-gate:prod                         │
│                                                                          │
│  ✓ [infra] ssl @ cn1a                                         完成      │
│      → docker.cnb.cool/lite-lake/infra-ssl:dev                           │
│                                                                          │
│  ✓ [biz] api-server @ cn1a                                    完成      │
│      → drm-hub.litelake.com/myapp/api:v2.0                               │
│                                                                          │
│  ⏳ [biz] web-front @ cn1b                                    拉取中... │
│      → drm-hub.litelake.com/myapp/web:v1.5                               │
│      ████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  55%          │
│                                                                          │
│  ○ [biz] scheduler @ cn1b                                     等待中    │
│      → drm-hub.litelake.com/myapp/scheduler:latest                      │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  阶段 1/2: 拉取镜像                                                        │
└──────────────────────────────────────────────────────────────────────────┘
```

#### 阶段二：部署服务

```
┌──────────────────────────────────────────────────────────────────────────┐
│  部署服务...                                         2/5 完成            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  40%       │
│                                                                          │
│  ✓ [infra] gateway @ cn1a                                     完成      │
│      → 停止旧容器                                                         │
│      → 创建容器 yo-prod-infra-gateway                                    │
│      → 配置反向代理                                                       │
│      → 健康检查通过                                                       │
│                                                                          │
│  ✓ [infra] ssl @ cn1a                                         完成      │
│      → 创建容器 yo-prod-infra-ssl                                        │
│      → 服务就绪                                                          │
│                                                                          │
│  ⏳ [biz] api-server @ cn1a                                    执行中... │
│      → 停止旧容器 yo-prod-svc-api-server-v1                              │
│      → 创建容器 yo-prod-svc-api-server                                   │
│      ████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░  70%          │
│                                                                          │
│  ○ [biz] web-front @ cn1b                                     等待中    │
│                                                                          │
│  ○ [biz] scheduler @ cn1b                                     等待中    │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  阶段 2/2: 部署服务                                                       │
└──────────────────────────────────────────────────────────────────────────┘
```

#### DNS 变更（并行执行）

```
┌──────────────────────────────────────────────────────────────────────────┐
│  DNS 变更...                                          1/2 完成            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░  50%         │
│                                                                          │
│  ✓ A @ example.com                                            完成      │
│      → 1.2.3.4 (TTL: 300) [cloudflare]                                   │
│                                                                          │
│  ⏳ CNAME api.example.com                                      执行中... │
│      → api.internal.example.com [cloudflare]                             │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  DNS 变更与应用部署并行执行                                                │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.9 执行流程说明

```
┌─────────────────────────────────────────────────────────────┐
│                      部署执行流程                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐                                           │
│  │ 用户确认执行  │                                           │
│  └──────┬───────┘                                           │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────────────────────────────────────────┐      │
│  │ 阶段 1: 拉取镜像（所有选中的服务）                  │      │
│  │ ├─ 并行拉取各服务器的镜像                          │      │
│  │ ├─ 显示每个镜像的拉取进度                          │      │
│  │ └─ 任一失败则中止，显示错误                        │      │
│  └──────────────────────┬───────────────────────────┘      │
│                         │                                   │
│                         ▼                                   │
│  ┌──────────────────────────────────────────────────┐      │
│  │ 阶段 2: 部署服务（全部镜像拉取成功后）              │      │
│  │ ├─ 按依赖顺序部署（infra → biz）                   │      │
│  │ ├─ 停止旧容器 → 创建新容器 → 健康检查              │      │
│  │ └─ 显示每步执行状态                                │      │
│  └──────────────────────┬───────────────────────────┘      │
│                         │                                   │
│                         ▼                                   │
│  ┌──────────────────────────────────────────────────┐      │
│  │ 并行: DNS 变更（与阶段 2 同时进行）                 │      │
│  │ ├─ 调用 DNS 提供商 API                             │      │
│  │ └─ 显示变更结果                                    │      │
│  └──────────────────────┬───────────────────────────┘      │
│                         │                                   │
│                         ▼                                   │
│  ┌──────────────┐                                           │
│  │ 显示最终结果  │                                           │
│  └──────────────┘                                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 5.10 执行结果界面

```
┌──────────────────────────────────────────────────────────────────────────┐
│  执行完成                                            4/5 成功            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  镜像拉取: 5/5 成功                                                       │
│  服务部署: 4/4 成功                                                       │
│  DNS 变更: 1/1 成功                                                       │
│                                                                          │
│  ✓ [infra] gateway @ cn1a                                     成功      │
│  ✓ [infra] ssl @ cn1a                                         成功      │
│  ✓ [biz] api-server @ cn1a                                    成功      │
│  ✓ [biz] web-front @ cn1b                                     成功      │
│  ✗ [biz] scheduler @ cn1b                                     失败      │
│      → Error: 镜像拉取失败: manifest unknown                             │
│      → 建议: 检查镜像标签是否正确                                         │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  [r] 重试失败项  [l] 查看详细日志  [q] 返回主界面                         │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.11 快捷键总览

| 快捷键 | 功能 | 上下文 |
|--------|------|--------|
| `↑/k` | 上移 | 主界面 |
| `↓/j` | 下移 | 主界面 |
| `Space` | 切换选中 | 主界面 |
| `Enter` | 展开/折叠 或 确认 | 主界面/确认框 |
| `Tab` | 切换 App/DNS 视图 | 主界面 |
| `a` | 全选当前节点 | 主界面 |
| `n` | 取消当前节点 | 主界面 |
| `A` | 全选所有 | 主界面 |
| `N` | 取消所有 | 主界面 |
| `/` | 搜索过滤 | 主界面 |
| `p` | 生成执行计划 | 主界面 |
| `r` | 刷新状态 | 主界面 |
| `?` | 帮助 | 任意 |
| `Esc` | 返回/取消 | 任意 |
| `q` | 退出 | 主界面 |

### 5.12 数据结构

```go
type TreeNode struct {
    ID       string      // 唯一标识
    Type     NodeType    // zone/server/infra/biz/domain/record
    Name     string      // 显示名称
    Selected bool        // 是否选中
    Expanded bool        // 是否展开
    Children []*TreeNode // 子节点
    Parent   *TreeNode   // 父节点
    Status   NodeStatus  // 状态
    Info     string      // 附加信息
}

type NodeType string

const (
    NodeTypeZone       NodeType = "zone"
    NodeTypeServer     NodeType = "server"
    NodeTypeInfra      NodeType = "infra"
    NodeTypeBiz        NodeType = "biz"
    NodeTypeDomain     NodeType = "domain"
    NodeTypeDNSRecord  NodeType = "record"
)

type NodeStatus string

const (
    StatusRunning    NodeStatus = "running"
    StatusStopped    NodeStatus = "stopped"
    StatusNeedsUpdate NodeStatus = "needs_update"
    StatusError      NodeStatus = "error"
    StatusSynced     NodeStatus = "synced"
)

type Selection struct {
    InfraServices []string
    BizServices   []string
    DNSRecords    []string // 格式: domain/type/name
}

// 执行阶段
type ExecutionPhase string

const (
    PhasePullImages  ExecutionPhase = "pull_images"   // 阶段1: 拉取镜像
    PhaseDeploy      ExecutionPhase = "deploy"        // 阶段2: 部署服务
    PhaseDNS         ExecutionPhase = "dns"           // DNS变更（并行）
)

// 镜像拉取状态
type ImagePullStatus struct {
    Server    string // 服务器名
    ServiceID string // 服务标识
    Image     string // 镜像地址
    Status    string // pulling/completed/failed
    Progress  int    // 进度百分比
    Error     string // 错误信息
}

// 部署任务状态
type DeployTaskStatus struct {
    Server    string // 服务器名
    ServiceID string // 服务标识
    Phase     string // 当前阶段: stopping/creating/healthcheck
    Status    string // running/completed/failed
    Error     string // 错误信息
}

func (n *TreeNode) IsPartiallySelected() bool {
    if len(n.Children) == 0 {
        return false
    }
    hasSelected := false
    hasUnselected := false
    for _, child := range n.Children {
        if child.Selected || child.IsPartiallySelected() {
            hasSelected = true
        }
        if !child.Selected {
            hasUnselected = true
        }
    }
    return hasSelected && hasUnselected
}

func (n *TreeNode) SelectRecursive(selected bool) {
    n.Selected = selected
    for _, child := range n.Children {
        child.SelectRecursive(selected)
    }
}

func (n *TreeNode) UpdateParentSelection() {
    if n.Parent == nil {
        return
    }
    allSelected := true
    anySelected := false
    for _, child := range n.Parent.Children {
        if child.Selected {
            anySelected = true
        } else {
            allSelected = false
        }
    }
    n.Parent.Selected = allSelected
    n.Parent.UpdateParentSelection()
}
```

---

## 6. 配置文件结构

### 6.1 目录结构

```
userdata/
├── prod/
│   ├── secrets.yaml         # Secret
│   ├── isps.yaml            # ISP
│   ├── registries.yaml      # Registry
│   ├── zones.yaml           # Zone
│   ├── servers.yaml         # Server
│   ├── infra_services.yaml  # InfraService
│   ├── biz_services.yaml    # BizService
│   ├── domains.yaml         # Domain
│   └── dns.yaml             # DNSRecord
│
├── staging/
│   └── ... (同上)
│
└── dev/
    └── ... (同上)
```

### 6.2 配置文件示例

#### secrets.yaml

```yaml
secrets:
  - name: db_password
    value: "super-secret-password"
    
  - name: cf_api_token
    value: "cloudflare-api-token"
    
  - name: ali_access_key
    value: "aliyun-access-key"
    
  - name: ali_secret_key
    value: "aliyun-secret-key"
```

#### isps.yaml

```yaml
isps:
  - name: cloudflare
    services: [dns]
    credentials:
      api_token: { secret: cf_api_token }
      
  - name: aliyun
    services: [dns, domain, server]
    credentials:
      access_key: { secret: ali_access_key }
      secret_key: { secret: ali_secret_key }
```

#### registries.yaml

```yaml
registries:
  - name: drm-hub
    url: https://drm-hub.litelake.com
    credentials:
      username: { secret: registry_user }
      password: { secret: registry_pass }
```

#### zones.yaml

```yaml
zones:
  - name: cn1
    description: "华东区域"
    isp: aliyun
    region: cn-shanghai
    
  - name: us1
    description: "美西区域"
    isp: aws
    region: us-west-1
```

#### servers.yaml

```yaml
servers:
  - name: cn1a
    zone: cn1
    os: ubuntu-22.04
    ip:
      public: 1.2.3.4
      private: 10.0.0.1
    ssh:
      host: 1.2.3.4
      port: 22
      user: root
      password: { secret: cn1a_ssh_pass }
    environment:
      registries: [drm-hub]
```

#### infra_services.yaml

```yaml
infra_services:
  # 网关服务
  - name: gateway
    type: gateway
    server: cn1a
    image: docker.cnb.cool/lite-lake/infra-gate:prod
    ports:
      http: 80
      https: 443
    config:
      source: volumes://infra-gate
      sync: true
    ssl:
      mode: remote
      endpoint: http://127.0.0.1:38567
    waf:
      enabled: false
      
  # SSL 证书服务 (infra-ssl)
  # 项目: D:\Projects\lite-lake\infra-ssl
  # 镜像: docker.cnb.cool/lite-lake/infra-ssl:dev
  - name: ssl
    type: ssl
    server: cn1a
    image: docker.cnb.cool/lite-lake/infra-ssl:dev
    ports:
      api: 38567
    config:
      auth:
        enabled: true
        apikey: { secret: ssl_api_key }
      storage:
        type: local
        path: /data/certs
      defaults:
        issue_provider: letsencrypt_prod
        storage_provider: local_default
```

#### biz_services.yaml

```yaml
biz_services:
  - name: api-server
    server: cn1a
    image: drm-hub.litelake.com/myapp/api:v1.0
    ports:
      - container: 8080
        host: 10001
    env:
      DB_HOST: postgres.internal
      DB_PASS: { secret: db_password }
    healthcheck:
      path: /health
      interval: 30s
    resources:
      cpu: "500m"
      memory: "512Mi"
    gateways:
      - hostname: api.example.com
        container_port: 8080
        path: /
        http: true
        https: true
```

#### domains.yaml

```yaml
domains:
  - name: example.com
    dns_isp: cloudflare
    isp: aliyun
    auto_renew: true
    
  - name: "*.example.com"
    dns_isp: cloudflare
    isp: aliyun
    parent: example.com
```

#### dns.yaml

```yaml
dns_records:
  - domain: example.com
    type: A
    name: "@"
    value: 1.2.3.4
    ttl: 300
    
  - domain: example.com
    type: A
    name: www
    value: 1.2.3.4
    ttl: 300
    
  - domain: example.com
    type: CNAME
    name: api
    value: api.internal.example.com
    ttl: 300
```

---

## 7. 迁移计划

### 7.1 实体变更对照

| 旧实体 | 新实体 | 变更说明 |
|--------|--------|----------|
| Secret | Secret | 无变化 |
| ISP | ISP | 移除 certificate 服务类型 |
| Registry | Registry | 无变化 |
| Zone | Zone | ISP 字段改为可选 |
| Server | Server | ISP 字段改为可选 |
| Gateway | InfraService (type=gateway) | 合并为 InfraService |
| (新增) | InfraService (type=ssl) | 新增 SSL 证书服务 |
| Service | BizService | 重命名 |
| Certificate | (移除) | 由 infra-ssl 服务自动管理 |
| Domain | Domain | 新增 dns_isp 字段 |
| DNSRecord | DNSRecord | ISP 从 Domain 继承 |

### 7.2 文件变更

| 旧文件 | 新文件 |
|--------|--------|
| secrets.yaml | secrets.yaml |
| isps.yaml | isps.yaml |
| registries.yaml | registries.yaml |
| zones.yaml | zones.yaml |
| servers.yaml | servers.yaml |
| gateways.yaml | infra_services.yaml |
| services.yaml | biz_services.yaml |
| certificates.yaml | (移除) |
| domains.yaml | domains.yaml |
| dns.yaml | dns.yaml |

### 7.3 迁移步骤

1. **备份数据**：备份 `userdata/` 目录
2. **转换配置**：运行迁移脚本转换配置文件格式
3. **验证配置**：运行 `yamlops validate` 验证新配置
4. **更新代码**：按新实体结构更新代码
5. **测试部署**：在 dev 环境测试完整流程
6. **生产迁移**：在生产环境应用迁移

---

## 8. 实现优先级

### Phase 1: 核心重构（必须）

1. 实体定义重构
   - 修改 Zone/Server 的 ISP 为可选
   - 创建 InfraService 实体
   - 重命名 Service 为 BizService
   - 移除 Certificate 实体
   - Domain 新增 dns_isp 字段

2. 配置加载器更新
   - 支持新的配置文件结构
   - 更新验证逻辑

### Phase 2: CLI 重构（必须）

1. 新命令结构
   - `yamlops app {plan,apply,list,show}`
   - `yamlops dns {plan,apply,list,show}`
   - `yamlops config {list,show}`
   - `yamlops validate`

2. 筛选逻辑
   - 实现 zone/server/infra/biz 筛选
   - 实现 domain/record 筛选

### Phase 3: TUI 重构（推荐）

1. 新界面结构
2. 分组展示
3. 交互优化

### Phase 4: 迁移工具（推荐）

1. 配置迁移脚本
2. 兼容性检查

---

## 附录 A: infra-ssl 服务详解

### A.1 服务概述

**项目信息：**
- 代码仓库：`D:\Projects\lite-lake\infra-ssl`
- 镜像地址：`docker.cnb.cool/lite-lake/infra-ssl:dev`

**核心功能：**
- SSL 证书自动签发（Let's Encrypt / ZeroSSL）
- 证书自动续期（剩余 ≤ 30 天自动续期）
- DNS-01 验证方式
- 多 DNS 提供商支持（Cloudflare / 阿里云 / 腾讯云）
- 多存储后端支持（本地 / 腾讯云 COS）

### A.2 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      infra-ssl 服务                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │  HTTP API   │───▶│  CertService │───▶│  Storage    │     │
│  │  (Gin)      │    │             │    │  Provider   │     │
│  └─────────────┘    └──────┬──────┘    └─────────────┘     │
│                            │                                │
│         ┌──────────────────┼──────────────────┐            │
│         ▼                  ▼                  ▼            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │ DNS Service │    │Issue Service│    │DomainService│     │
│  │             │    │             │    │             │     │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘     │
│         │                  │                  │            │
│         ▼                  ▼                  ▼            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              配置文件 (configs/*.yml)                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### A.3 API 接口

| 端点 | 方法 | 说明 | 认证 | 超时 |
|------|------|------|------|------|
| `/health` | GET | 健康检查 | 白名单 | - |
| `/version` | GET | 版本信息 | 白名单 | - |
| `/cert/get-as-json` | GET | 获取证书 JSON | API Key | 5 分钟 |
| `/cert/get-as-zip` | GET | 获取证书 ZIP | API Key | 5 分钟 |

**请求示例：**

```bash
# 获取证书 JSON
curl -H "X-API-Key: your-api-key" \
  "http://localhost:38567/cert/get-as-json?domain=api.example.com"

# 响应
{
  "domain": "example.com",
  "sub_domain": "api",
  "fullchain": "-----BEGIN CERTIFICATE-----\n...",
  "privkey": "-----BEGIN PRIVATE KEY-----\n...",
  "start_date": "2025-01-01T00:00:00Z",
  "expired_date": "2025-04-01T00:00:00Z",
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

### A.4 配置文件

证书服务需要挂载以下配置文件到容器内：

#### server.yml - 服务配置

```yaml
server:
  port: 38567
  host: "0.0.0.0"
  debug: false

auth:
  enabled: true
  apikey: "your-secure-api-key"

whitelist:
  - /health
  - /version
  - /swagger/
  - /
```

#### domain.yml - 域名配置

```yaml
domains:
  - code: domain_example_com
    domain: example.com
    dns_code: dns_cloudflare        # DNS 提供商 code
    issue_code: issue_letsencrypt   # 签发提供商 code
    storage_code: storage_local     # 存储提供商 code
    
  - code: domain_litefate_com
    domain: litefate.com
    dns_code: dns_aliyun
    issue_code: issue_zerossl
    storage_code: storage_tencent_cos
```

#### dns.yml - DNS 提供商配置

```yaml
dnss:
  - code: dns_cloudflare
    type: cloudflare
    params:
      token: { secret: cf_api_token }
      remark: Cloudflare DNS
      
  - code: dns_aliyun
    type: aliyun
    params:
      access_id: { secret: ali_access_id }
      access_secret: { secret: ali_access_secret }
      remark: 阿里云 DNS
      
  - code: dns_tencent
    type: tencent
    params:
      secret_id: { secret: tencent_secret_id }
      secret_key: { secret: tencent_secret_key }
      remark: 腾讯云 DNS
```

#### issue.yml - 签发提供商配置

```yaml
issues:
  - code: issue_letsencrypt
    type: letsencrypt
    params:
      email: admin@example.com
      
  - code: issue_zerossl
    type: zerossl
    params:
      email: admin@example.com
      eab_kid: { secret: zerossl_kid }
      eab_hmac_key: { secret: zerossl_hmac }
```

#### storage.yml - 存储提供商配置

```yaml
storages:
  - code: storage_local
    type: local
    params:
      path: /data/certs
      
  - code: storage_tencent_cos
    type: tencent_cos
    params:
      secret_id: { secret: tencent_secret_id }
      secret_key: { secret: tencent_secret_key }
      bucket: my-certs-bucket
      region: ap-shanghai
```

### A.5 证书签发流程

```
1. 网关服务请求证书
   │
   ▼
2. 证书服务解析域名 (domain + subdomain)
   │
   ▼
3. 查找本地有效证书
   ├─ 有效且 > 30 天 ──▶ 直接返回
   └─ 无效或即将过期 ──▶ 继续签发
   │
   ▼
4. 获取域名配置
   │
   ▼
5. 获取 DNS 提供商 (用于 DNS-01 验证)
   │
   ▼
6. 获取签发提供商 (Let's Encrypt / ZeroSSL)
   │
   ▼
7. 执行 ACME DNS-01 验证签发
   │
   ▼
8. 保存证书到存储
   │
   ▼
9. 返回证书信息
```

### A.6 与网关服务的集成

网关服务 (infra-gate) 通过 HTTP 调用证书服务获取证书：

```yaml
# infra-gate 配置
ssl:
  mode: remote
  endpoint: http://127.0.0.1:38567
  api_key: { secret: ssl_api_key }
```

网关服务在启动或配置重载时：
1. 对每个需要 HTTPS 的域名调用 `/cert/get-as-json`
2. 证书服务自动处理签发/续期
3. 网关服务使用返回的证书配置 HTTPS

### A.7 部署建议

**容器编排：**

```yaml
# docker-compose.yml 示例
services:
  infra-ssl:
    image: docker.cnb.cool/lite-lake/infra-ssl:dev
    container_name: yo-prod-infra-ssl
    ports:
      - "38567:38567"
    volumes:
      - ./configs:/app/configs:ro
      - ./data/certs:/data/certs
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    networks:
      - yamlops-prod
```

**健康检查：**

```bash
# 检查服务状态
curl http://localhost:38567/health

# 检查证书
curl -H "X-API-Key: your-key" \
  "http://localhost:38567/cert/get-as-json?domain=example.com"
```
