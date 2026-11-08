package contract

import (
	"context"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type SecretResolver interface {
	Resolve(ref valueobject.SecretRef) (string, error)
	ResolveAll(cfg *entity.Config) error
	GetResolvedValue(ref valueobject.SecretRef) string
}

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithContext(ctx context.Context) Logger
}

type NetworkManager interface {
	List() ([]NetworkInfo, error)
	Exists(name string) (bool, error)
	Inspect(name string) (*NetworkInfo, error)
	Create(spec *entity.ServerNetwork) error
	Ensure(spec *entity.ServerNetwork) error
	EnsureAll(networks []entity.ServerNetwork) []EnsureNetworkResult
}

type NetworkInfo struct {
	Name   string
	Driver string
	Scope  string
}

type EnsureNetworkResult struct {
	Name    string
	Success bool
	Error   error
}

type RegistryManager interface {
	EnsureLoggedIn(registryName string) (*LoginResult, error)
	LoginAll() []LoginResult
	GetRegistryURL(registryName string) (string, error)
}

type LoginResult struct {
	Name    string
	Success bool
	Message string
	Error   error
}
