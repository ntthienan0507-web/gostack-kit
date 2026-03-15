// Package retry provides exponential backoff retry logic for transient failures.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"time"
)

// Config defines retry behavior.
type Config struct {
	MaxRetries int           // Maximum number of retry attempts (0 = no retry)
	BaseDelay  time.Duration // Initial delay before first retry
	MaxDelay   time.Duration // Maximum delay between retries
	Multiplier float64       // Delay multiplier for exponential backoff
}

// DefaultConfig provides sensible defaults for API calls.
var DefaultConfig = Config{
	MaxRetries: 3,
	BaseDelay:  100 * time.Millisecond,
	MaxDelay:   10 * time.Second,
	Multiplier: 2.0,
}

// AggressiveConfig for critical operations needing more retries.
var AggressiveConfig = Config{
	MaxRetries: 5,
	BaseDelay:  50 * time.Millisecond,
	MaxDelay:   30 * time.Second,
	Multiplier: 2.0,
}

// Do executes fn with retry logic using the provided config.
// Returns nil on success, or the last error after all retries exhausted.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := fn(); err != nil {
			lastErr = err

			if !IsRetryable(err) {
				return err
			}

			if attempt == cfg.MaxRetries {
				break
			}

			delay := CalculateDelay(attempt, cfg)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		} else {
			return nil
		}
	}

	return lastErr
}

// DoWithResult executes fn with retry logic and returns both result and error.
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		res, err := fn()
		if err != nil {
			lastErr = err

			if !IsRetryable(err) {
				return result, err
			}

			if attempt == cfg.MaxRetries {
				break
			}

			delay := CalculateDelay(attempt, cfg)

			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(delay):
				continue
			}
		} else {
			return res, nil
		}
	}

	return result, lastErr
}

// CalculateDelay computes delay with exponential backoff and jitter.
// Jitter (0-1000ms) prevents thundering herd on concurrent retries.
func CalculateDelay(attempt int, cfg Config) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Add jitter (0-1000ms)
	jitter := float64(rand.IntN(1000)) * float64(time.Millisecond)
	delay += jitter

	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

// IsRetryable determines if an error is worth retrying.
// Retries: network errors, timeouts, 5xx, 429.
// No retry: 4xx (except 429), context cancelled/deadline.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context cancelled/deadline — don't retry
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network errors (timeout, DNS, connection refused)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// HTTP status code errors
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == http.StatusTooManyRequests
	}

	// Unknown errors — retry (conservative)
	return true
}

// HTTPError wraps HTTP status code errors for retryability classification.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates an HTTPError from status code.
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Message: message}
}
