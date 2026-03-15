package async

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWorkerPool_ProcessesTasks(t *testing.T) {
	wp := NewWorkerPool(2, 10, zap.NewNop())
	defer wp.Shutdown()

	var count atomic.Int32

	for range 5 {
		wp.Submit(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})
	}

	// Give workers time to process
	time.Sleep(50 * time.Millisecond)

	if got := count.Load(); got != 5 {
		t.Errorf("expected 5 tasks processed, got %d", got)
	}
}

func TestWorkerPool_RecoversPanic(t *testing.T) {
	wp := NewWorkerPool(1, 10, zap.NewNop())
	defer wp.Shutdown()

	var after atomic.Bool

	// Task that panics
	wp.Submit(func(ctx context.Context) error {
		panic("boom")
	})

	// Task after panic — should still run
	wp.Submit(func(ctx context.Context) error {
		after.Store(true)
		return nil
	})

	time.Sleep(50 * time.Millisecond)

	if !after.Load() {
		t.Error("worker should recover from panic and continue processing")
	}
}

func TestWorkerPool_HandlesErrors(t *testing.T) {
	wp := NewWorkerPool(1, 10, zap.NewNop())
	defer wp.Shutdown()

	var after atomic.Bool

	wp.Submit(func(ctx context.Context) error {
		return errors.New("task failed")
	})

	wp.Submit(func(ctx context.Context) error {
		after.Store(true)
		return nil
	})

	time.Sleep(50 * time.Millisecond)

	if !after.Load() {
		t.Error("worker should continue after task error")
	}
}

func TestWorkerPool_ShutdownDrains(t *testing.T) {
	wp := NewWorkerPool(1, 10, zap.NewNop())

	var count atomic.Int32

	for range 3 {
		wp.Submit(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
			return nil
		})
	}

	wp.Shutdown() // should wait for all 3

	if got := count.Load(); got != 3 {
		t.Errorf("shutdown should drain all tasks, got %d/3", got)
	}
}

func TestWorkerPool_TrySubmit_Full(t *testing.T) {
	wp := NewWorkerPool(1, 1, zap.NewNop())
	defer wp.Shutdown()

	// Block the worker
	wp.Submit(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	time.Sleep(5 * time.Millisecond) // let worker pick up first task

	// Queue should be open for 1
	ok := wp.TrySubmit(func(ctx context.Context) error { return nil })
	if !ok {
		t.Error("TrySubmit should succeed when queue has space")
	}

	// Now queue is full
	ok = wp.TrySubmit(func(ctx context.Context) error { return nil })
	if ok {
		t.Error("TrySubmit should return false when queue is full")
	}
}
