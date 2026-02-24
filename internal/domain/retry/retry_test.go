package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	}, WithMaxAttempts(3))

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return errors.New("persistent error")
	}, WithMaxAttempts(3))

	if !errors.Is(err, ErrMaxAttemptsExceeded) {
		t.Errorf("expected ErrMaxAttemptsExceeded, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	callCount := 0
	err := Do(ctx, func() error {
		callCount++
		return errors.New("error")
	}, WithMaxAttempts(3))

	if !errors.Is(err, ErrContextCanceled) {
		t.Errorf("expected ErrContextCanceled, got %v", err)
	}
	if callCount != 0 {
		t.Errorf("expected 0 calls, got %d", callCount)
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	customErr := errors.New("non-retryable error")
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return customErr
	}, WithMaxAttempts(3), WithIsRetryable(func(err error) bool {
		return false
	}))

	if !errors.Is(err, customErr) {
		t.Errorf("expected customErr, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	callCount := 0
	result, err := DoWithResult(context.Background(), func() (string, error) {
		callCount++
		if callCount < 2 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	}, WithMaxAttempts(3))

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %s", result)
	}
}

func TestDoWithResult_MaxAttemptsExceeded(t *testing.T) {
	callCount := 0
	result, err := DoWithResult(context.Background(), func() (string, error) {
		callCount++
		return "", errors.New("persistent error")
	}, WithMaxAttempts(3))

	if !errors.Is(err, ErrMaxAttemptsExceeded) {
		t.Errorf("expected ErrMaxAttemptsExceeded, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_ExponentialBackoff(t *testing.T) {
	delays := []time.Duration{}
	callCount := 0

	_, _ = DoWithResult(context.Background(), func() (string, error) {
		callCount++
		return "", errors.New("error")
	}, WithMaxAttempts(4), WithInitialDelay(10*time.Millisecond), WithMultiplier(2.0), WithOnRetry(func(attempt int, delay time.Duration, err error) {
		delays = append(delays, delay)
	}))

	expectedDelays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	if len(delays) != len(expectedDelays) {
		t.Errorf("expected %d delays, got %d", len(expectedDelays), len(delays))
		return
	}
	for i, expected := range expectedDelays {
		if delays[i] != expected {
			t.Errorf("delay[%d]: expected %v, got %v", i, expected, delays[i])
		}
	}
}

func TestDo_MaxDelayCap(t *testing.T) {
	delays := []time.Duration{}

	_, _ = DoWithResult(context.Background(), func() (string, error) {
		return "", errors.New("error")
	}, WithMaxAttempts(5), WithInitialDelay(100*time.Millisecond), WithMaxDelay(150*time.Millisecond), WithMultiplier(10.0), WithOnRetry(func(attempt int, delay time.Duration, err error) {
		delays = append(delays, delay)
	}))

	for _, delay := range delays {
		if delay > 150*time.Millisecond {
			t.Errorf("delay %v exceeded max delay 150ms", delay)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay 30s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", cfg.Multiplier)
	}
	if cfg.IsRetryable == nil {
		t.Error("expected IsRetryable to be set")
	}
}

func TestDo_ImmediateSuccess(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return nil
	}, WithMaxAttempts(3))

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}
