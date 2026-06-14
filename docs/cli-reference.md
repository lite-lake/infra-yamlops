# CLI Reference

YAMLOps CLI 命令行工具完整参考文档。

## 全局标志

| 标志 | 短标志 | 默认值 | 描述 |
|------|--------|--------|------|
| `--env` | `-e` | 必填 | 环境名称 (prod/staging/dev/demo) |
| `--config` | `-c` | `.` | 配置目录路径 |
| `--concurrency` | - | `5` | 服务器操作并发数 |

> **注意**：`-e` 为必填参数（`--help` 除外）。

## 命令概览

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

---

## 统一执行模式

所有变更命令（`service deploy/stop/restart/cleanup`、`server setup`、`dns deploy/pull`）遵循统一的三阶段执行模式：

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│   Plan   │ ──→ │ Confirm  │ ──→ │ Execute  │
│ (计划)    │     │ (确认)    │     │ (执行)    │
└──────────┘     └──────────┘     └──────────┘
  ↑ 干燥运行                     ↑ 跳过确认(--yes)
  (--dry-run)
```

**通用标志**：

| 标志 | 说明 | 适用命令 |
|------|------|---------|
| `--dry-run` | 只执行 Plan 阶段，显示变更计划但不执行 | 所有变更命令 |
| `--yes` | 跳过 Confirm 阶段，直接执行所有变更 | 所有变更命令 |
| `--force` | 即使配置无变更也生成部署计划 | service deploy / dns deploy / dns pull |

**输出格式**：

```
PLAN: <command> [(dry-run|forced)]
ENV:  <env>
TYPE: <type>                    # 仅 service 命令

ACTION   NAME           SERVER     DETAILS
create   api-server     srv-cn1    image: api:latest
update   web-server     srv-cn1    image: v2.0 -> v2.1
delete   old-worker     srv-cn2    -

SUMMARY: 1 created, 1 updated, 1 deleted
```

---

## 核心命令

### yamlops tui

启动交互式终端界面。

```bash
yamlops tui -e prod
yamlops tui -e prod --concurrency 10
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

---

## 配置管理命令

### yamlops cli config show

显示配置详情。

```bash
# 显示 ISP 列表
yamlops cli config show isps -e prod
yamlops cli config show isps -e prod --detail

# 显示 Registry 列表
yamlops cli config show registries -e prod
yamlops cli config show registries -e prod --detail

# 显示 Secret 列表
yamlops cli config show secrets -e prod
yamlops cli config show secrets -e prod --detail

# 显示特定 ISP 详情
yamlops cli config show isps aliyun -e prod
```

**参数**：

| 参数 | 说明 |
|------|------|
| `<entity>` | 实体类型：`isps` / `registries` / `secrets` |
| `[name]` | 可选，实体名称（无则列出全部） |
| `--detail` | 显示详细信息 |

**输出示例**：

```
# config show isps -e prod
ISP           TYPE        SERVICES
aliyun        aliyun      dns
cloudflare    cloudflare  dns

Total: 2 ISPs
```

**安全约束**：
- `config show secrets` 只显示 key 列表，不显示 value
- `config show isps` 不显示 API Key/Secret 等凭证

---

### yamlops cli config validate

验证 ISP/Registry/Secret 配置完整性。

```bash
yamlops cli config validate -e prod
```

---

## DNS 管理命令

### yamlops cli dns show

列出域名和 DNS 记录。

```bash
yamlops cli dns show -e prod
yamlops cli dns show -e prod --detail
yamlops cli dns show -e prod --domain example.com
yamlops cli dns show -e prod --domain example.com --record A:@
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--domain` | 域名筛选（逗号分隔多选） |
| `--record` | 记录筛选，格式 `TYPE:NAME`（逗号分隔） |
| `--detail` | 显示详细记录（类型、值、TTL） |

**输出示例**：

```
# dns show -e prod
DOMAIN            ISP          RECORDS
example.com       aliyun       3 records
api.example.com   cloudflare   2 records

Total: 2 domains, 5 records

# dns show -e prod --detail
DOMAIN            ISP          RECORDS
example.com       aliyun       3 records

DOMAIN: example.com
  ISP: aliyun
  Records:
    TYPE    NAME    VALUE               TTL
    A       @       1.2.3.4             600
    A       www     1.2.3.4             600
    CNAME   api     api.example.com     300

Total: 2 domains, 5 records
```

---

### yamlops cli dns validate

验证 DNS 配置。

```bash
yamlops cli dns validate -e prod
```

---

### yamlops cli dns deploy

部署 DNS 变更（统一执行模式）。

```bash
# 预览变更
yamlops cli dns deploy -e prod --dry-run

# 按域名筛选
yamlops cli dns deploy -e prod --domain example.com --dry-run

# 应用变更
yamlops cli dns deploy -e prod --domain example.com

# 跳过确认
yamlops cli dns deploy -e prod --domain example.com --yes

# 强制执行（即使无变更）
yamlops cli dns deploy -e prod --domain example.com --force
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--domain` | 域名筛选（逗号分隔多选） |
| `--record` | 记录筛选，格式 `TYPE:NAME`（逗号分隔） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--force` | 强制执行 |

---

### yamlops cli dns pull

从 ISP 拉取 DNS 数据（统一执行模式）。

#### pull domains

```bash
# 预览拉取结果
yamlops cli dns pull domains -e prod --isp aliyun --dry-run

# 拉取域名
yamlops cli dns pull domains -e prod --isp aliyun

# 跳过确认
yamlops cli dns pull domains -e prod --isp aliyun --yes

# 强制覆盖本地数据
yamlops cli dns pull domains -e prod --isp aliyun --force
```

#### pull records

```bash
# 预览拉取结果
yamlops cli dns pull records -e prod --domain example.com --dry-run

# 拉取记录
yamlops cli dns pull records -e prod --domain example.com

# 按 ISP 筛选拉取
yamlops cli dns pull records -e prod --isp aliyun --dry-run

# 跳过确认
yamlops cli dns pull records -e prod --domain example.com --yes
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--isp` | ISP 名称筛选（`pull domains` 必填；`pull records` 可选） |
| `--domain` | 域名筛选（`pull records` 可选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--force` | 强制覆盖本地数据 |

---

## 服务器管理命令

### yamlops cli server show

列出服务器。

```bash
yamlops cli server show -e prod
yamlops cli server show -e prod --detail
yamlops cli server show -e prod --zone cn-east
yamlops cli server show -e prod --server srv-cn1
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--detail` | 显示详细信息（IP、SSH、Provider、Networks） |

**输出示例**：

```
# server show -e prod
ZONE      SERVER
cn-east   srv-cn1
cn-east   srv-cn2
cn-west   srv-cn3

Total: 3 servers in 2 zones

# server show -e prod --detail
ZONE      SERVER
cn-east   srv-cn1

SERVER: srv-cn1
  Public IP:   192.168.1.10
  Private IP:  10.0.1.10
  SSH:         root@192.168.1.10:22
  Provider:    aliyun
  Networks:    public_net (bridge), internal_net (overlay)

Total: 3 servers in 2 zones
```

---

### yamlops cli server validate

验证服务器配置。

```bash
yamlops cli server validate -e prod
```

---

### yamlops cli server setup

设置服务器环境（统一执行模式）。

```bash
# 预览环境差异
yamlops cli server setup -e prod --dry-run

# 按服务器筛选
yamlops cli server setup -e prod --server srv-cn1 --dry-run

# 同步环境（交互确认）
yamlops cli server setup -e prod

# 跳过确认
yamlops cli server setup -e prod --yes
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |

---

## 服务管理命令

### yamlops cli service show

列出服务。

```bash
yamlops cli service show -e prod
yamlops cli service show -e prod --type biz
yamlops cli service show -e prod --type infra
yamlops cli service show -e prod --detail
yamlops cli service show -e prod --type biz --detail
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--detail` | 显示详细信息（Image、Ports、Endpoints、Gateways、Health） |

**`--type` 参数说明**：

| 值 | 含义 |
|----|------|
| `biz` | 仅业务服务（BizService） |
| `infra` | 仅基础设施服务（InfraService） |
| `biz,infra` | 全部服务（与不传 `--type` 等效） |

**输出示例**：

```
# service show -e prod --type biz
ZONE      SERVER    SERVICE
cn-east   srv-cn1   api-server
cn-east   srv-cn1   web-server
cn-east   srv-cn2   worker
cn-west   srv-cn3   scheduler

Total: 4 services across 3 servers in 2 zones

# service show -e prod --type biz --detail
ZONE      SERVER    SERVICE       IMAGE
cn-east   srv-cn1   api-server    docker.../api:latest
cn-east   srv-cn1   web-server    docker.../web:latest
cn-east   srv-cn2   worker        docker.../worker:latest
cn-west   srv-cn3   scheduler     docker.../scheduler:latest

SERVICE: api-server
  Image:      docker.cnb.cool/litelake/api:latest
  Ports:      8080:8080/tcp, 8443:8443/tcp
  Endpoints:  http://api.demo.litelake.cn
              https://api.demo.litelake.cn
  Gateways:   api.demo.litelake.cn (http+https)
  Health:     /health (interval: 30s)

Total: 4 services across 3 servers in 2 zones
```

---

### yamlops cli service validate

验证服务配置。

```bash
yamlops cli service validate -e prod
yamlops cli service validate -e prod --type biz
```

**验证内容**：
- 实体自验证（name、server、image、ports、healthcheck 等）
- 跨实体引用（server 存在性、secrets 存在性）
- 冲突检测（端口冲突、gateway hostname 重复）
- 唯一性约束（BizService/InfraService 不允许同名）

---

### yamlops cli service deploy

部署服务（统一执行模式）。

```bash
# 预览变更
yamlops cli service deploy -e prod --dry-run

# 按类别筛选
yamlops cli service deploy -e prod --type biz --dry-run

# 按网区/服务器筛选
yamlops cli service deploy -e prod --zone cn-east --dry-run

# 应用变更（交互确认）
yamlops cli service deploy -e prod --type biz

# 跳过确认
yamlops cli service deploy -e prod --type biz --yes

# 强制部署（即使无变更）
yamlops cli service deploy -e prod --type biz --force

# 组合：强制预览
yamlops cli service deploy -e prod --type biz --force --dry-run
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--force` | 强制部署（即使配置无变更） |
| `--concurrency` | 并发数（默认 5） |

---

### yamlops cli service stop

停止服务（统一执行模式）。

```bash
# 预览将停止的服务
yamlops cli service stop -e prod --type biz --dry-run

# 停止服务（交互确认）
yamlops cli service stop -e prod --type biz

# 跳过确认
yamlops cli service stop -e prod --type biz --yes
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--concurrency` | 并发数（默认 5） |

---

### yamlops cli service restart

重启服务（统一执行模式）。

```bash
# 预览将重启的服务
yamlops cli service restart -e prod --type biz --dry-run

# 重启服务（交互确认）
yamlops cli service restart -e prod --type biz

# 跳过确认
yamlops cli service restart -e prod --type biz --yes
```

**标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--concurrency` | 并发数（默认 5） |

---

### yamlops cli service cleanup

清理孤儿资源（统一执行模式）。

```bash
# 预览将清理的资源
yamlops cli service cleanup -e prod --dry-run

# 清理资源（交互确认）
yamlops cli service cleanup -e prod

# 跳过确认
yamlops cli service cleanup -e prod --yes
```

**清理内容**：
- 孤儿容器：名称匹配 `yo-{env}-*` 但不在配置中的容器
- 孤儿目录：`/data/yamlops/yo-{env}-*` 但不在配置中的目录

**标志**：

| 标志 | 说明 |
|------|------|
| `--type` | 服务类别筛选：`biz` / `infra` / `biz,infra`（默认全部） |
| `--zone` | 网区筛选（逗号分隔多选） |
| `--server` | 服务器筛选（逗号分隔多选） |
| `--dry-run` | 预览变更不执行 |
| `--yes` | 跳过确认 |
| `--concurrency` | 并发数（默认 5） |

---

## 常用工作流

### 标准部署流程

```bash
# 1. 验证配置
yamlops cli service validate -e prod

# 2. 预览变更
yamlops cli service deploy -e prod --dry-run

# 3. 应用变更
yamlops cli service deploy -e prod --yes
```

### 服务器初始化

```bash
# 预览环境差异
yamlops cli server setup -e prod --server srv-cn1 --dry-run

# 同步环境
yamlops cli server setup -e prod --server srv-cn1 --yes
```

### DNS 管理

```bash
# 拉取现有记录
yamlops cli dns pull records -e prod --domain example.com --yes

# 预览 DNS 变更
yamlops cli dns deploy -e prod --domain example.com --dry-run

# 应用变更
yamlops cli dns deploy -e prod --domain example.com --yes
```

### 单服务更新

```bash
# 查看服务详情
yamlops cli service show -e prod --type biz --detail

# 预览变更
yamlops cli service deploy -e prod --type biz --dry-run

# 应用更新
yamlops cli service deploy -e prod --type biz --yes
```

### 服务运维操作

```bash
# 停止服务
yamlops cli service stop -e prod --type biz --yes

# 重启服务
yamlops cli service restart -e prod --type biz --yes

# 重新部署服务
yamlops cli service deploy -e prod --type biz --yes

# 清理孤儿资源
yamlops cli service cleanup -e prod --yes
```

---

## 退出码

| 退出码 | 含义 | 场景 |
|--------|------|------|
| 0 | 成功 | 所有操作成功完成、验证通过、dry-run 完成 |
| 1 | 一般错误 | 参数错误、配置错误、环境不存在 |
| 2 | 验证失败 | 配置验证失败、SSH 连接失败 |
| 3 | 执行失败 | Docker 命令失败、部署失败、清理失败 |

---

## 错误处理

**错误消息格式**：

```
Error: {简短描述}
Details: {详细信息}
Suggestion: {修复建议}
```

**常见错误场景**：

| 错误场景 | Error | Suggestion |
|---------|-------|------------|
| `-e` 参数缺失 | `Environment flag is required` | `Use -e <env> to specify environment` |
| 服务名称不存在 | `Service 'xxx' not found` | `Run 'service show' to list available services` |
| ISP 名称不存在 | `ISP 'xxx' not found` | `Run 'dns pull domains' to list available ISPs` |
| ISP 不支持 DNS | `ISP 'xxx' does not support DNS service` | `Check ISP configuration in isps.yaml` |
| 配置验证失败 | `Configuration validation failed` | `Run 'validate' to see detailed errors` |
| SSH 连接失败 | `SSH connection to server 'xxx' failed` | `Check server SSH configuration` |
| Docker 执行失败 | `Docker command failed on server 'xxx'` | `Check Docker daemon status on the server` |

---

## 迁移指南

旧命令到新命令的迁移对照表，请参考 [迁移指南](MIGRATION.md)。
