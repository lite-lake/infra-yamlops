package dns

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	domainerr "github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/retry"
)

var validTTLs = []int{1, 5, 10, 20, 30, 60, 120, 180, 300, 600, 900, 1800, 3600, 7200, 18000, 43200, 86400}

func ParseTTL(ttlStr string) (int, error) {
	ttl, err := strconv.Atoi(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL: %s", ttlStr)
	}
	return NormalizeTTL(ttl), nil
}

func NormalizeTTL(ttl int) int {
	idx, _ := slices.BinarySearch(validTTLs, ttl)
	if idx < len(validTTLs) && validTTLs[idx] == ttl {
		return ttl
	}
	if idx > 0 {
		return validTTLs[idx-1]
	}
	return 1
}

func DefaultTTL() int {
	return 600
}

func GetFullDomain(subDomain, domain string) string {
	if subDomain == "@" || subDomain == "" {
		return domain
	}
	return strings.Join([]string{subDomain, domain}, ".")
}

func GetSubDomain(fullDomain, domain string) string {
	if fullDomain == domain {
		return "@"
	}
	suffix := "." + domain
	if strings.HasSuffix(fullDomain, suffix) {
		return strings.TrimSuffix(fullDomain, suffix)
	}
	return fullDomain
}

func ParseSRVValue(value string) (priority, weight, port float64, target string) {
	parts := strings.Fields(value)
	if len(parts) >= 4 {
		priority, _ = strconv.ParseFloat(parts[0], 64)
		weight, _ = strconv.ParseFloat(parts[1], 64)
		port, _ = strconv.ParseFloat(parts[2], 64)
		target = parts[3]
	}
	return
}

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
