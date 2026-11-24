package contract

import (
	"context"

	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type Handler interface {
	EntityType() string
	Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error)
}

type Result struct {
	Change   *valueobject.Change
	Success  bool
	Error    error
	Output   string
	Warnings []string
}

type DepsProvider interface {
	DNSDeps
	ServiceDeps
	CommonDeps
}

type DNSDeps interface {
	DNSProvider(ispName string) (DNSProvider, error)
}

type ServiceDeps interface {
	SSHClient(server string) (SSHClient, error)
}

type CommonDeps interface {
	ResolveSecret(ref *valueobject.SecretRef) (string, error)
}
