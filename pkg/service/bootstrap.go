package service

import (
	"context"
	"fmt"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"github.com/Combine-Capital/cqi/pkg/metrics"
	"github.com/Combine-Capital/cqi/pkg/tracing"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Bootstrap represents initialized infrastructure components.
// It provides convenient access to all observability components.
type Bootstrap struct {
	Config         *config.Config
	Logger         *logging.Logger
	TracerProvider *sdktrace.TracerProvider
	cleanup        []func(context.Context) error
}

// BootstrapOption is a functional option for configuring bootstrap behavior.
type BootstrapOption func(*bootstrapConfig)

type bootstrapConfig struct {
	skipMetrics bool
	skipTracing bool
	skipLogger  bool
}

// WithoutMetrics disables metrics initialization during bootstrap.
func WithoutMetrics() BootstrapOption {
	return func(c *bootstrapConfig) {
		c.skipMetrics = true
	}
}

// WithoutTracing disables tracing initialization during bootstrap.
func WithoutTracing() BootstrapOption {
	return func(c *bootstrapConfig) {
		c.skipTracing = true
	}
}

// WithoutLogger disables logger initialization during bootstrap.
// This is rarely needed but can be useful for testing.
func WithoutLogger() BootstrapOption {
	return func(c *bootstrapConfig) {
		c.skipLogger = true
	}
}

// NewBootstrap initializes all observability components from configuration.
// It creates logger, metrics, and tracing infrastructure in the correct order
// with appropriate error handling.
//
// Example:
//
//	cfg := config.MustLoad("config.yaml", "MYAPP")
//	bootstrap, err := service.NewBootstrap(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer bootstrap.Cleanup(ctx)
//
//	// Use bootstrap.Logger, bootstrap.Tracer, etc.
func NewBootstrap(ctx context.Context, cfg *config.Config, opts ...BootstrapOption) (*Bootstrap, error) {
	bc := &bootstrapConfig{}
	for _, opt := range opts {
		opt(bc)
	}

	b := &Bootstrap{
		Config:  cfg,
		cleanup: make([]func(context.Context) error, 0),
	}

	// Initialize logger
	if !bc.skipLogger {
		logger := logging.New(cfg.Log)
		b.Logger = logger
		logger.Info().
			Str("service", cfg.Service.Name).
			Str("version", cfg.Service.Version).
			Str("env", cfg.Service.Env).
			Msg("Service starting")
	}

	// Initialize metrics
	if !bc.skipMetrics && cfg.Metrics.Enabled {
		// Convert config.MetricsConfig to metrics.MetricsConfig
		metricsConfig := metrics.MetricsConfig{
			Enabled:   cfg.Metrics.Enabled,
			Port:      cfg.Metrics.Port,
			Path:      cfg.Metrics.Path,
			Namespace: cfg.Metrics.Namespace,
		}
		if err := metrics.Init(metricsConfig); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		b.cleanup = append(b.cleanup, metrics.Shutdown)

		if b.Logger != nil {
			b.Logger.Info().
				Int("port", cfg.Metrics.Port).
				Str("path", cfg.Metrics.Path).
				Msg("Metrics initialized")
		}
	}

	// Initialize tracing
	if !bc.skipTracing && cfg.Tracing.Enabled {
		serviceName := cfg.Service.Name
		if cfg.Tracing.ServiceName != "" {
			serviceName = cfg.Tracing.ServiceName
		}

		tracerProvider, shutdown, err := tracing.NewTracerProvider(ctx, cfg.Tracing, serviceName)
		if err != nil {
			// Cleanup already initialized components
			_ = b.Cleanup(ctx)
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
		b.TracerProvider = tracerProvider
		b.cleanup = append(b.cleanup, shutdown)

		if b.Logger != nil {
			b.Logger.Info().
				Str("endpoint", cfg.Tracing.Endpoint).
				Float64("sample_rate", cfg.Tracing.SampleRate).
				Msg("Tracing initialized")
		}
	}

	return b, nil
}

// Cleanup shuts down all initialized infrastructure components.
// It executes cleanup functions in reverse order (LIFO) to ensure
// proper dependency cleanup. Always defer this after creating Bootstrap.
func (b *Bootstrap) Cleanup(ctx context.Context) error {
	// Execute cleanup in reverse order
	for i := len(b.cleanup) - 1; i >= 0; i-- {
		if err := b.cleanup[i](ctx); err != nil {
			if b.Logger != nil {
				b.Logger.Error().Err(err).Msg("Cleanup error")
			}
			// Continue with other cleanup operations
		}
	}

	if b.Logger != nil {
		b.Logger.Info().Msg("Cleanup completed")
	}

	return nil
}

// AddCleanup adds a cleanup function to be executed during Cleanup.
// Cleanup functions are executed in reverse order (LIFO).
//
// Example:
//
//	bootstrap.AddCleanup(func(ctx context.Context) error {
//	    return db.Close()
//	})
func (b *Bootstrap) AddCleanup(fn func(context.Context) error) {
	b.cleanup = append(b.cleanup, fn)
}
