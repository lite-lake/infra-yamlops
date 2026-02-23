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
│   ├── valueobject/            # Value objects (SecretRef, Change, Scope, Plan)
│   ├── repository/             # Repository interfaces
│   ├── service/                # Domain services (Validator)
│   └── errors.go               # Domain errors
├── application/                # Application layer
│   ├── handler/                # Change handlers (Strategy Pattern)
│   ├── usecase/                # Executor, SSHPool
│   ├── deployment/             # Deployment generators (SSL, utils)
│   └── orchestrator/           # Workflow orchestration, state fetcher
├── infrastructure/
│   ├── persistence/            # Config loader
│   ├── dns/                    # DNS provider factory
│   └── state/                  # File-based state storage
├── interfaces/cli/             # Cobra commands, BubbleTea TUI
│   ├── workflow.go             # CLI workflow orchestration
│   ├── tui_server.go           # TUI server operations
│   ├── tui_dns.go              # TUI DNS operations
│   ├── tui_cleanup.go          # TUI cleanup operations
│   └── tui_stop.go             # TUI stop operations
├── constants/                  # Shared constants
│   └── constants.go            # Application-wide constants
├── environment/                # Environment setup (checker, syncer, templates)
├── plan/                       # Planner, Compose/Gate generators
├── providers/dns/              # Cloudflare, Aliyun, Tencent DNS
│   ├── provider.go             # Provider interface
│   ├── common.go               # Shared DNS logic
│   ├── cloudflare.go           # Cloudflare implementation
│   ├── aliyun.go               # Aliyun implementation
│   └── tencent.go              # Tencent implementation
├── ssh/                        # SSH client, SFTP
├── compose/                    # Docker Compose generator
├── gate/                       # infra-gate config generator
└── secrets/                    # SecretResolver
userdata/{env}/                 # User configs (prod/staging/dev/demo)
deployments/                    # Generated files (git-ignored)
```

## Code Style

### Imports

Group imports: standard library, third-party, internal packages. Separate with blank lines.

```go
import (
    "errors"
    "fmt"

    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

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
    if s.Zone == "" {
        return domain.RequiredField("zone")
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
    var ref alias
    if err := unmarshal(&ref); err != nil {
        return err
    }
    s.Plain = ref.Plain
    s.Secret = ref.Secret
    return nil
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
        {"valid", Server{Name: "server-1", Zone: "zone-1", ...}, nil},
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

### Secret References

```yaml
password: "plain-text"        # Plain text
password: {secret: db_pass}   # Reference to secrets.yaml
```

### Service Naming

Deployed services: `yo-{env}-{service-name}` (e.g., `yo-prod-api-server`)

### Docker Networks

Each environment: `yamlops-{env}` (e.g., `yamlops-prod`)

## Architecture Layers

| Layer | Package | Dependencies |
|-------|---------|--------------|
| Interface | interfaces/cli | → application |
| Application | application/ | → domain, infrastructure |
| Domain | domain/ | No external deps |
| Infrastructure | infrastructure/ | → domain (implements interfaces) |

## Architecture Improvements

### Handler Deps Interface Segregation (ISP)

Handler dependencies are split into focused interfaces:

```go
type DNSDeps interface {
    DNSProvider(ispName string) (DNSProvider, error)
    Domain(name string) (*entity.Domain, bool)
    ISP(name string) (*entity.ISP, bool)
}

type ServiceDeps interface {
    SSHClient(server string) (SSHClient, error)
    ServerInfo(name string) (*ServerInfo, bool)
    WorkDir() string
    Env() string
}

type CommonDeps interface {
    ResolveSecret(ref *valueobject.SecretRef) (string, error)
}

type DepsProvider interface {
    DNSDeps
    ServiceDeps
    CommonDeps
}
```

### Executor Dependency Injection (DIP)

Executor receives dependencies via constructor injection:

```go
func NewExecutor(cfg *ExecutorConfig) *Executor {
    if cfg.Registry == nil {
        cfg.Registry = handler.NewRegistry()
    }
    if cfg.SSHPool == nil {
        cfg.SSHPool = NewSSHPool()
    }
    if cfg.DNSFactory == nil {
        cfg.DNSFactory = infra.NewFactory()
    }
    return &Executor{
        plan:       cfg.Plan,
        registry:   cfg.Registry,
        sshPool:    cfg.SSHPool,
        dnsFactory: cfg.DNSFactory,
        ...
    }
}
```

### Unified Domain Errors

All domain errors defined in `internal/domain/errors.go`:

```go
var (
    ErrInvalidName     = errors.New("invalid name")
    ErrInvalidIP       = errors.New("invalid IP address")
    ErrInvalidPort     = errors.New("invalid port")
    ErrRequired        = errors.New("required field missing")
    ErrMissingSecret   = errors.New("missing secret reference")
    ErrPortConflict    = errors.New("port conflict")
)

func RequiredField(field string) error {
    return fmt.Errorf("%w: %s", ErrRequired, field)
}
```

### Handler Registry Pattern

Registry manages handlers by entity type:

```go
type Registry struct {
    handlers map[string]Handler
}

func (r *Registry) Register(h Handler) {
    r.handlers[h.EntityType()] = h
}

func (r *Registry) Get(entityType string) (Handler, bool) {
    h, ok := r.handlers[entityType]
    return h, ok
}
```

## Configuration Files

User configs in `userdata/{env}/`: secrets.yaml, isps.yaml, zones.yaml, servers.yaml, services_biz.yaml, services_infra.yaml, registries.yaml, dns.yaml, certificates.yaml

## Important Notes

- Never commit secrets to the repository
- `deployments/` directory is git-ignored
- Domain layer must have no external dependencies
- Handler pattern: each entity type has a corresponding Handler implementing `Apply(ctx, change, deps)`
- Use generics for common patterns (e.g., `planSimpleEntity[T]`)
