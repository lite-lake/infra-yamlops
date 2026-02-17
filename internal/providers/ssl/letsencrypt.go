package ssl

import (
	"context"
	"fmt"
	"time"
)

const (
	LetsEncryptProductionURL = "https://acme-v02.api.letsencrypt.org/directory"
	LetsEncryptStagingURL    = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

type LetsEncryptProvider struct {
	client *ACMEClient
}

func NewLetsEncryptProvider(dnsProvider DNSProvider) (*LetsEncryptProvider, error) {
	return NewLetsEncryptProviderWithDirectory(dnsProvider, LetsEncryptProductionURL)
}

func NewLetsEncryptProviderWithDirectory(dnsProvider DNSProvider, directoryURL string) (*LetsEncryptProvider, error) {
	client, err := NewACMEClient(directoryURL, dnsProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create acme client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.RegisterAccount(ctx, true); err != nil {
		return nil, fmt.Errorf("failed to register account: %w", err)
	}

	return &LetsEncryptProvider{client: client}, nil
}

func (p *LetsEncryptProvider) Name() string {
	return "letsencrypt"
}

func (p *LetsEncryptProvider) ObtainCertificate(domains []string) (*Certificate, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return p.client.ObtainCertificate(ctx, domains)
}

func (p *LetsEncryptProvider) RenewCertificate(cert *Certificate) (*Certificate, error) {
	if cert == nil {
		return nil, fmt.Errorf("certificate is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	domains := []string{cert.Domain}
	return p.client.RenewCertificate(ctx, cert, domains)
}
