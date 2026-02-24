package dns

import (
	"fmt"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/entity"
	"github.com/litelake/yamlops/internal/domain/valueobject"
	dnsprovider "github.com/litelake/yamlops/internal/providers/dns"
)

type CreatorFunc func(isp *entity.ISP, secrets map[string]string) (dnsprovider.Provider, error)

type Factory struct {
	creators map[string]CreatorFunc
}

func NewFactory() *Factory {
	return &Factory{
		creators: map[string]CreatorFunc{
			string(entity.ISPTypeCloudflare): createCloudflare,
			string(entity.ISPTypeAliyun):     createAliyun,
			string(entity.ISPTypeTencent):    createTencent,
		},
	}
}

func (f *Factory) Create(isp *entity.ISP, secrets map[string]string) (dnsprovider.Provider, error) {
	creator, ok := f.creators[string(isp.Type)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domainerr.ErrUnsupportedProvider, isp.Type)
	}
	return creator(isp, secrets)
}

func (f *Factory) Register(providerType string, creator CreatorFunc) {
	f.creators[providerType] = creator
}

func resolveCredential(creds map[string]valueobject.SecretRef, key string, secrets map[string]string) (string, error) {
	ref, ok := creds[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", domainerr.ErrMissingCredential, key)
	}
	return ref.Resolve(secrets)
}

func createCloudflare(isp *entity.ISP, secrets map[string]string) (dnsprovider.Provider, error) {
	apiToken, err := resolveCredential(isp.Credentials, "api_token", secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve api_token: %w", err)
	}
	accountID := ""
	if accountIDRef, ok := isp.Credentials["account_id"]; ok {
		accountID, err = accountIDRef.Resolve(secrets)
		if err != nil {
			return nil, fmt.Errorf("resolve account_id: %w", err)
		}
	}
	return dnsprovider.NewCloudflareProvider(apiToken, accountID), nil
}

func createAliyun(isp *entity.ISP, secrets map[string]string) (dnsprovider.Provider, error) {
	accessKeyID, err := resolveCredential(isp.Credentials, "access_key_id", secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve access_key_id: %w", err)
	}
	accessKeySecret, err := resolveCredential(isp.Credentials, "access_key_secret", secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve access_key_secret: %w", err)
	}
	return dnsprovider.NewAliyunProvider(accessKeyID, accessKeySecret)
}

func createTencent(isp *entity.ISP, secrets map[string]string) (dnsprovider.Provider, error) {
	secretID, err := resolveCredential(isp.Credentials, "secret_id", secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve secret_id: %w", err)
	}
	secretKey, err := resolveCredential(isp.Credentials, "secret_key", secrets)
	if err != nil {
		return nil, fmt.Errorf("resolve secret_key: %w", err)
	}
	return dnsprovider.NewTencentProvider(secretID, secretKey)
}
