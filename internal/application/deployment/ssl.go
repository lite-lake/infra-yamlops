package deployment

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain/entity"
)

func (g *Generator) generateSSLConfig(infra *entity.InfraService, config *entity.Config) (string, error) {
	ssl := infra.SSLConfig
	secrets := config.GetSecretsMap()

	apiKey, err := ssl.Auth.APIKey.Resolve(secrets)
	if err != nil {
		return "", fmt.Errorf("failed to resolve apikey: %w", err)
	}

	return fmt.Sprintf(`auth:
  enabled: %t
  apikey: %s
storage:
  type: %s
  path: %s
defaults:
  issue_provider: %s
  storage_provider: %s
`, ssl.Auth.Enabled, apiKey, ssl.Storage.Type, ssl.Storage.Path, ssl.Defaults.IssueProvider, ssl.Defaults.StorageProvider), nil
}
