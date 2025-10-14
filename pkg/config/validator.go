package config

import (
	"fmt"
	"time"
)

// Validate validates the configuration and returns an error if any required fields are missing
// or have invalid values.
func Validate(cfg *Config) error {
	// Validate Server config - at least one port should be configured
	if cfg.Server.HTTPPort == 0 && cfg.Server.GRPCPort == 0 {
		return fmt.Errorf("server.http_port or server.grpc_port is required")
	}

	// Validate Database config (if used)
	if cfg.Database.Host != "" {
		if cfg.Database.Port == 0 {
			return fmt.Errorf("database.port is required when database.host is set")
		}
		if cfg.Database.User == "" {
			return fmt.Errorf("database.user is required when database.host is set")
		}
		if cfg.Database.Database == "" {
			return fmt.Errorf("database.database is required when database.host is set")
		}
	}

	// Validate Cache config (if used)
	if cfg.Cache.Host != "" {
		if cfg.Cache.Port == 0 {
			return fmt.Errorf("cache.port is required when cache.host is set")
		}
	}

	// Validate EventBus config (if used with JetStream)
	if cfg.EventBus.Backend == "jetstream" && len(cfg.EventBus.Servers) > 0 {
		if cfg.EventBus.StreamName == "" {
			return fmt.Errorf("eventbus.stream_name is required when servers are configured")
		}
		if cfg.EventBus.ConsumerName == "" {
			return fmt.Errorf("eventbus.consumer_name is required when servers are configured")
		}
	}

	// Validate Tracing config (if enabled)
	if cfg.Tracing.Enabled {
		if cfg.Tracing.Endpoint == "" {
			return fmt.Errorf("tracing.endpoint is required when tracing is enabled")
		}
		if cfg.Tracing.SampleRate < 0 || cfg.Tracing.SampleRate > 1 {
			return fmt.Errorf("tracing.sample_rate must be between 0.0 and 1.0")
		}
	}

	// Validate Metrics config (if enabled)
	if cfg.Metrics.Enabled {
		if cfg.Metrics.Port == 0 {
			return fmt.Errorf("metrics.port is required when metrics are enabled")
		}
	}

	return nil
}

// applyDefaults applies default values to the configuration where values are not set.
func applyDefaults(cfg *Config) {
	// Service defaults
	if cfg.Service.Env == "" {
		cfg.Service.Env = "development"
	}

	// Server defaults
	if cfg.Server.HTTPPort == 0 && cfg.Server.GRPCPort == 0 {
		cfg.Server.HTTPPort = 8080 // Default to HTTP if no ports specified
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = 30 * time.Second
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20 // 1 MB
	}

	// Database defaults
	if cfg.Database.Port == 0 && cfg.Database.Host != "" {
		cfg.Database.Port = 5432
	}
	if cfg.Database.MaxConns == 0 {
		cfg.Database.MaxConns = 25
	}
	if cfg.Database.MinConns == 0 {
		cfg.Database.MinConns = 2
	}
	if cfg.Database.MaxConnLifetime == 0 {
		cfg.Database.MaxConnLifetime = time.Hour
	}
	if cfg.Database.MaxConnIdleTime == 0 {
		cfg.Database.MaxConnIdleTime = 10 * time.Minute
	}
	if cfg.Database.ConnectTimeout == 0 {
		cfg.Database.ConnectTimeout = 30 * time.Second
	}
	if cfg.Database.QueryTimeout == 0 {
		cfg.Database.QueryTimeout = 30 * time.Second
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "prefer"
	}

	// Cache defaults
	if cfg.Cache.Port == 0 && cfg.Cache.Host != "" {
		cfg.Cache.Port = 6379
	}
	if cfg.Cache.MaxRetries == 0 {
		cfg.Cache.MaxRetries = 3
	}
	if cfg.Cache.DialTimeout == 0 {
		cfg.Cache.DialTimeout = 5 * time.Second
	}
	if cfg.Cache.ReadTimeout == 0 {
		cfg.Cache.ReadTimeout = 3 * time.Second
	}
	if cfg.Cache.WriteTimeout == 0 {
		cfg.Cache.WriteTimeout = 3 * time.Second
	}
	if cfg.Cache.PoolSize == 0 {
		cfg.Cache.PoolSize = 10
	}
	if cfg.Cache.MinIdleConns == 0 {
		cfg.Cache.MinIdleConns = 2
	}
	if cfg.Cache.DefaultTTL == 0 {
		cfg.Cache.DefaultTTL = 5 * time.Minute
	}

	// EventBus defaults
	if cfg.EventBus.Backend == "" && len(cfg.EventBus.Servers) > 0 {
		cfg.EventBus.Backend = "jetstream"
	}
	if cfg.EventBus.Backend == "" {
		cfg.EventBus.Backend = "memory" // Default to memory for testing
	}
	if cfg.EventBus.StreamName == "" && len(cfg.EventBus.Servers) > 0 {
		cfg.EventBus.StreamName = "cqc_events"
	}
	if cfg.EventBus.ConsumerName == "" && len(cfg.EventBus.Servers) > 0 {
		cfg.EventBus.ConsumerName = "default"
	}
	if cfg.EventBus.MaxDeliver == 0 {
		cfg.EventBus.MaxDeliver = 3
	}
	if cfg.EventBus.AckWait == 0 {
		cfg.EventBus.AckWait = 30 * time.Second
	}
	if cfg.EventBus.MaxAckPending == 0 {
		cfg.EventBus.MaxAckPending = 1000
	}

	// Log defaults
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = "stdout"
	}

	// Metrics defaults
	if cfg.Metrics.Port == 0 && cfg.Metrics.Enabled {
		cfg.Metrics.Port = 9090
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Metrics.Namespace == "" && cfg.Service.Name != "" {
		cfg.Metrics.Namespace = cfg.Service.Name
	}

	// Tracing defaults
	if cfg.Tracing.SampleRate == 0 && cfg.Tracing.Enabled {
		cfg.Tracing.SampleRate = 0.1 // 10% sampling by default
	}
	if cfg.Tracing.ServiceName == "" {
		if cfg.Service.Name != "" {
			cfg.Tracing.ServiceName = cfg.Service.Name
		} else {
			cfg.Tracing.ServiceName = "cqi-service"
		}
	}
	if cfg.Tracing.Environment == "" {
		cfg.Tracing.Environment = cfg.Service.Env
	}
	if cfg.Tracing.ExportMode == "" {
		cfg.Tracing.ExportMode = "grpc"
	}
	if cfg.Tracing.BatchTimeout == 0 {
		cfg.Tracing.BatchTimeout = 5 * time.Second
	}
}
