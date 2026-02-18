package entity

import (
	"errors"
	"fmt"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type ISPService string

const (
	ISPServiceServer      ISPService = "server"
	ISPServiceDomain      ISPService = "domain"
	ISPServiceDNS         ISPService = "dns"
	ISPServiceCertificate ISPService = "certificate"
)

type ISPType string

const (
	ISPTypeAliyun     ISPType = "aliyun"
	ISPTypeCloudflare ISPType = "cloudflare"
	ISPTypeTencent    ISPType = "tencent"
)

type ISP struct {
	Name        string                           `yaml:"name"`
	Type        ISPType                          `yaml:"type"`
	Services    []ISPService                     `yaml:"services"`
	Credentials map[string]valueobject.SecretRef `yaml:"credentials"`
}

func (i *ISP) Validate() error {
	if i.Name == "" {
		return fmt.Errorf("%w: isp name is required", domain.ErrInvalidName)
	}
	if i.Type == "" {
		i.Type = ISPType(i.Name)
	}
	if len(i.Services) == 0 {
		return errors.New("at least one service is required")
	}
	if len(i.Credentials) == 0 {
		return errors.New("credentials are required")
	}
	for key, ref := range i.Credentials {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("credential %s: %w", key, err)
		}
	}
	return nil
}

func (i *ISP) HasService(service ISPService) bool {
	for _, s := range i.Services {
		if s == service {
			return true
		}
	}
	return false
}
