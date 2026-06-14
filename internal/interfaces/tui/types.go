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
	Prefix     string
}

func NewDomainDiff(name, isp, dnsisp, parent string, changeType valueobject.ChangeType) DomainDiff {
	prefix := "~"
	switch changeType {
	case valueobject.ChangeTypeCreate:
		prefix = "+"
	case valueobject.ChangeTypeDelete:
		prefix = "-"
	}
	return DomainDiff{Name: name, ISP: isp, DNSISP: dnsisp, Parent: parent, ChangeType: changeType, Prefix: prefix}
}

type RecordDiff struct {
	Domain     string
	DNSISP     string
	Type       entity.DNSRecordType
	Name       string
	Value      string
	TTL        int
	ChangeType valueobject.ChangeType
	Prefix     string
}

func NewRecordDiff(domain, dnsisp string, recType entity.DNSRecordType, name, value string, ttl int, changeType valueobject.ChangeType) RecordDiff {
	prefix := "~"
	switch changeType {
	case valueobject.ChangeTypeCreate:
		prefix = "+"
	case valueobject.ChangeTypeDelete:
		prefix = "-"
	}
	return RecordDiff{Domain: domain, DNSISP: dnsisp, Type: recType, Name: name, Value: value, TTL: ttl, ChangeType: changeType, Prefix: prefix}
}
