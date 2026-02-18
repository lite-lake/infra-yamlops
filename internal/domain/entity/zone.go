package entity

import (
	"errors"
	"fmt"

	"github.com/litelake/yamlops/internal/domain"
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
		return errors.New("region is required")
	}
	return nil
}
