package dns

import (
	"github.com/lite-lake/infra-yamlops/internal/constants"
)

type BaseProvider struct {
	defaultTTL int
}

func NewBaseProvider() *BaseProvider {
	return &BaseProvider{
		defaultTTL: constants.DefaultDNSRecordTTL,
	}
}

func (b *BaseProvider) NormalizeTTL(ttl int) int {
	if ttl == 0 {
		ttl = b.defaultTTL
	}
	return NormalizeTTL(ttl)
}

func (b *BaseProvider) SetDefaultTTL(ttl int) {
	b.defaultTTL = ttl
}
