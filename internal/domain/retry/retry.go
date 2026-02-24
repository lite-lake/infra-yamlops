package retry

import (
	"context"
	"errors"
	"time"

	"github.com/litelake/yamlops/internal/infrastructure/logger"
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

func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		IsRetryable:  DefaultIsRetryable,
		OnRetry:      defaultOnRetry,
	}
}

func DefaultIsRetryable(err error) bool {
	if err == nil {
		return false
	}
	return true
}

func defaultOnRetry(attempt int, delay time.Duration, err error) {
	logger.Debug("retry attempt", "attempt", attempt, "error", err, "delay", delay)
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
