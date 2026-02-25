# AGENTS.md

Guidelines for AI coding agents working in the YAMLOps codebase.

## Project Overview

YAMLOps is a Go-based infrastructure operations tool that manages servers, services, DNS, and SSL certificates through YAML configurations. Supports multi-environment (prod/staging/dev) with plan/apply workflow.

- **Go version**: 1.24+
- **Module path**: `github.com/litelake/yamlops`

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
│   ├── entity/                 # Entity definitions (server, zone, isp, secret, registry, domain, dns_record, biz_service, infra_service)
│   ├── valueobject/            # Value objects (SecretRef, Change, Scope, Plan)
│   ├── repository/             # Repository interfaces (ConfigLoader, StateRepository)
│   ├── service/                # Domain services (Validator, DifferService)
│   ├── retry/                  # Retry mechanism with Option pattern
│   └── errors.go               # Domain errors
├── application/
│   ├── handler/                # Change handlers (Strategy Pattern)
│   ├── usecase/                # Executor, SSHPool
│   └── deployment/             # Deployment generators (compose, gateway)
├── infrastructure/
│   ├── persistence/            # Config loader
│   ├── ssh/                    # SSH client, SFTP, shell_escape
│   ├── state/                  # File-based state storage
│   ├── secrets/                # SecretResolver
│   └── logger/                 # Logging infrastructure
├── interfaces/cli/             # Cobra commands, BubbleTea TUI
├── constants/                  # Shared constants
├── environment/                # Environment setup with embedded templates
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

    "github.com/litelake/yamlops/internal/domain"
    "github.com/litelake/yamlops/internal/domain/entity"
)
```

### Naming Conventions

- **Packages**: lowercase, single word (`config`, `plan`, `ssh`)
- **Types**: PascalCase (exported), camelCase (internal)
- **Interfaces**: end with `-er` (`Provider`, `Loader`, `Handler`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE
- **Errors**: prefix with `Err` (`ErrInvalidName`, `ErrPortConflict`)

### Error Handling

Define errors in `internal/domain/errors.go`. Use `fmt.Errorf` with `%w` for wrapping:

```go
// In internal/domain/errors.go
var (
    ErrInvalidName = errors.New("invalid name")
    ErrRequired    = errors.New("required field missing")
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}

// In entity validation
func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", domain.ErrInvalidName)
    }
    return nil
}
```

### Enums

Use iota with explicit type:

```go
type ChangeType int

const (
    ChangeTypeNoop ChangeType = iota
    ChangeTypeCreate
    ChangeTypeUpdate
    ChangeTypeDelete
)
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

### Option Pattern

Use for configurable constructors:

```go
type Option func(*Config)

func WithMaxAttempts(n int) Option {
    return func(c *Config) { c.MaxAttempts = n }
}

func DefaultConfig() *Config {
    return &Config{MaxAttempts: 3}
}

// Usage: cfg := DefaultConfig(); for _, opt := range opts { opt(cfg) }
```

### YAML Custom Deserialization

Support both shorthand and full forms:

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

## Key Patterns

### Handler Registry

```go
type Handler interface {
    EntityType() string
    Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

type Registry struct {
    handlers map[string]Handler
}

func (r *Registry) Register(h Handler) { r.handlers[h.EntityType()] = h }
func (r *Registry) Get(entityType string) (Handler, bool) { ... }
```

### Dependency Injection

Handler dependencies use interface segregation:

```go
type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}
```

### Fluent Builder Pattern

```go
func (c *Change) WithOldState(state interface{}) *Change {
    c.OldState = state
    return c
}
// Usage: NewChange(ChangeTypeCreate, "server", "s1").WithOldState(old).WithActions("create")
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
