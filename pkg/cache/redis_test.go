package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestClient spins up a miniredis instance and returns a connected Client.
func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)

	port, _ := strconv.Atoi(mr.Port())
	client, err := New(Config{
		Host:     mr.Host(),
		Port:     port,
		PoolSize: 2,
	}, zap.NewNop())
	require.NoError(t, err)

	t.Cleanup(func() { client.Close() })

	return client, mr
}

func TestSetAndGet(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	err := c.Set(ctx, "greeting", "hello", time.Minute)
	require.NoError(t, err)

	val, err := c.Get(ctx, "greeting")
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestGet_CacheMiss(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	_, err := c.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestDelete(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "k1", "v1", 0))
	require.NoError(t, c.Set(ctx, "k2", "v2", 0))

	err := c.Delete(ctx, "k1", "k2")
	require.NoError(t, err)

	_, err = c.Get(ctx, "k1")
	assert.ErrorIs(t, err, ErrCacheMiss)

	_, err = c.Get(ctx, "k2")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestSetJSON_GetJSON(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	want := User{ID: 42, Name: "Alice"}
	err := c.SetJSON(ctx, "user:42", want, time.Minute)
	require.NoError(t, err)

	got, err := GetJSON[User](ctx, c, "user:42")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetJSON_CacheMiss(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	type Payload struct{ X int }

	_, err := GetJSON[Payload](ctx, c, "missing")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestGetJSON_InvalidJSON(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "bad", "not-json{", 0))

	type Payload struct{ X int }
	_, err := GetJSON[Payload](ctx, c, "bad")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrCacheMiss)
}

func TestSet_TTLExpiry(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "temp", "gone-soon", time.Second))

	// Fast-forward miniredis clock
	mr.FastForward(2 * time.Second)

	_, err := c.Get(ctx, "temp")
	assert.ErrorIs(t, err, ErrCacheMiss)
}

func TestNew_BadAddress(t *testing.T) {
	_, err := New(Config{
		Host: "192.0.2.1", // RFC 5737 TEST-NET — guaranteed unreachable
		Port: 1,
	}, zap.NewNop())
	assert.Error(t, err)
}
