package repository

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/entity"
)

type ConfigLoader interface {
	Load(ctx context.Context, env string) (*entity.Config, error)
	Validate(cfg *entity.Config) error
}
