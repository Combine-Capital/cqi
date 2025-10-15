// Package config provides configuration management for CQI infrastructure components.
// It supports loading configuration from YAML files, JSON files, and environment variables
// with automatic validation and default value application.
//
// Example usage:
//
//	cfg, err := config.Load("config.yaml", "CQI")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Or panic on error:
//	cfg := config.MustLoad("config.yaml", "CQI")
package config

import (
	"time"
)

// Config represents the complete configuration for a CQI-based service.
type Config struct {
	Service    ServiceConfig    `mapstructure:"service"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Cache      CacheConfig      `mapstructure:"cache"`
	EventBus   EventBusConfig   `mapstructure:"eventbus"`
	Log        LogConfig        `mapstructure:"log"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Tracing    TracingConfig    `mapstructure:"tracing"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Registry   RegistryConfig   `mapstructure:"registry"`
	Runner     RunnerConfig     `mapstructure:"runner"`
	HTTPClient HTTPClientConfig `mapstructure:"http_client"`
}

// ServiceConfig contains general service information.
type ServiceConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"` // development, staging, production
}

// ServerConfig contains HTTP/gRPC server configuration.
type ServerConfig struct {
	HTTPPort         int           `mapstructure:"http_port"`
	GRPCPort         int           `mapstructure:"grpc_port"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout  time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes   int           `mapstructure:"max_header_bytes"`
	EnableReflection bool          `mapstructure:"enable_reflection"` // gRPC reflection
}

// DatabaseConfig contains PostgreSQL connection configuration.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"` // disable, require, verify-ca, verify-full
	MaxConns        int           `mapstructure:"max_conns"`
	MinConns        int           `mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
}

// CacheConfig contains Redis cache configuration.
type CacheConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DefaultTTL   time.Duration `mapstructure:"default_ttl"`
}

// EventBusConfig contains event bus (NATS JetStream) configuration.
type EventBusConfig struct {
	Backend       string        `mapstructure:"backend"`         // "jetstream" or "memory"
	Servers       []string      `mapstructure:"servers"`         // NATS server URLs
	StreamName    string        `mapstructure:"stream_name"`     // JetStream stream name
	ConsumerName  string        `mapstructure:"consumer_name"`   // Durable consumer name
	MaxDeliver    int           `mapstructure:"max_deliver"`     // Max delivery attempts
	AckWait       time.Duration `mapstructure:"ack_wait"`        // Acknowledgment timeout
	MaxAckPending int           `mapstructure:"max_ack_pending"` // Max outstanding unacked messages
}

// LogConfig contains structured logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, console
	Output string `mapstructure:"output"` // stdout, stderr, file path
}

// MetricsConfig contains Prometheus metrics configuration.
type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Port      int    `mapstructure:"port"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"` // Metric prefix
}

// TracingConfig contains OpenTelemetry tracing configuration.
type TracingConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Endpoint     string        `mapstructure:"endpoint"`      // OTLP endpoint (e.g., "localhost:4317")
	SampleRate   float64       `mapstructure:"sample_rate"`   // 0.0 to 1.0
	ServiceName  string        `mapstructure:"service_name"`  // Override service name for traces
	Environment  string        `mapstructure:"environment"`   // Environment tag
	ExportMode   string        `mapstructure:"export_mode"`   // "grpc" or "http"
	Insecure     bool          `mapstructure:"insecure"`      // Use insecure connection
	BatchTimeout time.Duration `mapstructure:"batch_timeout"` // Batch export timeout
}

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	// APIKeys is a list of valid API keys for API key authentication.
	// Each key should be a secure random string.
	APIKeys []string `mapstructure:"api_keys"`

	// JWTPublicKeyPath is the path to the RSA public key file (PEM format)
	// used to verify JWT signatures. If empty, JWT authentication is disabled.
	JWTPublicKeyPath string `mapstructure:"jwt_public_key_path"`

	// JWTIssuer is the expected value of the "iss" (issuer) claim in JWT tokens.
	// If empty, issuer validation is skipped.
	JWTIssuer string `mapstructure:"jwt_issuer"`

	// JWTAudience is the expected value of the "aud" (audience) claim in JWT tokens.
	// If empty, audience validation is skipped.
	JWTAudience string `mapstructure:"jwt_audience"`
}

// RegistryConfig contains service registry configuration.
type RegistryConfig struct {
	// Backend is the registry backend type: "local" or "redis".
	// "local" uses in-memory storage (development/testing).
	// "redis" uses Redis for distributed service discovery (production).
	Backend string `mapstructure:"backend"`

	// RedisAddr is the Redis server address (host:port) when using Redis backend.
	RedisAddr string `mapstructure:"redis_addr"`

	// RedisPassword is the Redis password (optional).
	RedisPassword string `mapstructure:"redis_password"`

	// RedisDB is the Redis database number (default: 0).
	RedisDB int `mapstructure:"redis_db"`

	// TTL is the time-to-live for service registrations in Redis.
	// Services must send heartbeats more frequently than this to stay registered.
	// Default: 30 seconds.
	TTL time.Duration `mapstructure:"ttl"`

	// HeartbeatInterval is how often to send heartbeat updates in Redis.
	// Should be less than TTL to prevent expiration.
	// Default: 10 seconds.
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
}

// RunnerConfig contains service orchestration runner configuration.
type RunnerConfig struct {
	// RestartPolicy is the default restart policy for services: "never", "always", or "on-failure".
	// Default: "on-failure".
	RestartPolicy string `mapstructure:"restart_policy"`

	// MaxRetries is the maximum number of restart attempts (0 = unlimited).
	// Default: 5.
	MaxRetries int `mapstructure:"max_retries"`

	// InitialBackoff is the initial delay before first restart.
	// Default: 1 second.
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`

	// MaxBackoff is the maximum delay between restarts.
	// Default: 60 seconds.
	MaxBackoff time.Duration `mapstructure:"max_backoff"`

	// BackoffMultiplier is the factor by which the backoff increases on each retry.
	// Default: 2.0.
	BackoffMultiplier float64 `mapstructure:"backoff_multiplier"`

	// EnableJitter adds randomness to backoff (±25%) to prevent thundering herd.
	// Default: true.
	EnableJitter bool `mapstructure:"enable_jitter"`
}

// HTTPClientConfig contains HTTP/REST client configuration.
type HTTPClientConfig struct {
	// BaseURL is the base URL for all requests (e.g., "https://api.example.com").
	// Individual requests can override this with absolute URLs.
	BaseURL string `mapstructure:"base_url"`

	// Timeout is the maximum duration for the entire request including retries.
	// Default: 30 seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// RetryCount is the maximum number of retry attempts.
	// Default: 3.
	RetryCount int `mapstructure:"retry_count"`

	// RetryWaitTime is the initial wait time between retries.
	// Default: 1 second.
	RetryWaitTime time.Duration `mapstructure:"retry_wait_time"`

	// RetryMaxWaitTime is the maximum wait time between retries.
	// Default: 10 seconds.
	RetryMaxWaitTime time.Duration `mapstructure:"retry_max_wait_time"`

	// RateLimitPerSecond is the maximum requests per second (0 = unlimited).
	// Default: 0 (disabled).
	RateLimitPerSecond float64 `mapstructure:"rate_limit_per_second"`

	// RateLimitBurst is the maximum burst size for rate limiting.
	// Default: 1.
	RateLimitBurst int `mapstructure:"rate_limit_burst"`

	// CircuitBreakerEnabled enables circuit breaker pattern.
	// Default: false.
	CircuitBreakerEnabled bool `mapstructure:"circuit_breaker_enabled"`

	// CircuitBreakerTimeout is the timeout before moving to half-open state.
	// Default: 60 seconds.
	CircuitBreakerTimeout time.Duration `mapstructure:"circuit_breaker_timeout"`

	// CircuitBreakerFailureThreshold is the number of failures before opening circuit.
	// Default: 5.
	CircuitBreakerFailureThreshold int `mapstructure:"circuit_breaker_failure_threshold"`

	// CircuitBreakerSuccessThreshold is the number of successes to close circuit.
	// Default: 2.
	CircuitBreakerSuccessThreshold int `mapstructure:"circuit_breaker_success_threshold"`

	// MaxIdleConns is the maximum number of idle connections across all hosts.
	// Default: 100.
	MaxIdleConns int `mapstructure:"max_idle_conns"`

	// MaxIdleConnsPerHost is the maximum idle connections per host.
	// Default: 10.
	MaxIdleConnsPerHost int `mapstructure:"max_idle_conns_per_host"`

	// MaxConnsPerHost is the maximum total connections per host (0 = unlimited).
	// Default: 0.
	MaxConnsPerHost int `mapstructure:"max_conns_per_host"`

	// IdleConnTimeout is the maximum time an idle connection stays open.
	// Default: 90 seconds.
	IdleConnTimeout time.Duration `mapstructure:"idle_conn_timeout"`

	// TLSHandshakeTimeout is the maximum time for TLS handshake.
	// Default: 10 seconds.
	TLSHandshakeTimeout time.Duration `mapstructure:"tls_handshake_timeout"`

	// ExpectContinueTimeout is the time to wait for a 100-continue response.
	// Default: 1 second.
	ExpectContinueTimeout time.Duration `mapstructure:"expect_continue_timeout"`
}
