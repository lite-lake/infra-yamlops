package valueobject

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain"
)

type SecretRef struct {
	Plain  string `yaml:"plain,omitempty"`
	Secret string `yaml:"secret,omitempty"`
}

func (s *SecretRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var plain string
	if err := unmarshal(&plain); err == nil {
		s.Plain = plain
		return nil
	}

	type alias SecretRef
	var ref alias
	if err := unmarshal(&ref); err != nil {
		return err
	}
	s.Plain = ref.Plain
	s.Secret = ref.Secret
	return nil
}

func (s *SecretRef) MarshalYAML() (interface{}, error) {
	if s.Secret != "" {
		return map[string]string{"secret": s.Secret}, nil
	}
	return s.Plain, nil
}

func (s *SecretRef) Resolve(secrets map[string]string) (string, error) {
	if s.Secret != "" {
		val, ok := secrets[s.Secret]
		if !ok {
			return "", fmt.Errorf("%w: %s", domain.ErrMissingSecret, s.Secret)
		}
		return val, nil
	}
	return s.Plain, nil
}

func (s *SecretRef) Validate() error {
	if s.Plain == "" && s.Secret == "" {
		return domain.ErrEmptyValue
	}
	return nil
}
