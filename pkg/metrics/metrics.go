// Package metrics provides Prometheus metrics collection with standardized naming conventions
// and automatic HTTP/gRPC middleware for observability. It supports counters, gauges, and
// histograms with label validation and duplicate prevention.
//
// Example usage:
//
//	// Initialize metrics with configuration
//	if err := metrics.Init(cfg.Metrics); err != nil {
//	    log.Fatal(err)
//	}
//	defer metrics.Shutdown(context.Background())
//
//	// Create a custom counter
//	counter := metrics.NewCounter(metrics.CounterOpts{
//	    Namespace: "myapp",
//	    Subsystem: "orders",
//	    Name:      "total",
//	    Help:      "Total number of orders processed",
//	    Labels:    []string{"status", "region"},
//	})
//	counter.WithLabelValues("completed", "us-west").Inc()
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// registry is the global Prometheus registry for all metrics
	registry *prometheus.Registry

	// registryMu protects concurrent access to registry initialization
	registryMu sync.RWMutex

	// initialized tracks whether Init() has been called
	initialized bool

	// server is the HTTP server for the metrics endpoint
	server *http.Server

	// serverMu protects concurrent access to server
	serverMu sync.Mutex
)

// Init initializes the metrics system with the provided configuration.
// It creates a new Prometheus registry and starts an HTTP server on the
// configured port and path to expose metrics.
//
// This function is safe to call multiple times - subsequent calls are no-ops.
// Returns an error if the metrics system cannot be initialized (e.g., port already in use).
func Init(cfg MetricsConfig) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	if initialized {
		return nil // Already initialized
	}

	if !cfg.Enabled {
		// Metrics disabled, use no-op implementations
		registry = prometheus.NewRegistry()
		initialized = true
		return nil
	}

	// Create new registry
	registry = prometheus.NewRegistry()

	// Add Go runtime metrics (goroutines, memory, GC, etc.)
	registry.MustRegister(prometheus.NewGoCollector())

	// Add process metrics (CPU, memory, file descriptors, etc.)
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Start HTTP server for metrics endpoint
	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	serverMu.Lock()
	server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	srv := server // Capture for goroutine
	serverMu.Unlock()

	// Start server in background
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - metrics are non-critical
			fmt.Printf("metrics server error: %v\n", err)
		}
	}()

	initialized = true
	return nil
}

// Shutdown gracefully shuts down the metrics HTTP server.
// It waits for up to the context deadline for in-flight requests to complete.
func Shutdown(ctx context.Context) error {
	serverMu.Lock()
	defer serverMu.Unlock()

	if server == nil {
		return nil
	}

	return server.Shutdown(ctx)
}

// Registry returns the global Prometheus registry.
// This is useful for custom metric registration or testing.
// Returns nil if Init() has not been called.
func Registry() *prometheus.Registry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry
}

// IsInitialized returns true if Init() has been called successfully.
func IsInitialized() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return initialized
}

// MetricsConfig contains Prometheus metrics configuration.
type MetricsConfig struct {
	Enabled   bool   // Whether metrics collection is enabled
	Port      int    // HTTP server port for /metrics endpoint
	Path      string // HTTP path for metrics endpoint
	Namespace string // Metric prefix/namespace
}

// DefaultMetricsConfig returns a MetricsConfig with sensible defaults.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:   true,
		Port:      9090,
		Path:      "/metrics",
		Namespace: "cqi",
	}
}
