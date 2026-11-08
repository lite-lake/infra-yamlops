package entity

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

type Zone struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	ISP         string `yaml:"isp,omitempty"`
	Region      string `yaml:"region"`
}

func (z *Zone) Validate() error {
	if z.Name == "" {
		return fmt.Errorf("%w: zone name is required", domain.ErrInvalidName)
	}
	if z.Region == "" {
		return domain.RequiredField("region")
	}
	return nil
}
