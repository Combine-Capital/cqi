// Package cache provides Redis caching with protobuf serialization for CQI infrastructure.
// It supports storing and retrieving protocol buffer messages with configurable TTL,
// cache-aside patterns, and health checking.
//
// Example usage:
//
//	cfg := config.CacheConfig{
//	    Host: "localhost",
//	    Port: 6379,
//	    DB:   0,
//	}
//
//	c, err := cache.NewRedis(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	// Store a protobuf message
//	user := &pb.User{Id: "123", Name: "John"}
//	err = c.Set(ctx, "user:123", user, 5*time.Minute)
//
//	// Retrieve the message
//	var retrieved pb.User
//	err = c.Get(ctx, "user:123", &retrieved)
//
//	// Use cache-aside pattern
//	key := cache.Key("user", userID)
//	err = c.GetOrLoad(ctx, key, &user, 5*time.Minute, func(ctx context.Context) (proto.Message, error) {
//	    return db.GetUser(ctx, userID)
//	})
package cache

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
)

// Cache defines the interface for cache operations with protobuf message support.
// All methods respect context cancellation and timeout.
type Cache interface {
	// Get retrieves a cached protobuf message by key and unmarshals it into dest.
	// Returns an error if the key doesn't exist or deserialization fails.
	// The dest parameter must be a pointer to a protobuf message.
	Get(ctx context.Context, key string, dest proto.Message) error

	// Set stores a protobuf message in the cache with the specified TTL.
	// The message is serialized to wire format before storage.
	// A TTL of 0 means no expiration.
	Set(ctx context.Context, key string, value proto.Message, ttl time.Duration) error

	// Delete removes a key from the cache.
	// Returns nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	// Returns true if the key exists, false otherwise.
	Exists(ctx context.Context, key string) (bool, error)

	// GetOrLoad retrieves a value from cache, or loads it using the provided loader function if not found.
	// The loaded value is automatically cached with the specified TTL.
	// This implements the cache-aside pattern.
	GetOrLoad(ctx context.Context, key string, dest proto.Message, ttl time.Duration, loader func(context.Context) (proto.Message, error)) error

	// CheckHealth verifies cache connectivity and returns an error if unavailable.
	CheckHealth(ctx context.Context) error

	// Close releases all resources associated with the cache.
	Close() error
}
