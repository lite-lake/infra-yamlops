package dns

import (
	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/contract"
)

var (
	ErrDomainNotFound  = domain.ErrDNSDomainNotFound
	ErrRecordNotFound  = domain.ErrDNSRecordNotFound
	ErrInvalidResponse = domain.ErrDNSError
)

type DNSRecord = contract.DNSRecord

type Provider = contract.DNSProvider
