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

	ErrNetworkTimeout     = errors.New("network timeout")
	ErrNetworkUnreachable = errors.New("network unreachable")
	ErrDNSResolveFailed   = errors.New("DNS resolution failed")
	ErrConnectionRefused  = errors.New("connection refused")
	ErrConnectionReset    = errors.New("connection reset")

	ErrSSHConnectFailed   = errors.New("SSH connection failed")
	ErrSSHAuthFailed      = errors.New("SSH authentication failed")
	ErrSSHSessionFailed   = errors.New("SSH session creation failed")
	ErrSSHCommandFailed   = errors.New("SSH command execution failed")
	ErrSSHHostKeyMismatch = errors.New("SSH host key mismatch")
	ErrSSHFileTransfer    = errors.New("SSH file transfer failed")

	ErrConfigReadFailed   = errors.New("config read failed")
	ErrConfigParseFailed  = errors.New("config parse failed")
	ErrConfigValidateFail = errors.New("config validation failed")
	ErrConfigNotFound     = errors.New("config not found")

	ErrStateReadFailed    = errors.New("state read failed")
	ErrStateWriteFailed   = errors.New("state write failed")
	ErrStateSerializeFail = errors.New("state serialization failed")
	ErrStateNotFound      = errors.New("state not found")

	ErrCertObtainFailed = errors.New("certificate obtain failed")
	ErrCertRenewFailed  = errors.New("certificate renew failed")
	ErrCertExpired      = errors.New("certificate expired")
	ErrCertInvalid      = errors.New("certificate invalid")

	ErrDNSError          = errors.New("DNS operation failed")
	ErrDNSRecordExists   = errors.New("DNS record already exists")
	ErrDNSRecordNotFound = errors.New("DNS record not found")
	ErrDNSDomainNotFound = errors.New("DNS domain not found")
)

func RequiredField(field string) error {
	return fmt.Errorf("%w: %s", ErrRequired, field)
}

func WrapOp(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

func WrapEntity(entity, name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s[%s]: %w", entity, name, err)
}

type OpError struct {
	Op    string
	Cause error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Cause)
}

func (e *OpError) Unwrap() error {
	return e.Cause
}

func NewOpError(op string, cause error) error {
	return &OpError{Op: op, Cause: cause}
}
