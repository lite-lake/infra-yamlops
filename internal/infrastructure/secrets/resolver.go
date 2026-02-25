package secrets

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type SecretResolver struct {
	secrets        map[string]string
	resolvedValues map[string]string
}

func NewSecretResolver(secrets []*entity.Secret) *SecretResolver {
	s := &SecretResolver{
		secrets:        make(map[string]string),
		resolvedValues: make(map[string]string),
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
			r.cacheResolved(ref, val)
		}
	}

	for i := range cfg.Servers {
		ref := cfg.Servers[i].SSH.Password
		val, err := r.Resolve(ref)
		if err != nil {
			return fmt.Errorf("servers[%s].ssh.password: %w", cfg.Servers[i].Name, err)
		}
		r.cacheResolved(ref, val)
	}

	for i := range cfg.Registries {
		usernameRef := cfg.Registries[i].Credentials.Username
		username, err := r.Resolve(usernameRef)
		if err != nil {
			return fmt.Errorf("registries[%s].credentials.username: %w", cfg.Registries[i].Name, err)
		}
		r.cacheResolved(usernameRef, username)

		passwordRef := cfg.Registries[i].Credentials.Password
		password, err := r.Resolve(passwordRef)
		if err != nil {
			return fmt.Errorf("registries[%s].credentials.password: %w", cfg.Registries[i].Name, err)
		}
		r.cacheResolved(passwordRef, password)
	}

	return nil
}

func (r *SecretResolver) GetResolvedValue(ref valueobject.SecretRef) string {
	key := cacheKey(ref)
	if val, ok := r.resolvedValues[key]; ok {
		return val
	}
	val, _ := r.Resolve(ref)
	return val
}

func (r *SecretResolver) cacheResolved(ref valueobject.SecretRef, val string) {
	r.resolvedValues[cacheKey(ref)] = val
}

func cacheKey(ref valueobject.SecretRef) string {
	return ref.Plain() + "|" + ref.Secret()
}
