package entity

import (
	"errors"
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
	AutoRenew   bool                `yaml:"auto_renew,omitempty"`
	RenewBefore string              `yaml:"renew_before,omitempty"`
}

func (c *Certificate) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: certificate name is required", domain.ErrInvalidName)
	}
	if len(c.Domains) == 0 {
		return errors.New("at least one domain is required")
	}
	for _, domain := range c.Domains {
		if domain == "" {
			return errors.New("domain cannot be empty")
		}
	}
	validProviders := map[CertificateProvider]bool{
		CertificateProviderLetsEncrypt: true,
		CertificateProviderZeroSSL:     true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid certificate provider: %s", c.Provider)
	}
	if c.DNSProvider == "" {
		return errors.New("dns_provider is required")
	}
	return nil
}
