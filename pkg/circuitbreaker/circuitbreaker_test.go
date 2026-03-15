package circuitbreaker

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestCB(failureThreshold uint32) *CircuitBreaker {
	cfg := Config{
		Name:             "test",
		MaxRequests:      1,
		Interval:         60 * time.Second,
		Timeout:          1 * time.Second,
		FailureThreshold: failureThreshold,
	}
	return New(cfg, zap.NewNop())
}

func TestExecute_Success(t *testing.T) {
	cb := newTestCB(3)

	result, err := Execute(cb, func() (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestExecuteVoid_Success(t *testing.T) {
	cb := newTestCB(3)

	err := ExecuteVoid(cb, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestExecute_FailureThreshold_OpensCircuit(t *testing.T) {
	cb := newTestCB(3)
	simulatedErr := errors.New("service down")

	// Trigger 3 consecutive failures to open the circuit.
	for i := 0; i < 3; i++ {
		_ = ExecuteVoid(cb, func() error {
			return simulatedErr
		})
	}

	// Next call should get the circuit open error.
	err := ExecuteVoid(cb, func() error {
		return nil
	})
	if err == nil {
		t.Fatal("expected circuit open error, got nil")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestExecute_OpenState_RejectsCall(t *testing.T) {
	cb := newTestCB(1)

	// One failure opens the circuit.
	_ = ExecuteVoid(cb, func() error {
		return errors.New("fail")
	})

	_, err := Execute(cb, func() (int, error) {
		return 42, nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestExecute_HalfOpen_Recovery(t *testing.T) {
	cb := newTestCB(1)

	// Open the circuit.
	_ = ExecuteVoid(cb, func() error {
		return errors.New("fail")
	})

	// Wait for timeout so circuit transitions to half-open.
	time.Sleep(1500 * time.Millisecond)

	// Successful call in half-open should close the circuit.
	result, err := Execute(cb, func() (string, error) {
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("expected no error after recovery, got %v", err)
	}
	if result != "recovered" {
		t.Fatalf("expected 'recovered', got %q", result)
	}

	// Circuit should now be closed — further calls should succeed.
	err = ExecuteVoid(cb, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error after circuit closed, got %v", err)
	}
}
