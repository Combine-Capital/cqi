package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resetMetrics resets the global metrics state for testing
func resetMetrics() {
	// First shutdown any running server
	serverMu.Lock()
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		server = nil
	}
	serverMu.Unlock()

	// Reset registry state
	registryMu.Lock()
	registry = nil
	initialized = false
	registryMu.Unlock()

	// Reset standard metrics - need to use reflection to reset sync.Once
	// For testing, we'll just set them to nil and create new ones
	httpRequestDuration = nil
	httpRequestCount = nil
	httpRequestSize = nil
	httpResponseSize = nil
	grpcCallDuration = nil
	grpcCallCount = nil

	// Reset the Once - this is a bit of a hack but necessary for testing
	standardMetricsOnce = sync.Once{}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MetricsConfig
		wantErr bool
	}{
		{
			name: "enabled with valid config",
			cfg: MetricsConfig{
				Enabled:   true,
				Port:      19090, // Use different port to avoid conflicts
				Path:      "/metrics",
				Namespace: "test",
			},
			wantErr: false,
		},
		{
			name: "disabled",
			cfg: MetricsConfig{
				Enabled:   false,
				Port:      19091,
				Path:      "/metrics",
				Namespace: "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetMetrics()
			defer func() {
				if server != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()
					_ = Shutdown(ctx)
				}
			}()

			err := Init(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if !IsInitialized() {
					t.Error("Init() succeeded but IsInitialized() = false")
				}
				if Registry() == nil {
					t.Error("Init() succeeded but Registry() = nil")
				}
			}

			// Give server time to start if enabled
			if tt.cfg.Enabled {
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}

func TestInitIdempotent(t *testing.T) {
	resetMetrics()
	defer func() {
		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			_ = Shutdown(ctx)
		}
	}()

	cfg := MetricsConfig{
		Enabled:   true,
		Port:      19092,
		Path:      "/metrics",
		Namespace: "test",
	}

	// Initialize multiple times
	err1 := Init(cfg)
	err2 := Init(cfg)
	err3 := Init(cfg)

	if err1 != nil {
		t.Errorf("First Init() error = %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Init() error = %v", err2)
	}
	if err3 != nil {
		t.Errorf("Third Init() error = %v", err3)
	}
}

func TestNewCounter(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19093, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	tests := []struct {
		name    string
		opts    CounterOpts
		wantErr bool
	}{
		{
			name: "valid counter with labels",
			opts: CounterOpts{
				Namespace: "test",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total requests",
				Labels:    []string{"method", "status"},
			},
			wantErr: false,
		},
		{
			name: "valid counter without subsystem",
			opts: CounterOpts{
				Namespace: "test",
				Name:      "events_total",
				Help:      "Total events",
				Labels:    []string{},
			},
			wantErr: false,
		},
		{
			name: "invalid metric name",
			opts: CounterOpts{
				Namespace: "test",
				Name:      "123-invalid",
				Help:      "Invalid name",
			},
			wantErr: true,
		},
		{
			name: "invalid label name",
			opts: CounterOpts{
				Namespace: "test",
				Name:      "valid_name",
				Help:      "Invalid label",
				Labels:    []string{"valid", "123-invalid"},
			},
			wantErr: true,
		},
		{
			name: "reserved label name",
			opts: CounterOpts{
				Namespace: "test",
				Name:      "valid_name",
				Help:      "Reserved label",
				Labels:    []string{"__reserved"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter, err := NewCounter(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCounter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && counter == nil {
				t.Error("NewCounter() returned nil counter")
			}

			// Test counter operations if successful
			if counter != nil && len(tt.opts.Labels) > 0 {
				// Create label values matching the number of labels
				labelValues := make([]string, len(tt.opts.Labels))
				for i := range labelValues {
					labelValues[i] = "value"
				}

				counter.Inc(labelValues...)
				counter.Add(5.0, labelValues...)
			}
		})
	}
}

func TestNewCounterDuplicate(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19094, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	opts := CounterOpts{
		Namespace: "test",
		Name:      "duplicate_counter",
		Help:      "Duplicate counter",
	}

	// First registration should succeed
	counter1, err := NewCounter(opts)
	if err != nil {
		t.Fatalf("First NewCounter() error = %v", err)
	}
	if counter1 == nil {
		t.Fatal("First NewCounter() returned nil")
	}

	// Second registration should fail
	counter2, err := NewCounter(opts)
	if err == nil {
		t.Error("Second NewCounter() should have failed but succeeded")
	}
	if counter2 != nil {
		t.Error("Second NewCounter() should return nil on error")
	}
}

func TestNewGauge(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19095, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	opts := GaugeOpts{
		Namespace: "test",
		Subsystem: "database",
		Name:      "connections_active",
		Help:      "Active database connections",
		Labels:    []string{"pool"},
	}

	gauge, err := NewGauge(opts)
	if err != nil {
		t.Fatalf("NewGauge() error = %v", err)
	}
	if gauge == nil {
		t.Fatal("NewGauge() returned nil")
	}

	// Test gauge operations
	gauge.Set(10, "primary")
	gauge.Inc("primary")
	gauge.Dec("primary")
	gauge.Add(5, "primary")
	gauge.Sub(3, "primary")
}

func TestNewHistogram(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19096, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	tests := []struct {
		name    string
		opts    HistogramOpts
		wantErr bool
	}{
		{
			name: "histogram with custom buckets",
			opts: HistogramOpts{
				Namespace: "test",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "Request duration",
				Labels:    []string{"method"},
				Buckets:   []float64{0.1, 0.5, 1.0, 5.0},
			},
			wantErr: false,
		},
		{
			name: "histogram with default buckets",
			opts: HistogramOpts{
				Namespace: "test",
				Name:      "duration_seconds",
				Help:      "Duration",
				Labels:    []string{},
				Buckets:   nil, // Use defaults
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hist, err := NewHistogram(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHistogram() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && hist == nil {
				t.Error("NewHistogram() returned nil")
			}

			// Test histogram operations if successful
			if hist != nil && len(tt.opts.Labels) > 0 {
				labelValues := make([]string, len(tt.opts.Labels))
				for i := range labelValues {
					labelValues[i] = "value"
				}
				hist.Observe(1.5, labelValues...)
			}
		})
	}
}

func TestInitStandardMetrics(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19097, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err := InitStandardMetrics("test")
	if err != nil {
		t.Fatalf("InitStandardMetrics() error = %v", err)
	}

	// Verify all standard metrics are initialized
	if GetHTTPRequestDuration() == nil {
		t.Error("HTTP request duration metric not initialized")
	}
	if GetHTTPRequestCount() == nil {
		t.Error("HTTP request count metric not initialized")
	}
	if GetHTTPRequestSize() == nil {
		t.Error("HTTP request size metric not initialized")
	}
	if GetHTTPResponseSize() == nil {
		t.Error("HTTP response size metric not initialized")
	}
	if GetGRPCCallDuration() == nil {
		t.Error("gRPC call duration metric not initialized")
	}
	if GetGRPCCallCount() == nil {
		t.Error("gRPC call count metric not initialized")
	}

	// Calling again should be idempotent
	err = InitStandardMetrics("test")
	if err != nil {
		t.Errorf("Second InitStandardMetrics() error = %v", err)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19098, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Wrap with metrics middleware
	middleware := HTTPMiddleware("test")
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if body != "Hello, World!" {
		t.Errorf("Expected body %q, got %q", "Hello, World!", body)
	}

	// Verify metrics were recorded (metrics should be initialized by middleware)
	if GetHTTPRequestDuration() == nil {
		t.Error("HTTP request duration metric not initialized by middleware")
	}
	if GetHTTPRequestCount() == nil {
		t.Error("HTTP request count metric not initialized by middleware")
	}
}

func TestHTTPMiddlewareError(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19099, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Create test handler that returns an error
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	// Wrap with metrics middleware
	middleware := HTTPMiddleware("test")
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest(http.MethodPost, "/error/path", strings.NewReader("request body"))
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19100, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	interceptor := UnaryServerInterceptor("test")

	tests := []struct {
		name       string
		handlerErr error
		wantStatus string
	}{
		{
			name:       "successful call",
			handlerErr: nil,
			wantStatus: "OK",
		},
		{
			name:       "call with error",
			handlerErr: status.Error(codes.NotFound, "not found"),
			wantStatus: "NOT_FOUND",
		},
		{
			name:       "call with context cancelled",
			handlerErr: context.Canceled,
			wantStatus: "CANCELLED",
		},
		{
			name:       "call with deadline exceeded",
			handlerErr: context.DeadlineExceeded,
			wantStatus: "DEADLINE_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return "response", tt.handlerErr
			}

			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/Method",
			}

			resp, err := interceptor(context.Background(), "request", info, handler)

			if err != tt.handlerErr {
				t.Errorf("Expected error %v, got %v", tt.handlerErr, err)
			}

			if tt.handlerErr == nil && resp != "response" {
				t.Errorf("Expected response %q, got %q", "response", resp)
			}
		})
	}
}

func TestStreamServerInterceptor(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19101, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	interceptor := StreamServerInterceptor("test")

	tests := []struct {
		name       string
		handlerErr error
	}{
		{
			name:       "successful stream",
			handlerErr: nil,
		},
		{
			name:       "stream with error",
			handlerErr: status.Error(codes.Internal, "internal error"),
		},
		{
			name:       "stream with EOF",
			handlerErr: io.EOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(srv interface{}, stream grpc.ServerStream) error {
				return tt.handlerErr
			}

			info := &grpc.StreamServerInfo{
				FullMethod: "/test.Service/StreamMethod",
			}

			err := interceptor(nil, nil, info, handler)

			if err != tt.handlerErr {
				t.Errorf("Expected error %v, got %v", tt.handlerErr, err)
			}
		})
	}
}

func TestComputeRequestSize(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		url      string
		headers  map[string][]string
		bodySize int64
		minSize  int64 // Minimum expected size
	}{
		{
			name:     "simple GET request",
			method:   http.MethodGet,
			url:      "/test",
			headers:  map[string][]string{"User-Agent": {"test"}},
			bodySize: 0,
			minSize:  10, // At least method + url
		},
		{
			name:     "POST request with body",
			method:   http.MethodPost,
			url:      "/api/data",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			bodySize: 1024,
			minSize:  1024, // At least body size
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			for k, v := range tt.headers {
				for _, val := range v {
					req.Header.Add(k, val)
				}
			}
			req.ContentLength = tt.bodySize

			size := computeRequestSize(req)
			if size < tt.minSize {
				t.Errorf("computeRequestSize() = %d, want at least %d", size, tt.minSize)
			}
		})
	}
}

func TestValidateMetricOpts(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		subsystem  string
		metricName string
		labels     []string
		wantErr    bool
	}{
		{
			name:       "valid metric",
			namespace:  "test",
			subsystem:  "http",
			metricName: "requests_total",
			labels:     []string{"method", "status"},
			wantErr:    false,
		},
		{
			name:       "valid metric without subsystem",
			namespace:  "test",
			subsystem:  "",
			metricName: "events",
			labels:     []string{},
			wantErr:    false,
		},
		{
			name:       "invalid metric name starting with number",
			namespace:  "",
			subsystem:  "",
			metricName: "123invalid",
			labels:     []string{},
			wantErr:    true,
		},
		{
			name:       "invalid label name",
			namespace:  "test",
			subsystem:  "",
			metricName: "valid",
			labels:     []string{"valid", "123invalid"},
			wantErr:    true,
		},
		{
			name:       "reserved label name",
			namespace:  "test",
			subsystem:  "",
			metricName: "valid",
			labels:     []string{"__reserved"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetricOpts(tt.namespace, tt.subsystem, tt.metricName, tt.labels)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMetricOpts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultMetricsConfig(t *testing.T) {
	cfg := DefaultMetricsConfig()

	if !cfg.Enabled {
		t.Error("DefaultMetricsConfig() Enabled = false, want true")
	}
	if cfg.Port != 9090 {
		t.Errorf("DefaultMetricsConfig() Port = %d, want 9090", cfg.Port)
	}
	if cfg.Path != "/metrics" {
		t.Errorf("DefaultMetricsConfig() Path = %q, want %q", cfg.Path, "/metrics")
	}
	if cfg.Namespace != "cqi" {
		t.Errorf("DefaultMetricsConfig() Namespace = %q, want %q", cfg.Namespace, "cqi")
	}
}

func TestWithLabelValues(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: false, Port: 19103, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Test Counter.WithLabelValues
	counter, err := NewCounter(CounterOpts{
		Namespace: "test",
		Name:      "counter_with_labels",
		Help:      "Test counter",
		Labels:    []string{"label1"},
	})
	if err != nil {
		t.Fatalf("NewCounter() error = %v", err)
	}

	c := counter.WithLabelValues("value1")
	c.Inc()
	c.Add(5)

	// Test Gauge.WithLabelValues
	gauge, err := NewGauge(GaugeOpts{
		Namespace: "test",
		Name:      "gauge_with_labels",
		Help:      "Test gauge",
		Labels:    []string{"label1"},
	})
	if err != nil {
		t.Fatalf("NewGauge() error = %v", err)
	}

	g := gauge.WithLabelValues("value1")
	g.Set(10)
	g.Inc()
	g.Dec()

	// Test Histogram.WithLabelValues
	hist, err := NewHistogram(HistogramOpts{
		Namespace: "test",
		Name:      "histogram_with_labels",
		Help:      "Test histogram",
		Labels:    []string{"label1"},
	})
	if err != nil {
		t.Fatalf("NewHistogram() error = %v", err)
	}

	h := hist.WithLabelValues("value1")
	h.Observe(1.5)
	h.Observe(2.5)
}

func TestMetricsBeforeInit(t *testing.T) {
	resetMetrics()

	// Try to create metrics before Init
	_, err := NewCounter(CounterOpts{
		Namespace: "test",
		Name:      "counter",
		Help:      "Test",
	})
	if err == nil {
		t.Error("NewCounter() before Init() should return error")
	}

	_, err = NewGauge(GaugeOpts{
		Namespace: "test",
		Name:      "gauge",
		Help:      "Test",
	})
	if err == nil {
		t.Error("NewGauge() before Init() should return error")
	}

	_, err = NewHistogram(HistogramOpts{
		Namespace: "test",
		Name:      "histogram",
		Help:      "Test",
	})
	if err == nil {
		t.Error("NewHistogram() before Init() should return error")
	}
}

func TestShutdown(t *testing.T) {
	resetMetrics()
	cfg := MetricsConfig{Enabled: true, Port: 19102, Path: "/metrics", Namespace: "test"}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Calling shutdown again should be safe
	err = Shutdown(ctx)
	if err != nil {
		t.Errorf("Second Shutdown() error = %v", err)
	}
}

func TestGRPCStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: "OK",
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			wantCode: "CANCELLED",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			wantCode: "DEADLINE_EXCEEDED",
		},
		{
			name:     "EOF",
			err:      io.EOF,
			wantCode: "UNAVAILABLE",
		},
		{
			name:     "gRPC status error",
			err:      status.Error(codes.NotFound, "not found"),
			wantCode: "NotFound",
		},
		{
			name:     "non-gRPC error",
			err:      fmt.Errorf("generic error"),
			wantCode: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := grpcStatusCode(tt.err)
			if code != tt.wantCode {
				t.Errorf("grpcStatusCode() = %q, want %q", code, tt.wantCode)
			}
		})
	}
}
