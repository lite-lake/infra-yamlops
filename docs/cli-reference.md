# CLI Reference

YAMLOps CLI 命令行工具完整参考文档。

## 全局标志

| 标志 | 短标志 | 默认值 | 描述 |
|------|--------|--------|------|
| `--env` | `-e` | `dev` | 环境名称 (prod/staging/dev/demo) |
| `--config` | `-c` | `.` | 配置目录路径 |

## 命令概览

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
│   ├── config
│   │   ├── list [type]        # 列出配置项
│   │   └── show <type> <name> # 显示配置详情
│   ├── app
│   │   ├── plan               # 应用部署计划
│   │   ├── apply              # 应用部署
│   │   ├── list [resource]    # 列出资源
│   │   └── show <resource> <name>
│   └── service
│       ├── deploy             # 部署服务
│       ├── stop               # 停止服务
│       ├── restart            # 重启服务
│       └── cleanup            # 清理孤儿资源
└── api                        # API 服务（计划中）
```

---

## 核心命令

### yamlops tui

启动交互式终端界面（默认行为）。

```bash
yamlops tui -e prod
yamlops tui --env staging --config /path/to/config
```

**TUI 快捷键：**

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 上移 |
| `↓` / `j` | 下移 |
| `Space` | 切换选择 |
| `Enter` | 确认/展开 |
| `Tab` | 切换视图（App / DNS） |
| `a` / `n` | 选择/取消当前项 |
| `A` / `N` | 全选/全不选 |
| `p` | 生成计划 |
| `r` | 刷新配置 |
| `s` | 同步（在服务器检查视图） |
| `x` | 取消操作 |
| `?` | 显示帮助 |
| `Esc` | 返回 |
| `q` / `Ctrl+C` | 退出 |

---

### yamlops cli plan

生成执行计划，预览将要进行的变更。

```bash
yamlops cli plan -e prod
yamlops cli plan -e prod --server srv-cn1
yamlops cli plan -e staging --zone cn-east
yamlops cli plan -e dev --domain example.com
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain` | 按域名过滤 |
| `--zone` | 按区域过滤 |
| `--server` | 按服务器过滤 |
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

### yamlops cli apply

应用变更到基础设施。

```bash
yamlops cli apply -e prod
yamlops cli apply -e prod --server srv-cn1
yamlops cli apply -e staging --zone cn-east
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain` | 按域名过滤 |
| `--zone` | 按区域过滤 |
| `--server` | 按服务器过滤 |
| `--service` | 按服务过滤 |

**工作流程：**

1. 加载配置并验证
2. 解析密钥引用
3. 生成部署文件
4. 获取远程状态
5. 生成执行计划
6. 显示变更预览
7. 请求确认
8. 执行变更（并行，按服务器分组）
9. 保存状态

---

### yamlops cli validate

验证 YAML 配置文件的有效性。

```bash
yamlops cli validate -e prod
yamlops cli validate -e staging
```

**验证内容：**

- YAML 语法正确性
- 必填字段完整性
- 引用完整性（Zone 引用 ISP、Server 引用 Zone 等）
- 端口冲突检测
- 域名冲突检测
- 格式验证（IP、CIDR、URL 等）

---

### yamlops cli list

列出指定类型的所有实体。

```bash
yamlops cli list secrets -e prod
yamlops cli list isps -e prod
yamlops cli list zones -e prod
yamlops cli list servers -e prod
yamlops cli list services -e prod
yamlops cli list registries -e prod
yamlops cli list domains -e prod
yamlops cli list records -e prod
```

**实体类型：**

| 类型 | 描述 |
|------|------|
| `secrets` | 密钥列表 |
| `isps` | 服务提供商列表 |
| `zones` | 网络区域列表 |
| `servers` | 服务器列表 |
| `services` | 业务服务列表 |
| `registries` | Docker 仓库列表 |
| `domains` | 域名列表 |
| `records` / `dns` | DNS 记录列表 |

---

### yamlops cli show

显示指定实体的详细信息。

```bash
yamlops cli show server srv-cn1 -e prod
yamlops cli show service api-server -e prod
yamlops cli show domain example.com -e prod
yamlops cli show isp aliyun -e prod
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

### yamlops cli clean

清理孤立的 Docker 容器和目录。

```bash
yamlops cli clean -e prod
yamlops cli clean -e staging
```

**清理内容：**

- 不在配置中的 Docker 容器（名称匹配 `yo-{env}-*`）
- 不在配置中的部署目录（`/data/yamlops/yo-{env}-*`）

---

## 环境管理命令

### yamlops cli env check

检查服务器环境状态。

```bash
yamlops cli env check -e prod
yamlops cli env check -e prod --server srv-cn1
yamlops cli env check -e prod --zone cn-east
```

**检查项：**

- SSH 连接
- Sudo 免密
- Docker 安装状态
- Docker Compose 安装状态
- APT 源配置
- Registry 登录状态

---

### yamlops cli env sync

同步服务器环境配置。

```bash
yamlops cli env sync -e prod
yamlops cli env sync -e prod --server srv-cn1
yamlops cli env sync -e prod --zone cn-east
```

**同步内容：**

- APT 源配置
- Docker 网络创建
- Docker Registry 登录

**注意：** 环境同步不会自动安装 Docker，Docker 需预先安装。

---

## DNS 管理命令

### yamlops cli dns plan

生成 DNS 变更计划。

```bash
yamlops cli dns plan -e prod
yamlops cli dns plan -e prod --domain example.com
yamlops cli dns plan -e prod --record www.example.com
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain` | 按域名过滤 |
| `--record` | 按记录过滤（格式：name.domain） |

---

### yamlops cli dns apply

应用 DNS 变更到提供商。

```bash
yamlops cli dns apply -e prod
yamlops cli dns apply -e prod --domain example.com
yamlops cli dns apply -e prod --auto-approve
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--domain` | 按域名过滤 |
| `--record` | 按记录过滤 |
| `--auto-approve` | 跳过确认提示 |

---

### yamlops cli dns list

列出 DNS 资源。

```bash
yamlops cli dns list -e prod
yamlops cli dns list domains -e prod
yamlops cli dns list records -e prod
```

**资源类型：**

- 空 / `all` - 列出所有
- `domains` / `domain` - 仅域名
- `records` / `record` / `dns` - 仅记录

---

### yamlops cli dns show

显示 DNS 资源详情。

```bash
yamlops cli dns show domain example.com -e prod
yamlops cli dns show record www.example.com -e prod
```

---

### yamlops cli dns pull

从提供商拉取 DNS 数据。

```bash
# 拉取域名
yamlops cli dns pull domains --isp aliyun -e prod

# 拉取记录
yamlops cli dns pull records --domain example.com -e prod
```

---

## 服务器管理命令

### yamlops cli server setup

完整设置服务器环境（检查 + 同步）。

```bash
yamlops cli server setup -e prod --server srv-cn1
yamlops cli server setup -e prod --zone cn-east
yamlops cli server setup -e prod --check-only
yamlops cli server setup -e prod --sync-only
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server` | 按服务器过滤 |
| `--zone` | 按区域过滤 |
| `--check-only` | 仅检查，不同步 |
| `--sync-only` | 仅同步，不检查 |

---

### yamlops cli server check

检查服务器状态。

```bash
yamlops cli server check -e prod --server srv-cn1
yamlops cli server check -e prod --zone cn-east
```

---

### yamlops cli server sync

同步服务器配置。

```bash
yamlops cli server sync -e prod --server srv-cn1
yamlops cli server sync -e prod --zone cn-east
```

---

## 配置管理命令

### yamlops cli config list

列出配置项。

```bash
yamlops cli config list -e prod
yamlops cli config list secrets -e prod
yamlops cli config list isps -e prod
yamlops cli config list registries -e prod
```

**类型：**

- 空 - 列出所有
- `secrets` - 密钥
- `isps` - 服务提供商
- `registries` - Docker 仓库

---

### yamlops cli config show

显示配置详情。

```bash
yamlops cli config show secret db_password -e prod
yamlops cli config show isp aliyun -e prod
yamlops cli config show registry dockerhub -e prod
```

---

## 应用管理命令

### yamlops cli app plan

生成应用部署计划。

```bash
yamlops cli app plan -e prod
yamlops cli app plan -e prod --server srv-cn1
yamlops cli app plan -e prod --zone cn-east
yamlops cli app plan -e prod --infra gateway-cn1
yamlops cli app plan -e prod --biz api-server
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--zone` | 按区域过滤 |
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |

---

### yamlops cli app apply

应用部署。

```bash
yamlops cli app apply -e prod
yamlops cli app apply -e prod --server srv-cn1 --auto-approve
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--zone` | 按区域过滤 |
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |
| `--auto-approve` | 自动确认 |

---

### yamlops cli app list

列出应用资源。

```bash
yamlops cli app list -e prod
yamlops cli app list zones -e prod
yamlops cli app list servers -e prod
yamlops cli app list infra -e prod
yamlops cli app list biz -e prod
```

---

### yamlops cli app show

显示应用资源详情。

```bash
yamlops cli app show server srv-cn1 -e prod
yamlops cli app show infra gateway-cn1 -e prod
yamlops cli app show biz api-server -e prod
```

---

## 服务管理命令

### yamlops cli service deploy

部署服务。如果服务已存在，将同步最新配置并重新创建容器。

```bash
# 部署所有服务
yamlops cli service deploy -e prod

# 部署指定服务器的服务
yamlops cli service deploy -e prod --server srv-cn1

# 部署指定的业务服务
yamlops cli service deploy -e prod --biz api-server

# 部署指定的基础设施服务
yamlops cli service deploy -e prod --infra gateway-cn1

# 强制部署（即使无变更）
yamlops cli service deploy -e prod --biz api-server --force

# 跳过确认直接部署
yamlops cli service deploy -e prod --yes
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |
| `--force` | 强制部署（即使无变更或使用 -b/-i 过滤时） |
| `--yes` | 跳过确认提示 |

**部署行为：**

- 新服务：创建目录、同步文件、拉取镜像、启动容器
- 已有服务：同步最新文件、拉取镜像、重新创建容器（`up -d --pull=always`）

---

### yamlops cli service stop

停止运行中的服务。数据将被保留。

```bash
# 停止所有服务
yamlops cli service stop -e prod

# 停止指定服务器的服务
yamlops cli service stop -e prod --server srv-cn1

# 停止指定的业务服务
yamlops cli service stop -e prod --biz api-server

# 跳过确认直接停止
yamlops cli service stop -e prod --yes
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |
| `--yes` | 跳过确认提示 |

**注意：** 此命令仅停止容器，不会删除容器或数据。

---

### yamlops cli service restart

重启服务。不拉取镜像、不同步文件，仅执行容器重启。

```bash
# 重启所有服务
yamlops cli service restart -e prod

# 重启指定服务器的服务
yamlops cli service restart -e prod --server srv-cn1

# 重启指定的业务服务
yamlops cli service restart -e prod --biz api-server

# 跳过确认直接重启
yamlops cli service restart -e prod --yes
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |
| `--yes` | 跳过确认提示 |

**注意：** 此命令仅执行 `docker compose restart`，不更新任何文件或镜像。

---

### yamlops cli service cleanup

扫描并清理孤儿资源（不在配置中的容器和目录）。

```bash
# 扫描并清理孤儿资源
yamlops cli service cleanup -e prod

# 清理指定服务器上的孤儿资源
yamlops cli service cleanup -e prod --server srv-cn1

# 跳过确认直接清理
yamlops cli service cleanup -e prod --yes
```

**标志：**

| 标志 | 描述 |
|------|------|
| `--server` | 按服务器过滤 |
| `--infra` | 按基础设施服务过滤 |
| `--biz` | 按业务服务过滤 |
| `--yes` | 跳过确认提示 |

**清理内容：**

- 孤儿容器：名称匹配 `yo-{env}-*` 但不在配置中的容器
- 孤儿目录：`/data/yamlops/yo-{env}-*` 但不在配置中的目录

**警告：** 此操作不可逆，请谨慎使用。

---

## 常用工作流

### 标准部署流程

```bash
# 1. 验证配置
yamlops cli validate -e prod

# 2. 生成计划
yamlops cli plan -e prod

# 3. 应用变更
yamlops cli apply -e prod
```

### 服务器初始化

```bash
# 完整设置
yamlops cli server setup -e prod --server srv-cn1

# 或分步执行
yamlops cli server check -e prod --server srv-cn1
yamlops cli server sync -e prod --server srv-cn1
```

### DNS 管理

```bash
# 拉取现有记录
yamlops cli dns pull records --domain example.com -e prod

# 生成变更计划
yamlops cli dns plan -e prod --domain example.com

# 应用变更
yamlops cli dns apply -e prod --domain example.com
```

### 单服务更新

```bash
# 查看服务详情
yamlops cli show service api-server -e prod

# 生成该服务的计划
yamlops cli app plan -e prod --biz api-server

# 应用更新
yamlops cli app apply -e prod --biz api-server
```

### 服务运维操作

```bash
# 停止服务（保留数据）
yamlops cli service stop -e prod --biz api-server

# 重启服务（不更新文件和镜像）
yamlops cli service restart -e prod --biz api-server

# 重新部署服务（同步最新配置）
yamlops cli service deploy -e prod --biz api-server

# 清理孤儿资源
yamlops cli service cleanup -e prod
```

### 清理孤立资源

```bash
# 扫描并清理
yamlops cli clean -e prod

# 或使用 service cleanup
yamlops cli service cleanup -e prod
```
