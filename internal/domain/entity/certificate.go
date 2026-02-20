package entity

import (
	"fmt"

	"github.com/litelake/yamlops/internal/domain"
)

type CertificateProvider string

const (
	CertificateProviderLetsEncrypt CertificateProvider = "letsencrypt"
	CertificateProviderZeroSSL     CertificateProvider = "zerossl"
)

type Certificate struct {
	Name        string              `yaml:"name"`
	Domains     []string            `yaml:"domains"`
	Provider    CertificateProvider `yaml:"provider"`
	DNSProvider string              `yaml:"dns_provider"`
	RenewBefore string              `yaml:"renew_before,omitempty"`
}

func (c *Certificate) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: certificate name is required", domain.ErrInvalidName)
	}
	if len(c.Domains) == 0 {
		return domain.RequiredField("domains")
	}
	for _, d := range c.Domains {
		if d == "" {
			return fmt.Errorf("%w: domain cannot be empty", domain.ErrEmptyValue)
		}
	}
	validProviders := map[CertificateProvider]bool{
		CertificateProviderLetsEncrypt: true,
		CertificateProviderZeroSSL:     true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("%w: certificate provider %s", domain.ErrInvalidType, c.Provider)
	}
	if c.DNSProvider == "" {
		return domain.RequiredField("dns_provider")
	}
	return nil
}
