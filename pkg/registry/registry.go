// Package registry provides service discovery and registration capabilities.
// It supports both local (in-memory) and distributed (Redis) backends for
// dynamic service discovery with TTL-based health checking and automatic heartbeats.
//
// Example usage:
//
//	// Create local registry for development
//	registry := registry.NewLocalRegistry()
//
//	// Register a service
//	info := registry.ServiceInfo{
//	    Name:           "api-service",
//	    Version:        "1.0.0",
//	    Address:        "192.168.1.10",
//	    Port:           8080,
//	    HealthEndpoint: "/health/ready",
//	}
//	if err := registry.Register(ctx, info); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Discover services
//	services, err := registry.Discover(ctx, "api-service")
//	if err != nil {
//	    log.Fatal(err)
//	}
package registry

import (
	"context"
	"time"
)

// Registry provides service registration and discovery functionality.
// Implementations must be safe for concurrent access.
type Registry interface {
	// Register registers a service instance with the registry.
	// The service remains registered until explicitly deregistered or its TTL expires.
	// Returns an error if registration fails.
	Register(ctx context.Context, service ServiceInfo) error

	// Deregister removes a service instance from the registry.
	// If the service is not found, this is a no-op.
	// Returns an error if deregistration fails.
	Deregister(ctx context.Context, serviceID string) error

	// Discover returns all healthy instances of a service by name.
	// Results are sorted by registration time (oldest first).
	// Returns an empty slice if no instances are found.
	Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error)

	// Close closes the registry and releases any resources.
	// After calling Close, the registry should not be used.
	Close() error
}

// ServiceInfo contains metadata about a registered service instance.
type ServiceInfo struct {
	// ID is a unique identifier for this service instance.
	// If empty during registration, one will be generated.
	ID string

	// Name is the service name (e.g., "api-service", "worker-service").
	// Multiple instances can share the same name.
	Name string

	// Version is the service version (e.g., "1.0.0", "v2.3.1").
	Version string

	// Address is the IP address or hostname where the service is reachable.
	Address string

	// Port is the primary port where the service listens.
	Port int

	// HealthEndpoint is the HTTP path for health checks (e.g., "/health/ready").
	// Optional but recommended for automatic health monitoring.
	HealthEndpoint string

	// Metadata contains additional key-value pairs for custom service information.
	// This can include tags, environment, region, etc.
	Metadata map[string]string

	// RegisteredAt is the timestamp when the service was registered.
	// This is set automatically during registration.
	RegisteredAt time.Time
}

// RegistryType represents the type of registry backend.
type RegistryType string

const (
	// RegistryTypeLocal uses in-memory storage for development/testing.
	RegistryTypeLocal RegistryType = "local"

	// RegistryTypeRedis uses Redis for distributed service discovery.
	RegistryTypeRedis RegistryType = "redis"
)
