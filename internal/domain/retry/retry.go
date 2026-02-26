package retry

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
)

var (
	ErrMaxAttemptsExceeded = errors.New("max retry attempts exceeded")
	ErrContextCanceled     = errors.New("context canceled")
)

type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	IsRetryable  func(error) bool
	OnRetry      func(attempt int, delay time.Duration, err error)
}

type Option func(*Config)

func WithMaxAttempts(n int) Option {
	return func(c *Config) {
		c.MaxAttempts = n
	}
}

func WithInitialDelay(d time.Duration) Option {
	return func(c *Config) {
		c.InitialDelay = d
	}
}

func WithMaxDelay(d time.Duration) Option {
	return func(c *Config) {
		c.MaxDelay = d
	}
}

func WithMultiplier(m float64) Option {
	return func(c *Config) {
		c.Multiplier = m
	}
}

func WithIsRetryable(fn func(error) bool) Option {
	return func(c *Config) {
		c.IsRetryable = fn
	}
}

func WithOnRetry(fn func(attempt int, delay time.Duration, err error)) Option {
	return func(c *Config) {
		c.OnRetry = fn
	}
}

type LogFunc func(msg string, args ...any)

func WithLogger(log LogFunc) Option {
	return func(c *Config) {
		c.OnRetry = func(attempt int, delay time.Duration, err error) {
			log("retry attempt", "attempt", attempt, "error", err, "delay", delay)
		}
	}
}

func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:  constants.DefaultRetryMaxAttempts,
		InitialDelay: constants.DefaultRetryInitialDelay,
		MaxDelay:     constants.DefaultRetryMaxDelay,
		Multiplier:   constants.DefaultRetryMultiplier,
		IsRetryable:  DefaultIsRetryable,
		OnRetry:      nil,
	}
}

func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if isSyscallRetryable(err) {
		return true
	}

	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"timeout",
		"timed out",
		"temporary",
		"network is unreachable",
		"no such host",
		"dns",
		"eof",
		"broken pipe",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func isSyscallRetryable(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ETIMEDOUT,
			syscall.ENETUNREACH, syscall.EHOSTUNREACH, syscall.EPIPE:
			return true
		}
	}
	return false
}

func Do(ctx context.Context, fn func() error, opts ...Option) error {
	_, err := DoWithResult(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	}, opts...)
	return err
}

func DoWithResult[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	var zero T

	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, errors.Join(ErrContextCanceled, ctx.Err())
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !cfg.IsRetryable(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts {
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, delay, err)
			}

			select {
			case <-ctx.Done():
				return zero, errors.Join(ErrContextCanceled, ctx.Err())
			case <-time.After(delay):
			}

			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return zero, errors.Join(ErrMaxAttemptsExceeded, lastErr)
}
