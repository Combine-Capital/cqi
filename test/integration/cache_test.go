// Package integration provides integration tests for cache operations.
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/cache"
	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/config"
)

// TestCacheConnection tests basic Redis connectivity.
func TestCacheConnection(t *testing.T) {
	ctx := context.Background()

	cfg := config.CacheConfig{
		Host:        "localhost",
		Port:        6379,
		Password:    "",
		DB:          0,
		DialTimeout: 5 * time.Second,
		PoolSize:    10,
	}

	redisCache, err := cache.NewRedis(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// Test basic connectivity
	if err := redisCache.CheckHealth(ctx); err != nil {
		t.Fatalf("Cache health check failed: %v", err)
	}
}

// TestCacheSetGet tests Set and Get operations.
func TestCacheSetGet(t *testing.T) {
	ctx := context.Background()

	redisCache := setupCache(t, ctx)
	defer redisCache.Close()

	// Create test message
	testMsg := &testproto.TestMessage{
		Id:    "test-1",
		Name:  "Test Message",
		Value: 42,
		Tags:  []string{"tag1", "tag2"},
	}

	// Test Set
	key := cache.Key("test", "set-get", testMsg.Id)
	if err := redisCache.Set(ctx, key, testMsg, 1*time.Minute); err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Test Get
	var retrieved testproto.TestMessage
	if err := redisCache.Get(ctx, key, &retrieved); err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}

	// Verify values
	if retrieved.Id != testMsg.Id {
		t.Errorf("Expected Id=%s, got Id=%s", testMsg.Id, retrieved.Id)
	}
	if retrieved.Name != testMsg.Name {
		t.Errorf("Expected Name=%s, got Name=%s", testMsg.Name, retrieved.Name)
	}
	if retrieved.Value != testMsg.Value {
		t.Errorf("Expected Value=%d, got Value=%d", testMsg.Value, retrieved.Value)
	}
	if len(retrieved.Tags) != len(testMsg.Tags) {
		t.Errorf("Expected %d tags, got %d", len(testMsg.Tags), len(retrieved.Tags))
	}
}

// TestCacheDelete tests Delete operation.
func TestCacheDelete(t *testing.T) {
	ctx := context.Background()

	redisCache := setupCache(t, ctx)
	defer redisCache.Close()

	// Set a value
	testMsg := &testproto.TestMessage{
		Id:    "test-delete",
		Name:  "To Be Deleted",
		Value: 100,
	}
	key := cache.Key("test", "delete", testMsg.Id)
	if err := redisCache.Set(ctx, key, testMsg, 1*time.Minute); err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Delete the value
	if err := redisCache.Delete(ctx, key); err != nil {
		t.Fatalf("Failed to delete from cache: %v", err)
	}

	// Verify it's gone
	var retrieved testproto.TestMessage
	err := redisCache.Get(ctx, key, &retrieved)
	if err == nil {
		t.Fatal("Expected error when getting deleted key, got nil")
	}
}

// TestCacheExists tests Exists operation.
func TestCacheExists(t *testing.T) {
	ctx := context.Background()

	redisCache := setupCache(t, ctx)
	defer redisCache.Close()

	key := cache.Key("test", "exists")

	// Test non-existent key
	exists, err := redisCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist, but it does")
	}

	// Set a value
	testMsg := &testproto.TestMessage{
		Id:    "test-exists",
		Name:  "Exists Test",
		Value: 123,
	}
	if err := redisCache.Set(ctx, key, testMsg, 1*time.Minute); err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Test existing key
	exists, err = redisCache.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist, but it doesn't")
	}
}

// TestCacheTTL tests TTL expiration.
func TestCacheTTL(t *testing.T) {
	ctx := context.Background()

	redisCache := setupCache(t, ctx)
	defer redisCache.Close()

	// Set with short TTL
	testMsg := &testproto.TestMessage{
		Id:    "test-ttl",
		Name:  "TTL Test",
		Value: 456,
	}
	key := cache.Key("test", "ttl", testMsg.Id)
	if err := redisCache.Set(ctx, key, testMsg, 100*time.Millisecond); err != nil {
		t.Fatalf("Failed to set cache with TTL: %v", err)
	}

	// Verify it exists initially
	var retrieved testproto.TestMessage
	if err := redisCache.Get(ctx, key, &retrieved); err != nil {
		t.Fatalf("Failed to get immediately after set: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Verify it's gone
	err := redisCache.Get(ctx, key, &retrieved)
	if err == nil {
		t.Fatal("Expected error when getting expired key, got nil")
	}
}

// setupCache creates a cache client for testing.
func setupCache(t *testing.T, ctx context.Context) *cache.RedisCache {
	t.Helper()

	cfg := config.CacheConfig{
		Host:        "localhost",
		Port:        6379,
		Password:    "",
		DB:          0,
		DialTimeout: 5 * time.Second,
		PoolSize:    10,
	}

	redisCache, err := cache.NewRedis(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}

	return redisCache
}
