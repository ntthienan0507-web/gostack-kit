// Package circuitbreaker wraps sony/gobreaker with structured error handling and logging.
package circuitbreaker

import (
	"errors"
	"fmt"
	"time"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

// ErrCircuitOpen is returned when the circuit breaker is open or receiving too many requests.
var ErrCircuitOpen = apperror.New(503, "common.circuit_open", "Service temporarily unavailable")

// Config defines circuit breaker behavior.
type Config struct {
	Name             string
	MaxRequests      uint32        // Max requests allowed in half-open state
	Interval         time.Duration // Cyclic period of closed state to clear counts
	Timeout          time.Duration // How long to stay open before transitioning to half-open
	FailureThreshold uint32        // Consecutive failures before opening
}

// DefaultConfig returns a Config with sensible defaults for the given name.
func DefaultConfig(name string) Config {
	return Config{
		Name:             name,
		MaxRequests:      5,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 5,
	}
}

// CircuitBreaker wraps gobreaker.CircuitBreaker with structured logging.
type CircuitBreaker struct {
	cb     *gobreaker.CircuitBreaker[any]
	logger *zap.Logger
}

// New creates a CircuitBreaker with the given config and logger.
func New(cfg Config, logger *zap.Logger) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.FailureThreshold
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Warn("circuit breaker state change",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	return &CircuitBreaker{
		cb:     gobreaker.NewCircuitBreaker[any](settings),
		logger: logger,
	}
}

// Execute runs fn through the circuit breaker and returns the typed result.
func Execute[T any](cb *CircuitBreaker, fn func() (T, error)) (T, error) {
	result, err := cb.cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, mapError(err)
	}
	return result.(T), nil
}

// ExecuteVoid runs fn through the circuit breaker for operations with no return value.
func ExecuteVoid(cb *CircuitBreaker, fn func() error) error {
	_, err := cb.cb.Execute(func() (any, error) {
		return nil, fn()
	})
	if err != nil {
		return mapError(err)
	}
	return nil
}

// mapError converts gobreaker sentinel errors into the structured ErrCircuitOpen.
func mapError(err error) error {
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("%w: %v", ErrCircuitOpen, err)
	}
	return err
}
