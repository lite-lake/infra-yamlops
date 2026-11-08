package repository

import (
	"context"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

type StateRepository interface {
	Load(ctx context.Context, env string) (*DeploymentState, error)
	Save(ctx context.Context, env string, state *DeploymentState) error
}

type DeploymentState struct {
	Secrets       map[string]string
	Registries    map[string]*entity.Registry
	ISPs          map[string]*entity.ISP
	Zones         map[string]*entity.Zone
	Servers       map[string]*entity.Server
	InfraServices map[string]*entity.InfraService
	Services      map[string]*entity.BizService
	Domains       map[string]*entity.Domain
	Records       map[string]*entity.DNSRecord
}

func NewDeploymentState() *DeploymentState {
	return &DeploymentState{
		Secrets:       make(map[string]string),
		Registries:    make(map[string]*entity.Registry),
		ISPs:          make(map[string]*entity.ISP),
		Zones:         make(map[string]*entity.Zone),
		Servers:       make(map[string]*entity.Server),
		InfraServices: make(map[string]*entity.InfraService),
		Services:      make(map[string]*entity.BizService),
		Domains:       make(map[string]*entity.Domain),
		Records:       make(map[string]*entity.DNSRecord),
	}
}
