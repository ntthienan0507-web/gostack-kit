package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client, err := New(Config{Host: mr.Host(), Port: mr.Server().Addr().Port}, zap.NewNop())
	require.NoError(t, err)
	return client, mr
}

// --- Tag-based invalidation ---

func TestSetWithTags_And_InvalidateByTag(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	// Cache 3 orders, all tagged with user:456
	c.SetWithTags(ctx, "order:101", "data1", 5*time.Minute, "user:456", "orders")
	c.SetWithTags(ctx, "order:102", "data2", 5*time.Minute, "user:456", "orders")
	c.SetWithTags(ctx, "order:103", "data3", 5*time.Minute, "user:789", "orders")

	// Verify all cached
	_, err := c.Get(ctx, "order:101")
	require.NoError(t, err)

	// Invalidate user:456 → should delete order:101 and order:102
	count, err := c.InvalidateByTag(ctx, "user:456")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// order:101, order:102 gone
	_, err = c.Get(ctx, "order:101")
	assert.ErrorIs(t, err, ErrCacheMiss)
	_, err = c.Get(ctx, "order:102")
	assert.ErrorIs(t, err, ErrCacheMiss)

	// order:103 still there (different user)
	val, err := c.Get(ctx, "order:103")
	require.NoError(t, err)
	assert.Equal(t, "data3", val)
}

func TestInvalidateByTag_EmptyTag(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	count, err := c.InvalidateByTag(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestInvalidateByTags_Multiple(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	c.SetWithTags(ctx, "order:1", "d1", 5*time.Minute, "user:1")
	c.SetWithTags(ctx, "order:2", "d2", 5*time.Minute, "user:2")

	count, err := c.InvalidateByTags(ctx, "user:1", "user:2")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	_, err = c.Get(ctx, "order:1")
	assert.ErrorIs(t, err, ErrCacheMiss)
	_, err = c.Get(ctx, "order:2")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSetJSONWithTags(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	type User struct {
		Name string `json:"name"`
	}

	err := c.SetJSONWithTags(ctx, "user:1", User{Name: "John"}, 5*time.Minute, "users")
	require.NoError(t, err)

	result, err := GetJSON[User](ctx, c, "user:1")
	require.NoError(t, err)
	assert.Equal(t, "John", result.Name)

	// Invalidate by tag
	count, err := c.InvalidateByTag(ctx, "users")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// --- Pattern-based invalidation ---

func TestInvalidateByPattern(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	c.Set(ctx, "order:1", "a", 5*time.Minute)
	c.Set(ctx, "order:2", "b", 5*time.Minute)
	c.Set(ctx, "order:3", "c", 5*time.Minute)
	c.Set(ctx, "user:1", "d", 5*time.Minute)

	// Delete all order:* keys
	count, err := c.InvalidateByPattern(ctx, "order:*")
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Orders gone
	_, err = c.Get(ctx, "order:1")
	assert.ErrorIs(t, err, ErrCacheMiss)

	// User still there
	val, err := c.Get(ctx, "user:1")
	require.NoError(t, err)
	assert.Equal(t, "d", val)
}

func TestInvalidateByPattern_NoMatch(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx := context.Background()

	count, err := c.InvalidateByPattern(ctx, "nothing:*")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// --- Pub/Sub invalidation ---

func TestPublishAndSubscribeInvalidation(t *testing.T) {
	c, _ := setupTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup: cache some data with tags
	c.SetWithTags(ctx, "order:1", "data", 5*time.Minute, "user:1")

	// Start subscriber in background
	go c.SubscribeInvalidation(ctx)
	time.Sleep(50 * time.Millisecond) // wait for subscriber to be ready

	// Publish invalidation
	err := c.PublishInvalidation(ctx, "user:1")
	require.NoError(t, err)

	// Give subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Key should be invalidated
	_, err = c.Get(ctx, "order:1")
	assert.ErrorIs(t, err, ErrCacheMiss)
}
