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
	Service  ServiceConfig  `mapstructure:"service"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Cache    CacheConfig    `mapstructure:"cache"`
	EventBus EventBusConfig `mapstructure:"eventbus"`
	Log      LogConfig      `mapstructure:"log"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Auth     AuthConfig     `mapstructure:"auth"`
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
