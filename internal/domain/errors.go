package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidName          = errors.New("invalid name")
	ErrInvalidIP            = errors.New("invalid IP address")
	ErrInvalidPort          = errors.New("invalid port")
	ErrInvalidProtocol      = errors.New("invalid protocol")
	ErrInvalidDomain        = errors.New("invalid domain")
	ErrInvalidCIDR          = errors.New("invalid CIDR")
	ErrInvalidURL           = errors.New("invalid URL")
	ErrInvalidTTL           = errors.New("invalid TTL")
	ErrInvalidDuration      = errors.New("invalid duration")
	ErrInvalidType          = errors.New("invalid type")
	ErrEmptyValue           = errors.New("empty value")
	ErrRequired             = errors.New("required field missing")
	ErrMissingSecret        = errors.New("missing secret reference")
	ErrConfigNotLoaded      = errors.New("config not loaded")
	ErrMissingReference     = errors.New("missing reference")
	ErrPortConflict         = errors.New("port conflict")
	ErrDomainConflict       = errors.New("domain conflict")
	ErrHostnameConflict     = errors.New("hostname conflict")
	ErrDNSSubdomainConflict = errors.New("dns subdomain conflict")
)

func RequiredField(field string) error {
	return fmt.Errorf("%w: %s", ErrRequired, field)
}
