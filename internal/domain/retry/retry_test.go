package retry

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		if callCount < 2 {
			return &net.DNSError{Err: "temporary error", IsTemporary: true}
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
		return &net.DNSError{Err: "connection refused", IsTemporary: true}
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
		return errors.New("timeout error")
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
			return "", io.ErrUnexpectedEOF
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
		return "", errors.New("connection refused")
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
		return "", errors.New("connection reset")
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
		return "", errors.New("timeout occurred")
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

func TestDefaultIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"net timeout error", &net.OpError{Err: &timeoutError{}}, true},
		{"net temporary error", &net.OpError{Err: &temporaryError{}}, true},
		{"error with timeout in message", errors.New("connection timeout"), true},
		{"error with connection refused", errors.New("connection refused by peer"), true},
		{"error with connection reset", errors.New("connection reset by peer"), true},
		{"error with temporary", errors.New("temporary failure in name resolution"), true},
		{"error with network unreachable", errors.New("network is unreachable"), true},
		{"error with dns", errors.New("dns lookup failed"), true},
		{"error with eof", errors.New("unexpected eof"), true},
		{"error with broken pipe", errors.New("broken pipe"), true},
		{"generic error", errors.New("something went wrong"), false},
		{"validation error", errors.New("invalid input"), false},
		{"permission denied", errors.New("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefaultIsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("DefaultIsRetryable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDo_NonRetryableError_ImmediateReturn(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return errors.New("validation failed")
	}, WithMaxAttempts(3))

	if callCount != 1 {
		t.Errorf("expected 1 call for non-retryable error, got %d", callCount)
	}
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDo_ContextErrorNotRetryable(t *testing.T) {
	callCount := 0
	err := Do(context.Background(), func() error {
		callCount++
		return context.Canceled
	}, WithMaxAttempts(3))

	if callCount != 1 {
		t.Errorf("expected 1 call for context.Canceled, got %d", callCount)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

type temporaryError struct{}

func (e *temporaryError) Error() string   { return "temporary" }
func (e *temporaryError) Timeout() bool   { return false }
func (e *temporaryError) Temporary() bool { return true }
