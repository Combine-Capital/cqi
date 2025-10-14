package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisRegistry implements Registry using Redis as the backend.
// It supports TTL-based health checking and automatic heartbeats to keep
// services alive in the registry.
type RedisRegistry struct {
	client   *redis.Client
	ttl      time.Duration
	interval time.Duration

	// Heartbeat management
	heartbeats map[string]context.CancelFunc
	mu         sync.RWMutex
}

// RedisRegistryConfig configures the Redis registry.
type RedisRegistryConfig struct {
	// RedisAddr is the Redis server address (host:port).
	RedisAddr string

	// Password is the Redis password (optional).
	Password string

	// DB is the Redis database number.
	DB int

	// TTL is the time-to-live for service registrations.
	// Services must send heartbeats more frequently than this to stay registered.
	// Default: 30 seconds.
	TTL time.Duration

	// HeartbeatInterval is how often to send heartbeat updates.
	// Should be less than TTL to prevent expiration.
	// Default: 10 seconds.
	HeartbeatInterval time.Duration
}

// NewRedisRegistry creates a new Redis-backed registry with automatic heartbeats.
func NewRedisRegistry(ctx context.Context, cfg RedisRegistryConfig) (*RedisRegistry, error) {
	// Set defaults
	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}

	// Validate heartbeat interval < TTL
	if cfg.HeartbeatInterval >= cfg.TTL {
		return nil, fmt.Errorf("heartbeat interval (%v) must be less than TTL (%v)", cfg.HeartbeatInterval, cfg.TTL)
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisRegistry{
		client:     client,
		ttl:        cfg.TTL,
		interval:   cfg.HeartbeatInterval,
		heartbeats: make(map[string]context.CancelFunc),
	}, nil
}

// Register registers a service instance in Redis with TTL and starts a heartbeat goroutine.
func (r *RedisRegistry) Register(ctx context.Context, service ServiceInfo) error {
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

	// Serialize service info
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service info: %w", err)
	}

	// Store in Redis with TTL
	key := r.serviceKey(service.ID)
	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Add to name index
	nameKey := r.nameIndexKey(service.Name)
	if err := r.client.SAdd(ctx, nameKey, service.ID).Err(); err != nil {
		return fmt.Errorf("failed to add service to name index: %w", err)
	}
	// Set TTL on name index key
	if err := r.client.Expire(ctx, nameKey, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set TTL on name index: %w", err)
	}

	// Start heartbeat goroutine
	r.startHeartbeat(service.ID, service.Name)

	return nil
}

// Deregister removes a service instance from Redis and stops its heartbeat.
func (r *RedisRegistry) Deregister(ctx context.Context, serviceID string) error {
	// Stop heartbeat first
	r.stopHeartbeat(serviceID)

	// Get service info to know the name
	key := r.serviceKey(serviceID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Service not found, that's okay
			return nil
		}
		return fmt.Errorf("failed to get service info: %w", err)
	}

	var service ServiceInfo
	if err := json.Unmarshal(data, &service); err != nil {
		// Couldn't unmarshal, but still try to delete
		r.client.Del(ctx, key)
		return nil
	}

	// Remove from name index
	nameKey := r.nameIndexKey(service.Name)
	r.client.SRem(ctx, nameKey, serviceID)

	// Delete service key
	r.client.Del(ctx, key)

	return nil
}

// Discover returns all healthy instances of a service by name.
// Only returns services that haven't expired (TTL still valid).
func (r *RedisRegistry) Discover(ctx context.Context, serviceName string) ([]ServiceInfo, error) {
	nameKey := r.nameIndexKey(serviceName)

	// Get all service IDs for this name
	serviceIDs, err := r.client.SMembers(ctx, nameKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []ServiceInfo{}, nil
		}
		return nil, fmt.Errorf("failed to get service IDs: %w", err)
	}

	var services []ServiceInfo
	for _, id := range serviceIDs {
		key := r.serviceKey(id)
		data, err := r.client.Get(ctx, key).Bytes()
		if err != nil {
			if err == redis.Nil {
				// Service expired, remove from index
				r.client.SRem(ctx, nameKey, id)
				continue
			}
			// Log error but continue with other services
			continue
		}

		var service ServiceInfo
		if err := json.Unmarshal(data, &service); err != nil {
			// Corrupted data, skip
			continue
		}

		services = append(services, service)
	}

	// Sort by registration time
	sort.Slice(services, func(i, j int) bool {
		return services[i].RegisteredAt.Before(services[j].RegisteredAt)
	})

	return services, nil
}

// Close stops all heartbeats and closes the Redis connection.
func (r *RedisRegistry) Close() error {
	// Stop all heartbeats
	r.mu.Lock()
	for id, cancel := range r.heartbeats {
		cancel()
		delete(r.heartbeats, id)
	}
	r.mu.Unlock()

	// Close Redis connection
	return r.client.Close()
}

// startHeartbeat starts a goroutine that periodically refreshes the service TTL.
func (r *RedisRegistry) startHeartbeat(serviceID, serviceName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop existing heartbeat if any
	if cancel, exists := r.heartbeats[serviceID]; exists {
		cancel()
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	r.heartbeats[serviceID] = cancel

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Refresh TTL on service key
				key := r.serviceKey(serviceID)
				r.client.Expire(ctx, key, r.ttl)

				// Refresh TTL on name index
				nameKey := r.nameIndexKey(serviceName)
				r.client.Expire(ctx, nameKey, r.ttl)
			}
		}
	}()
}

// stopHeartbeat stops the heartbeat goroutine for a service.
func (r *RedisRegistry) stopHeartbeat(serviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cancel, exists := r.heartbeats[serviceID]; exists {
		cancel()
		delete(r.heartbeats, serviceID)
	}
}

// serviceKey returns the Redis key for a service instance.
func (r *RedisRegistry) serviceKey(serviceID string) string {
	return fmt.Sprintf("registry:service:%s", serviceID)
}

// nameIndexKey returns the Redis key for the service name index (set of IDs).
func (r *RedisRegistry) nameIndexKey(serviceName string) string {
	return fmt.Sprintf("registry:name:%s", serviceName)
}
