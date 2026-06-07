package tui

import (
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type DomainDiff struct {
	Name       string
	ISP        string
	DNSISP     string
	Parent     string
	ChangeType valueobject.ChangeType
}

type RecordDiff struct {
	Domain     string
	DNSISP     string
	Type       entity.DNSRecordType
	Name       string
	Value      string
	TTL        int
	ChangeType valueobject.ChangeType
}
