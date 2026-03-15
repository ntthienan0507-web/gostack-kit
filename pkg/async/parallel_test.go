package async

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallel_AllSucceed(t *testing.T) {
	var count atomic.Int32

	err := Parallel(context.Background(),
		func(ctx context.Context) error { count.Add(1); return nil },
		func(ctx context.Context) error { count.Add(1); return nil },
		func(ctx context.Context) error { count.Add(1); return nil },
	)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("expected 3 tasks run, got %d", got)
	}
}

func TestParallel_ReturnsFirstError(t *testing.T) {
	expected := errors.New("task 2 failed")

	err := Parallel(context.Background(),
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return expected },
		func(ctx context.Context) error { return nil },
	)

	if err == nil {
		t.Error("expected error")
	}
}

func TestParallel_RunsConcurrently(t *testing.T) {
	start := time.Now()

	err := Parallel(context.Background(),
		func(ctx context.Context) error { time.Sleep(50 * time.Millisecond); return nil },
		func(ctx context.Context) error { time.Sleep(50 * time.Millisecond); return nil },
		func(ctx context.Context) error { time.Sleep(50 * time.Millisecond); return nil },
	)

	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	// Should take ~50ms not ~150ms
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected parallel execution (~50ms), took %v", elapsed)
	}
}

func TestParallelCollect_AllSucceed(t *testing.T) {
	results, err := ParallelCollect(context.Background(),
		func(ctx context.Context) (int, error) { return 1, nil },
		func(ctx context.Context) (int, error) { return 2, nil },
		func(ctx context.Context) (int, error) { return 3, nil },
	)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Results should be in order
	for i, v := range results {
		if v != i+1 {
			t.Errorf("results[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestParallelCollect_ReturnsOnFirstError(t *testing.T) {
	expected := errors.New("fail")

	results, err := ParallelCollect(context.Background(),
		func(ctx context.Context) (string, error) {
			time.Sleep(100 * time.Millisecond) // slow
			return "a", nil
		},
		func(ctx context.Context) (string, error) {
			return "", expected // fast fail
		},
	)

	if !errors.Is(err, expected) {
		t.Errorf("expected error, got %v", err)
	}
	if results != nil {
		t.Error("results should be nil on error")
	}
}
