# AGENTS.md

在 YAMLOps 代码库中工作的 AI 编码代理指南。

## 项目概述

YAMLOps 是一个基于 Go 的基础设施运维工具，通过 YAML 配置管理服务器、服务、DNS 和 SSL 证书。支持多环境（prod/staging/dev/demo）和 plan/apply 工作流。

- **Go 版本**：1.24+
- **模块路径**：`github.com/lite-lake/infra-yamlops`

## 构建命令

```bash
go build -o yamlops ./cmd/yamlops       # Linux/macOS
go build -o yamlops.exe ./cmd/yamlops   # Windows
go mod tidy && go mod download          # 下载依赖
```

## 测试命令

```bash
go test ./...                                    # 运行所有测试
go test ./internal/domain/entity/...             # 运行包测试
go test ./internal/domain/entity -run TestServer -v  # 单个测试，详细输出
go test ./internal/domain/entity -run TestServer_Validate -v  # 特定测试
go test -v -cover ./...                          # 带覆盖率
go test -race ./...                              # 带竞争检测
```

## 检查命令

```bash
go fmt ./...                  # 格式化代码
go vet ./...                  # 运行 go vet
staticcheck ./...             # 运行 staticcheck（如果已安装）
```

## 项目结构

```
cmd/yamlops/                    # CLI 入口点
internal/
├── domain/                     # 领域层（无外部依赖）
│   ├── entity/                 # 实体定义
│   ├── valueobject/            # 值对象
│   ├── repository/             # 仓库接口
│   ├── service/                # 领域服务
│   ├── contract/               # 接口契约（DNS、SSH 等）
│   └── errors.go               # 领域错误
├── application/
│   ├── handler/                # 变更处理器
│   ├── usecase/                # 执行器、SSHPool
│   ├── deployment/             # 部署生成器
│   ├── plan/                   # 计划协调
│   └── orchestrator/           # 工作流编排
├── infrastructure/
│   ├── persistence/            # 配置加载器
│   ├── ssh/                    # SSH 客户端、SFTP
│   ├── state/                  # 基于文件的状态存储
│   ├── dns/                    # DNS 工厂
│   ├── secrets/                # SecretResolver
│   ├── logger/                 # 日志记录
│   ├── network/                # Docker 网络管理器
│   ├── registry/               # Docker 注册表管理器
│   └── generator/              # Compose 和 Gate 配置生成器
├── interfaces/cli/             # Cobra 命令、BubbleTea TUI
├── constants/                  # 共享常量
├── environment/                # 环境设置
├── version/                    # 版本信息
└── providers/dns/              # Cloudflare、阿里云、腾讯云 DNS 提供商
userdata/{env}/                 # 用户配置目录（当前仓库无示例数据，请自行创建）
deployments/                    # 生成的文件（git 忽略）
```

## 代码风格

### 导入

分组导入：标准库、第三方、内部包。用空行分隔。

```go
import (
    "context"
    "errors"
    "fmt"

    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    "github.com/lite-lake/infra-yamlops/internal/domain"
    "github.com/lite-lake/infra-yamlops/internal/domain/entity"
)
```

### 命名约定

- **包**：小写，单个单词（`config`、`plan`、`ssh`）
- **类型**：PascalCase（导出）、camelCase（内部）
- **接口**：以 `-er` 结尾（`Provider`、`Loader`、`Handler`）
- **常量**：PascalCase 或 UPPER_SNAKE_CASE
- **错误**：以 `Err` 为前缀（`ErrInvalidName`、`ErrRequired`）

### 错误处理

在 `internal/domain/errors.go` 中定义错误。使用 `fmt.Errorf` 和 `%w` 包装：

```go
var (
    ErrInvalidName = errors.New("invalid name")
    ErrRequired    = errors.New("required field missing")
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}
```

### 结构体标签

使用 yaml 标签；可选字段使用 `omitempty`：

```go
type Server struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description,omitempty"`
}
```

### 构造函数

使用 `New<Type>` 函数：

```go
func NewLoader(env, baseDir string) *Loader {
    return &Loader{env: env, baseDir: baseDir}
}
```

## 测试指南

将测试放在源文件旁边。使用表驱动测试和 `errors.Is()` 进行错误检查：

```go
func TestServer_Validate(t *testing.T) {
    tests := []struct {
        name    string
        server  Server
        wantErr error
    }{
        {"missing name", Server{}, domain.ErrInvalidName},
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

## 架构层

| 层 | 包 | 依赖 |
|-------|---------|--------------|
| 接口 | interfaces/cli | → application |
| 应用 | application/ | → domain, infrastructure |
| 领域 | domain/ | 无外部依赖 |
| 基础设施 | infrastructure/ | → domain（实现接口） |

## 重要注意事项

- 永远不要将密钥提交到仓库
- `deployments/` 目录被 git 忽略
- 领域层必须没有外部依赖
- Handler 模式：每个实体类型都有一个实现 `Apply(ctx, change, deps)` 的 Handler
- 对通用模式使用泛型（例如：`DoWithResult[T]`、`planSimpleEntity[T]`）
- 服务命名：`yo-{env}-{service-name}`（例如：`yo-prod-api-server`）

## 业务服务 (BizService) 字段参考

定义在 `internal/domain/entity/biz_service.go`。

| 字段 | YAML 标签 | 类型 | 必填 | 说明 |
|------|-----------|------|------|------|
| name | name | string | ✅ | 服务名称 |
| server | server | string | ✅ | 部署目标服务器 |
| image | image | string | ⚠️ | Docker 镜像（与 external_backends 二选一） |
| external_backends | external_backends | []string | ⚠️ | 外部后端 URL（与 image 二选一） |
| registry | registry | string | ❌ | Docker Registry 名称 |
| ports | ports | []ServicePort | ❌ | 端口映射 |
| env | env | map[string]SecretRef | ❌ | 环境变量 |
| volumes | volumes | []ServiceVolume | ❌ | 卷挂载 |
| healthcheck | healthcheck | ServiceHealthcheck | ❌ | 健康检查配置 |
| gateways | gateways | []ServiceGatewayRoute | ❌ | 网关路由 |
| internal | internal | bool | ❌ | 是否内部服务 |

### image 与 external_backends 互斥规则

- **二选一**：`image` 和 `external_backends` 不能同时指定，也不能都不指定
- 使用 `image` 时：yamlops 生成 Docker Compose 并部署本地容器
- 使用 `external_backends` 时：无本地容器，infra-gate 直接反向代理到外部 URL；健康检查自动禁用

### ServiceGatewayRoute 字段

| 字段 | YAML 标签 | 类型 | 默认值 | 说明 |
|------|-----------|------|--------|------|
| hostname | hostname | string | (必填) | 域名 |
| container_port | container_port | int | 80 | 容器端口 |
| path | path | string | / | 路由路径 |
| http | http | bool | false | 启用 HTTP |
| https | https | bool | false | 启用 HTTPS |
| gzip_enabled | gzip_enabled | *bool | nil(继承全局) | 启用 gzip |
| override_host | override_host | string | (空) | 覆盖发给上游的 Host 头 |
| strip_proxy_headers | strip_proxy_headers | *bool | nil(false) | 剥离所有代理相关头部 |

### strip_proxy_headers 说明

当设置为 `true` 时，infra-gate 不会添加以下代理相关头部：
- `X-Forwarded-For`
- `X-Forwarded-Proto`
- `X-Forwarded-Host`
- `X-Forwarded-Port`
- `Forwarded` (RFC 7239)
- `X-Lite-Gate-Forwarded-For`
- `X-Lite-Gate-Forwarded-Host`
- `X-Lite-Gate-Trace-Id`
- `Via`

适用场景：需要伪装成客户端直接请求，不暴露代理身份的场景（如反向代理到第三方 API）。

示例配置：
```yaml
services:
  - name: xiaomi-llm-proxy
    server: demo-server
    external_backends:
      - https://api.xiaomi.com
    gateways:
      - hostname: llm.example.com
        https: true
        strip_proxy_headers: true  # 禁用所有代理头部
```

## 服务操作

| 操作 | CLI 命令 | 描述 |
|-----------|-------------|-------------|
| 部署 | `yamlops service deploy` | 同步文件、拉取镜像、创建/重建容器 |
| 停止 | `yamlops service stop` | 停止容器（数据保留） |
| 重启 | `yamlops service restart` | 重启容器（不同步文件/镜像） |
| 清理 | `yamlops service cleanup` | 移除孤立容器和目录 |

## 注意事项

- **Mac 系统**：在调用任何 skill 之前记得先 `source ~/.zshrc`
