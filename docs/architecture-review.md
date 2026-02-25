# YAMLOps 架构审查报告

**审查日期**: 2026-02-25  
**审查范围**: 全部源代码  
**审查方法**: 分模块并行深度分析

---

## 执行摘要

| 层级 | 高 | 中 | 低 | 合计 |
|------|----|----|-----|------|
| Domain | 4 | 10 | 8 | 22 |
| Application | 4 | 6 | 5 | 15 |
| Infrastructure | 1 | 7 | 8 | 16 |
| CLI/TUI/Providers | 5 | 7 | 5 | 17 |
| 整体架构 | 2 | 3 | 2 | 7 |
| **总计** | **16** | **33** | **28** | **77** |

---

## 高优先级问题 (立即修复)

### 1. [安全] Secret 明文泄露风险
**位置**: `internal/infrastructure/secrets/resolver.go:35,44,56,57`

**问题**: `ResolveAll` 将明文密码写回结构体，日志输出时可能泄露密钥。

**建议**:
- 保持原始 SecretRef 不变，仅在需要时解析
- 为 SecretRef 实现 `slog.LogValuer` 接口隐藏敏感值

---

### 2. [安全] SSH nil channel panic
**位置**: `internal/infrastructure/ssh/client.go:404`

**问题**: `streamReader` 在 ch 为 nil 且有错误时会向 nil channel 发送导致 panic。

```go
if err != nil {
    if ch != nil {  // 需要添加此检查
        ch <- err.Error()
        close(ch)
    }
}
```

---

### 3. [架构] SSHClient 接口重复定义
**位置**: 
- `internal/application/handler/types.go:198`
- `internal/infrastructure/network/manager.go:13`
- `internal/infrastructure/registry/manager.go:24`

**问题**: 同一接口在 3 处重复定义，维护困难。

**建议**: 统一定义在 `internal/domain/interfaces` 或 `internal/interfaces/ssh.go`。

---

### 4. [架构] constants 包被 domain 层依赖
**位置**: `internal/domain/entity` → `internal/constants`

**问题**: Domain 层应完全无外部依赖，但引用了 constants 包。

**建议**: 将 domain 需要的常量移入 domain 层内部定义。

---

### 5. [设计] 值对象可变性问题
**位置**: `internal/domain/valueobject/` 所有文件

**问题**: Change, Plan, Scope 等值对象的字段是公开的，可以被外部直接修改，违反值对象不可变原则。

**建议**: 
- 将字段改为私有
- 提供只读访问方法
- `WithXxx` 方法返回新实例

---

### 6. [设计] BaseDeps 违反依赖注入原则
**位置**: `internal/application/handler/types.go:50-181`

**问题**: 使用大量 setter 方法手动设置依赖，而非构造函数注入。

**建议**: 使用构造函数 `NewBaseDeps(opts ...BaseDepsOption)` 统一初始化。

---

### 7. [设计] Executor 职责过多
**位置**: `internal/application/usecase/executor.go`

**问题**: 同时负责 handler 注册、SSH 连接管理、变更执行，违反单一职责原则。

**建议**: 拆分为 `HandlerRegistry`、`ChangeExecutor` 两个独立组件。

---

### 8. [验证] Env 字段 SecretRef 未验证
**位置**: `internal/domain/entity/biz_service.go:136-166`

**问题**: `BizService.Validate()` 没有验证 `Env map[string]SecretRef` 中的值。

```go
for key, ref := range s.Env {
    if err := ref.Validate(); err != nil {
        return fmt.Errorf("env[%s]: %w", key, err)
    }
}
```

---

### 9. [验证] 端口冲突检测不完整
**位置**: `internal/domain/service/validator.go:183-214`

**问题**: 分别检测 infra services 和 biz services 的端口冲突，但没有检测两者之间的冲突。

**建议**: 合并为单一端口映射表，检测所有服务类型的端口冲突。

---

### 10. [TUI] 巨型 switch 重复
**位置**: `internal/interfaces/cli/tui_actions.go:40-198`

**问题**: `handleUp`/`handleDown` 各约 60 行，结构几乎相同，每个 ViewState 都需要处理。

**建议**: 使用状态模式或定义 `CursorHandler` 接口。

---

### 11. [TUI] Stop/Restart/Cleanup 功能高度重复
**位置**: `tui_stop.go`, `tui_restart.go`, `tui_cleanup.go`

**问题**: 渲染、计数、执行逻辑几乎完全相同。

**建议**: 抽取通用的 `ServiceOperation` 结构。

---

### 12. [CLI] 大量重复的实体查找和显示逻辑
**位置**: `show.go`, `app.go`, `dns.go`, `config_cmd.go`

**问题**: `runShow`、`runAppShow`、`runDNSShow`、`runConfigShow` 存在几乎相同的实体查找和 YAML 序列化逻辑。

**建议**: 抽取通用的 `showEntity` 函数。

---

### 13. [并发] State 无文件锁
**位置**: `internal/infrastructure/state/file_store.go`

**问题**: 多进程并发写入可能导致数据丢失。

**建议**: 使用 `github.com/gofrs/flock`。

---

### 14. [DNS] 错误处理不一致
**位置**: `internal/providers/dns/tencent.go` vs `aliyun.go`, `cloudflare.go`

**问题**: TencentProvider 使用 `fmt.Errorf`，而其他使用 `domainerr.WrapOp`。

**建议**: 统一使用 domain 层定义的错误包装函数。

---

### 15. [DNS] Provider 接口缺少 context 参数
**位置**: `internal/providers/dns/provider.go:19-26`

**问题**: 所有方法都没有 context 参数，无法支持超时和取消。

**建议**: 添加 context 参数。

---

### 16. [SSH] 无 Keep-Alive 机制
**位置**: `internal/infrastructure/ssh/client.go`

**问题**: 长连接可能被防火墙/NAT 断开。

**建议**: 添加 `SendKeepalive` goroutine。

---

## 中优先级问题 (计划修复)

### Domain 层

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| 1 | ServerEquals 缺少 Networks 比较 | `differ_servers.go:65-90` | 添加网络字段比较 |
| 2 | 错误类型不一致 | `entity/domain.go:33-34` | 使用 `domain.RequiredField()` |
| 3 | Secret 实体验证不完整 | `entity/secret.go:14-18` | 验证 Value 非空 |
| 4 | ISP Type 默认值在 Validate 中设置 | `entity/isp.go:37-39` | 移至构造函数 |
| 5 | Plan.Clone() 浅拷贝 | `valueobject/plan.go:76-93` | 考虑深拷贝 |
| 6 | Scope.Matches 逻辑冲突 | `valueobject/scope.go:70-96` | 明确 Service/Services 优先级 |
| 7 | DNSRecord TTL 缺少默认值和最小值 | `entity/dns_record.go:48-50` | 设置默认值 300，最小值 60 |
| 8 | Healthcheck Interval/Timeout 未验证 | `entity/biz_service.go:18-26` | 验证时间格式 |
| 9 | DeploymentState 位置不当 | `repository/state.go:14-22` | 移至 entity 包 |

### Application 层

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| 10 | Handler 命名不一致 | 各 handler 文件 | 统一使用 `extractEntityFromChange` |
| 11 | YAML 字符串拼接 | `compose_infra.go:82-98` | 使用 yaml.Marshal |
| 12 | Registry 缺少 Unregister | `handler/registry.go` | 添加 `Unregister(entityType)` |
| 13 | Planner 职责边界模糊 | `plan/planner.go` | 分离 deployment 生成 |
| 14 | SSHPool 缺少连接健康检查 | `ssh_pool.go:35-58` | 添加 `IsConnected()` |
| 15 | StateFetcher 方法过长且重复 | `orchestrator/state_fetcher.go` | 抽取泛型函数 |

### Infrastructure 层

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| 16 | generateShortID 忽略错误 | `logger/context.go:42` | 检查错误或降级处理 |
| 17 | shellEscape 重复实现 | `registry/manager.go:13-15` | 复用 `ssh.ShellEscape` |
| 18 | isLoggedIn 检测方法不可靠 | `registry/manager.go:94-118` | 使用 credential helper |
| 19 | 密钥内存中明文存储 | `secrets/resolver.go:12` | 考虑 memguard |

### CLI/TUI 层

| # | 问题 | 位置 | 建议 |
|---|------|------|------|
| 20 | Filters 和 AppFilters 结构体重复 | `context.go`, `app.go` | 合并 |
| 21 | workflow.go 可能是多余包装 | `workflow.go` | 移除或添加逻辑 |
| 22 | Model 结构体过于庞大 | `tui_model.go:427-445` | 按功能域拆分 |
| 23 | ViewState 枚举值过多 | `tui_model.go:120-148` | 使用状态机模式 |
| 24 | Provider 接口设计不完整 | `providers/dns/provider.go` | 添加常用方法到接口 |
| 25 | Checker 和 Syncer 重复方法 | `environment/checker.go`, `syncer.go` | 抽取公共函数 |

---

## 低优先级问题 (可选优化)

### Domain 层
- InfraService 结构过于复杂，混合 Gateway 和 SSL 字段
- OpError 与 WrapOp 功能重叠
- ErrDNSSubdomainConflict 命名不准确
- ConfigLoader.Validate 方法位置不当
- retry syscall 兼容性问题 (Windows)
- Config 缺少实体名称唯一性验证
- Change.Equals 不比较状态
- Config.GetAllDNSRecords 每次创建新切片

### Application 层
- deployment/utils.go 函数未使用
- 重复的 default handler 注册逻辑
- ServerHandler 中的 map 字面量
- Planner 缺少 nil 验证
- DNSHandler update 回退到 create 行为不一致

### Infrastructure 层
- Shell 转义未处理控制字符
- map 迭代顺序不确定
- DNS Factory.Register 非线程安全
- WithContext 返回自身未利用上下文
- Metrics 累积延迟无法重置
- gate generator 内联 struct 定义重复
- generator 缺少输入验证
- network/manager.go 依赖 docker 输出格式

### CLI/TUI 层
- Context 结构体可扩展
- Confirm 函数边界情况处理
- 使用自定义 max 而非内置函数
- ParseAliyunTTL/ParseTencentTTL 总是返回默认值
- SyncDockerNetwork 是多余包装

---

## 架构改进建议

### 当前架构问题

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI/TUI                              │
│  (大量重复代码、Model 过大、ViewState 过多)                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Application                            │
│  (Executor 职责过多、BaseDeps 使用 setter 注入)              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│   Domain (无外部依赖)    │      Infrastructure               │
│   (值对象可变、验证不完整)│   (SSHClient 重复定义、无文件锁)   │
└─────────────────────────────────────────────────────────────┘
```

### 建议架构改进

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI/TUI                              │
│  (抽取公共函数、拆分 Model、状态机模式)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Application                            │
│  HandlerRegistry │ ChangeExecutor │ ConnectionPool          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│   Domain (不可变值对象)   │   Infrastructure                 │
│   (完整验证)             │   (统一接口定义、文件锁、安全日志)  │
└─────────────────────────────────────────────────────────────┘
```

---

## 修复优先级路线图

### 第一阶段 (1-2 周) - 安全与稳定性
1. 修复 Secret 明文泄露风险
2. 修复 SSH nil channel panic
3. 添加 State 文件锁
4. 补全 Env 验证
5. 修复端口冲突检测

### 第二阶段 (2-4 周) - 架构改进
1. 统一 SSHClient 接口定义
2. 重构 Executor 拆分职责
3. 重构 BaseDeps 使用构造函数
4. 将值对象改为不可变设计
5. 解决 constants 包依赖问题

### 第三阶段 (4-6 周) - 代码质量
1. 消除 TUI 重复代码
2. 消除 CLI 重复代码
3. 统一 DNS Provider 错误处理
4. 添加 Provider 接口 context 参数

---

## 结论

YAMLOps 项目整体架构遵循 DDD 分层设计，Domain 层保持纯净无外部依赖，这是值得肯定的。主要问题集中在：

1. **安全性**: Secret 处理、SSH 错误处理存在风险
2. **代码重复**: TUI/CLI 存在大量重复代码
3. **接口设计**: SSHClient 重复定义、Provider 接口不完整

建议按照上述路线图分阶段修复，优先解决安全和稳定性问题。
