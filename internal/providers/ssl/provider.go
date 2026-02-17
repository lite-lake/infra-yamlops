package ssl

import (
	"time"
)

type Certificate struct {
	Domain      string
	Certificate []byte
	PrivateKey  []byte
	NotBefore   time.Time
	NotAfter    time.Time
}

type Provider interface {
	Name() string
	ObtainCertificate(domains []string) (*Certificate, error)
	RenewCertificate(cert *Certificate) (*Certificate, error)
}
