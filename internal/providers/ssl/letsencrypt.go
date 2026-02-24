package ssl

const (
	LetsEncryptProductionURL = "https://acme-v02.api.letsencrypt.org/directory"
	LetsEncryptStagingURL    = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

func NewLetsEncryptProvider(dnsProvider DNSProvider) (*ACMEProvider, error) {
	return NewLetsEncryptProviderWithDirectory(dnsProvider, LetsEncryptProductionURL)
}

func NewLetsEncryptProviderWithDirectory(dnsProvider DNSProvider, directoryURL string) (*ACMEProvider, error) {
	return NewACMEProvider("letsencrypt", directoryURL, dnsProvider, "", "")
}
