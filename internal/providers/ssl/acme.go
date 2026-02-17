package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
)

type DNSProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}

type ACMEClient struct {
	client       *acme.Client
	dnsProvider  DNSProvider
	directoryURL string
	accountKey   crypto.Signer
}

func NewACMEClient(directoryURL string, dnsProvider DNSProvider) (*ACMEClient, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	return &ACMEClient{
		client: &acme.Client{
			Key:          key,
			DirectoryURL: directoryURL,
		},
		dnsProvider:  dnsProvider,
		directoryURL: directoryURL,
		accountKey:   key,
	}, nil
}

func NewACMEClientWithEAB(directoryURL, kid, hmacKey string, dnsProvider DNSProvider) (*ACMEClient, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	_, err = base64.RawURLEncoding.DecodeString(hmacKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hmac key: %w", err)
	}

	return &ACMEClient{
		client: &acme.Client{
			Key:          key,
			DirectoryURL: directoryURL,
		},
		dnsProvider:  dnsProvider,
		directoryURL: directoryURL,
		accountKey:   key,
	}, nil
}

func (c *ACMEClient) RegisterAccount(ctx context.Context, termsAgreed bool) error {
	account := &acme.Account{
		Contact:                []string{},
		ExternalAccountBinding: nil,
	}

	_, err := c.client.Register(ctx, account, func(tosURL string) bool {
		return termsAgreed
	})
	if err != nil {
		if ae, ok := err.(*acme.Error); ok && ae.StatusCode == 409 {
			return nil
		}
		return fmt.Errorf("failed to register account: %w", err)
	}
	return nil
}

func (c *ACMEClient) ObtainCertificate(ctx context.Context, domains []string) (*Certificate, error) {
	order, err := c.client.AuthorizeOrder(ctx, acme.DomainIDs(domains...))
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	for _, authzURL := range order.AuthzURLs {
		authz, err := c.client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get authorization: %w", err)
		}

		if authz.Status == acme.StatusValid {
			continue
		}

		var challenge *acme.Challenge
		for _, ch := range authz.Challenges {
			if ch.Type == "dns-01" {
				challenge = ch
				break
			}
		}

		if challenge == nil {
			return nil, fmt.Errorf("no dns-01 challenge found for domain %s", authz.Identifier.Value)
		}

		keyAuth, err := c.keyAuth(challenge.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to compute key authorization: %w", err)
		}

		if err := c.dnsProvider.Present(authz.Identifier.Value, challenge.Token, keyAuth); err != nil {
			return nil, fmt.Errorf("failed to present dns challenge: %w", err)
		}

		defer c.dnsProvider.CleanUp(authz.Identifier.Value, challenge.Token, keyAuth)

		challenge, err = c.client.Accept(ctx, challenge)
		if err != nil {
			return nil, fmt.Errorf("failed to accept challenge: %w", err)
		}

		if err := c.waitForChallenge(ctx, challenge.URI); err != nil {
			return nil, fmt.Errorf("challenge failed: %w", err)
		}

		if err := c.waitForAuthorization(ctx, authzURL); err != nil {
			return nil, fmt.Errorf("authorization failed: %w", err)
		}
	}

	order, err = c.waitOrder(ctx, order.URI)
	if err != nil {
		return nil, fmt.Errorf("order failed: %w", err)
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate key: %w", err)
	}

	csr, err := c.createCSR(domains, certKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	der, _, err := c.client.CreateCert(ctx, csr, 0, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := encodePEM(der, "CERTIFICATE")
	keyPEM, err := encodePrivateKey(certKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	cert, err := parseCertificate(der[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &Certificate{
		Domain:      domains[0],
		Certificate: certPEM,
		PrivateKey:  keyPEM,
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
	}, nil
}

func (c *ACMEClient) RenewCertificate(ctx context.Context, oldCert *Certificate, domains []string) (*Certificate, error) {
	return c.ObtainCertificate(ctx, domains)
}

func (c *ACMEClient) keyAuth(token string) (string, error) {
	thumbprint, err := acme.JWKThumbprint(c.client.Key.Public())
	if err != nil {
		return "", err
	}

	return token + "." + thumbprint, nil
}

func dns01KeyAuth(keyAuth string) string {
	h := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (c *ACMEClient) waitForChallenge(ctx context.Context, uri string) error {
	for {
		challenge, err := c.client.GetChallenge(ctx, uri)
		if err != nil {
			return err
		}

		switch challenge.Status {
		case acme.StatusValid:
			return nil
		case acme.StatusInvalid:
			return fmt.Errorf("challenge invalid: %s", challenge.Error.Error())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *ACMEClient) waitForAuthorization(ctx context.Context, uri string) error {
	for {
		authz, err := c.client.GetAuthorization(ctx, uri)
		if err != nil {
			return err
		}

		switch authz.Status {
		case acme.StatusValid:
			return nil
		case acme.StatusInvalid:
			return fmt.Errorf("authorization invalid")
		case acme.StatusRevoked:
			return fmt.Errorf("authorization revoked")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *ACMEClient) waitOrder(ctx context.Context, uri string) (*acme.Order, error) {
	for {
		order, err := c.client.WaitOrder(ctx, uri)
		if err != nil {
			return nil, err
		}

		switch order.Status {
		case acme.StatusValid:
			return order, nil
		case acme.StatusInvalid:
			return nil, fmt.Errorf("order invalid")
		case acme.StatusReady:
			return order, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (c *ACMEClient) createCSR(domains []string, key crypto.Signer) ([]byte, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: domains[0]},
	}

	if len(domains) > 1 {
		var san []string
		var ips []net.IP
		for _, d := range domains {
			if ip := net.ParseIP(d); ip != nil {
				ips = append(ips, ip)
			} else {
				san = append(san, d)
			}
		}
		template.DNSNames = san
		template.IPAddresses = ips
	} else {
		if ip := net.ParseIP(domains[0]); ip != nil {
			template.IPAddresses = []net.IP{ip}
		} else {
			template.DNSNames = domains
		}
	}

	return x509.CreateCertificateRequest(rand.Reader, template, key)
}

func encodePEM(der [][]byte, blockType string) []byte {
	var buf []byte
	for _, b := range der {
		buf = append(buf, pem.EncodeToMemory(&pem.Block{
			Type:  blockType,
			Bytes: b,
		})...)
	}
	return buf
}

func encodePrivateKey(key crypto.Signer) ([]byte, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: b,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}
}

func parseCertificate(der []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(der)
}

func extractBaseDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}
