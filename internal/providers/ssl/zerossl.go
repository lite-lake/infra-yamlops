package ssl

import (
	"context"
	"fmt"
	"time"
)

const (
	ZeroSSLProductionURL = "https://acme.zerossl.com/v2/DV90"
)

type ZeroSSLProvider struct {
	client     *ACMEClient
	eabKid     string
	eabHmacKey string
}

func NewZeroSSLProvider(dnsProvider DNSProvider, eabKid, eabHmacKey string) (*ZeroSSLProvider, error) {
	return NewZeroSSLProviderWithDirectory(dnsProvider, eabKid, eabHmacKey, ZeroSSLProductionURL)
}

func NewZeroSSLProviderWithDirectory(dnsProvider DNSProvider, eabKid, eabHmacKey, directoryURL string) (*ZeroSSLProvider, error) {
	if eabKid == "" {
		return nil, fmt.Errorf("eab kid is required for zerossl")
	}
	if eabHmacKey == "" {
		return nil, fmt.Errorf("eab hmac key is required for zerossl")
	}

	client, err := NewACMEClientWithEAB(directoryURL, eabKid, eabHmacKey, dnsProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create acme client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.RegisterAccount(ctx, true); err != nil {
		return nil, fmt.Errorf("failed to register account: %w", err)
	}

	return &ZeroSSLProvider{
		client:     client,
		eabKid:     eabKid,
		eabHmacKey: eabHmacKey,
	}, nil
}

func (p *ZeroSSLProvider) Name() string {
	return "zerossl"
}

func (p *ZeroSSLProvider) ObtainCertificate(domains []string) (*Certificate, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains provided")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return p.client.ObtainCertificate(ctx, domains)
}

func (p *ZeroSSLProvider) RenewCertificate(cert *Certificate) (*Certificate, error) {
	if cert == nil {
		return nil, fmt.Errorf("certificate is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	domains := []string{cert.Domain}
	return p.client.RenewCertificate(ctx, cert, domains)
}
