package valueobject

import (
	"fmt"
	"log/slog"

	"github.com/litelake/yamlops/internal/domain"
)

type SecretRef struct {
	plain  string `yaml:"plain,omitempty"`
	secret string `yaml:"secret,omitempty"`
}

func NewSecretRef(plain, secret string) *SecretRef {
	return &SecretRef{plain: plain, secret: secret}
}

func NewSecretRefPlain(plain string) *SecretRef {
	return &SecretRef{plain: plain}
}

func NewSecretRefSecret(secret string) *SecretRef {
	return &SecretRef{secret: secret}
}

func (s *SecretRef) Plain() string  { return s.plain }
func (s *SecretRef) Secret() string { return s.secret }

func (s *SecretRef) Equals(other *SecretRef) bool {
	if other == nil {
		return false
	}
	return s.plain == other.plain && s.secret == other.secret
}

func (s *SecretRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var plain string
	if err := unmarshal(&plain); err == nil {
		s.plain = plain
		return nil
	}

	var ref struct {
		Plain  string `yaml:"plain,omitempty"`
		Secret string `yaml:"secret,omitempty"`
	}
	if err := unmarshal(&ref); err != nil {
		return err
	}
	s.plain = ref.Plain
	s.secret = ref.Secret
	return nil
}

func (s *SecretRef) MarshalYAML() (interface{}, error) {
	if s.secret != "" {
		return map[string]string{"secret": s.secret}, nil
	}
	return s.plain, nil
}

func (s *SecretRef) Resolve(secrets map[string]string) (string, error) {
	if s.secret != "" {
		val, ok := secrets[s.secret]
		if !ok {
			return "", fmt.Errorf("%w: %s", domain.ErrMissingSecret, s.secret)
		}
		return val, nil
	}
	return s.plain, nil
}

func (s *SecretRef) Validate() error {
	if s.plain == "" && s.secret == "" {
		return domain.ErrEmptyValue
	}
	return nil
}

func (s *SecretRef) LogValue() slog.Value {
	return slog.StringValue("***")
}
