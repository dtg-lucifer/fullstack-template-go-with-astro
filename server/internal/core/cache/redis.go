// Package cache provides a Redis client wrapper and a Cache interface.
// Use the interface in modules so they stay decoupled from the concrete client.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is the interface every module should depend on.
// It covers the operations needed for session storage, rate-limiting buckets,
// short-lived tokens, and general key-value caching.
type Cache interface {
	// Get returns the value stored at key, or ("", ErrCacheMiss) if absent.
	Get(ctx context.Context, key string) (string, error)

	// Set stores value at key with the given TTL. Pass 0 for no expiry.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Del removes one or more keys. Missing keys are silently ignored.
	Del(ctx context.Context, keys ...string) error

	// Exists returns true if all the given keys exist.
	Exists(ctx context.Context, keys ...string) (bool, error)

	// Ping checks connectivity. Returns nil on success.
	Ping(ctx context.Context) error

	// Close releases the underlying connection pool.
	Close() error
}

// ErrCacheMiss is returned by Get when the key does not exist.
var ErrCacheMiss = redis.Nil

// Client wraps *redis.Client and satisfies the Cache interface.
type Client struct {
	rdb *redis.Client
}

// ClientConfig holds the parameters needed to connect to Redis.
type ClientConfig struct {
	Addr     string // host:port, e.g. "localhost:6379"
	Password string // empty string means no auth
	DB       int    // logical database index (0–15)
	PoolSize int    // max number of socket connections
}

// New dials Redis, pings it to verify connectivity, and returns a Client.
func New(ctx context.Context, cfg ClientConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("[REDIS] failed to connect: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Get returns the string value stored at key.
// Returns ("", ErrCacheMiss) when the key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Set stores value at key. ttl=0 means the key never expires.
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Del removes one or more keys. Missing keys are silently ignored.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists returns true if every key in keys exists.
func (c *Client) Exists(ctx context.Context, keys ...string) (bool, error) {
	n, err := c.rdb.Exists(ctx, keys...).Result()
	return n == int64(len(keys)), err
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close releases the connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Underlying returns the raw *redis.Client for operations not covered by the
// Cache interface (e.g. pub/sub, sorted sets, streams).
// Prefer the interface methods wherever possible.
func (c *Client) Underlying() *redis.Client {
	return c.rdb
}
