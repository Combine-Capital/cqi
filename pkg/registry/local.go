package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LocalRegistry implements Registry using an in-memory sync.Map.
// It's suitable for development, testing, and single-instance deployments.
// For distributed service discovery, use RedisRegistry instead.
type LocalRegistry struct {
	services sync.Map // map[string]ServiceInfo (key: service ID)
	mu       sync.RWMutex
}

// NewLocalRegistry creates a new in-memory registry.
func NewLocalRegistry() *LocalRegistry {
	return &LocalRegistry{}
}

// Register registers a service instance in the local registry.
// If the service ID is empty, a UUID will be generated.
func (r *LocalRegistry) Register(ctx context.Context, service ServiceInfo) error {
	// Validate required fields
	if service.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if service.Address == "" {
		return fmt.Errorf("service address is required")
	}
	if service.Port <= 0 || service.Port > 65535 {
		return fmt.Errorf("service port must be between 1 and 65535")
	}

	// Generate ID if not provided
	if service.ID == "" {
		service.ID = uuid.New().String()
	}

	// Set registration timestamp
	service.RegisteredAt = time.Now()

	// Initialize metadata if nil
	if service.Metadata == nil {
		service.Metadata = make(map[string]string)
	}

	// Store in registry
	r.services.Store(service.ID, service)

	return nil
}

// Deregister removes a service instance from the local registry.
func (r *LocalRegistry) Deregister(ctx context.Context, serviceID string) error {
	r.services.Delete(serviceID)
	return nil
}

// Discover returns all registered instances of a service by name.
// Results are sorted by registration time (oldest first).
func (r *LocalRegistry) Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error) {
	var results []ServiceInfo

	// Iterate over all services
	r.services.Range(func(key, value interface{}) bool {
		service := value.(ServiceInfo)
		if service.Name == serviceName {
			results = append(results, service)
		}
		return true
	})

	// Sort by registration time
	sort.Slice(results, func(i, j int) bool {
		return results[i].RegisteredAt.Before(results[j].RegisteredAt)
	})

	return results, nil
}

// Close closes the local registry (no-op for local registry).
func (r *LocalRegistry) Close() error {
	// Clear all services
	r.services.Range(func(key, value interface{}) bool {
		r.services.Delete(key)
		return true
	})
	return nil
}
