package distlock_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/gostack-kit/pkg/distlock"
)

// newTestRedis spins up an in-memory Redis (miniredis) and returns a
// connected *redis.Client. The server is closed when the test finishes.
func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	return client, mr
}

func TestAcquireAndRelease(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	lock, err := distlock.Acquire(ctx, client, "test:basic", distlock.Config{
		TTL: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, lock)
	assert.Equal(t, "test:basic", lock.Key())

	// Key should exist in Redis while lock is held.
	val, err := client.Get(ctx, "test:basic").Result()
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	// Release should succeed.
	err = lock.Release(ctx)
	require.NoError(t, err)

	// Key should be gone after release.
	_, err = client.Get(ctx, "test:basic").Result()
	assert.ErrorIs(t, err, redis.Nil)
}

func TestAcquireFailsWhenAlreadyLocked(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	// First lock succeeds.
	lock1, err := distlock.Acquire(ctx, client, "test:conflict", distlock.Config{
		TTL: 10 * time.Second,
	})
	require.NoError(t, err)
	defer lock1.Release(ctx)

	// Second lock (different caller, no retry) should fail.
	lock2, err := distlock.Acquire(ctx, client, "test:conflict", distlock.Config{
		TTL: 10 * time.Second,
	})
	assert.Nil(t, lock2)
	assert.ErrorIs(t, err, distlock.ErrLockNotAcquired)
}

func TestLockAutoExpiresAfterTTL(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	_, err := distlock.Acquire(ctx, client, "test:expire", distlock.Config{
		TTL: 1 * time.Second,
	})
	require.NoError(t, err)
	// Note: we intentionally do NOT release.

	// Fast-forward miniredis past the TTL.
	mr.FastForward(2 * time.Second)

	// Now a new caller should be able to acquire the same key.
	lock2, err := distlock.Acquire(ctx, client, "test:expire", distlock.Config{
		TTL: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, lock2)
	defer lock2.Release(ctx)
}

func TestReleaseOnlyWorksForOwner(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	// Holder A acquires the lock.
	lockA, err := distlock.Acquire(ctx, client, "test:owner", distlock.Config{
		TTL: 10 * time.Second,
	})
	require.NoError(t, err)

	// Simulate a different owner by manually setting a different value.
	// We create a fake Lock that points to the same key but with a wrong value.
	// Since Lock fields are unexported, we test indirectly: release lockA,
	// re-acquire with a new owner, then try releasing lockA again (stale handle).
	err = lockA.Release(ctx)
	require.NoError(t, err)

	lockB, err := distlock.Acquire(ctx, client, "test:owner", distlock.Config{
		TTL: 10 * time.Second,
	})
	require.NoError(t, err)

	// lockA.Release is now a no-op because lockA's UUID no longer matches.
	err = lockA.Release(ctx)
	require.NoError(t, err) // no error, but key should still exist

	// lockB's key should still be held.
	val, err := client.Get(ctx, "test:owner").Result()
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	// lockB release should work.
	err = lockB.Release(ctx)
	require.NoError(t, err)
}

func TestExtend(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	lock, err := distlock.Acquire(ctx, client, "test:extend", distlock.Config{
		TTL: 1 * time.Second,
	})
	require.NoError(t, err)

	// Extend the TTL to 10 seconds.
	err = lock.Extend(ctx, 10*time.Second)
	require.NoError(t, err)

	// Fast-forward past the original TTL but not the extended one.
	mr.FastForward(2 * time.Second)

	// Lock should still be held.
	val, err := client.Get(ctx, "test:extend").Result()
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	defer lock.Release(ctx)
}

func TestExtendFailsForNonOwner(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	lock, err := distlock.Acquire(ctx, client, "test:extend-fail", distlock.Config{
		TTL: 10 * time.Second,
	})
	require.NoError(t, err)

	// Release and re-acquire with a different owner.
	err = lock.Release(ctx)
	require.NoError(t, err)

	lock2, err := distlock.Acquire(ctx, client, "test:extend-fail", distlock.Config{
		TTL: 10 * time.Second,
	})
	require.NoError(t, err)
	defer lock2.Release(ctx)

	// Old lock handle should fail to extend.
	err = lock.Extend(ctx, 10*time.Second)
	assert.ErrorIs(t, err, distlock.ErrLockNotAcquired)
}

func TestAcquireWithRetry(t *testing.T) {
	client, mr := newTestRedis(t)
	ctx := context.Background()

	// First holder acquires with a short TTL.
	lock1, err := distlock.Acquire(ctx, client, "test:retry", distlock.Config{
		TTL: 500 * time.Millisecond,
	})
	require.NoError(t, err)
	_ = lock1 // intentionally not releasing

	// Fast-forward so the lock expires during retry window.
	go func() {
		time.Sleep(50 * time.Millisecond)
		mr.FastForward(1 * time.Second)
	}()

	// Second caller retries and should eventually succeed.
	lock2, err := distlock.Acquire(ctx, client, "test:retry", distlock.Config{
		TTL:           5 * time.Second,
		RetryInterval: 50 * time.Millisecond,
		RetryTimeout:  2 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, lock2)
	defer lock2.Release(ctx)
}

func TestAcquireRetryTimeout(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	// First holder acquires with a long TTL (won't expire).
	lock1, err := distlock.Acquire(ctx, client, "test:retry-timeout", distlock.Config{
		TTL: 1 * time.Minute,
	})
	require.NoError(t, err)
	defer lock1.Release(ctx)

	// Second caller retries but times out.
	lock2, err := distlock.Acquire(ctx, client, "test:retry-timeout", distlock.Config{
		TTL:           5 * time.Second,
		RetryInterval: 50 * time.Millisecond,
		RetryTimeout:  200 * time.Millisecond,
	})
	assert.Nil(t, lock2)
	assert.ErrorIs(t, err, distlock.ErrLockNotAcquired)
}

func TestConcurrentAcquireOnlyOneWins(t *testing.T) {
	client, _ := newTestRedis(t)
	ctx := context.Background()

	const goroutines = 20
	var acquired atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			lock, err := distlock.Acquire(ctx, client, "test:concurrent", distlock.Config{
				TTL: 10 * time.Second,
			})
			if err == nil && lock != nil {
				acquired.Add(1)
				// Hold the lock briefly then release.
				time.Sleep(10 * time.Millisecond)
				lock.Release(ctx)
			}
		}()
	}

	wg.Wait()

	// Exactly one goroutine should have acquired the lock.
	assert.Equal(t, int32(1), acquired.Load(),
		"expected exactly 1 goroutine to acquire the lock, got %d", acquired.Load())
}

func TestAcquireRespectsContextCancellation(t *testing.T) {
	client, _ := newTestRedis(t)

	// Hold the lock so retry loop is needed.
	lock1, err := distlock.Acquire(context.Background(), client, "test:ctx-cancel", distlock.Config{
		TTL: 1 * time.Minute,
	})
	require.NoError(t, err)
	defer lock1.Release(context.Background())

	// Cancel the context quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	lock2, err := distlock.Acquire(ctx, client, "test:ctx-cancel", distlock.Config{
		TTL:           5 * time.Second,
		RetryInterval: 50 * time.Millisecond,
		RetryTimeout:  5 * time.Second, // long retry, but context cancels first
	})
	assert.Nil(t, lock2)
	assert.Error(t, err)
}
