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
go test ./...                           # Run all tests
go test ./internal/config/...           # Run specific package tests
go test ./internal/entities -run TestSecretRef  # Run single test
go test -v ./...                        # Verbose output
go test -cover ./...                    # With coverage
```

## Lint Commands

```bash
go fmt ./...                            # Format code
go vet ./...                            # Run go vet
staticcheck ./...                       # Run staticcheck (if installed)
```

## Project Structure

```
cmd/yamlops/            # CLI entry point
internal/
├── cli/                # BubbleTea TUI interface
├── config/             # Config loading and validation
├── entities/           # Entity definitions and schemas
├── plan/               # Change planning logic
├── apply/              # Change execution
├── ssh/                # SSH client operations
├── compose/            # Docker Compose generation
├── gate/               # infra-gate config generation
└── providers/          # External service providers
    ├── dns/            # Cloudflare, Aliyun, Tencent DNS
    └── ssl/            # Let's Encrypt, ZeroSSL
userdata/{env}/         # User configuration files (prod/staging/dev)
deployments/            # Generated deployment files (git-ignored)
```

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

    "github.com/litelake/yamlops/internal/entities"
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

1. Place test files next to source: `internal/entities/entities_test.go`
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
- `domains.yaml` - Domain configs
- `dns.yaml` - DNS records
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
