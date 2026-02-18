# AGENTS.md

Guidelines for AI coding agents working in the YAMLOps codebase.

## Project Overview

YAMLOps is a Go-based infrastructure operations tool that manages servers, services, DNS, and SSL certificates through YAML configurations. Supports multi-environment (prod/staging/dev) with plan/apply workflow.

## Build Commands

```bash
go build -o yamlops ./cmd/yamlops       # Linux/macOS
go build -o yamlops.exe ./cmd/yamlops   # Windows
go mod tidy && go mod download          # Download dependencies
```

## Test Commands

```bash
go test ./...                                    # Run all tests
go test ./internal/infrastructure/persistence/... # Run persistence tests
go test ./internal/domain/entity -run TestSecretRef  # Run single test
go test -v ./...                                 # Verbose output
go test -cover ./...                             # With coverage
```

## Lint Commands

```bash
go fmt ./...                            # Format code
go vet ./...                            # Run go vet
staticcheck ./...                       # Run staticcheck (if installed)
```

## Project Structure

```
cmd/yamlops/                        # CLI entry point (minimal main.go)
internal/
├── domain/                         # Domain layer (no external dependencies)
│   ├── entity/                     # Entity definitions (Server, Service, etc.)
│   ├── valueobject/                # Value objects (SecretRef, Change, Scope, Plan)
│   ├── repository/                 # Repository interfaces (StateRepository, ConfigLoader)
│   ├── service/                    # Domain services (PlannerService)
│   └── errors.go                   # Domain errors
├── application/                    # Application layer
│   ├── handler/                    # Change handlers (Strategy Pattern)
│   │   ├── types.go                # Handler interface, ApplyDeps, Result
│   │   ├── registry.go             # Handler registry
│   │   ├── dns_handler.go          # DNS record handler
│   │   ├── service_handler.go      # Service deployment handler
│   │   ├── gateway_handler.go      # Gateway config handler
│   │   ├── server_handler.go       # Server handler
│   │   ├── certificate_handler.go  # Certificate handler
│   │   ├── registry_handler.go     # Docker registry handler
│   │   └── noop_handler.go         # No-op handler for non-deployable entities
│   └── usecase/                    # Use cases
│       └── executor.go             # Orchestrates handlers
├── infrastructure/                 # Infrastructure layer
│   └── persistence/                # Persistence implementations
│       └── config_loader.go        # Config loader (generic implementation)
├── interfaces/                     # Interface layer
│   └── cli/                        # CLI commands (Cobra)
│       ├── root.go                 # Root command, global flags
│       ├── plan.go                 # Plan command
│       ├── apply.go                # Apply command
│       ├── validate.go             # Validate command
│       ├── env.go                  # Environment check/sync
│       ├── dns.go                  # DNS plan/apply commands
│       ├── dns_pull.go             # Pull domains/records from providers
│       ├── list.go                 # List entities
│       ├── show.go                 # Show entity details
│       ├── clean.go                # Clean orphan services
│       └── tui.go                  # TUI entry point
├── plan/                           # Planning coordination layer
│   ├── planner.go                  # Orchestrates planning
│   ├── generator_compose.go        # Docker Compose generation
│   └── generator_gate.go           # infra-gate config generation
├── config/                         # Config utilities
│   └── secrets.go                  # SecretResolver
├── providers/                      # External service providers
│   ├── dns/                        # Cloudflare, Aliyun, Tencent DNS
│   └── ssl/                        # Let's Encrypt, ZeroSSL
├── ssh/                            # SSH client operations
├── compose/                        # Docker Compose utilities
├── gate/                           # infra-gate utilities
└── cli/                            # BubbleTea TUI interface
userdata/{env}/                     # User configuration files (prod/staging/dev)
deployments/                        # Generated deployment files (git-ignored)
```

### Architecture Layers

| Layer | Package | Responsibility | Dependencies |
|-------|---------|----------------|--------------|
| Interface | interfaces/ | Handle CLI requests | → application |
| Application | application/ | Orchestrate use cases | → domain, infrastructure |
| Domain | domain/ | Core business logic | No external deps |
| Infrastructure | infrastructure/ | External services, persistence | → domain (implements interfaces) |

## Code Style

### Imports

Group imports in order: standard library, third-party, internal packages. Separate groups with blank lines.

```go
import (
    "errors"
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"

    "github.com/litelake/yamlops/internal/domain/entity"
)
```

### Naming Conventions

- **Packages**: lowercase, single word (`config`, `plan`, `ssh`)
- **Types**: PascalCase for exported, camelCase for internal
- **Interfaces**: typically end with `-er` (`Provider`, `Loader`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE
- **Error variables**: prefix with `Err` (`ErrInvalidName`, `ErrPortConflict`)

### Error Definitions

Define errors as package-level variables:

```go
var (
    ErrInvalidName   = errors.New("invalid name")
    ErrMissingSecret = errors.New("missing secret reference")
)
```

### Enum Types

Use iota for enums:

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

Use yaml tags for serializable structs. Use `omitempty` for optional fields:

```go
type Server struct {
    Name        string `yaml:"name"`
    Zone        string `yaml:"zone"`
    Description string `yaml:"description,omitempty"`
}
```

### Error Handling

Use `fmt.Errorf` with `%w` to wrap errors:

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
return fmt.Errorf("%w: secret '%s' not found", ErrMissingReference, name)
```

### Validation Pattern

Each entity type should implement `Validate() error`:

```go
func (s *Server) Validate() error {
    if s.Name == "" {
        return fmt.Errorf("%w: server name is required", ErrInvalidName)
    }
    return nil
}
```

### Constructor Pattern

Use `New<Type>` functions as constructors:

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
    return unmarshal((*alias)(s))
}
```

## Test Guidelines

1. Place test files next to source: `internal/domain/entity/entity_test.go`
2. Use table-driven tests for multiple cases
3. Test both success and failure paths

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

## Configuration Files

User configs in `userdata/{env}/`:
- `secrets.yaml` - Secret values
- `isps.yaml` - ISP credentials
- `zones.yaml` - Network zones
- `gateways.yaml` - Gateway configs
- `servers.yaml` - Server definitions
- `services.yaml` - Service definitions
- `registries.yaml` - Docker registry credentials
- `dns.yaml` - Domain configs with embedded DNS records
- `certificates.yaml` - SSL certificate configs

## Key Patterns

### Secret References

```yaml
password: "plain-text"      # Plain text
password: {secret: db_pass} # Reference to secrets.yaml
```

### Service Naming

Deployed services use: `yo-{env}-{service-name}` (e.g., `yo-prod-api-server`)

### Docker Networks

Each environment uses: `yamlops-{env}`

## Important Notes

- Never commit secrets to the repository
- `deployments/` directory is git-ignored
- Go version: 1.24+
- Module path: `github.com/litelake/yamlops`
