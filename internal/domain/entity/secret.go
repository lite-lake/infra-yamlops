package entity

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

type Secret struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func (s *Secret) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidName)
	}
	return nil
}
