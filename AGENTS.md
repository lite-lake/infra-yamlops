# AGENTS.md

面向在 YAMLOps 代码库中工作的智能编码代理的指南。

## 项目概述

YAMLOps 是一个基于 Go 语言的基础设施运维工具，通过 YAML 配置管理服务器、服务、DNS 和 SSL 证书。它支持多环境（prod/staging/dev），采用 plan/apply 工作流程。

## 构建命令

```bash
# 构建项目
go build -o yamlops ./cmd/yamlops

# Windows 系统
go build -o yamlops.exe ./cmd/yamlops

# 下载依赖
go mod tidy
go mod download
```

## 测试命令

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/config/...

# 运行单个测试文件
go test ./internal/entities -run TestSecretRef

# 详细输出运行测试
go test -v ./...

# 运行测试并生成覆盖率报告
go test -cover ./...
```

## 代码检查命令

```bash
# 格式化代码
go fmt ./...

# 运行 go vet
go vet ./...

# 运行 staticcheck（如已安装）
staticcheck ./...
```

## 开发用 CLI 命令

```bash
# 验证配置
./yamlops validate -e dev

# 生成执行计划
./yamlops plan -e dev

# 应用变更
./yamlops apply -e dev

# 列出实体
./yamlops list servers -e prod
./yamlops list services -e prod

# 检查环境
./yamlops env check -e prod

# 启动 TUI 模式
./yamlops -e dev
```

## 项目结构

```
.
├── cmd/yamlops/          # CLI 入口
├── internal/
│   ├── cli/              # BubbleTea TUI 界面
│   ├── config/           # 配置加载与验证
│   ├── entities/         # 实体定义与模式
│   ├── plan/             # 变更计划逻辑
│   ├── apply/            # 变更执行
│   ├── ssh/              # SSH 客户端操作
│   ├── compose/          # Docker Compose 生成
│   ├── gate/             # infra-gate 配置生成
│   └── providers/        # 外部服务提供商集成
│       ├── dns/          # DNS 提供商（Cloudflare、阿里云、腾讯云）
│       └── ssl/          # SSL 提供商（Let's Encrypt、ZeroSSL）
├── userdata/             # 用户配置文件
│   ├── prod/
│   ├── staging/
│   └── dev/
└── deployments/          # 生成的部署文件（已忽略）
```

## 代码风格指南

### 导入

按以下顺序分组导入，组之间用空行分隔：

```go
import (
    // 标准库
    "errors"
    "fmt"
    "os"
    
    // 第三方包
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
    
    // 内部包
    "github.com/litelake/yamlops/internal/entities"
)
```

### 命名规范

- **包名**：小写，单个单词（如 `config`、`plan`、`ssh`）
- **类型**：导出类型用 PascalCase，内部类型用 camelCase
- **接口**：通常以 `-er` 结尾（如 `Provider`、`sftpClient`）
- **常量**：PascalCase 或 UPPER_SNAKE_CASE（用于错误类型）
- **错误变量**：以 `Err` 为前缀（如 `ErrMissingReference`、`ErrPortConflict`）

### 类型定义

将错误定义为包级变量：

```go
var (
    ErrInvalidName   = errors.New("invalid name")
    ErrMissingSecret = errors.New("missing secret reference")
)
```

使用 iota 定义枚举类型：

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

可序列化的结构体必须使用 yaml 标签：

```go
type Server struct {
    Name        string `yaml:"name"`
    Zone        string `yaml:"zone"`
    Description string `yaml:"description,omitempty"`
}
```

可选字段使用 `omitempty`。

### 错误处理

使用 `fmt.Errorf` 和 `%w` 包装错误，提供上下文信息：

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

return fmt.Errorf("%w: secret '%s' not found", ErrMissingReference, name)
```

验证方法应返回带有字段上下文的错误：

```go
func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", ErrInvalidName)
    }
    return nil
}
```

### 验证模式

每个实体类型应实现 `Validate() error` 方法：

```go
func (s *Service) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: service name is required", ErrInvalidName)
    }
    if s.Port <= 0 || s.Port > 65535 {
        return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidPort)
    }
    return nil
}
```

### 构造函数模式

使用 `New<Type>` 函数作为构造函数：

```go
func NewLoader(env, baseDir string) *Loader {
    return &Loader{
        env:     env,
        baseDir: baseDir,
    }
}
```

### 接口模式

为外部依赖定义接口：

```go
type Provider interface {
    Name() string
    ListRecords(domain string) ([]DNSRecord, error)
    CreateRecord(domain string, record *DNSRecord) error
}
```

### YAML 反序列化

使用自定义反序列化器支持简写和完整两种形式：

```go
func (s *SecretRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
    var plain string
    if err := unmarshal(&plain); err == nil {
        s.Plain = plain
        return nil
    }
    
    type alias SecretRef
    var ref alias
    return unmarshal(&ref)
}
```

## 配置文件

用户配置存储在 `userdata/{env}/` 目录：

- `secrets.yaml` - 密钥值
- `isps.yaml` - 服务商凭证
- `zones.yaml` - 网络区域
- `gateways.yaml` - 网关配置
- `servers.yaml` - 服务器定义
- `services.yaml` - 服务定义
- `registries.yaml` - Docker 仓库凭证
- `domains.yaml` - 域名配置
- `dns.yaml` - DNS 记录
- `certificates.yaml` - SSL 证书配置

## 关键模式

### 密钥引用模式

密钥可以指定为明文或引用：

```yaml
# 明文
password: "my-password"

# 引用 secrets.yaml
password:
  secret: db_password
```

### 服务命名规范

部署到服务器的服务使用命名模式：
`yo-{env}-{service-name}`（如 `yo-prod-api-server`）

### Docker 网络

每个环境使用独立的网络：`yamlops-{env}`

## 测试指南

编写测试时：

1. 测试文件放在源文件旁边：`internal/entities/entities_test.go`
2. 多个测试用例时使用表驱动测试
3. 同时测试成功和失败路径
4. 充分测试验证逻辑

测试示例：

```go
func TestServer_Validate(t *testing.T) {
    tests := []struct {
        name    string
        server  entities.Server
        wantErr bool
    }{
        {"valid", entities.Server{Name: "test", Zone: "zone1"}, false},
        {"missing name", entities.Server{}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.server.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Server.Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## 注意事项

- 禁止将密钥提交到代码库
- `deployments/` 目录已被忽略 - 生成的文件不应提交
- Go 版本：1.24+
- 模块路径：`github.com/litelake/yamlops`
