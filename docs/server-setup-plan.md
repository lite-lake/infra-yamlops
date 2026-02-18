# 服务器初始化功能计划报告

## 1. 背景

根据设计文档 (`docs/design.md` 第 6.2 节)，yamlops 应提供完整的服务器环境验证和同步功能。本文档对比现状与设计规划的差距，并提出实现方案。

---

## 2. 设计规划 vs 现状对比

### 2.1 环境验证项目 (env check)

| 检查项 | 设计规划 | 现状 | 差距 |
|--------|----------|------|------|
| SSH 连接 | 验证能否通过 SSH 连接 | ✅ 隐式验证 | 无 |
| Sudo 免密 | 验证用户是否可以免密码执行 sudo | ❌ 未实现 | **缺失** |
| Docker 安装 | 验证 Docker 是否已安装 | ❌ 未实现 | **缺失** |
| Docker Compose | 验证 docker compose 命令可用 | ❌ 未实现 | **缺失** |
| APT 源 | 检查当前使用的 APT 源 | ❌ 未实现 | **缺失** |
| Registry 登录 | 检查是否已登录配置的 Registry | ❌ 未实现 | **缺失** |

### 2.2 环境同步项目 (env sync)

| 同步项 | 设计规划 | 现状 | 差距 |
|--------|----------|------|------|
| APT 源 | 根据配置切换 APT 源 (tuna/aliyun/tencent/official) | ❌ 未实现 | **缺失** |
| Registry 登录 | 登录配置的 Docker 仓库 | ❌ 未实现 | **缺失** |
| Docker 网络 | 创建 yamlops 网络 | ✅ 已实现 | 无 |

### 2.3 数据定义

| 数据项 | 状态 | 位置 |
|--------|------|------|
| `ServerEnvironment.APTSource` | ✅ 已定义 | `internal/domain/entity/server.go:51` |
| `ServerEnvironment.Registries` | ✅ 已定义 | `internal/domain/entity/server.go:52` |
| `Registry` 实体 | ✅ 已定义 | `internal/domain/entity/registry.go` |

---

## 3. 差距评估

### 3.1 总体评估

| 类别 | 完成度 | 说明 |
|------|--------|------|
| 环境验证 | 16% (1/6) | 仅 SSH 连接验证 |
| 环境同步 | 33% (1/3) | 仅 Docker 网络创建 |
| **总体** | **22%** | 差距较大 |

### 3.2 主要问题

1. **env check 功能不完整**
   - 没有验证免密 sudo 权限
   - 没有检查 Docker/Docker Compose 安装状态
   - 没有检查 APT 源配置
   - 没有检查 Registry 登录状态

2. **env sync 功能不完整**
   - APT 源配置逻辑未实现
   - Registry 登录逻辑未实现

3. **缺少独立的服务器设置入口**
   - 用户无法单独执行服务器初始化操作
   - TUI 界面没有服务器设置菜单

---

## 4. 实现方案

### 4.1 方案概述

建议采用渐进式实现：

1. **Phase 1**: 完善现有 env check/sync 命令
2. **Phase 2**: 新增 server setup 独立命令
3. **Phase 3**: TUI 界面集成

### 4.2 Phase 1: 完善环境验证与同步

#### 4.2.1 增强 env check

```
检查流程:
1. SSH 连接测试
2. Sudo 免密验证 (sudo -n true)
3. Docker 安装检查 (docker --version)
4. Docker Compose 检查 (docker compose version)
5. APT 源检查 (cat /etc/apt/sources.list.d/*.list)
6. Registry 登录状态 (docker info | grep Username)
```

**输出示例**:
```
[srv-east-01] Environment Check
  SSH Connection:    ✅ OK
  Sudo Passwordless: ✅ OK
  Docker:            ✅ 24.0.5
  Docker Compose:    ✅ 2.20.2
  APT Source:        ⚠️  current: official, expected: tuna
  Registry:
    - registry-aliyun: ❌ Not logged in
    - registry-cnb:    ✅ Logged in as user@example.com
```

#### 4.2.2 增强 env sync

```
同步流程:
1. 配置 APT 源 (如需要)
2. 登录 Registry (如需要)
3. 创建 Docker 网络
```

**APT 源配置支持**:

| 源代码 | 源名称 | 模板文件 |
|--------|--------|----------|
| tuna | 清华大学 | apt-tuna.list |
| aliyun | 阿里云 | apt-aliyun.list |
| tencent | 腾讯云 | apt-tencent.list |
| official | Ubuntu 官方 | 不修改 |

### 4.3 Phase 2: 新增 server setup 命令

#### 4.3.1 命令设计

```bash
# 完整设置 (验证 + 同步)
yamlops server setup --server srv-east-01

# 仅验证
yamlops server setup --server srv-east-01 --check-only

# 仅同步
yamlops server setup --server srv-east-01 --sync-only

# 交互式选择
yamlops server setup --interactive
```

#### 4.3.2 代码结构

```
internal/
├── server/
│   ├── checker.go       # 环境检查器
│   ├── syncer.go        # 环境同步器
│   └── templates/
│       └── apt/
│           ├── tuna.list
│           ├── aliyun.list
│           └── tencent.list
└── interfaces/cli/
    └── server.go        # server 命令入口
```

### 4.4 Phase 3: TUI 界面集成

#### 4.4.1 新增菜单项

```
┌─────────────────────────────────────────────────────────────┐
│  YAMLOps v1.0.0                              [prod]         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  > Plan & Apply                                             │
│    Infrastructure              网区/网关/服务器/服务        │
│    Global Resources            域名/DNS/SSL                 │
│    Server Setup                服务器环境设置 (NEW)         │
│    Environment Check           服务器环境验证               │
│    Manage Entities             管理实体配置                 │
│    View Status                 查看部署状态                 │
│    Settings                    设置                         │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ↑/↓ Select  Enter Confirm  q Quit                         │
└─────────────────────────────────────────────────────────────┘
```

#### 4.4.2 Server Setup 子菜单

```
┌─────────────────────────────────────────────────────────────┐
│  Server Setup                                               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  > Select Server                                            │
│    Check Environment                                        │
│    Sync Environment                                         │
│    Full Setup (Check + Sync)                                │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  Server: srv-east-01                                        │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. 详细任务清单

### 5.1 核心功能

| 任务 | 优先级 | 预估工时 | 依赖 |
|------|--------|----------|------|
| 实现 Sudo 免密验证 | P0 | 0.5h | - |
| 实现 Docker 安装检查 | P0 | 0.5h | - |
| 实现 Docker Compose 检查 | P0 | 0.5h | - |
| 实现 APT 源检查 | P1 | 1h | - |
| 实现 APT 源配置 | P1 | 2h | - |
| 实现 Registry 登录状态检查 | P1 | 1h | - |
| 实现 Registry 登录操作 | P1 | 1h | - |
| 新增 server setup 命令 | P2 | 2h | 上述功能 |
| TUI Server Setup 菜单 | P2 | 2h | server setup 命令 |

### 5.2 APT 源模板

| 模板 | 说明 | 系统 |
|------|------|------|
| apt-tuna.list | 清华源 | Ubuntu 20.04/22.04 |
| apt-aliyun.list | 阿里云源 | Ubuntu 20.04/22.04 |
| apt-tencent.list | 腾讯云源 | Ubuntu 20.04/22.04 |

### 5.3 错误处理

| 场景 | 处理方式 |
|------|----------|
| SSH 连接失败 | 返回错误，跳过该服务器 |
| Sudo 需要密码 | 警告用户，建议配置 sudoers |
| Docker 未安装 | 警告用户，提供安装命令建议 |
| APT 源配置失败 | 回滚原有配置 |
| Registry 登录失败 | 记录错误，继续处理其他 Registry |

---

## 6. 代码实现计划

### 6.1 新增文件

```
internal/
├── server/
│   ├── checker.go           # Checker 结构体和检查方法
│   ├── syncer.go            # Syncer 结构体和同步方法
│   ├── types.go             # 公共类型定义
│   └── templates/
│       └── apt/
│           ├── tuna.list
│           ├── aliyun.list
│           └── tencent.list
└── interfaces/cli/
    └── server_cmd.go        # server 命令实现
```

### 6.2 修改文件

| 文件 | 修改内容 |
|------|----------|
| `internal/interfaces/cli/env.go` | 重构，调用 server/checker.go |
| `internal/interfaces/cli/tui.go` | 新增 Server Setup 菜单 |
| `internal/ssh/client.go` | 可能需要新增辅助方法 |

### 6.3 Checker 接口设计

```go
package server

type CheckResult struct {
    Name    string
    Status  CheckStatus
    Message string
    Detail  string
}

type CheckStatus int

const (
    CheckStatusOK CheckStatus = iota
    CheckStatusWarning
    CheckStatusError
)

type Checker struct {
    client    *ssh.Client
    server    *entity.Server
    registries []*entity.Registry
    secrets   map[string]string
}

func (c *Checker) CheckAll() []CheckResult
func (c *Checker) CheckSSH() CheckResult
func (c *Checker) CheckSudo() CheckResult
func (c *Checker) CheckDocker() CheckResult
func (c *Checker) CheckDockerCompose() CheckResult
func (c *Checker) CheckAPTSource() CheckResult
func (c *Checker) CheckRegistries() []CheckResult
```

### 6.4 Syncer 接口设计

```go
package server

type SyncResult struct {
    Name    string
    Success bool
    Message string
    Error   error
}

type Syncer struct {
    client    *ssh.Client
    server    *entity.Server
    registries []*entity.Registry
    secrets   map[string]string
}

func (s *Syncer) SyncAll() []SyncResult
func (s *Syncer) SyncAPTSource() SyncResult
func (s *Syncer) SyncRegistries() []SyncResult
func (s *Syncer) SyncDockerNetwork() SyncResult
```

---

## 7. 实施建议

### 7.1 推荐顺序

1. **Week 1**: 实现 Checker (P0 任务)
   - Sudo 免密验证
   - Docker 安装检查
   - Docker Compose 检查

2. **Week 2**: 实现完整 Checker 和 APT 源功能
   - APT 源检查
   - APT 源模板
   - APT 源配置

3. **Week 3**: 实现 Registry 和 Syncer
   - Registry 登录状态检查
   - Registry 登录操作
   - Syncer 完整实现

4. **Week 4**: 命令和 TUI 集成
   - server setup 命令
   - TUI Server Setup 菜单

### 7.2 兼容性考虑

- APT 源模板需要支持 Ubuntu 20.04 和 22.04
- Docker Compose 命令需要同时支持 `docker-compose` (v1) 和 `docker compose` (v2)
- 非 Debian/Ubuntu 系统应跳过 APT 相关检查

---

## 8. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| APT 源配置可能导致系统不可用 | 高 | 配置前备份，失败时回滚 |
| Registry 凭证泄露 | 高 | 使用 SecretRef，不在日志中输出密码 |
| SSH 操作超时 | 中 | 设置合理超时时间，支持重试 |
| 不同 OS 版本兼容性 | 中 | 检测 OS 版本，使用对应模板 |

---

## 9. 总结

当前服务器环境管理功能完成度约 **22%**，主要缺失：
- 完整的环境验证 (5/6 项未实现)
- APT 源配置功能
- Registry 登录功能
- 独立的服务器设置入口

建议按 Phase 1-3 逐步实现，优先完成 P0 级别的核心检查功能，再扩展到完整的设置流程和 TUI 集成。

预估总工时: **约 12 小时**
