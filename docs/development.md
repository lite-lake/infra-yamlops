# 开发指南

YAMLOps 项目开发指南，包括构建、测试、代码风格和项目结构。

## 构建命令

### 编译

```bash
# Linux/macOS
go build -o yamlops ./cmd/yamlops

# Windows
go build -o yamlops.exe ./cmd/yamlops

# 指定版本
go build -ldflags "-X internal/interfaces/cli.Version=1.0.0" -o yamlops ./cmd/yamlops
```

### 依赖管理

```bash
# 下载依赖
go mod download

# 整理依赖
go mod tidy

# 验证依赖
go mod verify
```

### 交叉编译

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o yamlops-linux-amd64 ./cmd/yamlops

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o yamlops-linux-arm64 ./cmd/yamlops

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o yamlops-darwin-amd64 ./cmd/yamlops

# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o yamlops-darwin-arm64 ./cmd/yamlops

# Windows
GOOS=windows GOARCH=amd64 go build -o yamlops-windows-amd64.exe ./cmd/yamlops
```

---

## 测试命令

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行指定包测试
go test ./internal/domain/entity/...

# 运行单个测试
go test ./internal/domain/entity -run TestServer -v

# 运行特定测试方法
go test ./internal/domain/entity -run TestServer_Validate -v

# 带覆盖率
go test -v -cover ./...

# 带竞态检测
go test -race ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 测试模式

使用表驱动测试：

```go
func TestServer_Validate(t *testing.T) {
    tests := []struct {
        name    string
        server  Server
        wantErr error
    }{
        {"missing name", Server{}, domain.ErrInvalidName},
        {"missing zone", Server{Name: "server-1"}, domain.ErrRequired},
        {"valid", Server{Name: "s1", Zone: "z1", SSH: ServerSSH{...}}, nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.server.Validate()
            if tt.wantErr != nil {
                if !errors.Is(err, tt.wantErr) {
                    t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
                }
            } else if err != nil {
                t.Errorf("Validate() unexpected error = %v", err)
            }
        })
    }
}
```

---

## 代码风格规范

### 导入分组

```go
import (
    // 1. 标准库
    "context"
    "errors"
    "fmt"

    // 2. 第三方库
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    // 3. 内部包
    "github.com/litelake/yamlops/internal/domain"
    "github.com/litelake/yamlops/internal/domain/entity"
)
```

### 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 包名 | 小写，单词 | `config`, `plan`, `ssh` |
| 导出类型 | PascalCase | `Server`, `DifferService` |
| 内部类型 | camelCase | `configLoader`, `sshClient` |
| 接口 | -er 后缀 | `Provider`, `Loader`, `Handler` |
| 常量 | PascalCase 或 UPPER_SNAKE_CASE | `DefaultPort`, `MAX_RETRIES` |
| 错误 | Err 前缀 | `ErrInvalidName`, `ErrPortConflict` |

### 错误处理

在 `internal/domain/errors.go` 中统一定义错误：

```go
var (
    ErrInvalidName  = errors.New("invalid name")
    ErrRequired     = errors.New("required field missing")
    ErrPortConflict = errors.New("port conflict")
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}
```

使用 `fmt.Errorf` 和 `%w` 包装错误：

```go
func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", domain.ErrInvalidName)
    }
    return nil
}
```

### 枚举类型

使用 iota 和显式类型：

```go
type ChangeType int

const (
    ChangeTypeNoop ChangeType = iota
    ChangeTypeCreate
    ChangeTypeUpdate
    ChangeTypeDelete
)
```

### 结构体标签

使用 yaml 标签，可选字段加 `omitempty`：

```go
type Server struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description,omitempty"`
    Zone        string `yaml:"zone"`
}
```

### 构造函数

使用 `New<Type>` 命名：

```go
func NewLoader(env, baseDir string) *Loader {
    return &Loader{env: env, baseDir: baseDir}
}
```

### Option 模式

```go
type Option func(*Config)

func WithMaxAttempts(n int) Option {
    return func(c *Config) { c.MaxAttempts = n }
}

func DefaultConfig() *Config {
    return &Config{MaxAttempts: 3}
}

// 使用
cfg := DefaultConfig()
for _, opt := range opts {
    opt(cfg)
}
```

### YAML 自定义反序列化

支持简写和完整形式：

```go
func (s *SecretRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
    var plain string
    if err := unmarshal(&plain); err == nil {
        s.Plain = plain
        return nil
    }
    type alias SecretRef
    return unmarshal((*alias)(s))
}
```

---

## 代码检查

### 格式化

```bash
go fmt ./...
```

### 静态检查

```bash
# go vet
go vet ./...

# staticcheck（需安装）
staticcheck ./...
```

### 安装 staticcheck

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

---

## 项目结构

```
cmd/yamlops/                    # CLI 入口点
internal/
├── domain/                     # 领域层（无外部依赖）
│   ├── entity/                 # 实体定义
│   │   ├── server.go           # 服务器实体
│   │   ├── zone.go             # 区域实体
│   │   ├── isp.go              # ISP 实体
│   │   ├── secret.go           # 密钥实体
│   │   ├── registry.go         # 仓库实体
│   │   ├── domain.go           # 域名实体
│   │   ├── dns_record.go       # DNS 记录实体
│   │   ├── biz_service.go      # 业务服务实体
│   │   └── infra_service.go    # 基础设施服务实体
│   ├── valueobject/            # 值对象
│   │   ├── secret_ref.go       # 密钥引用
│   │   ├── change.go           # 变更
│   │   ├── scope.go            # 作用域
│   │   └── plan.go             # 执行计划
│   ├── repository/             # 仓储接口
│   │   ├── config_loader.go    # 配置加载接口
│   │   └── state.go            # 状态存储接口
│   ├── service/                # 领域服务
│   │   ├── validator.go        # 验证器
│   │   ├── differ.go           # 差异检测
│   │   └── differ_generic.go   # 泛型差异检测
│   ├── retry/                  # 重试机制
│   │   └── retry.go            # Option 模式重试
│   └── errors.go               # 领域错误
├── application/                # 应用层
│   ├── handler/                # 变更处理器（策略模式）
│   │   ├── types.go            # 依赖接口定义
│   │   ├── dns_handler.go      # DNS 处理器
│   │   ├── service_handler.go  # 服务处理器
│   │   ├── server_handler.go   # 服务器处理器
│   │   ├── noop_handler.go     # 空操作处理器
│   │   ├── registry.go         # 处理器注册表
│   │   └── utils.go            # 工具函数
│   ├── usecase/                # 用例执行器
│   │   ├── executor.go         # 执行器
│   │   └── ssh_pool.go         # SSH 连接池
│   ├── deployment/             # 部署文件生成器
│   │   ├── generator.go        # 主入口
│   │   ├── compose_service.go  # Compose 生成
│   │   ├── gateway.go          # Gateway 配置
│   │   └── utils.go            # 工具函数
│   ├── plan/                   # 规划器
│   │   └── planner.go          # Option 模式规划器
│   └── orchestrator/           # 工作流编排
│       ├── workflow.go         # 主入口
│       └── state_fetcher.go    # 状态获取
├── infrastructure/             # 基础设施层
│   ├── persistence/            # 配置加载
│   │   └── config_loader.go    # 配置加载实现
│   ├── state/                  # 状态存储
│   │   └── file_store.go       # 文件状态存储
│   ├── ssh/                    # SSH 客户端
│   │   ├── client.go           # SSH 客户端
│   │   ├── sftp.go             # SFTP 传输
│   │   └── shell_escape.go     # Shell 转义
│   ├── dns/                    # DNS 工厂
│   │   └── factory.go          # DNS Provider 工厂
│   ├── generator/              # 生成器
│   │   ├── compose/            # Docker Compose
│   │   └── gate/               # infra-gate 配置
│   ├── secrets/                # 密钥解析器
│   │   └── resolver.go         # SecretResolver
│   └── logger/                 # 日志
│       ├── logger.go           # 主入口
│       ├── context.go          # 上下文日志
│       └── metrics.go          # 指标记录
├── interfaces/                 # 接口层
│   └── cli/                    # CLI 命令
│       ├── root.go             # 根命令
│       ├── plan.go             # Plan 命令
│       ├── apply.go            # Apply 命令
│       ├── validate.go         # Validate 命令
│       ├── list.go             # List 命令
│       ├── show.go             # Show 命令
│       ├── clean.go            # Clean 命令
│       ├── env.go              # Env 命令
│       ├── dns.go              # DNS 命令
│       ├── server_cmd.go       # Server 命令
│       ├── config_cmd.go       # Config 命令
│       ├── app.go              # App 命令
│       ├── tui.go              # TUI 主入口
│       └── ...                 # 其他 TUI 文件
├── constants/                  # 常量
│   └── constants.go            # 路径、格式常量
├── environment/                # 环境管理
│   ├── checker.go              # 环境检查
│   ├── syncer.go               # 环境同步
│   └── templates.go            # 配置模板
└── providers/                  # 外部服务提供者
    └── dns/                    # DNS 提供者
        ├── provider.go         # 接口定义
        ├── common.go           # 公共逻辑
        ├── cloudflare.go       # Cloudflare
        ├── aliyun.go           # 阿里云
        └── tencent.go          # 腾讯云
userdata/{env}/                 # 用户配置
deployments/                    # 生成文件（git-ignored）
```

---

## 架构分层

| 层 | 包 | 依赖 |
|----|----|----|
| 接口层 | interfaces/cli | → application |
| 应用层 | application/ | → domain, infrastructure |
| 领域层 | domain/ | 无外部依赖 |
| 基础设施层 | infrastructure/ | → domain（实现接口） |

**依赖规则：**

```
Interface → Application → Domain ← Infrastructure
                              ↑
                              └── 依赖倒置：Infrastructure 实现 Domain 接口
```

---

## 设计模式

| 模式 | 应用位置 |
|------|----------|
| 策略模式 | Handler 注册表，不同 Handler 实现 |
| 工厂模式 | DNS Provider 创建 |
| 适配器模式 | DNS Provider 适配 |
| 对象池模式 | SSH 连接池 |
| 依赖注入 (DIP) | Executor 通过 ExecutorConfig 接收依赖 |
| 接口隔离 (ISP) | DNSDeps / ServiceDeps / CommonDeps |
| 注册表模式 | Handler Registry |
| Option 模式 | Planner 配置、Retry 重试机制 |
| 泛型编程 | planSimpleEntity 函数 |

---

## 开发工作流

### 1. 创建新功能

```bash
# 1. 创建分支
git checkout -b feature/new-feature

# 2. 编写代码
# ...

# 3. 运行测试
go test ./...

# 4. 代码检查
go fmt ./...
go vet ./...
staticcheck ./...

# 5. 提交代码
git add .
git commit -m "feat: add new feature"
```

### 2. 添加新实体

1. 在 `internal/domain/entity/` 创建实体定义
2. 在 `internal/domain/errors.go` 添加相关错误
3. 在 `internal/domain/service/validator.go` 添加验证逻辑
4. 在 `internal/application/handler/` 创建处理器
5. 在 `internal/infrastructure/persistence/config_loader.go` 添加加载逻辑

### 3. 添加新 DNS 提供者

1. 在 `internal/providers/dns/` 创建提供者实现
2. 实现 `Provider` 接口
3. 在 `internal/infrastructure/dns/factory.go` 注册工厂方法

---

## 注意事项

- **永远不要提交密钥到仓库**
- `deployments/` 目录已被 git-ignore
- 领域层不能有外部依赖
- 每个 Handler 必须实现 `Apply(ctx, change, deps)` 方法
- 服务命名：`yo-{env}-{service-name}`
- 使用 `errors.Is()` 进行错误比较
