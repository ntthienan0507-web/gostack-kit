// Package scheduler provides cron-based job scheduling with distributed locking.
//
// Unlike pkg/cron (which is a lightweight single-instance scheduler), this package
// uses github.com/robfig/cron/v3 for robust cron parsing and supports distributed
// locking via Redis to ensure only one instance runs each job across multiple pods.
//
// Usage:
//
//	s := scheduler.New(logger, redisClient)
//	s.Register(scheduler.Job{
//	    Name:     "cleanup-tokens",
//	    Schedule: "0 2 * * *",
//	    Fn:       cleanupExpiredTokens,
//	    Timeout:  10 * time.Minute,
//	})
//	s.Start(ctx) // blocks until ctx cancelled
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/distlock"
)

// DefaultTimeout is the default max execution time for a job.
const DefaultTimeout = 5 * time.Minute

// Job defines a scheduled task.
type Job struct {
	// Name is the unique job identifier. Also used as the distributed lock key.
	Name string

	// Schedule is a cron expression: "*/5 * * * *" (every 5 min), "@hourly", "@daily".
	// Supports the extended 6-field format with seconds: "*/1 * * * * *" (every second).
	Schedule string

	// Fn is the function to execute on each trigger.
	Fn func(ctx context.Context) error

	// Timeout is the max execution time. Default: 5m.
	Timeout time.Duration
}

// Scheduler runs jobs on cron schedules with distributed locking.
// Only 1 instance across all pods runs each job (via Redis distlock).
type Scheduler struct {
	cron        *cron.Cron
	logger      *zap.Logger
	redisClient *redis.Client
	jobs        []Job
	mu          sync.Mutex
}

// New creates a scheduler.
// If redisClient is nil, distributed locking is disabled (single-instance mode).
func New(logger *zap.Logger, redisClient *redis.Client) *Scheduler {
	return &Scheduler{
		logger:      logger,
		redisClient: redisClient,
	}
}

// Register adds a job to the scheduler. Call before Start.
func (s *Scheduler) Register(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.Timeout == 0 {
		job.Timeout = DefaultTimeout
	}

	s.jobs = append(s.jobs, job)
	s.logger.Info("scheduler job registered",
		zap.String("name", job.Name),
		zap.String("schedule", job.Schedule),
		zap.Duration("timeout", job.Timeout),
	)
}

// Start begins the scheduler. Blocks until context is cancelled.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	// Use cron with seconds support for flexibility.
	s.cron = cron.New(cron.WithSeconds())

	for _, job := range s.jobs {
		j := job // capture
		_, err := s.cron.AddFunc(j.Schedule, func() {
			s.execute(ctx, j)
		})
		if err != nil {
			s.mu.Unlock()
			return fmt.Errorf("scheduler: invalid schedule for job %q: %w", j.Name, err)
		}
	}
	s.mu.Unlock()

	s.logger.Info("scheduler started", zap.Int("jobs", len(s.jobs)))
	s.cron.Start()

	<-ctx.Done()
	s.logger.Info("scheduler stopping")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	s.logger.Info("scheduler stopped")
	return nil
}

// Stop gracefully stops all jobs.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		stopCtx := s.cron.Stop()
		<-stopCtx.Done()
		s.logger.Info("scheduler stopped")
	}
}

// execute runs a single job with distributed locking, timeout, and panic recovery.
func (s *Scheduler) execute(ctx context.Context, job Job) {
	start := time.Now()
	logger := s.logger.With(zap.String("job", job.Name))

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("scheduler job panicked",
				zap.Any("panic", r),
				zap.Duration("duration", time.Since(start)),
			)
		}
	}()

	// Distributed lock (if Redis available)
	if s.redisClient != nil {
		lockKey := "scheduler:" + job.Name
		lock, err := distlock.Acquire(ctx, s.redisClient, lockKey, distlock.Config{
			TTL: job.Timeout + 30*time.Second, // TTL > timeout so lock outlives execution
		})
		if errors.Is(err, distlock.ErrLockNotAcquired) {
			logger.Debug("scheduler job skipped (another instance holds lock)")
			return
		}
		if err != nil {
			logger.Error("scheduler job lock failed", zap.Error(err))
			return
		}
		defer func() {
			if err := lock.Release(context.Background()); err != nil {
				logger.Warn("scheduler job lock release failed", zap.Error(err))
			}
		}()
	}

	// Timeout enforcement
	execCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	logger.Info("scheduler job starting")

	if err := job.Fn(execCtx); err != nil {
		logger.Error("scheduler job failed",
			zap.Duration("duration", time.Since(start)),
			zap.Error(err),
		)
		return
	}

	logger.Info("scheduler job completed",
		zap.Duration("duration", time.Since(start)),
	)
}
