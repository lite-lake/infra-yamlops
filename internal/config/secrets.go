package config

import (
	"fmt"

	"github.com/litelake/yamlops/internal/entities"
)

type SecretResolver struct {
	secrets map[string]string
}

func NewSecretResolver(secrets []*entities.Secret) *SecretResolver {
	s := &SecretResolver{
		secrets: make(map[string]string),
	}
	for _, secret := range secrets {
		s.secrets[secret.Name] = secret.Value
	}
	return s
}

func (r *SecretResolver) Resolve(ref entities.SecretRef) (string, error) {
	return ref.Resolve(r.secrets)
}

func (r *SecretResolver) ResolveAll(cfg *entities.Config) error {
	for i := range cfg.ISPs {
		for key, ref := range cfg.ISPs[i].Credentials {
			val, err := r.Resolve(ref)
			if err != nil {
				return fmt.Errorf("isps[%s].credentials[%s]: %w", cfg.ISPs[i].Name, key, err)
			}
			cfg.ISPs[i].Credentials[key] = entities.SecretRef{Plain: val}
		}
	}

	for i := range cfg.Servers {
		val, err := r.Resolve(cfg.Servers[i].SSH.Password)
		if err != nil {
			return fmt.Errorf("servers[%s].ssh.password: %w", cfg.Servers[i].Name, err)
		}
		cfg.Servers[i].SSH.Password = entities.SecretRef{Plain: val}
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
		cfg.Registries[i].Credentials.Username = entities.SecretRef{Plain: username}
		cfg.Registries[i].Credentials.Password = entities.SecretRef{Plain: password}
	}

	return nil
}
