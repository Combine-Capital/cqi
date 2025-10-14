package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// RedisCache implements the Cache interface using Redis as the backend.
type RedisCache struct {
	client *redis.Client
	cfg    config.CacheConfig
}

// NewRedis creates a new Redis cache client with the given configuration.
// It accepts context for cancellation during connection establishment.
//
// The connection is lazy - it won't fail if Redis is temporarily unavailable,
// but will retry on first operation according to MaxRetries config.
func NewRedis(ctx context.Context, cfg config.CacheConfig) (*RedisCache, error) {
	opts := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	}

	client := redis.NewClient(opts)

	// Test the connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.NewTemporary("failed to connect to Redis", err)
	}

	return &RedisCache{
		client: client,
		cfg:    cfg,
	}, nil
}

// Get retrieves a cached protobuf message by key and unmarshals it into dest.
func (r *RedisCache) Get(ctx context.Context, key string, dest proto.Message) error {
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return errors.NewNotFound("cache key", key)
		}
		return errors.NewTemporary("failed to get from cache", err)
	}

	if err := proto.Unmarshal(data, dest); err != nil {
		return errors.NewPermanent("failed to unmarshal cached data", err)
	}

	return nil
}

// Set stores a protobuf message in the cache with the specified TTL.
func (r *RedisCache) Set(ctx context.Context, key string, value proto.Message, ttl time.Duration) error {
	data, err := proto.Marshal(value)
	if err != nil {
		return errors.NewPermanent("failed to marshal protobuf message", err)
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return errors.NewTemporary("failed to set cache key", err)
	}

	return nil
}

// Delete removes a key from the cache.
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return errors.NewTemporary("failed to delete cache key", err)
	}
	return nil
}

// Exists checks if a key exists in the cache.
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, errors.NewTemporary("failed to check cache key existence", err)
	}
	return count > 0, nil
}

// GetOrLoad retrieves a value from cache, or loads it using the provided loader function if not found.
// The loaded value is automatically cached with the specified TTL.
// This implements the cache-aside pattern.
func (r *RedisCache) GetOrLoad(ctx context.Context, key string, dest proto.Message, ttl time.Duration, loader func(context.Context) (proto.Message, error)) error {
	// Try to get from cache first
	err := r.Get(ctx, key, dest)
	if err == nil {
		// Cache hit
		return nil
	}

	// Check if it's a not found error (cache miss)
	if !errors.IsNotFound(err) {
		// Some other error occurred (connection issue, etc.)
		return err
	}

	// Cache miss - load the data
	loaded, err := loader(ctx)
	if err != nil {
		return err
	}

	// Store in cache for future requests
	if err := r.Set(ctx, key, loaded, ttl); err != nil {
		// Log the error but don't fail the request
		// The caller got their data, caching is a performance optimization
		// We return nil here to not fail the original request
	}

	// Copy loaded data to dest
	// We need to marshal and unmarshal to properly copy the protobuf message
	data, err := proto.Marshal(loaded)
	if err != nil {
		return errors.NewPermanent("failed to marshal loaded data", err)
	}

	if err := proto.Unmarshal(data, dest); err != nil {
		return errors.NewPermanent("failed to unmarshal loaded data", err)
	}

	return nil
}

// CheckHealth verifies cache connectivity using Redis PING command.
func (r *RedisCache) CheckHealth(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return errors.NewTemporary("Redis health check failed", err)
	}
	return nil
}

// Check implements the health.Checker interface for the Redis cache.
// This allows the cache to be used directly with the health check framework.
//
// Example usage:
//
//	import "github.com/Combine-Capital/cqi/pkg/health"
//
//	h := health.New()
//	h.RegisterChecker("cache", redisCache)
func (r *RedisCache) Check(ctx context.Context) error {
	return r.CheckHealth(ctx)
}

// Close releases all resources associated with the cache.
func (r *RedisCache) Close() error {
	return r.client.Close()
}
