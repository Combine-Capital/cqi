package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Standard HTTP metrics
	httpRequestDuration *Histogram
	httpRequestCount    *Counter
	httpRequestSize     *Histogram
	httpResponseSize    *Histogram

	// Standard gRPC metrics
	grpcCallDuration *Histogram
	grpcCallCount    *Counter

	// Ensure standard metrics are initialized only once
	standardMetricsOnce sync.Once
)

// InitStandardMetrics initializes standard HTTP and gRPC metrics.
// This function is called automatically by the middleware, but can be called
// explicitly to ensure metrics are registered before use.
// It is safe to call multiple times - subsequent calls are no-ops.
func InitStandardMetrics(namespace string) error {
	var initErr error

	standardMetricsOnce.Do(func() {
		// Initialize HTTP metrics
		httpRequestDuration, initErr = NewHistogram(HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Labels:    []string{"method", "path", "status_code"},
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		})
		if initErr != nil {
			return
		}

		httpRequestCount, initErr = NewCounter(CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
			Labels:    []string{"method", "path", "status_code"},
		})
		if initErr != nil {
			return
		}

		httpRequestSize, initErr = NewHistogram(HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "HTTP request size in bytes",
			Labels:    []string{"method", "path"},
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~100MB
		})
		if initErr != nil {
			return
		}

		httpResponseSize, initErr = NewHistogram(HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Labels:    []string{"method", "path", "status_code"},
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to ~100MB
		})
		if initErr != nil {
			return
		}

		// Initialize gRPC metrics
		grpcCallDuration, initErr = NewHistogram(HistogramOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "call_duration_seconds",
			Help:      "gRPC call duration in seconds",
			Labels:    []string{"method", "status_code"},
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		})
		if initErr != nil {
			return
		}

		grpcCallCount, initErr = NewCounter(CounterOpts{
			Namespace: namespace,
			Subsystem: "grpc",
			Name:      "calls_total",
			Help:      "Total number of gRPC calls",
			Labels:    []string{"method", "status_code"},
		})
		if initErr != nil {
			return
		}
	})

	return initErr
}

// GetHTTPRequestDuration returns the standard HTTP request duration histogram.
// Returns nil if standard metrics have not been initialized.
func GetHTTPRequestDuration() *Histogram {
	return httpRequestDuration
}

// GetHTTPRequestCount returns the standard HTTP request count counter.
// Returns nil if standard metrics have not been initialized.
func GetHTTPRequestCount() *Counter {
	return httpRequestCount
}

// GetHTTPRequestSize returns the standard HTTP request size histogram.
// Returns nil if standard metrics have not been initialized.
func GetHTTPRequestSize() *Histogram {
	return httpRequestSize
}

// GetHTTPResponseSize returns the standard HTTP response size histogram.
// Returns nil if standard metrics have not been initialized.
func GetHTTPResponseSize() *Histogram {
	return httpResponseSize
}

// GetGRPCCallDuration returns the standard gRPC call duration histogram.
// Returns nil if standard metrics have not been initialized.
func GetGRPCCallDuration() *Histogram {
	return grpcCallDuration
}

// GetGRPCCallCount returns the standard gRPC call count counter.
// Returns nil if standard metrics have not been initialized.
func GetGRPCCallCount() *Counter {
	return grpcCallCount
}
