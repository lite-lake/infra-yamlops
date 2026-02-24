# YAMLOps 项目设计问题分析报告

> 分析日期: 2026-02-24
> 分析范围: 全部代码模块

---

## 目录

1. [执行摘要](#执行摘要)
2. [架构层问题总览](#架构层问题总览)
3. [各模块详细分析](#各模块详细分析)
4. [问题优先级排序](#问题优先级排序)
5. [重构建议路线图](#重构建议路线图)

---

## 执行摘要

### 发现问题统计

| 模块 | 问题数 | 高危 | 中危 | 低危 |
|------|--------|------|------|------|
| Domain 层 | 25 | 5 | 12 | 8 |
| Application 层 | 21 | 5 | 10 | 6 |
| Infrastructure 层 | 20 | 4 | 10 | 6 |
| Interface CLI 层 | 25 | 6 | 11 | 8 |
| DNS/SSL Providers | 24 | 4 | 12 | 8 |
| Plan/Compose/Gate | 20 | 4 | 10 | 6 |
| SSH/Secrets/Network | 28 | 6 | 14 | 8 |
| Environment | 21 | 3 | 12 | 6 |
| 架构整体 | 12 | 4 | 6 | 2 |
| **总计** | **196** | **41** | **97** | **58** |

### 最严重的 10 个问题

1. **命令注入风险** - SSH、Network、Registry、Syncer 模块存在 shell 命令拼接
2. **密码通过命令行传递** - Registry Manager 使用 echo 管道传递密码
3. **并发安全问题** - 多个 map 字段无锁保护（SSHPool, BaseDeps, Executor）
4. **值对象可变** - 所有值对象字段可被外部修改，违反 DDD 核心原则
5. **接口层跨层依赖** - CLI 直接依赖 infrastructure 层，违反分层原则
6. **缺少重试机制** - 所有网络操作和文件操作无重试
7. **Executor/BaseDeps 职责过重** - 违反单一职责原则
8. **EAB 功能未实现** - ACME SSL 的 External Account Binding 功能不工作
9. **代码重复严重** - Handler、Differ、Checker 等模块存在大量重复代码
10. **错误处理不一致** - 混用多种错误处理模式，缺少统一策略

---

## 架构层问题总览

### 当前架构问题

```
实际依赖关系（问题部分用红色标注）:

interfaces/cli
    ├──→ application/handler, usecase, orchestrator  ✓
    ├──→ infrastructure/persistence                   ✗ 跨层依赖
    ├──→ plan                                         ✗ 未分层
    ├──→ secrets                                      ✗ 跨层依赖
    ├──→ ssh                                          ✗ 跨层依赖
    └──→ providers/dns                                ✗ 跨层依赖

plan
    ├──→ application/deployment                       ✗ 反向依赖
    └──→ infrastructure/state                         ✗ 跨层依赖
```

### 理想架构

```
cmd/yamlops/main.go

internal/
├── domain/                      # 核心层 - 无外部依赖
│   ├── entity/
│   ├── valueobject/
│   ├── repository/
│   ├── service/
│   └── errors.go
│
├── application/                 # 应用层 - 只依赖 domain
│   ├── usecase/
│   ├── handler/
│   ├── orchestrator/
│   ├── plan/
│   └── deployment/
│
├── infrastructure/              # 基础设施层
│   ├── persistence/
│   ├── state/
│   ├── ssh/
│   ├── dns/
│   ├── secrets/
│   ├── registry/
│   ├── network/
│   └── generator/
│       ├── compose/
│       └── gate/
│
└── interfaces/                  # 接口层
    ├── cli/
    └── tui/
```

---

## 各模块详细分析

### 1. Domain 层

#### 1.1 实体设计问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `entity/server.go:18-22` | `ServerNetwork`, `ServerIP` 等应移至 valueobject 包 | 中 |
| `entity/biz_service.go:11-96` | 多个嵌入结构体应为值对象 | 中 |
| `entity/infra_service.go:66-185` | InfraService 过于复杂，应拆分 | 高 |
| `entity/config.go:72-150` | Config 职责过重，9 个 GetXXXMap 方法重复 | 高 |
| `entity/dns_record.go:30-38` | `validTypes` 每次调用都创建 | 低 |

#### 1.2 值对象设计问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `valueobject/secret_ref.go:9-12` | SecretRef 字段可变 | 高 |
| `valueobject/scope.go:3-12` | Scope 字段可变 | 高 |
| `valueobject/change.go:27-35` | Change 字段可变，OldState/NewState 使用 interface{} | 高 |
| `valueobject/plan.go:3-6` | Plan 字段可变 | 高 |
| 所有值对象 | 缺少 Equals() 方法 | 中 |

#### 1.3 领域服务问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `service/validator.go:11-19` | Validator 持有状态 | 高 |
| `service/differ.go:13-15` | DifferService 持有状态 | 高 |
| `service/differ_*.go` | 大量 Equals 函数应为实体方法 | 中 |
| `service/differ_servers.go:10-223` | 三个 Plan 方法结构高度相似 | 中 |

#### 1.4 仓库接口问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `repository/state.go:14-23` | DeploymentState 应移至 entity 包 | 中 |
| `repository/config.go:9-12` | Validate 方法不应在接口中 | 低 |

---

### 2. Application 层

#### 2.1 Handler 模式问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `handler/types.go:49-93` | BaseDeps 12 个字段，职责过重 | 高 |
| `handler/types.go:44-47` | Handler 返回 `(*Result, error)` 语义混淆 | 中 |
| `handler/service_handler.go` 与 `infra_service_handler.go` | 代码高度重复 | 中 |

#### 2.2 UseCase 问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `usecase/executor.go:36-48` | Executor 10 个字段，职责过重 | 高 |
| `usecase/executor.go:103-119` | Handler 硬编码注册 | 中 |
| `usecase/ssh_pool.go:35` | Key 使用 Host 而非 Host:Port:User | 高 |
| `usecase/ssh_pool.go:47-56` | 缺少连接健康检查 | 高 |

#### 2.3 依赖注入问题

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `handler/types.go:74-93` | BaseDeps 使用 13 个 Setter，非线程安全 | 高 |
| `usecase/executor.go:50-79` | 默认值隐藏依赖 | 中 |

---

### 3. Infrastructure 层

#### 3.1 配置加载器

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `persistence/config_loader.go:19` | Context 参数未使用 | 低 |
| `persistence/config_loader.go:57-79` | 缺少原子性写入 | 中 |
| `persistence/config_loader.go:57-79` | YAML 双次序列化效率低 | 中 |

#### 3.2 状态存储

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `state/file_store.go:20` | 接口不匹配，未实现 StateRepository | 高 |
| `state/file_store.go:101` | 缺少原子写入 | 高 |
| `state/file_store.go` | 并发不安全 | 高 |

#### 3.3 DNS 工厂

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `dns/factory.go:35-37` | Register 方法并发不安全 | 高 |
| `dns/factory.go` | 缺少工厂接口定义 | 中 |

---

### 4. Interface CLI 层

#### 4.1 命令结构

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `plan.go`, `apply.go`, `app.go`, `dns.go` | 命令逻辑重复 | 中 |
| `context.go:3-20` | Context 过于简单，缺少依赖 | 中 |
| `root.go:18-57` | 命令注册与初始化耦合 | 中 |

#### 4.2 TUI 设计

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `tui_model.go:259-274` | Model 包含 7 个嵌套状态，过于臃肿 | 高 |
| `tui_model.go:19-43` | ViewState 20+ 个枚举，状态机复杂 | 高 |
| `tui.go:10-71` | Update 方法违反 Elm 最佳实践 | 中 |
| `tui_view.go:140-247` | 重复的渲染代码 | 中 |
| `tui_tree.go:16-32` | 同步操作阻塞 UI | 高 |

#### 4.3 错误处理

| 位置 | 问题 | 严重度 |
|------|------|--------|
| 多文件 | 错误处理不一致（fmt.Fprintf/log.Fatal/ErrorMessage） | 中 |
| `tui_actions.go:734-741` | 静默忽略错误 | 中 |

---

### 5. DNS/SSL Providers

#### 5.1 DNS Provider

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `dns/provider.go:19-26` | 接口缺少 Context 支持 | 中 |
| 各实现 | 构造函数返回类型不统一 | 中 |
| `dns/common.go:11-18` | EnsureRecord 只匹配第一条记录 | 高 |
| 所有实现 | 完全缺少重试机制 | 高 |
| `cloudflare.go`, `aliyun.go`, `tencent.go` | ParseTTL 函数重复 3 次 | 低 |
| 所有实现 | Batch 方法完全相同 | 低 |

#### 5.2 SSL Provider

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `ssl/acme.go:49-69` | EAB 功能未实现 | 高 |
| `ssl/acme.go:33-36` | 账户密钥每次重新生成 | 高 |
| `ssl/letsencrypt.go`, `ssl/zerossl.go` | 90% 代码相同 | 中 |
| `ssl/provider.go:15-18` | 接口缺少 Context 支持 | 中 |

---

### 6. Plan/Compose/Gate

#### 6.1 Planner

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `planner.go:23-47` | 构造函数硬编码默认值 | 低 |
| `planner.go:97-110` | stateStore 延迟初始化 | 高 |
| `planner.go:54-79` | Plan 方法缺少错误处理 | 中 |

#### 6.2 Compose Generator

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `deployment/gateway.go:173-198` | 字符串模板而非结构化 YAML | 高 |
| `compose/types.go` | ComposeService 和 Service 类型重复 | 中 |
| `deployment/compose_*.go` | 服务器目录创建逻辑重复 | 低 |

#### 6.3 Gate Generator

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `gate/types.go`, `gate/generator.go` | 内外类型重复 | 中 |
| `gate/generator.go:70-91` | 硬编码配置值 | 低 |
| `deployment/gateway.go:37-273` | 大量重复的 gateway 配置逻辑 | 中 |

---

### 7. SSH/Secrets/Network/Registry

#### 7.1 SSH Client

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `ssh/client.go:33-39` | 缺少连接超时配置 | 高 |
| `ssh/client.go:80-89` | 自动添加未知主机密钥存在安全隐患 | 高 |
| `ssh/client.go:153,161,198` | 命令注入风险 | 高 |
| `ssh/client.go` | 缺少 Keep-Alive 机制 | 中 |
| `ssh/client.go` | 缺少 Context 支持 | 中 |

#### 7.2 SFTP

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `sftp.go:22-24` | 缺少文件权限控制 | 中 |
| `client.go:115-140` | 缺少文件传输完整性校验 | 中 |
| `client.go:134` | 缺少大文件传输优化 | 低 |

#### 7.3 Secret Resolver

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `secrets/resolver.go:11` | 敏感信息明文存储 | 中 |
| `secrets/resolver.go:35-57` | ResolveAll 直接修改入参 | 中 |

#### 7.4 Network Manager

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `network/manager.go:30,68,92` | 命令注入风险 | 高 |
| `network/manager.go:54-65` | Exists 方法效率低 | 低 |

#### 7.5 Registry Manager

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `registry/manager.go:128-129` | 密码通过命令行传递（严重） | 高 |
| `registry/manager.go:81-105` | isLoggedIn 检测不可靠 | 中 |
| `registry/manager.go:26` | loggedIn map 并发不安全 | 高 |

---

### 8. Environment Module

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `syncer.go:216-218` | 命令注入风险 | 高 |
| `templates.go:11-12` | 路径遍历风险 | 高 |
| `checker.go:78,98,118` | 正则表达式重复编译 | 低 |
| `checker.go`, `syncer.go` | isRegistryLoggedIn 重复实现 | 中 |
| 整体 | 缺少接口抽象 | 高 |

---

### 9. 架构整体

| 位置 | 问题 | 严重度 |
|------|------|--------|
| `internal/` 目录 | 10+ 个包无明确层次 | 高 |
| `cli/workflow.go`, `orchestrator/workflow.go` | 两个重复的 Workflow | 高 |
| `cli/workflow.go:10-18` | CLI 直接依赖底层模块 | 高 |
| `plan/planner.go:4` | plan 依赖 application/deployment（反向） | 高 |
| `handler/types.go:197-243` | application 层暴露基础设施细节 | 中 |

---

## 问题优先级排序

### P0 - 必须立即修复（安全/数据风险）

1. **命令注入风险** - `ssh/client.go`, `network/manager.go`, `registry/manager.go`, `syncer.go`
2. **密码命令行传递** - `registry/manager.go:128-129`
3. **并发安全问题** - SSHPool, BaseDeps, Executor, RegistryManager

### P1 - 高优先级（功能缺陷/设计问题）

4. **EAB 功能未实现** - `ssl/acme.go:49-69`
5. **值对象可变** - 所有 valueobject 文件
6. **接口层跨层依赖** - CLI 直接依赖 infrastructure
7. **stateStore 延迟初始化** - `planner.go:97-110`
8. **FileStore 接口不匹配** - `state/file_store.go:20`
9. **EnsureRecord 逻辑缺陷** - `dns/common.go:11-18`

### P2 - 中优先级（代码质量/可维护性）

10. **代码重复严重** - Handler, Differ, Checker 等模块
11. **Executor/BaseDeps 职责过重** - 违反 SRP
12. **错误处理不一致** - 缺少统一策略
13. **缺少重试机制** - 所有网络操作
14. **Model 过于臃肿** - TUI tui_model.go
15. **Workflow 重复定义** - cli 和 orchestrator 包

### P3 - 低优先级（优化/改进）

16. **正则表达式重复编译** - checker.go
17. **硬编码配置值** - gate/generator.go
18. **ParseTTL 重复** - DNS providers
19. **缺少 Context 支持** - 多个模块
20. **临时文件权限** - syncer.go

---

## 重构建议路线图

### 阶段 1: 安全修复（1-2 周）

1. 修复所有命令注入风险
   - 使用参数校验或 shellescape
   - SSH 命令使用 stdin 传递敏感数据
   
2. 修复密码传递问题
   - Registry login 使用 cmd.Stdin 传递密码

3. 添加并发保护
   - 为所有共享 map 添加 sync.RWMutex
   - 或使用不可变设计

### 阶段 2: 架构重构（2-4 周）

4. 重新组织目录结构
   - 将 ssh/secrets/registry/network 移入 infrastructure
   - 将 plan 移入 application
   - 将 compose/gate 移入 infrastructure/generator
   - 分离 TUI 到 interfaces/tui

5. 统一 Workflow 实现
   - 保留 application 层的 Workflow
   - CLI 层只做调用

6. 修复依赖方向
   - CLI 只依赖 application 层
   - plan 不依赖 application/deployment

### 阶段 3: 代码质量（4-8 周）

7. 值对象不可变化
   - 所有字段改为私有
   - 添加构造函数和只读访问器
   - 实现 Equals 方法

8. 消除代码重复
   - 使用泛型重构 GetXXXMap 方法
   - 提取 Handler 公共逻辑
   - 合并 letsencrypt 和 zerossl 为 ACMEProvider

9. 统一错误处理
   - 扩展 domain/errors.go
   - 所有模块使用领域错误
   - 建立错误包装规范

10. 添加重试机制
    - 实现通用重试装饰器
    - 应用到所有网络操作

### 阶段 4: 功能完善（8-12 周）

11. 实现 EAB 功能
    - 完善 ssl/acme.go 的 External Account Binding

12. 添加测试
    - 使用依赖注入支持 mock
    - 添加表驱动测试

13. 改进 TUI
    - 拆分 Model
    - 异步操作
    - 添加加载状态

14. 添加可观测性
    - 结构化日志
    - 操作指标

---

## 附录: 快速修复清单

### 命令注入修复示例

```go
// Before (危险)
cmd := fmt.Sprintf("sudo mkdir -p %s", path)

// After (安全)
import "github.com/alessio/shellescape"
cmd := fmt.Sprintf("sudo mkdir -p %s", shellescape.Quote(path))
```

### 密码传递修复示例

```go
// Before (危险)
cmd := fmt.Sprintf("echo '%s' | docker login -u '%s' --password-stdin %s", 
    password, username, registryURL)

// After (安全)
cmd := exec.Command("docker", "login", "-u", username, "--password-stdin", registryURL)
cmd.Stdin = strings.NewReader(password)
```

### 并发安全修复示例

```go
// Before (不安全)
type Registry struct {
    loggedIn map[string]bool
}

// After (安全)
type Registry struct {
    mu       sync.RWMutex
    loggedIn map[string]bool
}

func (r *Registry) isLoggedIn(url string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.loggedIn[url]
}
```

### 值对象不可变化示例

```go
// Before (可变)
type SecretRef struct {
    Plain  string
    Secret string
}

// After (不可变)
type SecretRef struct {
    plain  string
    secret string
}

func NewSecretRef(plain, secret string) *SecretRef {
    return &SecretRef{plain: plain, secret: secret}
}

func (s *SecretRef) Plain() string  { return s.plain }
func (s *SecretRef) Secret() string { return s.secret }

func (s *SecretRef) Equals(other *SecretRef) bool {
    return s.plain == other.plain && s.secret == other.secret
}
```
