package config

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type SecretResolver struct {
	secrets map[string]string
}

func NewSecretResolver(secrets []*entity.Secret) *SecretResolver {
	s := &SecretResolver{
		secrets: make(map[string]string),
	}
	for _, secret := range secrets {
		s.secrets[secret.Name] = secret.Value
	}
	return s
}

func (r *SecretResolver) Resolve(ref valueobject.SecretRef) (string, error) {
	return ref.Resolve(r.secrets)
}

func (r *SecretResolver) ResolveAll(cfg *entity.Config) error {
	for i := range cfg.ISPs {
		for key, ref := range cfg.ISPs[i].Credentials {
			val, err := r.Resolve(ref)
			if err != nil {
				return fmt.Errorf("isps[%s].credentials[%s]: %w", cfg.ISPs[i].Name, key, err)
			}
			cfg.ISPs[i].Credentials[key] = valueobject.SecretRef{Plain: val}
		}
	}

	for i := range cfg.Servers {
		val, err := r.Resolve(cfg.Servers[i].SSH.Password)
		if err != nil {
			return fmt.Errorf("servers[%s].ssh.password: %w", cfg.Servers[i].Name, err)
		}
		cfg.Servers[i].SSH.Password = valueobject.SecretRef{Plain: val}
	}

	for i := range cfg.Registries {
		username, err := r.Resolve(cfg.Registries[i].Credentials.Username)
		if err != nil {
			return fmt.Errorf("registries[%s].credentials.username: %w", cfg.Registries[i].Name, err)
		}
		password, err := r.Resolve(cfg.Registries[i].Credentials.Password)
		if err != nil {
			return fmt.Errorf("registries[%s].credentials.password: %w", cfg.Registries[i].Name, err)
		}
		cfg.Registries[i].Credentials.Username = valueobject.SecretRef{Plain: username}
		cfg.Registries[i].Credentials.Password = valueobject.SecretRef{Plain: password}
	}

	return nil
}
