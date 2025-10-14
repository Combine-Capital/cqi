package registry

import (
	"context"
	"testing"
	"time"
)

// TestLocalRegistry tests the local in-memory registry implementation.
func TestLocalRegistry(t *testing.T) {
	t.Run("Register and Discover", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

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

		err := registry.Register(ctx, service)
		if err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Discover the service to get the registered version with generated ID
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 1 {
			t.Fatalf("Expected 1 service, got %d", len(services))
		}

		discovered := services[0]

		// Verify ID was generated
		if discovered.ID == "" {
			t.Error("Service ID should be generated")
		}
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
	})

	t.Run("Register multiple instances", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		// Register three instances of the same service
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
			time.Sleep(10 * time.Millisecond) // Ensure different registration times
		}

		// Discover all instances
		services, err := registry.Discover(ctx, "api-service")
		if err != nil {
			t.Fatalf("Failed to discover services: %v", err)
		}

		if len(services) != 3 {
			t.Fatalf("Expected 3 services, got %d", len(services))
		}

		// Verify sorted by registration time (oldest first)
		for i := 0; i < len(services)-1; i++ {
			if services[i].RegisteredAt.After(services[i+1].RegisteredAt) {
				t.Error("Services should be sorted by registration time")
			}
		}
	})

	t.Run("Deregister service", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

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

	t.Run("Discover non-existent service", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		services, err := registry.Discover(ctx, "non-existent")
		if err != nil {
			t.Fatalf("Discover should not error for non-existent service: %v", err)
		}

		if len(services) != 0 {
			t.Errorf("Expected 0 services, got %d", len(services))
		}
	})

	t.Run("Validation errors", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

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
				name: "Invalid port - zero",
				service: ServiceInfo{
					Name:    "api-service",
					Address: "192.168.1.10",
					Port:    0,
				},
				wantErr: true,
			},
			{
				name: "Invalid port - negative",
				service: ServiceInfo{
					Name:    "api-service",
					Address: "192.168.1.10",
					Port:    -1,
				},
				wantErr: true,
			},
			{
				name: "Invalid port - too large",
				service: ServiceInfo{
					Name:    "api-service",
					Address: "192.168.1.10",
					Port:    65536,
				},
				wantErr: true,
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
	})

	t.Run("Close registry", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		// Register services
		for i := 1; i <= 3; i++ {
			service := ServiceInfo{
				Name:    "api-service",
				Address: "192.168.1.10",
				Port:    8080 + i,
			}
			registry.Register(ctx, service)
		}

		// Close registry
		if err := registry.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Verify all services are removed
		services, _ := registry.Discover(ctx, "api-service")
		if len(services) != 0 {
			t.Errorf("Expected 0 services after close, got %d", len(services))
		}
	})

	t.Run("Custom service ID", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		service := ServiceInfo{
			ID:      "custom-id-123",
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
		}

		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		// Deregister using custom ID
		if err := registry.Deregister(ctx, "custom-id-123"); err != nil {
			t.Fatalf("Failed to deregister service: %v", err)
		}

		services, _ := registry.Discover(ctx, "api-service")
		if len(services) != 0 {
			t.Errorf("Expected 0 services, got %d", len(services))
		}
	})

	t.Run("Metadata initialization", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		service := ServiceInfo{
			Name:     "api-service",
			Address:  "192.168.1.10",
			Port:     8080,
			Metadata: nil, // Explicitly nil
		}

		if err := registry.Register(ctx, service); err != nil {
			t.Fatalf("Failed to register service: %v", err)
		}

		services, _ := registry.Discover(ctx, "api-service")
		if len(services) != 1 {
			t.Fatalf("Expected 1 service, got %d", len(services))
		}

		if services[0].Metadata == nil {
			t.Error("Metadata should be initialized to empty map")
		}
	})
}

// TestServiceInfo tests the ServiceInfo struct.
func TestServiceInfo(t *testing.T) {
	t.Run("Registration timestamp", func(t *testing.T) {
		registry := NewLocalRegistry()
		ctx := context.Background()

		before := time.Now()
		service := ServiceInfo{
			Name:    "api-service",
			Address: "192.168.1.10",
			Port:    8080,
		}
		registry.Register(ctx, service)
		after := time.Now()

		services, _ := registry.Discover(ctx, "api-service")
		if len(services) != 1 {
			t.Fatalf("Expected 1 service, got %d", len(services))
		}

		registered := services[0].RegisteredAt
		if registered.Before(before) || registered.After(after) {
			t.Errorf("RegisteredAt %v should be between %v and %v", registered, before, after)
		}
	})
}

// TestRegistryInterface verifies that our implementations satisfy the Registry interface.
func TestRegistryInterface(t *testing.T) {
	var _ Registry = (*LocalRegistry)(nil)
	var _ Registry = (*RedisRegistry)(nil)
}

// TestConcurrentAccess tests concurrent registration and discovery.
func TestConcurrentAccess(t *testing.T) {
	registry := NewLocalRegistry()
	ctx := context.Background()

	// Spawn multiple goroutines registering services
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			service := ServiceInfo{
				Name:    "api-service",
				Address: "192.168.1.10",
				Port:    8080 + id,
			}
			registry.Register(ctx, service)
			done <- true
		}(i)
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	// Discover all services
	services, err := registry.Discover(ctx, "api-service")
	if err != nil {
		t.Fatalf("Failed to discover services: %v", err)
	}

	if len(services) != 10 {
		t.Errorf("Expected 10 services, got %d", len(services))
	}
}

// BenchmarkLocalRegisterDiscover benchmarks registration and discovery.
func BenchmarkLocalRegisterDiscover(b *testing.B) {
	registry := NewLocalRegistry()
	ctx := context.Background()

	service := ServiceInfo{
		Name:    "api-service",
		Address: "192.168.1.10",
		Port:    8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Register(ctx, service)
		registry.Discover(ctx, "api-service")
	}
}
