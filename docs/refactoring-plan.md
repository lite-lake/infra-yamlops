# YAMLOps 代码重构方案

## 文档信息

| 项目 | 内容 |
|------|------|
| 创建日期 | 2026-02-17 |
| 完成日期 | 2026-02-17 |
| 目标版本 | v2.0 |
| 状态 | ✅ 已完成 |

---

## 一、背景与目标

### 1.1 当前代码问题

| 文件 | 行数 | 问题描述 |
|------|------|----------|
| `cmd/yamlops/main.go` | 683 | 混合了命令定义、业务逻辑、SSH操作，违反单一职责 |
| `internal/plan/planner.go` | 1061 | 包含计划生成、状态管理、文件生成，职责过多 |
| `internal/apply/executor.go` | 622 | 使用 switch-case 处理不同实体，违反开闭原则 |
| `internal/config/loader.go` | 520 | 10个结构相同的 load* 函数，存在大量重复 |
| `internal/entities/entities.go` | 757 | 所有实体定义在一个文件，难以维护 |

### 1.2 重构目标

1. **可维护性**：单个文件不超过 300 行
2. **可扩展性**：添加新实体类型只需新增文件，不修改现有代码
3. **可测试性**：通过依赖注入实现单元测试
4. **职责清晰**：遵循 Clean Architecture 分层

### 1.3 设计原则

- **SRP**：每个文件/结构体只有一个变更理由
- **OCP**：对扩展开放，对修改关闭
- **DIP**：依赖抽象而非具体实现
- **DRY**：消除重复代码

---

## 二、目标架构

### 2.1 目录结构

```
internal/
├── domain/                          # 领域层（无外部依赖）
│   ├── entity/                     # 实体定义
│   │   ├── secret.go
│   │   ├── isp.go
│   │   ├── zone.go
│   │   ├── server.go
│   │   ├── service.go
│   │   ├── gateway.go
│   │   ├── domain.go
│   │   ├── dns_record.go
│   │   ├── certificate.go
│   │   ├── registry.go
│   │   └── config.go               # 聚合根
│   ├── valueobject/                # 值对象
│   │   ├── secret_ref.go
│   │   ├── scope.go
│   │   └── change.go
│   ├── repository/                 # 仓储接口
│   │   ├── state.go
│   │   └── config.go
│   └── service/                    # 领域服务
│       ├── comparator.go
│       └── validator.go
│
├── application/                    # 应用层
│   ├── usecase/                   # 用例
│   │   ├── planner.go
│   │   ├── executor.go
│   │   └── validator.go
│   ├── handler/                   # 变更处理器
│   │   ├── handler.go             # 接口定义
│   │   ├── dns_handler.go
│   │   ├── service_handler.go
│   │   ├── gateway_handler.go
│   │   ├── server_handler.go
│   │   └── registry.go            # 处理器注册
│   └── port/                      # 端口（外部依赖接口）
│       ├── ssh.go
│       └── dns_provider.go
│
├── infrastructure/                 # 基础设施层
│   ├── persistence/
│   │   ├── state_repository.go
│   │   └── config_loader.go
│   ├── provider/
│   │   ├── dns/
│   │   │   ├── provider.go
│   │   │   ├── factory.go
│   │   │   ├── aliyun.go
│   │   │   ├── cloudflare.go
│   │   │   └── tencent.go
│   │   └── ssl/
│   ├── ssh/
│   └── generator/
│       ├── compose/
│       └── gate/
│
└── interfaces/                     # 接口层
    ├── cli/
    │   ├── root.go
    │   ├── plan.go
    │   ├── apply.go
    │   ├── validate.go
    │   ├── env.go
    │   ├── list.go
    │   ├── show.go
    │   └── clean.go
    └── tui/
        └── tui.go
```

### 2.2 分层职责

| 层级 | 包路径 | 职责 | 依赖方向 |
|------|--------|------|----------|
| 接口层 | interfaces/ | 接收请求，调用应用层 | → application |
| 应用层 | application/ | 编排用例，协调领域对象 | → domain, infrastructure |
| 领域层 | domain/ | 核心业务逻辑，实体，值对象 | 无外部依赖 |
| 基础设施层 | infrastructure/ | 外部服务实现，持久化 | → domain (实现接口) |

### 2.3 核心设计模式

#### 2.3.1 Strategy Pattern - 变更处理器

解决 `executor.go` 中 switch-case 违反 OCP 的问题。

```go
// application/handler/handler.go
type ChangeHandler interface {
    EntityType() string
    Apply(ctx context.Context, change *domain.Change, deps *ApplyDeps) (*Result, error)
    Validate(change *domain.Change) error
}

// application/handler/registry.go
type HandlerRegistry struct {
    handlers map[string]ChangeHandler
}

func NewHandlerRegistry() *HandlerRegistry {
    return &HandlerRegistry{handlers: make(map[string]ChangeHandler)}
}

func (r *HandlerRegistry) Register(h ChangeHandler) {
    r.handlers[h.EntityType()] = h
}

func (r *HandlerRegistry) Get(entityType string) (ChangeHandler, bool) {
    h, ok := r.handlers[entityType]
    return h, ok
}
```

#### 2.3.2 Repository Pattern - 状态持久化

解耦领域层与具体存储实现。

```go
// domain/repository/state.go
type StateRepository interface {
    Load(ctx context.Context, env string) (*DeploymentState, error)
    Save(ctx context.Context, env string, state *DeploymentState) error
}

// domain/repository/config.go  
type ConfigLoader interface {
    Load(ctx context.Context, env string) (*entity.Config, error)
    Validate(cfg *entity.Config) error
}
```

#### 2.3.3 Factory Pattern - Provider 创建

```go
// infrastructure/provider/dns/factory.go
type Factory struct{}

func (f *Factory) Create(isp *entity.ISP, secrets map[string]string) (Provider, error) {
    // 根据 ISP 配置创建对应的 Provider
}
```

---

## 三、分阶段执行计划

### Phase 1: 拆分实体文件

**目标**：将 `entities.go` (757行) 拆分为独立文件

**风险等级**：低（纯文件移动，不改变逻辑）

**任务清单**：

| 序号 | 任务 | 输入 | 输出 |
|------|------|------|------|
| 1.1 | 创建 `domain/entity/` 目录 | - | 目录结构 |
| 1.2 | 提取 Secret | entities.go:71-81 | domain/entity/secret.go |
| 1.3 | 提取 ISP | entities.go:83-123 | domain/entity/isp.go |
| 1.4 | 提取 Zone | entities.go:125-143 | domain/entity/zone.go |
| 1.5 | 提取 Server | entities.go:278-305 | domain/entity/server.go |
| 1.6 | 提取 Service | entities.go:408-457 | domain/entity/service.go |
| 1.7 | 提取 Gateway | entities.go:194-233 | domain/entity/gateway.go |
| 1.8 | 提取 Domain | entities.go:493-517 | domain/entity/domain.go |
| 1.9 | 提取 DNSRecord | entities.go:531-565 | domain/entity/dns_record.go |
| 1.10 | 提取 Certificate | entities.go:574-606 | domain/entity/certificate.go |
| 1.11 | 提取 Registry | entities.go:474-491 | domain/entity/registry.go |
| 1.12 | 提取 Config | entities.go:608-745 | domain/entity/config.go |
| 1.13 | 提取 SecretRef | entities.go:24-69 | domain/valueobject/secret_ref.go |
| 1.14 | 提取错误定义 | entities.go:12-22 | domain/errors.go |
| 1.15 | 更新所有 import 路径 | 全项目 | 修改 import |
| 1.16 | 删除旧文件 | entities.go | - |
| 1.17 | 运行测试 | - | go test ./... |

**验收标准**：
- [ ] 所有测试通过
- [ ] `go build ./...` 无错误
- [ ] 每个实体文件不超过 150 行
- [ ] 功能与重构前完全一致

**注意事项**：
- 保持 `entity` 包名，import 路径变为 `github.com/litelake/yamlops/internal/domain/entity`
- 所有引用该包的文件需要更新 import

---

### Phase 2: 抽取仓储接口

**目标**：定义领域层的仓储接口，实现依赖反转

**风险等级**：低（仅新增接口，不改变现有逻辑）

**任务清单**：

| 序号 | 任务 | 输出 |
|------|------|------|
| 2.1 | 创建 `domain/repository/` 目录 | 目录结构 |
| 2.2 | 定义 StateRepository 接口 | domain/repository/state.go |
| 2.3 | 定义 ConfigLoader 接口 | domain/repository/config.go |
| 2.4 | 创建 DeploymentState 值对象 | domain/valueobject/deployment_state.go |

**接口定义**：

```go
// domain/repository/state.go
package repository

import (
    "context"
    "github.com/litelake/yamlops/internal/domain/entity"
)

type StateRepository interface {
    Load(ctx context.Context, env string) (*DeploymentState, error)
    Save(ctx context.Context, env string, state *DeploymentState) error
}

type DeploymentState struct {
    Services   map[string]*entity.Service
    Gateways   map[string]*entity.Gateway
    Servers    map[string]*entity.Server
    Zones      map[string]*entity.Zone
    Domains    map[string]*entity.Domain
    Records    map[string]*entity.DNSRecord
    Certs      map[string]*entity.Certificate
    Registries map[string]*entity.Registry
    ISPs       map[string]*entity.ISP
}

// domain/repository/config.go
package repository

type ConfigLoader interface {
    Load(ctx context.Context, env string) (*entity.Config, error)
    Validate(cfg *entity.Config) error
}
```

**验收标准**：
- [ ] 接口编译通过
- [ ] 无循环依赖

---

### Phase 3: 重构 Config Loader

**目标**：使用泛型消除 `loader.go` 中的重复代码

**风险等级**：中（修改核心加载逻辑）

**任务清单**：

| 序号 | 任务 | 说明 |
|------|------|------|
| 3.1 | 实现泛型加载函数 | 消除 10 个重复的 load* 函数 |
| 3.2 | 实现 ConfigLoader 接口 | infrastructure/persistence/config_loader.go |
| 3.3 | 更新 main.go 使用新实现 | 修改 import |
| 3.4 | 编写单元测试 | 覆盖加载逻辑 |
| 3.5 | 保留旧 loader 兼容期 | 标记 deprecated |

**泛型加载函数示例**：

```go
// infrastructure/persistence/config_loader.go
package persistence

type fileLoader[T any] struct {
    key string
}

func loadFile[T any](path string, key string) ([]T, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var wrapper struct {
        Items []T `yaml:"key"`
    }
    wrapper.Items = make([]T, 0)
    
    // 使用动态 key
    var raw map[string]interface{}
    if err := yaml.Unmarshal(data, &raw); err != nil {
        return nil, err
    }
    
    // ... 实现泛型解析
}
```

**验收标准**：
- [ ] 所有测试通过
- [ ] 代码行数减少 50% 以上
- [ ] 加载功能与原有一致

---

### Phase 4: 实现变更处理器（Strategy Pattern）

**目标**：将 `executor.go` 的 switch-case 重构为策略模式

**风险等级**：中（修改核心执行逻辑）

**任务清单**：

| 序号 | 任务 | 输出 |
|------|------|------|
| 4.1 | 定义 Handler 接口 | application/handler/handler.go |
| 4.2 | 实现 HandlerRegistry | application/handler/registry.go |
| 4.3 | 实现 DNSHandler | application/handler/dns_handler.go |
| 4.4 | 实现 ServiceHandler | application/handler/service_handler.go |
| 4.5 | 实现 GatewayHandler | application/handler/gateway_handler.go |
| 4.6 | 实现 ServerHandler | application/handler/server_handler.go |
| 4.7 | 实现 NoopHandler | application/handler/noop_handler.go (用于 isp/zone/domain 等) |
| 4.8 | 重构 Executor 使用 Handler | application/usecase/executor.go |
| 4.9 | 编写 Handler 单元测试 | application/handler/*_test.go |

**Handler 接口**：

```go
// application/handler/handler.go
package handler

import (
    "context"
    "github.com/litelake/yamlops/internal/domain/entity"
    "github.com/litelake/yamlops/internal/domain/valueobject"
)

type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps *ApplyDeps) (*Result, error)
}

type ApplyDeps struct {
    SSHClient   SSHClient
    DNSProvider DNSProvider
    Secrets     map[string]string
    Domains     map[string]*entity.Domain
    ISPs        map[string]*entity.ISP
    WorkDir     string
    Env         string
}

type Result struct {
    Success bool
    Error   error
    Output  string
}
```

**验收标准**：
- [ ] 所有 Handler 单元测试通过
- [ ] 集成测试通过
- [ ] 添加新实体类型只需新增 Handler，无需修改 Executor

---

### Phase 5: 拆分 CLI 命令

**目标**：将 `main.go` (683行) 拆分为独立命令文件

**风险等级**：低（纯代码组织，不改变逻辑）

**任务清单**：

| 序号 | 任务 | 输出 |
|------|------|------|
| 5.1 | 创建 `interfaces/cli/` 目录 | 目录结构 |
| 5.2 | 提取 root 命令定义 | interfaces/cli/root.go |
| 5.3 | 提取 plan 命令 | interfaces/cli/plan.go |
| 5.4 | 提取 apply 命令 | interfaces/cli/apply.go |
| 5.5 | 提取 validate 命令 | interfaces/cli/validate.go |
| 5.6 | 提取 env 命令 | interfaces/cli/env.go |
| 5.7 | 提取 list 命令 | interfaces/cli/list.go |
| 5.8 | 提取 show 命令 | interfaces/cli/show.go |
| 5.9 | 提取 clean 命令 | interfaces/cli/clean.go |
| 5.10 | 简化 main.go | cmd/yamlops/main.go (仅调用 cli.Execute()) |

**命令文件模板**：

```go
// interfaces/cli/plan.go
package cli

import (
    "github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
    Use:   "plan [scope]",
    Short: "Generate execution plan",
    Args:  cobra.MaximumNArgs(1),
    RunE:  runPlan,
}

func init() {
    rootCmd.AddCommand(planCmd)
    planCmd.Flags().StringVar(&domainFilter, "domain", "", "Filter by domain")
    planCmd.Flags().StringVar(&zoneFilter, "zone", "", "Filter by zone")
    planCmd.Flags().StringVar(&serverFilter, "server", "", "Filter by server")
    planCmd.Flags().StringVar(&serviceFilter, "service", "", "Filter by service")
}

func runPlan(cmd *cobra.Command, args []string) error {
    // 实现逻辑
}
```

**验收标准**：
- [ ] 所有 CLI 命令功能正常
- [ ] main.go 不超过 50 行
- [ ] 每个命令文件不超过 100 行

---

### Phase 6: 重构 Planner

**目标**：将 `planner.go` (1061行) 拆分为职责单一的模块

**风险等级**：高（修改核心计划逻辑）

**任务清单**：

| 序号 | 任务 | 输出 |
|------|------|------|
| 6.1 | 提取 DeploymentState | domain/valueobject/deployment_state.go |
| 6.2 | 提取 Scope | domain/valueobject/scope.go |
| 6.3 | 提取 Change | domain/valueobject/change.go |
| 6.4 | 创建 EntityPlanner 泛型 | domain/service/planner.go |
| 6.5 | 实现各实体的 Planner 实现 | domain/service/*_planner.go |
| 6.6 | 提取部署文件生成 | infrastructure/generator/deployment.go |
| 6.7 | 重构 Planner 用例 | application/usecase/planner.go |
| 6.8 | 编写单元测试 | 覆盖计划生成逻辑 |

**泛型 Planner 模板**：

```go
// domain/service/planner.go
package service

type EntityPlanner[T any] interface {
    GetConfigMap(cfg *entity.Config) map[string]*T
    GetStateMap(state *DeploymentState) map[string]*T
    Equals(a, b *T) bool
    GetScopeKey(entity *T) ScopeKey
}

func PlanEntities[T any](
    planner EntityPlanner[T],
    cfg *entity.Config,
    state *DeploymentState,
    scope *Scope,
) []*Change {
    // 通用计划生成逻辑
}
```

**验收标准**：
- [ ] 所有测试通过
- [ ] 代码重复减少 70%
- [ ] 每个文件不超过 200 行

---

## 四、需求边界

### 4.1 本次重构包含

| 内容 | 说明 |
|------|------|
| 代码重组 | 按分层架构重新组织目录结构 |
| 接口抽取 | 定义 Repository、Handler 等接口 |
| 重复消除 | 使用泛型和模板方法消除重复代码 |
| 单一职责 | 拆分大文件为小文件 |

### 4.2 本次重构不包含

| 内容 | 原因 |
|------|------|
| 新功能 | 仅重构，不添加新特性 |
| API 变更 | CLI 命令和参数保持不变 |
| 配置格式 | YAML 配置文件格式不变 |
| 数据库迁移 | 项目无数据库，状态文件格式不变 |
| 性能优化 | 非本次目标 |
| TUI 重构 | TUI 模块保持现状 |

### 4.3 兼容性保证

| 兼容项 | 保证方式 |
|--------|----------|
| CLI 命令 | 命令名称、参数、输出格式不变 |
| 配置文件 | userdata/ 下的 YAML 格式不变 |
| 状态文件 | .state.yaml 格式不变 |
| 部署目录 | deployments/ 目录结构不变 |
| 远程路径 | /data/yamlops/ 路径不变 |

---

## 五、测试要求

### 5.1 单元测试

每个阶段完成后必须通过：

```bash
go test ./... -v
go test ./... -cover
```

覆盖率要求：
- 领域层：80%+
- 应用层：70%+
- 基础设施层：60%+

### 5.2 集成测试

在 `userdata/test/` 目录准备测试配置：

```bash
# 验证命令
./yamlops validate --env test
./yamlops plan --env test
./yamlops list servers --env test
```

### 5.3 回归测试

每个阶段完成后，对比重构前后的输出：

```bash
# 重构前
./yamlops-old plan --env dev > old-plan.txt

# 重构后
./yamlops-new plan --env dev > new-plan.txt

# 对比
diff old-plan.txt new-plan.txt
```

---

## 六、风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| import 路径变更导致编译失败 | 高 | 使用 IDE 的重构功能，全局搜索替换 |
| 接口设计不合理导致返工 | 中 | 每阶段先设计接口，评审后再实现 |
| 测试覆盖不足导致回归 | 高 | 每阶段必须补充测试 |
| 泛型实现复杂度超预期 | 中 | 可回退到代码生成或保持现状 |

---

## 七、交付物

每个阶段交付：

1. **代码变更**：PR 包含完整的代码变更
2. **测试报告**：测试覆盖率报告
3. **变更日志**：CHANGELOG.md 更新
4. **文档更新**：AGENTS.md 中相关命令的更新

最终交付：

1. 重构后的完整代码
2. 测试覆盖率报告（>70%）
3. 更新的 AGENTS.md
4. 架构图文档

---

## 八、执行顺序建议

```
Phase 1 (拆分实体) ──→ Phase 2 (抽取接口) ──→ Phase 5 (拆分CLI)
                                                    ↓
Phase 3 (重构Loader) ←── Phase 4 (变更处理器) ←────┘
                                                    ↓
                              Phase 6 (重构Planner) ←┘
```

**建议执行顺序**：1 → 2 → 5 → 3 → 4 → 6

- Phase 1, 2, 5 风险低，可并行或快速完成
- Phase 3, 4 风险中，需要仔细测试
- Phase 6 风险高，放在最后，可选择性执行

---

## 九、参考资料

- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)
- [Go Generics](https://go.dev/doc/tutorial/generics)
- [Effective Go](https://go.dev/doc/effective_go)

---

## 附录 A：重构前代码统计

| 文件 | 行数 | 函数数 | 依赖数 |
|------|------|--------|--------|
| cmd/yamlops/main.go | 683 | 15 | 6 |
| internal/entities/entities.go | 757 | 50 | 4 |
| internal/plan/planner.go | 1061 | 35 | 5 |
| internal/apply/executor.go | 622 | 20 | 5 |
| internal/config/loader.go | 520 | 25 | 2 |
| internal/cli/tui.go | 519 | 20 | 2 |

## 附录 B：重构后代码统计

| 目录/文件 | 行数 | 说明 |
|-----------|------|------|
| cmd/yamlops/main.go | 7 | 精简入口 |
| **domain/entity/** | | 11 个实体文件 |
| - secret.go | 19 | |
| - isp.go | 51 | |
| - zone.go | 28 | |
| - server.go | 82 | |
| - service.go | 162 | |
| - gateway.go | 99 | |
| - domain.go | 36 | |
| - dns_record.go | 56 | |
| - certificate.go | 49 | |
| - registry.go | 43 | |
| - config.go | 158 | |
| **domain/valueobject/** | | 4 个值对象 |
| - secret_ref.go | 54 | |
| - change.go | 34 | |
| - scope.go | 28 | |
| - plan.go | 56 | |
| **domain/repository/** | | 2 个仓储接口 |
| - config.go | 14 | |
| - state.go | 27 | |
| **domain/service/** | | 3 个计划服务 |
| - planner.go | 200 | |
| - planner_records.go | 196 | |
| - planner_servers.go | 214 | |
| **application/handler/** | | 9 个处理器 |
| - types.go | 89 | |
| - registry.go | 26 | |
| - dns_handler.go | 175 | |
| - service_handler.go | 138 | |
| - gateway_handler.go | 131 | |
| - server_handler.go | 38 | |
| - certificate_handler.go | 25 | |
| - registry_handler.go | 25 | |
| - noop_handler.go | 27 | |
| **application/usecase/** | | |
| - executor.go | 191 | |
| **infrastructure/persistence/** | | |
| - config_loader.go | 442 | 泛型实现 |
| - config_loader_test.go | 232 | |
| **interfaces/cli/** | | 9 个命令文件 |
| - root.go | 48 | |
| - plan.go | 85 | |
| - apply.go | 107 | |
| - validate.go | 39 | |
| - env.go | 139 | |
| - list.go | 89 | |
| - show.go | 98 | |
| - clean.go | 153 | |
| - tui.go | 15 | |
| **plan/** | | |
| - planner.go | 189 | 协调层 |
| - generator_compose.go | 143 | |
| - generator_gate.go | 167 | |

---

## 重构完成状态

**完成日期**: 2026-02-17

| Phase | 任务 | 状态 |
|-------|------|------|
| 1 | 拆分实体文件 | ✅ 完成 |
| 2 | 抽取仓储接口 | ✅ 完成 |
| 3 | 重构 Config Loader | ✅ 完成 |
| 4 | 实现变更处理器 (Strategy Pattern) | ✅ 完成 |
| 5 | 拆分 CLI 命令 | ✅ 完成 |
| 6 | 重构 Planner | ✅ 完成 |
| - | 清理兼容层代码 | ✅ 完成 |

**关键改进**:
- `main.go`: 683行 → 7行
- `entities.go`: 757行 → 12个独立文件 (每个 ≤162行)
- `executor.go`: switch-case → Strategy Pattern (9个独立Handler)
- `loader.go`: 10个重复函数 → 泛型 `loadEntity[T]`
- `planner.go`: 1061行 → 多个职责单一的文件

**已删除的兼容层**:
- `internal/apply/` (旧的 switch-case executor)
- `internal/config/loader.go` (包装器)
- `internal/plan/plan.go` (类型别名)

---

*文档结束*
