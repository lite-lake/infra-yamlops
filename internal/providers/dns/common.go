package dns

import (
	"context"
	"net"
	"strings"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/retry"
)

func IsRetryableDNSError(err error) bool {
	if err == nil {
		return false
	}

	if errors, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range errors.Unwrap() {
			if IsRetryableDNSError(e) {
				return true
			}
		}
	}

	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	errStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"rate limit",
		"too many requests",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay int
	MaxDelay     int
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 500,
		MaxDelay:     30000,
	}
}

func EnsureRecord(ctx context.Context, provider Provider, domain string, desired *DNSRecord, retryCfg *RetryConfig) error {
	if retryCfg == nil {
		retryCfg = DefaultRetryConfig()
	}

	var records []DNSRecord
	err := retry.Do(ctx, func() error {
		var err error
		records, err = provider.ListRecords(domain)
		return err
	}, retry.WithMaxAttempts(retryCfg.MaxAttempts), retry.WithInitialDelay(500), retry.WithIsRetryable(IsRetryableDNSError))
	if err != nil {
		return domainerr.WrapOp("list records", err)
	}

	for _, existing := range records {
		if existing.Type == desired.Type && existing.Name == desired.Name {
			if existing.Value == desired.Value && existing.TTL == desired.TTL {
				return nil
			}
			return retry.Do(ctx, func() error {
				return provider.UpdateRecord(domain, existing.ID, desired)
			}, retry.WithMaxAttempts(retryCfg.MaxAttempts), retry.WithIsRetryable(IsRetryableDNSError))
		}
	}
	return retry.Do(ctx, func() error {
		return provider.CreateRecord(domain, desired)
	}, retry.WithMaxAttempts(retryCfg.MaxAttempts), retry.WithIsRetryable(IsRetryableDNSError))
}

func EnsureRecordSimple(provider Provider, domain string, desired *DNSRecord) error {
	return EnsureRecord(context.Background(), provider, domain, desired, nil)
}
