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

	ErrSSHConnectFailed      = errors.New("SSH connection failed")
	ErrSSHAuthFailed         = errors.New("SSH authentication failed")
	ErrSSHSessionFailed      = errors.New("SSH session creation failed")
	ErrSSHCommandFailed      = errors.New("SSH command execution failed")
	ErrSSHHostKeyMismatch    = errors.New("SSH host key mismatch")
	ErrSSHFileTransfer       = errors.New("SSH file transfer failed")
	ErrSSHClientNotAvailable = errors.New("SSH client not available")

	ErrConfigReadFailed   = errors.New("config read failed")
	ErrConfigParseFailed  = errors.New("config parse failed")
	ErrConfigValidateFail = errors.New("config validation failed")
	ErrConfigNotFound     = errors.New("config not found")

	ErrStateReadFailed    = errors.New("state read failed")
	ErrStateWriteFailed   = errors.New("state write failed")
	ErrStateSerializeFail = errors.New("state serialization failed")
	ErrStateNotFound      = errors.New("state not found")

	ErrDNSError          = errors.New("DNS operation failed")
	ErrDNSRecordExists   = errors.New("DNS record already exists")
	ErrDNSRecordNotFound = errors.New("DNS record not found")
	ErrDNSDomainNotFound = errors.New("DNS domain not found")

	ErrISPNotFound         = errors.New("ISP not found")
	ErrISPNoDNSService     = errors.New("ISP does not provide DNS service")
	ErrServerNotRegistered = errors.New("server not registered")
	ErrRegistryNotFound    = errors.New("registry not found")
	ErrRegistryLoginFailed = errors.New("registry login failed")
	ErrUnsupportedProvider = errors.New("unsupported provider type")
	ErrMissingCredential   = errors.New("missing credential")

	ErrFileReadFailed        = errors.New("file read failed")
	ErrFileWriteFailed       = errors.New("file write failed")
	ErrFileNotFound          = errors.New("file not found")
	ErrTempFileFailed        = errors.New("temp file operation failed")
	ErrDirectoryCreateFailed = errors.New("directory creation failed")
	ErrDirectoryRemoveFailed = errors.New("directory removal failed")

	ErrNetworkCreateFailed  = errors.New("network creation failed")
	ErrNetworkInspectFailed = errors.New("network inspection failed")
	ErrNetworkListFailed    = errors.New("network list failed")
	ErrNetworkCheckFailed   = errors.New("network check failed")

	ErrComposeGenerateFailed = errors.New("compose generation failed")
	ErrComposeSyncFailed     = errors.New("compose sync failed")
	ErrDockerComposeFailed   = errors.New("docker compose failed")
	ErrServiceInvalid        = errors.New("service invalid")
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
