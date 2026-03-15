package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errors.New("cache: key not found")

// Config holds Redis connection settings.
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
		PoolSize: 10,
	}
}

// Client wraps a redis.Client with convenience methods.
type Client struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// New creates a Redis client, pings the server, and returns a ready Client.
// No globals — the caller owns the returned Client.
func New(cfg Config, logger *zap.Logger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	logger.Info("redis connected", zap.String("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)))

	return &Client{rdb: rdb, logger: logger}, nil
}

// Get returns the value for key. Returns ErrCacheMiss when the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get %q: %w", key, err)
	}
	return val, nil
}

// Set stores a key-value pair with an optional TTL. Pass 0 for no expiration.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

// Delete removes one or more keys from the cache.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// GetJSON retrieves a key and unmarshals the JSON value into T.
// Returns ErrCacheMiss when the key does not exist.
func GetJSON[T any](ctx context.Context, c *Client, key string) (T, error) {
	var zero T

	raw, err := c.Get(ctx, key)
	if err != nil {
		return zero, err
	}

	var result T
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return zero, fmt.Errorf("cache unmarshal %q: %w", key, err)
	}
	return result, nil
}

// SetJSON marshals value as JSON and stores it with an optional TTL.
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal %q: %w", key, err)
	}
	return c.Set(ctx, key, string(data), ttl)
}

// Close releases the underlying Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}
