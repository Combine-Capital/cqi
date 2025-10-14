package cache

import (
	"context"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/alicebob/miniredis/v2"
	"google.golang.org/protobuf/proto"
)

// setupTestRedis creates a test Redis server and cache instance.
func setupTestRedis(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()

	// Start mini redis server
	mr := miniredis.RunT(t)

	cfg := config.CacheConfig{
		Host:         mr.Host(),
		Port:         mr.Server().Addr().Port,
		DB:           0,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		DefaultTTL:   5 * time.Minute,
	}

	cache, err := NewRedis(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create Redis cache: %v", err)
	}

	return cache, mr
}

func TestNewRedis(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		cache, mr := setupTestRedis(t)
		defer cache.Close()
		defer mr.Close()

		if cache == nil {
			t.Fatal("Expected cache instance, got nil")
		}
	})

	t.Run("connection failure", func(t *testing.T) {
		cfg := config.CacheConfig{
			Host:        "invalid-host-that-does-not-exist",
			Port:        9999,
			DB:          0,
			MaxRetries:  1,
			DialTimeout: 100 * time.Millisecond,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		_, err := NewRedis(ctx, cfg)
		if err == nil {
			t.Fatal("Expected error for invalid connection, got nil")
		}

		if !errors.IsTemporary(err) {
			t.Errorf("Expected temporary error, got: %v", err)
		}
	})
}

func TestRedisCache_SetAndGet(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("set and get message", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:    "test-123",
			Name:  "Test User",
			Value: 42,
			Tags:  []string{"tag1", "tag2"},
		}

		// Set the message
		err := cache.Set(ctx, "test:key", msg, 1*time.Minute)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		// Get the message
		var retrieved testproto.TestMessage
		err = cache.Get(ctx, "test:key", &retrieved)
		if err != nil {
			t.Fatalf("Failed to get from cache: %v", err)
		}

		// Verify the data
		if retrieved.Id != msg.Id {
			t.Errorf("Expected ID %s, got %s", msg.Id, retrieved.Id)
		}
		if retrieved.Name != msg.Name {
			t.Errorf("Expected Name %s, got %s", msg.Name, retrieved.Name)
		}
		if retrieved.Value != msg.Value {
			t.Errorf("Expected Value %d, got %d", msg.Value, retrieved.Value)
		}
		if len(retrieved.Tags) != len(msg.Tags) {
			t.Errorf("Expected %d tags, got %d", len(msg.Tags), len(retrieved.Tags))
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		var msg testproto.TestMessage
		err := cache.Get(ctx, "non-existent", &msg)
		if err == nil {
			t.Fatal("Expected error for non-existent key, got nil")
		}

		if !errors.IsNotFound(err) {
			t.Errorf("Expected NotFound error, got: %v", err)
		}
	})

	t.Run("set with TTL expiration", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:   "expire-test",
			Name: "Expiring",
		}

		// Set with very short TTL
		err := cache.Set(ctx, "expire:key", msg, 1*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		// Fast-forward time in miniredis
		mr.FastForward(2 * time.Millisecond)

		// Try to get - should be expired
		var retrieved testproto.TestMessage
		err = cache.Get(ctx, "expire:key", &retrieved)
		if err == nil {
			t.Fatal("Expected error for expired key, got nil")
		}

		if !errors.IsNotFound(err) {
			t.Errorf("Expected NotFound error for expired key, got: %v", err)
		}
	})

	t.Run("set with zero TTL (no expiration)", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:   "no-expire",
			Name: "Never Expires",
		}

		// Set with zero TTL
		err := cache.Set(ctx, "no-expire:key", msg, 0)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		// Fast-forward time
		mr.FastForward(10 * time.Second)

		// Should still exist
		var retrieved testproto.TestMessage
		err = cache.Get(ctx, "no-expire:key", &retrieved)
		if err != nil {
			t.Fatalf("Failed to get from cache: %v", err)
		}

		if retrieved.Id != msg.Id {
			t.Errorf("Expected ID %s, got %s", msg.Id, retrieved.Id)
		}
	})
}

func TestRedisCache_Delete(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("delete existing key", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:   "delete-test",
			Name: "To Be Deleted",
		}

		// Set the key
		err := cache.Set(ctx, "delete:key", msg, 1*time.Minute)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		// Delete the key
		err = cache.Delete(ctx, "delete:key")
		if err != nil {
			t.Fatalf("Failed to delete key: %v", err)
		}

		// Verify it's gone
		var retrieved testproto.TestMessage
		err = cache.Get(ctx, "delete:key", &retrieved)
		if err == nil {
			t.Fatal("Expected error after delete, got nil")
		}

		if !errors.IsNotFound(err) {
			t.Errorf("Expected NotFound error, got: %v", err)
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		// Should not return error
		err := cache.Delete(ctx, "non-existent-key")
		if err != nil {
			t.Errorf("Expected no error for deleting non-existent key, got: %v", err)
		}
	})
}

func TestRedisCache_Exists(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("exists for present key", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:   "exists-test",
			Name: "Exists",
		}

		err := cache.Set(ctx, "exists:key", msg, 1*time.Minute)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		exists, err := cache.Exists(ctx, "exists:key")
		if err != nil {
			t.Fatalf("Failed to check existence: %v", err)
		}

		if !exists {
			t.Error("Expected key to exist, but it doesn't")
		}
	})

	t.Run("exists for absent key", func(t *testing.T) {
		exists, err := cache.Exists(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Failed to check existence: %v", err)
		}

		if exists {
			t.Error("Expected key not to exist, but it does")
		}
	})
}

func TestRedisCache_GetOrLoad(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("cache hit", func(t *testing.T) {
		msg := &testproto.TestMessage{
			Id:   "cached",
			Name: "Cached Value",
		}

		// Pre-populate cache
		err := cache.Set(ctx, "cached:key", msg, 1*time.Minute)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		loaderCalled := false
		loader := func(ctx context.Context) (proto.Message, error) {
			loaderCalled = true
			return &testproto.TestMessage{
				Id:   "loaded",
				Name: "Loaded Value",
			}, nil
		}

		var result testproto.TestMessage
		err = cache.GetOrLoad(ctx, "cached:key", &result, 1*time.Minute, loader)
		if err != nil {
			t.Fatalf("GetOrLoad failed: %v", err)
		}

		// Loader should not be called
		if loaderCalled {
			t.Error("Expected loader not to be called for cache hit")
		}

		// Should get the cached value
		if result.Id != "cached" {
			t.Errorf("Expected cached ID 'cached', got '%s'", result.Id)
		}
	})

	t.Run("cache miss - load and populate", func(t *testing.T) {
		loaderCalled := false
		loader := func(ctx context.Context) (proto.Message, error) {
			loaderCalled = true
			return &testproto.TestMessage{
				Id:    "loaded-123",
				Name:  "Loaded Value",
				Value: 999,
			}, nil
		}

		var result testproto.TestMessage
		err := cache.GetOrLoad(ctx, "uncached:key", &result, 1*time.Minute, loader)
		if err != nil {
			t.Fatalf("GetOrLoad failed: %v", err)
		}

		// Loader should be called
		if !loaderCalled {
			t.Error("Expected loader to be called for cache miss")
		}

		// Should get the loaded value
		if result.Id != "loaded-123" {
			t.Errorf("Expected loaded ID 'loaded-123', got '%s'", result.Id)
		}

		// Value should now be in cache
		var cached testproto.TestMessage
		err = cache.Get(ctx, "uncached:key", &cached)
		if err != nil {
			t.Fatalf("Failed to get cached value after load: %v", err)
		}

		if cached.Id != result.Id {
			t.Errorf("Cached value doesn't match loaded value")
		}
	})

	t.Run("loader returns error", func(t *testing.T) {
		loader := func(ctx context.Context) (proto.Message, error) {
			return nil, errors.NewTemporary("loader failed", nil)
		}

		var result testproto.TestMessage
		err := cache.GetOrLoad(ctx, "error:key", &result, 1*time.Minute, loader)

		if err == nil {
			t.Fatal("Expected error from loader, got nil")
		}

		if !errors.IsTemporary(err) {
			t.Errorf("Expected temporary error from loader, got: %v", err)
		}
	})
}

func TestRedisCache_CheckHealth(t *testing.T) {
	t.Run("healthy connection", func(t *testing.T) {
		cache, mr := setupTestRedis(t)
		defer cache.Close()
		defer mr.Close()

		ctx := context.Background()
		err := cache.CheckHealth(ctx)
		if err != nil {
			t.Fatalf("Expected healthy connection, got error: %v", err)
		}
	})

	t.Run("unhealthy connection", func(t *testing.T) {
		cache, mr := setupTestRedis(t)
		defer cache.Close()

		// Close the server
		mr.Close()

		ctx := context.Background()
		err := cache.CheckHealth(ctx)
		if err == nil {
			t.Fatal("Expected error for unhealthy connection, got nil")
		}

		if !errors.IsTemporary(err) {
			t.Errorf("Expected temporary error, got: %v", err)
		}
	})
}

func TestCheckHealthWithTimeout(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	t.Run("health check succeeds within timeout", func(t *testing.T) {
		err := CheckHealthWithTimeout(cache, 1*time.Second)
		if err != nil {
			t.Fatalf("Expected successful health check, got error: %v", err)
		}
	})

	t.Run("health check fails", func(t *testing.T) {
		// Close the server
		mr.Close()

		err := CheckHealthWithTimeout(cache, 100*time.Millisecond)
		if err == nil {
			t.Fatal("Expected error for failed health check, got nil")
		}

		if !errors.IsTemporary(err) {
			t.Errorf("Expected temporary error, got: %v", err)
		}
	})
}

func TestKey(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		parts    []string
		expected string
	}{
		{
			name:     "simple key",
			prefix:   "user",
			parts:    []string{"123"},
			expected: "user:123",
		},
		{
			name:     "multiple parts",
			prefix:   "portfolio",
			parts:    []string{"abc", "stats"},
			expected: "portfolio:abc:stats",
		},
		{
			name:     "empty parts filtered",
			prefix:   "asset",
			parts:    []string{"xyz", "", "price"},
			expected: "asset:xyz:price",
		},
		{
			name:     "no parts",
			prefix:   "global",
			parts:    []string{},
			expected: "global",
		},
		{
			name:     "empty prefix",
			prefix:   "",
			parts:    []string{"test", "key"},
			expected: "test:key",
		},
		{
			name:     "all empty",
			prefix:   "",
			parts:    []string{"", ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Key(tt.prefix, tt.parts...)
			if result != tt.expected {
				t.Errorf("Expected key '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRedisCache_ContextCancellation(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	t.Run("get with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var msg testproto.TestMessage
		err := cache.Get(ctx, "test:key", &msg)
		if err == nil {
			t.Fatal("Expected error for cancelled context, got nil")
		}
	})

	t.Run("set with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		msg := &testproto.TestMessage{Id: "test"}
		err := cache.Set(ctx, "test:key", msg, 1*time.Minute)
		if err == nil {
			t.Fatal("Expected error for cancelled context, got nil")
		}
	})
}

func TestRedisCache_InvalidData(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("get with corrupted data", func(t *testing.T) {
		// Set raw corrupted data directly in Redis
		mr.Set("corrupted:key", "not-a-valid-protobuf-message")

		var msg testproto.TestMessage
		err := cache.Get(ctx, "corrupted:key", &msg)
		if err == nil {
			t.Fatal("Expected error for corrupted data, got nil")
		}

		if !errors.IsPermanent(err) {
			t.Errorf("Expected permanent error for corrupted data, got: %v", err)
		}
	})
}

func TestRedisCache_Close(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer mr.Close()

	err := cache.Close()
	if err != nil {
		t.Fatalf("Failed to close cache: %v", err)
	}

	// After close, operations should fail
	ctx := context.Background()
	var msg testproto.TestMessage
	err = cache.Get(ctx, "test:key", &msg)
	if err == nil {
		t.Error("Expected error after close, got nil")
	}
}

func TestRedisCache_EdgeCases(t *testing.T) {
	cache, mr := setupTestRedis(t)
	defer cache.Close()
	defer mr.Close()

	ctx := context.Background()

	t.Run("delete with context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := cache.Delete(ctx, "test:key")
		if err == nil {
			t.Error("Expected error for cancelled context, got nil")
		}
	})

	t.Run("exists with context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := cache.Exists(ctx, "test:key")
		if err == nil {
			t.Error("Expected error for cancelled context, got nil")
		}
	})

	t.Run("GetOrLoad with cache set failure is ignored", func(t *testing.T) {
		loaderCalled := false
		loader := func(ctx context.Context) (proto.Message, error) {
			loaderCalled = true
			return &testproto.TestMessage{
				Id:   "loaded",
				Name: "Value",
			}, nil
		}

		var result testproto.TestMessage
		// This should succeed even though cache is closed after load
		err := cache.GetOrLoad(ctx, "will-load:key", &result, 1*time.Minute, loader)
		if err != nil {
			t.Fatalf("GetOrLoad should succeed even if cache set fails: %v", err)
		}

		if !loaderCalled {
			t.Error("Loader should have been called")
		}

		if result.Id != "loaded" {
			t.Errorf("Expected loaded value, got: %s", result.Id)
		}
	})
}
