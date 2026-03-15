package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ============================================================================
// Tag-based Cache Invalidation
// ============================================================================
//
// Problem: Order response contains user info. User updates profile → all cached
// orders with that user are stale. But you only know order IDs at cache time,
// not at invalidation time.
//
// Solution: Tag each cache entry with related entities. Invalidate by tag.
//
//	// Cache order:123, tagged with user:456 and product:789
//	cache.SetWithTags(ctx, "order:123", data, 5*time.Minute, "user:456", "product:789")
//
//	// User 456 updates profile → invalidate all caches tagged with user:456
//	cache.InvalidateByTag(ctx, "user:456")
//	// Deletes: order:123 (and any other keys tagged with user:456)
//
// Implementation: Redis SET per tag → stores member keys. On invalidation,
// read set members → delete all keys + the tag set itself.
// ============================================================================

const tagPrefix = "tag:"

// SetWithTags stores a value and associates it with one or more tags.
// When any tag is invalidated, this key is deleted too.
//
// Example:
//
//	cache.SetWithTags(ctx, "order:123", orderJSON, 5*time.Minute,
//	    "user:456",        // invalidate when user 456 changes
//	    "orders",          // invalidate when any order changes
//	    "product:789",     // invalidate when product 789 changes
//	)
func (c *Client) SetWithTags(ctx context.Context, key string, value any, ttl time.Duration, tags ...string) error {
	// Store the value
	if err := c.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Associate key with each tag (Redis SET: tag:xxx → {key1, key2, ...})
	pipe := c.rdb.Pipeline()
	for _, tag := range tags {
		tagKey := tagPrefix + tag
		pipe.SAdd(ctx, tagKey, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.logger.Warn("cache tag association failed", zap.String("key", key), zap.Error(err))
	}
	return nil
}

// SetJSONWithTags marshals value as JSON, stores it, and associates with tags.
//
// Example:
//
//	cache.SetJSONWithTags(ctx, "order:123", orderResp, 5*time.Minute,
//	    "user:456", "orders",
//	)
func (c *Client) SetJSONWithTags(ctx context.Context, key string, value any, ttl time.Duration, tags ...string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %q: %w", key, err)
	}
	return c.SetWithTags(ctx, key, string(data), ttl, tags...)
}

// InvalidateByTag deletes all cache keys associated with the given tag.
//
// Example:
//
//	// User 456 updated → purge everything related
//	count, _ := cache.InvalidateByTag(ctx, "user:456")
//	// Deleted 5 keys: order:101, order:102, profile:456, feed:456
func (c *Client) InvalidateByTag(ctx context.Context, tag string) (int64, error) {
	tagKey := tagPrefix + tag

	// Get all keys associated with this tag
	keys, err := c.rdb.SMembers(ctx, tagKey).Result()
	if err != nil {
		return 0, fmt.Errorf("cache get tag members %q: %w", tag, err)
	}

	if len(keys) == 0 {
		return 0, nil
	}

	// Delete all associated keys + the tag set itself
	allKeys := append(keys, tagKey)
	deleted, err := c.rdb.Del(ctx, allKeys...).Result()
	if err != nil {
		return 0, fmt.Errorf("cache invalidate tag %q: %w", tag, err)
	}

	c.logger.Debug("cache invalidated by tag",
		zap.String("tag", tag),
		zap.Int("keys", len(keys)),
		zap.Int64("deleted", deleted),
	)

	// deleted includes the tag key itself, subtract 1 for actual cache keys
	return deleted - 1, nil
}

// InvalidateByTags deletes cache keys for multiple tags at once.
func (c *Client) InvalidateByTags(ctx context.Context, tags ...string) (int64, error) {
	var total int64
	for _, tag := range tags {
		n, err := c.InvalidateByTag(ctx, tag)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// ============================================================================
// Pattern-based Invalidation
// ============================================================================

// InvalidateByPattern deletes all keys matching a glob pattern.
// Uses SCAN (non-blocking) — safe for production, but can be slow on huge datasets.
//
// Example:
//
//	cache.InvalidateByPattern(ctx, "order:*")     // all orders
//	cache.InvalidateByPattern(ctx, "user:456:*")  // all caches for user 456
func (c *Client) InvalidateByPattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var total int64

	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return total, fmt.Errorf("cache scan %q: %w", pattern, err)
		}

		if len(keys) > 0 {
			deleted, err := c.rdb.Del(ctx, keys...).Result()
			if err != nil {
				return total, fmt.Errorf("cache del pattern %q: %w", pattern, err)
			}
			total += deleted
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if total > 0 {
		c.logger.Debug("cache invalidated by pattern",
			zap.String("pattern", pattern),
			zap.Int64("deleted", total),
		)
	}

	return total, nil
}

// ============================================================================
// Event-driven Invalidation (Redis Pub/Sub)
// ============================================================================
//
// For multi-instance/multi-service: publish invalidation events via Redis Pub/Sub.
// All instances subscribe → all caches stay in sync.
//
//	Instance A: update user → PublishInvalidation("user:456")
//	Instance B: receives → InvalidateByTag("user:456")
//	Instance C: receives → InvalidateByTag("user:456")
//
// Start subscriber in app startup:
//
//	go cacheClient.SubscribeInvalidation(ctx)

const invalidationChannel = "cache:invalidate"

// PublishInvalidation publishes invalidation events to all subscribers.
// All instances listening on SubscribeInvalidation will delete the tagged keys.
//
// Example:
//
//	// After updating user 456 — all instances purge related caches
//	cache.PublishInvalidation(ctx, "user:456", "profile:456")
func (c *Client) PublishInvalidation(ctx context.Context, tags ...string) error {
	msg := strings.Join(tags, ",")
	if err := c.rdb.Publish(ctx, invalidationChannel, msg).Err(); err != nil {
		return fmt.Errorf("cache publish invalidation: %w", err)
	}
	c.logger.Debug("cache invalidation published", zap.Strings("tags", tags))
	return nil
}

// SubscribeInvalidation listens for invalidation events and auto-invalidates.
// Run as a background goroutine — blocks until context is cancelled.
//
//	go cacheClient.SubscribeInvalidation(ctx)
func (c *Client) SubscribeInvalidation(ctx context.Context) {
	sub := c.rdb.Subscribe(ctx, invalidationChannel)
	defer sub.Close()

	ch := sub.Channel()
	c.logger.Info("cache invalidation subscriber started")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("cache invalidation subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			tags := strings.Split(msg.Payload, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				if _, err := c.InvalidateByTag(ctx, tag); err != nil {
					c.logger.Error("cache invalidation failed",
						zap.String("tag", tag),
						zap.Error(err),
					)
				}
			}
		}
	}
}
