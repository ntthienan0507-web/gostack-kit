package async

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Task is a unit of background work.
// The context passed to Task is the worker pool's own context (background),
// NOT the HTTP request context — safe to use after the handler returns.
//
// WARNING: Never capture *gin.Context in a Task closure.
// Copy needed values BEFORE submitting:
//
//	// ✅ CORRECT — copy values out first
//	userID := ctx.GetString("user_id")
//	email := user.Email
//	app.Workers.Submit(func(ctx context.Context) error {
//	    return sendEmail(ctx, userID, email)
//	})
//
//	// ❌ WRONG — gin.Context captured in closure, recycled after handler returns
//	app.Workers.Submit(func(ctx context.Context) error {
//	    return sendEmail(ctx, ginCtx.GetString("user_id"), ginCtx.GetString("email"))
//	})
type Task func(ctx context.Context) error

// WorkerPool manages a fixed pool of goroutines processing tasks from a queue.
// Use for: sending emails, webhooks, audit logs, notifications — anything async.
//
// Tasks receive a detached background context, NOT the HTTP request context.
// This prevents context leak when the HTTP handler returns before the task completes.
type WorkerPool struct {
	tasks  chan Task
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
}

// NewWorkerPool creates and starts a pool with the given number of workers.
// Queue size controls backpressure — Submit blocks when queue is full.
func NewWorkerPool(workers, queueSize int, logger *zap.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	wp := &WorkerPool{
		tasks:  make(chan Task, queueSize),
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	wp.wg.Add(workers)
	for i := range workers {
		go wp.worker(i)
	}

	logger.Info("worker pool started", zap.Int("workers", workers), zap.Int("queue_size", queueSize))
	return wp
}

// Submit adds a task to the queue. Blocks if queue is full.
// Returns false if the pool is shut down.
//
// The task receives the pool's background context, NOT the caller's context.
// Always copy data out of gin.Context BEFORE calling Submit.
func (wp *WorkerPool) Submit(task Task) bool {
	select {
	case wp.tasks <- task:
		return true
	case <-wp.ctx.Done():
		return false
	}
}

// TrySubmit adds a task without blocking. Returns false if queue is full or pool is shut down.
func (wp *WorkerPool) TrySubmit(task Task) bool {
	select {
	case wp.tasks <- task:
		return true
	default:
		return false
	}
}

// Shutdown stops accepting new tasks and waits for in-flight tasks to complete.
func (wp *WorkerPool) Shutdown() {
	wp.cancel()
	close(wp.tasks)
	wp.wg.Wait()
	wp.logger.Info("worker pool stopped")
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for task := range wp.tasks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					wp.logger.Error("worker task panicked",
						zap.Int("worker", id),
						zap.Any("panic", r),
					)
				}
			}()

			if err := task(wp.ctx); err != nil {
				wp.logger.Error("worker task failed",
					zap.Int("worker", id),
					zap.Error(err),
				)
			}
		}()
	}
}
