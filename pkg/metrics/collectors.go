package metrics

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// validMetricName validates metric names according to Prometheus conventions
	validMetricName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

	// validLabelName validates label names according to Prometheus conventions
	validLabelName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// Counter is a Prometheus counter that can only increase.
type Counter struct {
	vec *prometheus.CounterVec
}

// CounterOpts specifies options for creating a counter.
type CounterOpts struct {
	Namespace string   // Metric namespace (e.g., "cqi", "myapp")
	Subsystem string   // Metric subsystem (e.g., "http", "database")
	Name      string   // Metric name (e.g., "requests_total")
	Help      string   // Human-readable help text
	Labels    []string // Label names for this metric
}

// NewCounter creates and registers a new counter with the global registry.
// The full metric name will be "{namespace}_{subsystem}_{name}".
// Returns an error if the metric name or labels are invalid, or if a metric
// with the same name is already registered.
func NewCounter(opts CounterOpts) (*Counter, error) {
	if !IsInitialized() {
		return nil, fmt.Errorf("metrics not initialized, call Init() first")
	}

	// Validate inputs
	if err := validateMetricOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.Labels); err != nil {
		return nil, err
	}

	// Create counter vector
	vec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: opts.Namespace,
			Subsystem: opts.Subsystem,
			Name:      opts.Name,
			Help:      opts.Help,
		},
		opts.Labels,
	)

	// Register with global registry
	if err := registry.Register(vec); err != nil {
		return nil, fmt.Errorf("failed to register counter: %w", err)
	}

	return &Counter{vec: vec}, nil
}

// Inc increments the counter by 1 for the given label values.
func (c *Counter) Inc(labelValues ...string) {
	c.vec.WithLabelValues(labelValues...).Inc()
}

// Add increments the counter by the given value for the given label values.
// The value must be non-negative.
func (c *Counter) Add(value float64, labelValues ...string) {
	c.vec.WithLabelValues(labelValues...).Add(value)
}

// WithLabelValues returns a counter for the given label values.
// This allows more efficient access when incrementing the same labels repeatedly.
func (c *Counter) WithLabelValues(labelValues ...string) prometheus.Counter {
	return c.vec.WithLabelValues(labelValues...)
}

// Gauge is a Prometheus gauge that can increase or decrease.
type Gauge struct {
	vec *prometheus.GaugeVec
}

// GaugeOpts specifies options for creating a gauge.
type GaugeOpts struct {
	Namespace string   // Metric namespace (e.g., "cqi", "myapp")
	Subsystem string   // Metric subsystem (e.g., "http", "database")
	Name      string   // Metric name (e.g., "connections_active")
	Help      string   // Human-readable help text
	Labels    []string // Label names for this metric
}

// NewGauge creates and registers a new gauge with the global registry.
// The full metric name will be "{namespace}_{subsystem}_{name}".
// Returns an error if the metric name or labels are invalid, or if a metric
// with the same name is already registered.
func NewGauge(opts GaugeOpts) (*Gauge, error) {
	if !IsInitialized() {
		return nil, fmt.Errorf("metrics not initialized, call Init() first")
	}

	// Validate inputs
	if err := validateMetricOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.Labels); err != nil {
		return nil, err
	}

	// Create gauge vector
	vec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: opts.Namespace,
			Subsystem: opts.Subsystem,
			Name:      opts.Name,
			Help:      opts.Help,
		},
		opts.Labels,
	)

	// Register with global registry
	if err := registry.Register(vec); err != nil {
		return nil, fmt.Errorf("failed to register gauge: %w", err)
	}

	return &Gauge{vec: vec}, nil
}

// Set sets the gauge to the given value for the given label values.
func (g *Gauge) Set(value float64, labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Set(value)
}

// Inc increments the gauge by 1 for the given label values.
func (g *Gauge) Inc(labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Inc()
}

// Dec decrements the gauge by 1 for the given label values.
func (g *Gauge) Dec(labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Dec()
}

// Add adds the given value to the gauge for the given label values.
func (g *Gauge) Add(value float64, labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Add(value)
}

// Sub subtracts the given value from the gauge for the given label values.
func (g *Gauge) Sub(value float64, labelValues ...string) {
	g.vec.WithLabelValues(labelValues...).Sub(value)
}

// WithLabelValues returns a gauge for the given label values.
// This allows more efficient access when updating the same labels repeatedly.
func (g *Gauge) WithLabelValues(labelValues ...string) prometheus.Gauge {
	return g.vec.WithLabelValues(labelValues...)
}

// Histogram is a Prometheus histogram that samples observations.
type Histogram struct {
	vec *prometheus.HistogramVec
}

// HistogramOpts specifies options for creating a histogram.
type HistogramOpts struct {
	Namespace string    // Metric namespace (e.g., "cqi", "myapp")
	Subsystem string    // Metric subsystem (e.g., "http", "database")
	Name      string    // Metric name (e.g., "request_duration_seconds")
	Help      string    // Human-readable help text
	Labels    []string  // Label names for this metric
	Buckets   []float64 // Histogram buckets (use nil for default)
}

// NewHistogram creates and registers a new histogram with the global registry.
// The full metric name will be "{namespace}_{subsystem}_{name}".
// Returns an error if the metric name or labels are invalid, or if a metric
// with the same name is already registered.
func NewHistogram(opts HistogramOpts) (*Histogram, error) {
	if !IsInitialized() {
		return nil, fmt.Errorf("metrics not initialized, call Init() first")
	}

	// Validate inputs
	if err := validateMetricOpts(opts.Namespace, opts.Subsystem, opts.Name, opts.Labels); err != nil {
		return nil, err
	}

	// Use default buckets if none provided
	buckets := opts.Buckets
	if buckets == nil {
		buckets = prometheus.DefBuckets
	}

	// Create histogram vector
	vec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: opts.Namespace,
			Subsystem: opts.Subsystem,
			Name:      opts.Name,
			Help:      opts.Help,
			Buckets:   buckets,
		},
		opts.Labels,
	)

	// Register with global registry
	if err := registry.Register(vec); err != nil {
		return nil, fmt.Errorf("failed to register histogram: %w", err)
	}

	return &Histogram{vec: vec}, nil
}

// Observe adds an observation to the histogram for the given label values.
func (h *Histogram) Observe(value float64, labelValues ...string) {
	h.vec.WithLabelValues(labelValues...).Observe(value)
}

// WithLabelValues returns a histogram observer for the given label values.
// This allows more efficient access when observing the same labels repeatedly.
func (h *Histogram) WithLabelValues(labelValues ...string) prometheus.Observer {
	return h.vec.WithLabelValues(labelValues...)
}

// validateMetricOpts validates metric options according to Prometheus naming conventions.
func validateMetricOpts(namespace, subsystem, name string, labels []string) error {
	// Construct full metric name
	var fullName strings.Builder
	if namespace != "" {
		fullName.WriteString(namespace)
		fullName.WriteString("_")
	}
	if subsystem != "" {
		fullName.WriteString(subsystem)
		fullName.WriteString("_")
	}
	fullName.WriteString(name)

	// Validate metric name
	if !validMetricName.MatchString(fullName.String()) {
		return fmt.Errorf("invalid metric name: %s (must match %s)", fullName.String(), validMetricName.String())
	}

	// Validate label names
	for _, label := range labels {
		if !validLabelName.MatchString(label) {
			return fmt.Errorf("invalid label name: %s (must match %s)", label, validLabelName.String())
		}
		// Reserved label names
		if strings.HasPrefix(label, "__") {
			return fmt.Errorf("label name %s is reserved (starts with __)", label)
		}
	}

	return nil
}
