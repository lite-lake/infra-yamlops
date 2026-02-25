# CLI Reference

YAMLOps CLI 命令行工具完整参考文档。

## 全局标志

| 标志 | 短标志 | 默认值 | 描述 |
|------|--------|--------|------|
| `--env` | `-e` | `dev` | 环境名称 (prod/staging/dev/demo) |
| `--config` | `-c` | `.` | 配置目录路径 |
| `--version` | `-v` | `false` | 显示版本信息 |

## 命令概览

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

---

## 核心命令

### yamlops (TUI)

启动交互式终端界面（默认行为）。

```bash
yamlops -e prod
yamlops --env staging --config /path/to/config
```

**TUI 快捷键：**

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 上移 |
| `↓` / `j` | 下移 |
| `Space` | 切换选择 |
| `Enter` | 确认/展开 |
| `Tab` | 切换视图 |
| `a` / `n` | 选择/取消当前项 |
| `A` / `N` | 全选/全不选 |
| `p` | 生成计划 |
| `r` | 刷新配置 |
| `s` | 同步（在服务器检查视图） |
| `x` | 取消操作 |
| `Esc` | 返回 |
| `q` / `Ctrl+C` | 退出 |

---

### yamlops plan

生成执行计划，预览将要进行的变更。

```bash
yamlops plan -e prod
yamlops plan -e prod --server srv-cn1
yamlops plan -e staging --zone cn-east
yamlops plan -e dev --domain example.com
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain`, `-d` | 按域名过滤 |
| `--zone`, `-z` | 按区域过滤 |
| `--server`, `-s` | 按服务器过滤 |
| `--service` | 按服务过滤 |

**输出示例：**

```
Execution Plan:
===============
+ server: srv-cn1
    - Create server environment
~ service: api-server
    - Update docker compose configuration
- service: old-service
    - Remove service
```

---

### yamlops apply

应用变更到基础设施。

```bash
yamlops apply -e prod
yamlops apply -e prod --server srv-cn1
yamlops apply -e staging --zone cn-east
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain`, `-d` | 按域名过滤 |
| `--zone`, `-z` | 按区域过滤 |
| `--server`, `-s` | 按服务器过滤 |
| `--service` | 按服务过滤 |

**工作流程：**

1. 加载配置并验证
2. 生成执行计划
3. 显示变更预览
4. 请求确认
5. 生成部署文件
6. 执行变更
7. 保存状态

---

### yamlops validate

验证 YAML 配置文件的有效性。

```bash
yamlops validate -e prod
yamlops validate -e staging
```

**验证内容：**

- YAML 语法正确性
- 必填字段完整性
- 引用完整性（Zone 引用 ISP、Server 引用 Zone 等）
- 端口冲突检测
- 域名冲突检测
- 格式验证（IP、CIDR、URL 等）

---

### yamlops list

列出指定类型的所有实体。

```bash
yamlops list secrets -e prod
yamlops list isps -e prod
yamlops list zones -e prod
yamlops list servers -e prod
yamlops list services -e prod
yamlops list registries -e prod
yamlops list domains -e prod
yamlops list records -e prod
```

**实体类型：**

| 类型 | 描述 |
|------|------|
| `secrets` | 密钥列表 |
| `isps` | 服务提供商列表 |
| `zones` | 网络区域列表 |
| `servers` | 服务器列表 |
| `services` | 业务服务列表 |
| `infra_services` | 基础设施服务列表 |
| `registries` | Docker 仓库列表 |
| `domains` | 域名列表 |
| `records` / `dns` | DNS 记录列表 |

---

### yamlops show

显示指定实体的详细信息。

```bash
yamlops show server srv-cn1 -e prod
yamlops show service api-server -e prod
yamlops show domain example.com -e prod
yamlops show isp aliyun -e prod
```

**实体类型：**

- `secret` - 密钥
- `isp` - 服务提供商
- `zone` - 网络区域
- `infra_service` - 基础设施服务
- `server` - 服务器
- `service` - 业务服务
- `registry` - Docker 仓库
- `domain` - 域名

---

### yamlops clean

清理孤立的 Docker 容器和目录。

```bash
yamlops clean -e prod
yamlops clean -e staging
```

**清理内容：**

- 不在配置中的 Docker 容器（名称匹配 `yo-{env}-*`）
- 不在配置中的部署目录（`/data/yamlops/yo-{env}-*`）

---

## 环境管理命令

### yamlops env check

检查服务器环境状态。

```bash
yamlops env check -e prod
yamlops env check -e prod --server srv-cn1
yamlops env check -e prod --zone cn-east
```

**检查项：**

- Docker 安装状态
- Docker Compose 安装状态
- APT 源配置
- Registry 登录状态

---

### yamlops env sync

同步服务器环境配置。

```bash
yamlops env sync -e prod
yamlops env sync -e prod --server srv-cn1
yamlops env sync -e prod --zone cn-east
```

**同步内容：**

- Docker 安装（如未安装）
- Docker Compose 安装（如未安装）
- APT 源配置
- Docker Registry 登录

---

## DNS 管理命令

### yamlops dns plan

生成 DNS 变更计划。

```bash
yamlops dns plan -e prod
yamlops dns plan -e prod --domain example.com
yamlops dns plan -e prod --record www.example.com
```

**标志：**

| 标志 | 短标志 | 描述 |
|------|--------|------|
| `--domain` | `-d` | 按域名过滤 |
| `--record` | `-r` | 按记录过滤（格式：name.domain） |

---

### yamlops dns apply

应用 DNS 变更到提供商。

```bash
yamlops dns apply -e prod
yamlops dns apply -e prod --domain example.com
yamlops dns apply -e prod --auto-approve
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain`, `-d` | 按域名过滤 |
| `--record`, `-r` | 按记录过滤 |
| `--auto-approve` | 跳过确认提示 |

---

### yamlops dns list

列出 DNS 资源。

```bash
yamlops dns list -e prod
yamlops dns list domains -e prod
yamlops dns list records -e prod
```

**资源类型：**

- 空 / `all` - 列出所有
- `domains` / `domain` - 仅域名
- `records` / `record` / `dns` - 仅记录

---

### yamlops dns show

显示 DNS 资源详情。

```bash
yamlops dns show domain example.com -e prod
yamlops dns show record www.example.com -e prod
```

---

### yamlops dns pull

从提供商拉取 DNS 数据。

```bash
# 拉取域名
yamlops dns pull domains --isp aliyun -e prod

# 拉取记录
yamlops dns pull records --domain example.com -e prod
```

---

## 服务器管理命令

### yamlops server setup

完整设置服务器环境（检查 + 同步）。

```bash
yamlops server setup -e prod --server srv-cn1
yamlops server setup -e prod --zone cn-east
yamlops server setup -e prod --check-only
yamlops server setup -e prod --sync-only
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server`, `-s` | 按服务器过滤 |
| `--zone`, `-z` | 按区域过滤 |
| `--check-only` | 仅检查，不同步 |
| `--sync-only` | 仅同步，不检查 |

---

### yamlops server check

检查服务器状态。

```bash
yamlops server check -e prod --server srv-cn1
yamlops server check -e prod --zone cn-east
```

---

### yamlops server sync

同步服务器配置。

```bash
yamlops server sync -e prod --server srv-cn1
yamlops server sync -e prod --zone cn-east
```

---

## 配置管理命令

### yamlops config list

列出配置项。

```bash
yamlops config list -e prod
yamlops config list secrets -e prod
yamlops config list isps -e prod
yamlops config list registries -e prod
```

**类型：**

- 空 - 列出所有
- `secrets` - 密钥
- `isps` - 服务提供商
- `registries` - Docker 仓库

---

### yamlops config show

显示配置详情。

```bash
yamlops config show secret db_password -e prod
yamlops config show isp aliyun -e prod
yamlops config show registry dockerhub -e prod
```

---

## 应用管理命令

### yamlops app plan

生成应用部署计划。

```bash
yamlops app plan -e prod
yamlops app plan -e prod --server srv-cn1
yamlops app plan -e prod --zone cn-east
yamlops app plan -e prod --infra gateway-cn1
yamlops app plan -e prod --biz api-server
```

**标志：**

| 标志 | 短标志 | 描述 |
|------|--------|------|
| `--zone` | `-z` | 按区域过滤 |
| `--server` | `-s` | 按服务器过滤 |
| `--infra` | `-i` | 按基础设施服务过滤 |
| `--biz` | `-b` | 按业务服务过滤 |

---

### yamlops app apply

应用部署。

```bash
yamlops app apply -e prod
yamlops app apply -e prod --server srv-cn1 --auto-approve
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--zone`, `-z` | 按区域过滤 |
| `--server`, `-s` | 按服务器过滤 |
| `--infra`, `-i` | 按基础设施服务过滤 |
| `--biz`, `-b` | 按业务服务过滤 |
| `--auto-approve` | 自动确认 |

---

### yamlops app list

列出应用资源。

```bash
yamlops app list -e prod
yamlops app list zones -e prod
yamlops app list servers -e prod
yamlops app list infra -e prod
yamlops app list biz -e prod
```

---

### yamlops app show

显示应用资源详情。

```bash
yamlops app show server srv-cn1 -e prod
yamlops app show infra gateway-cn1 -e prod
yamlops app show biz api-server -e prod
```

---

## 常用工作流

### 标准部署流程

```bash
# 1. 验证配置
yamlops validate -e prod

# 2. 生成计划
yamlops plan -e prod

# 3. 应用变更
yamlops apply -e prod
```

### 服务器初始化

```bash
# 完整设置
yamlops server setup -e prod --server srv-cn1

# 或分步执行
yamlops server check -e prod --server srv-cn1
yamlops server sync -e prod --server srv-cn1
```

### DNS 管理

```bash
# 拉取现有记录
yamlops dns pull records --domain example.com -e prod

# 生成变更计划
yamlops dns plan -e prod --domain example.com

# 应用变更
yamlops dns apply -e prod --domain example.com
```

### 单服务更新

```bash
# 查看服务详情
yamlops show service api-server -e prod

# 生成该服务的计划
yamlops app plan -e prod --biz api-server

# 应用更新
yamlops app apply -e prod --biz api-server
```

### 清理孤立资源

```bash
# 扫描并清理
yamlops clean -e prod
```
