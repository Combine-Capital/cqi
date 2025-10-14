package registry

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// TestRedisRegistry tests the Redis registry implementation using miniredis.
func TestRedisRegistry(t *testing.T) {
	// Start mini Redis server
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	t.Run("Create and connect", func(t *testing.T) {
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		if registry.ttl != 30*time.Second {
			t.Errorf("Expected TTL 30s, got %v", registry.ttl)
		}
		if registry.interval != 10*time.Second {
			t.Errorf("Expected interval 10s, got %v", registry.interval)
		}
	})

	t.Run("Default configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr: mr.Addr(),
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		if registry.ttl != 30*time.Second {
			t.Errorf("Expected default TTL 30s, got %v", registry.ttl)
		}
		if registry.interval != 10*time.Second {
			t.Errorf("Expected default interval 10s, got %v", registry.interval)
		}
	})

	t.Run("Invalid config - heartbeat >= TTL", func(t *testing.T) {
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               10 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		_, err := NewRedisRegistry(ctx, cfg)
		if err == nil {
			t.Error("Expected error when heartbeat interval >= TTL")
		}
	})

	t.Run("Connection failure", func(t *testing.T) {
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr: "invalid:1234",
			TTL:       30 * time.Second,
		}

		_, err := NewRedisRegistry(ctx, cfg)
		if err == nil {
			t.Error("Expected error when connecting to invalid address")
		}
	})

	t.Run("Register and Discover", func(t *testing.T) {
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               5 * time.Second,
			HeartbeatInterval: 1 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Register a service
		service := ServiceInfo{
			Name:           "api-service",
			Version:        "1.0.0",
			Address:        "192.168.1.10",
			Port:           8080,
			HealthEndpoint: "/health/ready",
			Metadata: map[string]string{
				"env": "production",
			},
		}

		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Discover the service
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 1 {
			t.Fatalf("Expected 1 service, got %d", len(services))
		}

		discovered := services[0]
		if discovered.Name != service.Name {
			t.Errorf("Expected name %s, got %s", service.Name, discovered.Name)
		}
		if discovered.Version != service.Version {
			t.Errorf("Expected version %s, got %s", service.Version, discovered.Version)
		}
		if discovered.Address != service.Address {
			t.Errorf("Expected address %s, got %s", service.Address, discovered.Address)
		}
		if discovered.Port != service.Port {
			t.Errorf("Expected port %d, got %d", service.Port, discovered.Port)
		}
		if discovered.HealthEndpoint != service.HealthEndpoint {
			t.Errorf("Expected health endpoint %s, got %s", service.HealthEndpoint, discovered.HealthEndpoint)
		}
	})

	t.Run("Register multiple instances", func(t *testing.T) {
		mr.FlushAll() // Clean slate
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Register three instances
		for i := 1; i <= 3; i++ {
			service := ServiceInfo{
				Name:    "api-service",
				Version: "1.0.0",
				Address: "192.168.1.10",
				Port:    8080 + i,
			}
			if err := registry.Register(ctx, service); err != nil {
				t.Fatalf("Failed to register service %d: %v", i, err)
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Discover all instances
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 3 {
			t.Fatalf("Expected 3 services, got %d", len(services))
		}

		// Verify sorted by registration time
		for i := 0; i < len(services)-1; i++ {
			if services[i].RegisteredAt.After(services[i+1].RegisteredAt) {
				t.Error("Services should be sorted by registration time")
			}
		}
	})

	t.Run("Deregister service", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Register a service
		service := ServiceInfo{
			ID:      "test-id",
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
		}
		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Deregister the service
		if err := registry.Deregister(ctx, service.ID); err != nil {
			t.Fatalf("Failed to deregister service: %v", err)
		}

		// Verify service is gone
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("Expected 0 services after deregister, got %d", len(services))
		}
	})

	t.Run("Deregister non-existent service", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Deregister non-existent service should not error
		if err := registry.Deregister(ctx, "non-existent"); err != nil {
			t.Errorf("Deregister non-existent should not error: %v", err)
		}
	})

	t.Run("Discover non-existent service", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		services, err := registry.Discover(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Discover should not error for non-existent service: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("Expected 0 services, got %d", len(services))
		}
	})

	t.Run("Heartbeat keeps service alive", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               2 * time.Second,
			HeartbeatInterval: 500 * time.Millisecond,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Register a service
		service := ServiceInfo{
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
		}
		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Wait longer than TTL
		time.Sleep(3 * time.Second)

		// Service should still be there due to heartbeat
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 1 {
			t.Errorf("Expected 1 service (kept alive by heartbeat), got %d", len(services))
		}
	})

	t.Run("Service expires without heartbeat", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               1 * time.Second,
			HeartbeatInterval: 500 * time.Millisecond,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}

		// Register a service
		service := ServiceInfo{
			ID:      "test-id",
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
		}
		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Stop heartbeat by deregistering
		registry.stopHeartbeat(service.ID)

		// Fast-forward time in miniredis
		mr.FastForward(2 * time.Second)

		// Service should be expired
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("Expected 0 services (expired), got %d", len(services))
		}

		registry.Close()
	})

	t.Run("Redis key format", func(t *testing.T) {
		registry := &RedisRegistry{}

		serviceKey := registry.serviceKey("test-id")
		if serviceKey != "registry:service:test-id" {
			t.Errorf("Unexpected service key: %s", serviceKey)
		}

		nameKey := registry.nameIndexKey("api-service")
		if nameKey != "registry:name:api-service" {
			t.Errorf("Unexpected name index key: %s", nameKey)
		}
	})

	t.Run("Corrupted data handling", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Manually insert corrupted data
		registry.client.Set(ctx, "registry:service:corrupted", "invalid json", 30*time.Second)
		registry.client.SAdd(ctx, "registry:name:api-service", "corrupted")

		// Discover should handle corrupted data gracefully
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Discover should handle corrupted data: %v", err)
		}

		// Should return empty list (corrupted entry skipped)
		if len(services) != 0 {
			t.Errorf("Expected 0 services (corrupted data skipped), got %d", len(services))
		}
	})
}

// TestRedisRegistryValidation tests validation for Redis registry.
func TestRedisRegistryValidation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	ctx := context.Background()
	cfg := RedisRegistryConfig{
		RedisAddr:         mr.Addr(),
		TTL:               30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
	}

	registry, err := NewRedisRegistry(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Redis registry: %v", err)
	}
	defer registry.Close()

	tests := []struct {
		name    string
		service ServiceInfo
		wantErr bool
	}{
		{
			name: "Missing service name",
			service: ServiceInfo{
				Address: "192.168.1.10",
				Port:    8080,
			},
			wantErr: true,
		},
		{
			name: "Missing address",
			service: ServiceInfo{
				Name: "api-service",
				Port: 8080,
			},
			wantErr: true,
		},
		{
			name: "Invalid port",
			service: ServiceInfo{
				Name:    "api-service",
				Address: "192.168.1.10",
				Port:    0,
			},
			wantErr: true,
		},
		{
			name: "Valid service",
			service: ServiceInfo{
				Name:    "api-service",
				Address: "192.168.1.10",
				Port:    8080,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(ctx, tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestServiceInfoSerialization tests JSON serialization of ServiceInfo.
func TestServiceInfoSerialization(t *testing.T) {
	service := ServiceInfo{
		ID:             "test-id",
		Name:           "api-service",
		Version:        "1.0.0",
		Address:        "192.168.1.10",
		Port:           8080,
		HealthEndpoint: "/health/ready",
		Metadata: map[string]string{
			"env":    "production",
			"region": "us-west",
		},
		RegisteredAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Marshal
	data, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("Failed to marshal service: %v", err)
	}

	// Unmarshal
	var decoded ServiceInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal service: %v", err)
	}

	// Verify
	if decoded.ID != service.ID {
		t.Errorf("ID mismatch: %s != %s", decoded.ID, service.ID)
	}
	if decoded.Name != service.Name {
		t.Errorf("Name mismatch: %s != %s", decoded.Name, service.Name)
	}
	if decoded.Port != service.Port {
		t.Errorf("Port mismatch: %d != %d", decoded.Port, service.Port)
	}
	if decoded.Metadata["env"] != service.Metadata["env"] {
		t.Errorf("Metadata mismatch")
	}
}

// TestRedisErrorPaths tests error handling in Redis operations.
func TestRedisErrorPaths(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()

	t.Run("Deregister with corrupted service data", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Set corrupted service data
		serviceID := "corrupted-id"
		key := registry.serviceKey(serviceID)
		registry.client.Set(ctx, key, "invalid json data", 30*time.Second)

		// Deregister should handle corrupted data gracefully
		err = registry.Deregister(ctx, serviceID)
		if err != nil {
			t.Errorf("Deregister should handle corrupted data gracefully: %v", err)
		}

		// Verify service was deleted despite corruption
		exists, _ := registry.client.Exists(ctx, key).Result()
		if exists == 1 {
			t.Error("Service key should have been deleted")
		}
	})

	t.Run("Discover with Redis error simulation", func(t *testing.T) {
		mr.FlushAll()
		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}
		defer registry.Close()

		// Register a service first
		service := ServiceInfo{
			Name:    "test-service",
			Address: "localhost",
			Port:    8080,
		}
		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Now close miniredis to simulate connection errors
		mr.Close()

		// Discover should return an error
		_, err = registry.Discover(ctx, "test-service")
		if err == nil {
			t.Error("Expected error when Redis connection is closed")
		}
	})

	t.Run("Register with connection closed", func(t *testing.T) {
		mr2, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to start miniredis: %v", err)
		}

		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr2.Addr(),
			TTL:               30 * time.Second,
			HeartbeatInterval: 10 * time.Second,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}

		// Close miniredis to simulate connection errors
		mr2.Close()

		// Register should return an error
		service := ServiceInfo{
			Name:    "test-service",
			Address: "localhost",
			Port:    8080,
		}
		err = registry.Register(ctx, service)
		if err == nil {
			t.Error("Expected error when Redis connection is closed")
		}

		registry.Close()
	})

	t.Run("Heartbeat with Redis errors", func(t *testing.T) {
		mr3, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to start miniredis: %v", err)
		}

		ctx := context.Background()
		cfg := RedisRegistryConfig{
			RedisAddr:         mr3.Addr(),
			TTL:               2 * time.Second,
			HeartbeatInterval: 500 * time.Millisecond,
		}

		registry, err := NewRedisRegistry(ctx, cfg)
		if err != nil {
			t.Fatalf("Failed to create Redis registry: %v", err)
		}

		// Register a service to start heartbeat
		service := ServiceInfo{
			Name:    "test-service",
			Address: "localhost",
			Port:    8080,
		}
		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Give heartbeat time to start
		time.Sleep(100 * time.Millisecond)

		// Close miniredis to cause heartbeat errors
		mr3.Close()

		// Wait for a few heartbeat attempts
		time.Sleep(1 * time.Second)

		// Close should not panic even with heartbeat errors
		registry.Close()
	})
}
