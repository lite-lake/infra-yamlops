package ssl

import (
	domainerr "github.com/litelake/yamlops/internal/domain"
)

const (
	ZeroSSLProductionURL = "https://acme.zerossl.com/v2/DV90"
)

func NewZeroSSLProvider(dnsProvider DNSProvider, eabKid, eabHmacKey string) (*ACMEProvider, error) {
	return NewZeroSSLProviderWithDirectory(dnsProvider, eabKid, eabHmacKey, ZeroSSLProductionURL)
}

func NewZeroSSLProviderWithDirectory(dnsProvider DNSProvider, eabKid, eabHmacKey, directoryURL string) (*ACMEProvider, error) {
	if eabKid == "" {
		return nil, domainerr.RequiredField("eab kid")
	}
	if eabHmacKey == "" {
		return nil, domainerr.RequiredField("eab hmac key")
	}

	return NewACMEProvider("zerossl", directoryURL, dnsProvider, eabKid, eabHmacKey)
}
