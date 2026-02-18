package domain

import "errors"

var (
	ErrInvalidName          = errors.New("invalid name")
	ErrInvalidIP            = errors.New("invalid IP address")
	ErrInvalidPort          = errors.New("invalid port")
	ErrInvalidURL           = errors.New("invalid URL")
	ErrInvalidDomain        = errors.New("invalid domain")
	ErrInvalidTTL           = errors.New("invalid TTL")
	ErrEmptyValue           = errors.New("empty value")
	ErrMissingSecret        = errors.New("missing secret reference")
	ErrInvalidDuration      = errors.New("invalid duration")
	ErrConfigNotLoaded      = errors.New("config not loaded")
	ErrMissingReference     = errors.New("missing reference")
	ErrPortConflict         = errors.New("port conflict")
	ErrDomainConflict       = errors.New("domain conflict")
	ErrHostnameConflict     = errors.New("hostname conflict")
	ErrDNSSubdomainConflict = errors.New("dns subdomain conflict")
)
