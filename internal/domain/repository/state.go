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
	Services   map[string]*entity.BizService
	Gateways   map[string]*entity.Gateway
	Servers    map[string]*entity.Server
	Zones      map[string]*entity.Zone
	Domains    map[string]*entity.Domain
	Records    map[string]*entity.DNSRecord
	Certs      map[string]*entity.Certificate
	Registries map[string]*entity.Registry
	ISPs       map[string]*entity.ISP
}

func NewDeploymentState() *DeploymentState {
	return &DeploymentState{
		Services:   make(map[string]*entity.BizService),
		Gateways:   make(map[string]*entity.Gateway),
		Servers:    make(map[string]*entity.Server),
		Zones:      make(map[string]*entity.Zone),
		Domains:    make(map[string]*entity.Domain),
		Records:    make(map[string]*entity.DNSRecord),
		Certs:      make(map[string]*entity.Certificate),
		Registries: make(map[string]*entity.Registry),
		ISPs:       make(map[string]*entity.ISP),
	}
}
