package retry

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), DefaultConfig, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return NewHTTPError(http.StatusServiceUnavailable, "service unavailable")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return NewHTTPError(http.StatusBadRequest, "bad request")
	})

	if err == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retries for 4xx), got %d", attempts)
	}
}

func TestDo_ExhaustedRetries(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return NewHTTPError(http.StatusServiceUnavailable, "service unavailable")
	})

	if err == nil {
		t.Error("expected error after exhausted retries")
	}
	if attempts != 3 { // initial + 2 retries
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	err := Do(ctx, DefaultConfig, func() error {
		attempts++
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if attempts != 0 {
		t.Errorf("expected 0 attempts, got %d", attempts)
	}
}

func TestDo_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	cfg := Config{
		MaxRetries: 10,
		BaseDelay:  5 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 1.5,
	}

	err := Do(ctx, cfg, func() error {
		attempts++
		return NewHTTPError(http.StatusServiceUnavailable, "unavailable")
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestDo_RetryOnNetworkTimeout(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	netErr := &net.OpError{Op: "dial", Err: &timeoutError{}}
	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return netErr
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_RetryOn500(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return NewHTTPError(http.StatusInternalServerError, "internal server error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_RetryOn429(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return NewHTTPError(http.StatusTooManyRequests, "too many requests")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_DontRetryOn4xx(t *testing.T) {
	codes := []int{400, 401, 403, 404, 409, 422}
	cfg := fastConfig(3)

	for _, code := range codes {
		attempts := 0
		err := Do(context.Background(), cfg, func() error {
			attempts++
			return NewHTTPError(code, "client error")
		})

		if err == nil {
			t.Errorf("expected error for status %d", code)
		}
		if attempts != 1 {
			t.Errorf("status %d: expected 1 attempt, got %d", code, attempts)
		}
	}
}

// --- DoWithResult ---

func TestDoWithResult_Success(t *testing.T) {
	attempts := 0
	result, err := DoWithResult(context.Background(), DefaultConfig, func() (string, error) {
		attempts++
		return "ok", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoWithResult_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	cfg := fastConfig(2)

	result, err := DoWithResult(context.Background(), cfg, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, NewHTTPError(http.StatusServiceUnavailable, "unavailable")
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestDoWithResult_ExhaustedRetries(t *testing.T) {
	attempts := 0
	cfg := fastConfig(1)

	result, err := DoWithResult(context.Background(), cfg, func() (string, error) {
		attempts++
		return "", NewHTTPError(http.StatusServiceUnavailable, "unavailable")
	})

	if err == nil {
		t.Error("expected error")
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// --- CalculateDelay ---

func TestCalculateDelay_ExponentialBackoff(t *testing.T) {
	cfg := Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
	}

	for attempt := 0; attempt < 3; attempt++ {
		delay := CalculateDelay(attempt, cfg)
		expectedMin := time.Duration(float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
		maxJitter := 1000 * time.Millisecond

		if delay < expectedMin {
			t.Errorf("attempt %d: delay %v < expected min %v", attempt, delay, expectedMin)
		}
		if delay > expectedMin+maxJitter {
			t.Errorf("attempt %d: delay %v > expected max %v", attempt, delay, expectedMin+maxJitter)
		}
	}
}

func TestCalculateDelay_MaxDelayCap(t *testing.T) {
	cfg := Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   500 * time.Millisecond,
		Multiplier: 2.0,
	}

	delay := CalculateDelay(10, cfg) // 100ms * 2^10 = 102s → capped

	if delay > cfg.MaxDelay {
		t.Errorf("delay %v should be capped at %v", delay, cfg.MaxDelay)
	}
}

func TestDelayWithJitter(t *testing.T) {
	cfg := Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
	}

	delays := make([]time.Duration, 10)
	for i := range delays {
		delays[i] = CalculateDelay(0, cfg)
	}

	hasDifference := false
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[i-1] {
			hasDifference = true
			break
		}
	}

	if !hasDifference {
		t.Error("expected jitter to produce varying delays")
	}
}

// --- IsRetryable ---

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"generic error", errors.New("oops"), true},
		{"context cancelled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"HTTP 500", NewHTTPError(500, ""), true},
		{"HTTP 502", NewHTTPError(502, ""), true},
		{"HTTP 503", NewHTTPError(503, ""), true},
		{"HTTP 429", NewHTTPError(429, ""), true},
		{"HTTP 400", NewHTTPError(400, ""), false},
		{"HTTP 401", NewHTTPError(401, ""), false},
		{"HTTP 403", NewHTTPError(403, ""), false},
		{"HTTP 404", NewHTTPError(404, ""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

// --- Helpers ---

// fastConfig returns a Config with minimal delays for fast tests.
func fastConfig(maxRetries int) Config {
	return Config{
		MaxRetries: maxRetries,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Multiplier: 2.0,
	}
}

// timeoutError mocks a network timeout error.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
