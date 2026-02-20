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
go test ./internal/domain/entity -run TestSecretRef  # Run single test
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
│   ├── service/                # Domain services (PlannerService, Validator)
│   └── errors.go               # Domain errors
├── application/                # Application layer
│   ├── handler/                # Change handlers (Strategy Pattern)
│   └── usecase/                # Executor, SSHPool
├── infrastructure/persistence/ # Config loader
├── interfaces/cli/             # Cobra commands, BubbleTea TUI
│   ├── workflow.go             # CLI workflow orchestration
│   ├── tui_server.go           # TUI server operations
│   ├── tui_dns.go              # TUI DNS operations
│   ├── tui_cleanup.go          # TUI cleanup operations
│   └── tui_stop.go             # TUI stop operations
├── constants/                  # Shared constants
│   └── constants.go            # Application-wide constants
├── plan/                       # Planner, Compose/Gate generators
├── providers/dns/              # Cloudflare, Aliyun, Tencent DNS
│   ├── common.go               # Shared DNS logic
│   ├── factory.go              # DNS provider factory
│   └── provider.go             # Provider interface
├── providers/ssl/              # Let's Encrypt, ZeroSSL
├── ssh/                        # SSH client, SFTP
├── compose/                    # Docker Compose generator
├── gate/                       # infra-gate config generator
└── config/                     # SecretResolver
userdata/{env}/                 # User configs (prod/staging/dev)
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

Define errors as package-level variables. Wrap with `fmt.Errorf` and `%w`:

```go
var ErrInvalidName = errors.New("invalid name")

func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", ErrInvalidName)
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

### Validation Pattern

Each entity implements `Validate() error`:

```go
func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", ErrInvalidName)
    }
    return nil
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
    return unmarshal((*alias)(s))
}
```

## Test Guidelines

Place tests next to source. Use table-driven tests:

```go
func TestServer_Validate(t *testing.T) {
    tests := []struct {
        name    string
        server  Server
        wantErr bool
    }{
        {"valid", Server{Name: "test", Zone: "zone1"}, false},
        {"missing name", Server{}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.server.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
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
type DNSReader interface {
    GetDNSRecords(zone, name string) ([]DNSRecord, error)
}

type DNSWriter interface {
    CreateDNSRecord(record DNSRecord) error
    UpdateDNSRecord(record DNSRecord) error
    DeleteDNSRecord(record DNSRecord) error
}
```

### Executor Dependency Injection (DIP)

Executor receives dependencies via constructor injection:

```go
func NewExecutor(sshPool SSHPool, dnsWriter DNSWriter) *Executor {
    return &Executor{sshPool: sshPool, dnsWriter: dnsWriter}
}
```

### Unified Domain Errors

All domain errors defined in `internal/domain/errors.go`:

```go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrValidation    = errors.New("validation failed")
)
```

### Thread-Safe Registry

Registry uses `sync.RWMutex` for concurrent access:

```go
type Registry struct {
    mu    sync.RWMutex
    items map[string]Item
}

func (r *Registry) Get(name string) (Item, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    item, ok := r.items[name]
    if !ok {
        return Item{}, ErrNotFound
    }
    return item, nil
}
```

## Configuration Files

User configs in `userdata/{env}/`: secrets.yaml, isps.yaml, zones.yaml, servers.yaml, services.yaml, infra_services.yaml, registries.yaml, dns.yaml, certificates.yaml

## Important Notes

- Never commit secrets to the repository
- `deployments/` directory is git-ignored
- Domain layer must have no external dependencies
- Handler pattern: each entity type has a corresponding Handler implementing `Apply(ctx, change, deps)`
- Use generics for common patterns (e.g., `planSimpleEntity[T]`)
