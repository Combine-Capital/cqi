package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
)

func TestNewPool(t *testing.T) {
	t.Run("creates pool with valid size", func(t *testing.T) {
		cfg := config.WebSocketConfig{
			URL:      "ws://localhost:8080",
			PoolSize: 3,
		}

		pool, err := NewPool(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Failed to create pool: %v", err)
		}
		defer pool.Close()

		if pool == nil {
			t.Fatal("Expected non-nil pool")
		}
	})

	t.Run("validates pool size", func(t *testing.T) {
		cfg := config.WebSocketConfig{
			URL:      "ws://localhost:8080",
			PoolSize: 0,
		}

		_, err := NewPool(context.Background(), cfg)
		if err == nil {
			t.Fatal("Expected validation error for zero pool size")
		}
	})

	t.Run("validates negative pool size", func(t *testing.T) {
		cfg := config.WebSocketConfig{
			URL:      "ws://localhost:8080",
			PoolSize: -1,
		}

		_, err := NewPool(context.Background(), cfg)
		if err == nil {
			t.Fatal("Expected validation error for negative pool size")
		}
	})
}

func TestPool_Connect(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     3,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	if pool.Size() != 3 {
		t.Errorf("Expected pool size 3, got: %d", pool.Size())
	}

	healthyCount := pool.HealthyCount()
	if healthyCount != 3 {
		t.Errorf("Expected 3 healthy connections, got: %d", healthyCount)
	}
}

func TestPool_Get(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     3,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	// Get connection multiple times (should round-robin)
	conn1 := pool.Get()
	if conn1 == nil {
		t.Fatal("Expected non-nil connection")
	}

	conn2 := pool.Get()
	if conn2 == nil {
		t.Fatal("Expected non-nil connection")
	}

	// Connections should be different (round-robin)
	if conn1 == conn2 {
		t.Error("Expected different connections from round-robin")
	}
}

func TestPool_GetHealthy(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     3,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	// Get healthy connection
	conn := pool.GetHealthy()
	if conn == nil {
		t.Fatal("Expected non-nil healthy connection")
	}

	if !conn.IsConnected() {
		t.Error("Expected connection to be connected")
	}
}

func TestPool_Size(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     5,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	if pool.Size() != 5 {
		t.Errorf("Expected pool size 5, got: %d", pool.Size())
	}
}

func TestPool_HealthyCount(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     3,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	healthyCount := pool.HealthyCount()
	if healthyCount != 3 {
		t.Errorf("Expected 3 healthy connections, got: %d", healthyCount)
	}
}

func TestPool_Close(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PoolSize:     3,
		PingInterval: 1 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	err = pool.Connect(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}

	// Close pool
	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close pool: %v", err)
	}

	// Healthy count should be 0 after close
	healthyCount := pool.HealthyCount()
	if healthyCount != 0 {
		t.Errorf("Expected 0 healthy connections after close, got: %d", healthyCount)
	}
}

func TestPool_GetFromEmptyPool(t *testing.T) {
	cfg := config.WebSocketConfig{
		URL:      "ws://localhost:8080",
		PoolSize: 1,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Don't connect, pool is empty
	conn := pool.Get()
	if conn != nil {
		t.Error("Expected nil connection from empty pool")
	}
}
