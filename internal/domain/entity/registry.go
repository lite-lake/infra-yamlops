package entity

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type RegistryCredentials struct {
	Username valueobject.SecretRef `yaml:"username"`
	Password valueobject.SecretRef `yaml:"password"`
}

func (c *RegistryCredentials) Validate() error {
	if err := c.Username.Validate(); err != nil {
		return fmt.Errorf("username: %w", err)
	}
	if err := c.Password.Validate(); err != nil {
		return fmt.Errorf("password: %w", err)
	}
	return nil
}

type Registry struct {
	Name        string              `yaml:"name"`
	URL         string              `yaml:"url"`
	Credentials RegistryCredentials `yaml:"credentials"`
}

func (r *Registry) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("%w: registry name is required", domain.ErrInvalidName)
	}
	if r.URL == "" {
		return domain.RequiredField("url")
	}
	if err := r.Credentials.Validate(); err != nil {
		return err
	}
	return nil
}
