package ssl

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"time"

	domainerr "github.com/litelake/yamlops/internal/domain"
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
	eab          *acme.ExternalAccountBinding
}

func NewACMEClient(directoryURL string, dnsProvider DNSProvider) (*ACMEClient, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, domainerr.WrapOp("generate account key", err)
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
		return nil, domainerr.WrapOp("generate account key", err)
	}

	keyBytes, err := base64.RawURLEncoding.DecodeString(hmacKey)
	if err != nil {
		return nil, domainerr.WrapOp("decode hmac key", err)
	}

	return &ACMEClient{
		client: &acme.Client{
			Key:          key,
			DirectoryURL: directoryURL,
		},
		dnsProvider:  dnsProvider,
		directoryURL: directoryURL,
		accountKey:   key,
		eab: &acme.ExternalAccountBinding{
			KID: kid,
			Key: keyBytes,
		},
	}, nil
}

func (c *ACMEClient) RegisterAccount(ctx context.Context, termsAgreed bool) error {
	account := &acme.Account{
		Contact:                []string{},
		ExternalAccountBinding: c.eab,
	}

	_, err := c.client.Register(ctx, account, func(tosURL string) bool {
		return termsAgreed
	})
	if err != nil {
		if ae, ok := err.(*acme.Error); ok && ae.StatusCode == 409 {
			return nil
		}
		return domainerr.WrapOp("register account", err)
	}
	return nil
}

func (c *ACMEClient) ObtainCertificate(ctx context.Context, domains []string) (*Certificate, error) {
	order, err := c.client.AuthorizeOrder(ctx, acme.DomainIDs(domains...))
	if err != nil {
		return nil, domainerr.WrapOp("create order", domainerr.ErrCertObtainFailed)
	}

	for _, authzURL := range order.AuthzURLs {
		authz, err := c.client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, domainerr.WrapOp("get authorization", err)
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
			return nil, domainerr.WrapEntity("dns-01 challenge", authz.Identifier.Value, domainerr.ErrCertInvalid)
		}

		keyAuth, err := c.keyAuth(challenge.Token)
		if err != nil {
			return nil, domainerr.WrapOp("compute key authorization", err)
		}

		if err := c.dnsProvider.Present(authz.Identifier.Value, challenge.Token, keyAuth); err != nil {
			return nil, domainerr.WrapOp("present dns challenge", err)
		}

		defer c.dnsProvider.CleanUp(authz.Identifier.Value, challenge.Token, keyAuth)

		challenge, err = c.client.Accept(ctx, challenge)
		if err != nil {
			return nil, domainerr.WrapOp("accept challenge", err)
		}

		if err := c.waitForChallenge(ctx, challenge.URI); err != nil {
			return nil, domainerr.WrapOp("challenge", err)
		}

		if err := c.waitForAuthorization(ctx, authzURL); err != nil {
			return nil, domainerr.WrapOp("authorization", err)
		}
	}

	order, err = c.waitOrder(ctx, order.URI)
	if err != nil {
		return nil, domainerr.WrapOp("order", domainerr.ErrCertObtainFailed)
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, domainerr.WrapOp("generate certificate key", err)
	}

	csr, err := c.createCSR(domains, certKey)
	if err != nil {
		return nil, domainerr.WrapOp("create CSR", err)
	}

	der, _, err := c.client.CreateCert(ctx, csr, 0, true)
	if err != nil {
		return nil, domainerr.WrapOp("create certificate", domainerr.ErrCertObtainFailed)
	}

	certPEM := encodePEM(der, "CERTIFICATE")
	keyPEM, err := encodePrivateKey(certKey)
	if err != nil {
		return nil, domainerr.WrapOp("encode private key", err)
	}

	cert, err := parseCertificate(der[0])
	if err != nil {
		return nil, domainerr.WrapOp("parse certificate", domainerr.ErrCertInvalid)
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

type ACMEProvider struct {
	client       *ACMEClient
	name         string
	directoryURL string
}

func NewACMEProvider(name, directoryURL string, dnsProvider DNSProvider, eabKid, eabHmacKey string) (*ACMEProvider, error) {
	var client *ACMEClient
	var err error

	if eabKid != "" && eabHmacKey != "" {
		client, err = NewACMEClientWithEAB(directoryURL, eabKid, eabHmacKey, dnsProvider)
	} else {
		client, err = NewACMEClient(directoryURL, dnsProvider)
	}
	if err != nil {
		return nil, domainerr.WrapOp("create acme client", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.RegisterAccount(ctx, true); err != nil {
		return nil, domainerr.WrapOp("register account", err)
	}

	return &ACMEProvider{
		client:       client,
		name:         name,
		directoryURL: directoryURL,
	}, nil
}

func (p *ACMEProvider) Name() string {
	return p.name
}

func (p *ACMEProvider) ObtainCertificate(domains []string) (*Certificate, error) {
	if len(domains) == 0 {
		return nil, domainerr.WrapOp("obtain certificate", domainerr.ErrRequired)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return p.client.ObtainCertificate(ctx, domains)
}

func (p *ACMEProvider) RenewCertificate(cert *Certificate) (*Certificate, error) {
	if cert == nil {
		return nil, domainerr.WrapOp("renew certificate", domainerr.ErrCertInvalid)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	domains := []string{cert.Domain}
	return p.client.RenewCertificate(ctx, cert, domains)
}
