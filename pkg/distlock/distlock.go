// Package distlock provides a distributed lock backed by Redis.
//
// It uses the SET NX PX pattern (atomic acquire with TTL) to ensure
// mutual exclusion across multiple processes or pods.
//
// When to use:
//   - Cron jobs running on multiple instances (only one should execute).
//   - Webhook deduplication (process a payment callback only once).
//   - Resource initialization (run a DB migration only once across pods).
//   - Rate-limited external API calls (ensure only one caller at a time).
//
// When NOT to use:
//   - Single-instance apps — use sync.Mutex instead.
//   - Database row-level locking — use SELECT FOR UPDATE.
//   - Short-lived in-process operations — the Redis round-trip adds overhead.
//
// Example usage:
//
//	lock, err := distlock.Acquire(ctx, redisClient, "cron:daily-report", distlock.Config{
//	    TTL:          30 * time.Second,
//	    RetryTimeout: 5 * time.Second,
//	})
//	if errors.Is(err, distlock.ErrLockNotAcquired) {
//	    log.Println("another instance is already running")
//	    return
//	}
//	if err != nil {
//	    return fmt.Errorf("acquire lock: %w", err)
//	}
//	defer lock.Release(ctx)
//
//	// ... do exclusive work ...
package distlock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrLockNotAcquired is returned when the lock cannot be acquired,
// either because another holder owns it or the retry timeout expired.
var ErrLockNotAcquired = errors.New("distlock: lock not acquired")

// DefaultTTL is the default lock expiry if Config.TTL is zero.
const DefaultTTL = 30 * time.Second

// DefaultRetryInterval is the default interval between acquire attempts.
const DefaultRetryInterval = 100 * time.Millisecond

// Config controls the behaviour of Acquire.
type Config struct {
	// TTL is the lock expiry duration. If the holder crashes, the lock
	// auto-releases after TTL. Set this longer than the expected operation
	// duration plus a safety buffer.
	//
	// Too short: lock expires mid-operation, allowing two holders.
	// Too long: a crashed holder blocks others unnecessarily.
	//
	// Default: 30s.
	TTL time.Duration

	// RetryInterval is how often Acquire re-attempts the SET NX command
	// when the lock is already held. Default: 100ms.
	RetryInterval time.Duration

	// RetryTimeout is the maximum time Acquire will keep retrying before
	// returning ErrLockNotAcquired. Zero means try exactly once (no retry).
	RetryTimeout time.Duration
}

// defaults fills in zero-valued fields with sensible defaults.
func (c *Config) defaults() {
	if c.TTL == 0 {
		c.TTL = DefaultTTL
	}
	if c.RetryInterval == 0 {
		c.RetryInterval = DefaultRetryInterval
	}
}

// Lock represents a held distributed lock. Call Release when done, or let the
// TTL expire automatically (e.g. if the process crashes).
type Lock struct {
	client *redis.Client
	key    string
	value  string // random UUID — proves ownership
}

// Key returns the Redis key this lock is held on.
func (l *Lock) Key() string { return l.key }

// Acquire tries to obtain a distributed lock on the given key.
//
// On success it returns a *Lock whose Release method must be called when the
// critical section is complete. On failure it returns ErrLockNotAcquired.
//
// The lock value is a random UUID so that Release can verify ownership before
// deleting the key (preventing one holder from releasing another's lock).
//
// Example — webhook deduplication:
//
//	lock, err := distlock.Acquire(ctx, rdb, "webhook:"+eventID, distlock.Config{TTL: 60 * time.Second})
//	if errors.Is(err, distlock.ErrLockNotAcquired) {
//	    return // already processed by another pod
//	}
//	defer lock.Release(ctx)
//	processWebhook(event)
func Acquire(ctx context.Context, client *redis.Client, key string, cfg Config) (*Lock, error) {
	cfg.defaults()

	value := uuid.New().String()

	// Try once before entering the retry loop.
	acquired, err := tryAcquire(ctx, client, key, value, cfg.TTL)
	if err != nil {
		return nil, fmt.Errorf("distlock acquire: %w", err)
	}
	if acquired {
		return &Lock{client: client, key: key, value: value}, nil
	}

	// No retry requested — fail immediately.
	if cfg.RetryTimeout == 0 {
		return nil, ErrLockNotAcquired
	}

	// Retry loop with timeout.
	deadline := time.Now().Add(cfg.RetryTimeout)
	ticker := time.NewTicker(cfg.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("distlock acquire: %w", ctx.Err())
		case t := <-ticker.C:
			if t.After(deadline) {
				return nil, ErrLockNotAcquired
			}
			acquired, err := tryAcquire(ctx, client, key, value, cfg.TTL)
			if err != nil {
				return nil, fmt.Errorf("distlock acquire: %w", err)
			}
			if acquired {
				return &Lock{client: client, key: key, value: value}, nil
			}
		}
	}
}

// tryAcquire performs a single SET key value NX PX ttl attempt.
func tryAcquire(ctx context.Context, client *redis.Client, key, value string, ttl time.Duration) (bool, error) {
	ok, err := client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// releaseScript atomically checks ownership then deletes the key.
// This prevents releasing a lock that has already expired and been
// re-acquired by another holder.
//
//	KEYS[1] = lock key
//	ARGV[1] = expected value (owner UUID)
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

// Release explicitly releases the lock. Only the original owner (matching UUID)
// can release it; if the lock has already expired or been taken by someone else,
// Release is a safe no-op.
//
// Always call Release in a defer after a successful Acquire:
//
//	lock, err := distlock.Acquire(ctx, rdb, "my-key", distlock.Config{})
//	if err != nil { ... }
//	defer lock.Release(ctx)
func (l *Lock) Release(ctx context.Context) error {
	result, err := releaseScript.Run(ctx, l.client, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("distlock release %q: %w", l.key, err)
	}
	if result == 0 {
		// Lock was already expired or owned by someone else — not an error,
		// but the caller should be aware the lock was not held at release time.
		return nil
	}
	return nil
}

// extendScript atomically checks ownership then resets the TTL.
//
//	KEYS[1] = lock key
//	ARGV[1] = expected value (owner UUID)
//	ARGV[2] = new TTL in milliseconds
var extendScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

// Extend resets the lock TTL to the given duration. Only the current owner can
// extend. Use this for long-running operations that may exceed the original TTL.
//
// Example — long-running export:
//
//	lock, _ := distlock.Acquire(ctx, rdb, "export:users", distlock.Config{TTL: 30 * time.Second})
//	defer lock.Release(ctx)
//
//	for batch := range batches {
//	    processBatch(batch)
//	    lock.Extend(ctx, 30*time.Second) // reset TTL after each batch
//	}
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	ms := ttl.Milliseconds()
	result, err := extendScript.Run(ctx, l.client, []string{l.key}, l.value, ms).Int64()
	if err != nil {
		return fmt.Errorf("distlock extend %q: %w", l.key, err)
	}
	if result == 0 {
		return ErrLockNotAcquired
	}
	return nil
}
