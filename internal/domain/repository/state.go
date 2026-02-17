package repository

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/entity"
)

type StateRepository interface {
	Load(ctx context.Context, env string) (*DeploymentState, error)
	Save(ctx context.Context, env string, state *DeploymentState) error
}

type DeploymentState struct {
	Services   map[string]*entity.Service
	Gateways   map[string]*entity.Gateway
	Servers    map[string]*entity.Server
	Zones      map[string]*entity.Zone
	Domains    map[string]*entity.Domain
	Records    map[string]*entity.DNSRecord
	Certs      map[string]*entity.Certificate
	Registries map[string]*entity.Registry
	ISPs       map[string]*entity.ISP
}
