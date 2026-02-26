# AGENTS.md

Guidelines for AI coding agents working in the YAMLOps codebase.

## Project Overview

YAMLOps is a Go-based infrastructure operations tool that manages servers, services, DNS, and SSL certificates through YAML configurations. Supports multi-environment (prod/staging/dev/demo) with plan/apply workflow.

- **Go version**: 1.24+
- **Module path**: `github.com/lite-lake/infra-yamlops`

## Build Commands

```bash
go build -o yamlops ./cmd/yamlops       # Linux/macOS
go build -o yamlops.exe ./cmd/yamlops   # Windows
go mod tidy && go mod download          # Download dependencies
```

## Test Commands

```bash
go test ./...                                    # Run all tests
go test ./internal/domain/entity/...             # Run package tests
go test ./internal/domain/entity -run TestServer -v  # Single test, verbose
go test ./internal/domain/entity -run TestServer_Validate -v  # Specific test
go test -v -cover ./...                          # With coverage
go test -race ./...                              # With race detection
```

## Lint Commands

```bash
go fmt ./...                  # Format code
go vet ./...                  # Run go vet
staticcheck ./...             # Run staticcheck (if installed)
```

## Project Structure

```
cmd/yamlops/                    # CLI entry point
internal/
├── domain/                     # Domain layer (no external deps)
│   ├── entity/                 # Entity definitions
│   ├── valueobject/            # Value objects
│   ├── repository/             # Repository interfaces
│   ├── service/                # Domain services
│   ├── contract/               # Interface contracts (DNS, SSH, etc.)
│   └── errors.go               # Domain errors
├── application/
│   ├── handler/                # Change handlers
│   ├── usecase/                # Executor, SSHPool
│   ├── deployment/             # Deployment generators
│   ├── plan/                   # Planner coordination
│   └── orchestrator/           # Workflow orchestration
├── infrastructure/
│   ├── persistence/            # Config loader
│   ├── ssh/                    # SSH client, SFTP
│   ├── state/                  # File-based state storage
│   ├── dns/                    # DNS Factory
│   ├── secrets/                # SecretResolver
│   ├── logger/                 # Logging
│   ├── network/                # Docker network manager
│   ├── registry/               # Docker registry manager
│   └── generator/              # Compose and Gate config generators
├── interfaces/cli/             # Cobra commands, BubbleTea TUI
├── constants/                  # Shared constants
├── environment/                # Environment setup
├── version/                    # Version information
└── providers/dns/              # Cloudflare, Aliyun, Tencent DNS providers
userdata/{env}/                 # User configs (prod/staging/dev/demo)
deployments/                    # Generated files (git-ignored)
```

## Code Style

### Imports

Group imports: standard library, third-party, internal packages. Separate with blank lines.

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

### Naming Conventions

- **Packages**: lowercase, single word (`config`, `plan`, `ssh`)
- **Types**: PascalCase (exported), camelCase (internal)
- **Interfaces**: end with `-er` (`Provider`, `Loader`, `Handler`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE
- **Errors**: prefix with `Err` (`ErrInvalidName`, `ErrRequired`)

### Error Handling

Define errors in `internal/domain/errors.go`. Use `fmt.Errorf` with `%w` for wrapping:

```go
var (
    ErrInvalidName = errors.New("invalid name")
    ErrRequired    = errors.New("required field missing")
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}
```

### Struct Tags

Use yaml tags; `omitempty` for optional fields:

```go
type Server struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description,omitempty"`
}
```

### Constructors

Use `New<Type>` functions:

```go
func NewLoader(env, baseDir string) *Loader {
    return &Loader{env: env, baseDir: baseDir}
}
```

## Test Guidelines

Place tests next to source. Use table-driven tests with `errors.Is()` for error checking:

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

## Architecture Layers

| Layer | Package | Dependencies |
|-------|---------|--------------|
| Interface | interfaces/cli | → application |
| Application | application/ | → domain, infrastructure |
| Domain | domain/ | No external deps |
| Infrastructure | infrastructure/ | → domain (implements interfaces) |

## Important Notes

- Never commit secrets to the repository
- `deployments/` directory is git-ignored
- Domain layer must have no external dependencies
- Handler pattern: each entity type has a Handler implementing `Apply(ctx, change, deps)`
- Use generics for common patterns (e.g., `DoWithResult[T]`, `planSimpleEntity[T]`)
- Service naming: `yo-{env}-{service-name}` (e.g., `yo-prod-api-server`)

## Service Operations

| Operation | CLI Command | Description |
|-----------|-------------|-------------|
| Deploy | `yamlops service deploy` | Sync files, pull images, create/recreate containers |
| Stop | `yamlops service stop` | Stop containers (data preserved) |
| Restart | `yamlops service restart` | Restart containers (no file/image sync) |
| Cleanup | `yamlops service cleanup` | Remove orphan containers and directories |
