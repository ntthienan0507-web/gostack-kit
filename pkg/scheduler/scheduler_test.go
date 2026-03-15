package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testLogger() *zap.Logger {
	l, _ := zap.NewDevelopment()
	return l
}

func TestJobRunsOnSchedule(t *testing.T) {
	var count atomic.Int32

	s := New(testLogger(), nil)
	s.Register(Job{
		Name:     "test-tick",
		Schedule: "*/1 * * * * *", // every second (6-field with seconds)
		Fn: func(ctx context.Context) error {
			count.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3500*time.Millisecond)
	defer cancel()

	go func() {
		_ = s.Start(ctx)
	}()

	<-ctx.Done()
	// Give a moment for in-flight jobs to finish.
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	got := count.Load()
	assert.GreaterOrEqual(t, got, int32(2), "expected at least 2 executions, got %d", got)
}

func TestJobTimeout(t *testing.T) {
	var timedOut atomic.Bool

	s := New(testLogger(), nil)
	s.Register(Job{
		Name:     "slow-job",
		Schedule: "*/1 * * * * *",
		Timeout:  500 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				timedOut.Store(true)
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		_ = s.Start(ctx)
	}()

	<-ctx.Done()
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	assert.True(t, timedOut.Load(), "job should have been timed out via context")
}

func TestJobRegistration(t *testing.T) {
	s := New(testLogger(), nil)

	s.Register(Job{Name: "job-a", Schedule: "*/5 * * * * *", Fn: func(ctx context.Context) error { return nil }})
	s.Register(Job{Name: "job-b", Schedule: "@every 1s", Fn: func(ctx context.Context) error { return nil }})

	assert.Len(t, s.jobs, 2)
	assert.Equal(t, "job-a", s.jobs[0].Name)
	assert.Equal(t, "job-b", s.jobs[1].Name)
}

func TestJobDefaultTimeout(t *testing.T) {
	s := New(testLogger(), nil)
	s.Register(Job{
		Name:     "default-timeout",
		Schedule: "*/1 * * * * *",
		Fn:       func(ctx context.Context) error { return nil },
	})

	assert.Equal(t, DefaultTimeout, s.jobs[0].Timeout)
}

func TestStartInvalidSchedule(t *testing.T) {
	s := New(testLogger(), nil)
	s.Register(Job{
		Name:     "bad-schedule",
		Schedule: "not-a-cron-expr",
		Fn:       func(ctx context.Context) error { return nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := s.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid schedule")
}
