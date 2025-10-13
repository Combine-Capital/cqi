package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// HTTPMiddleware is an HTTP middleware that automatically records standard metrics
// for HTTP requests including duration, count, request size, and response size.
// It initializes standard metrics on first use with the provided namespace.
func HTTPMiddleware(namespace string) func(http.Handler) http.Handler {
	// Initialize standard metrics on first middleware creation
	if err := InitStandardMetrics(namespace); err != nil {
		// Log error but continue - metrics are non-critical
		fmt.Printf("failed to initialize standard metrics: %v\n", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Record request size
			requestSize := computeRequestSize(r)
			if httpRequestSize != nil {
				httpRequestSize.Observe(float64(requestSize), r.Method, r.URL.Path)
			}

			// Wrap response writer to capture status code and response size
			wrapped := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			statusCode := strconv.Itoa(wrapped.statusCode)

			if httpRequestDuration != nil {
				httpRequestDuration.Observe(duration, r.Method, r.URL.Path, statusCode)
			}

			if httpRequestCount != nil {
				httpRequestCount.Inc(r.Method, r.URL.Path, statusCode)
			}

			if httpResponseSize != nil {
				httpResponseSize.Observe(float64(wrapped.bytesWritten), r.Method, r.URL.Path, statusCode)
			}
		})
	}
}

// metricsResponseWriter wraps http.ResponseWriter to capture status code and response size.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	written      bool
}

func (m *metricsResponseWriter) WriteHeader(code int) {
	if !m.written {
		m.statusCode = code
		m.written = true
		m.ResponseWriter.WriteHeader(code)
	}
}

func (m *metricsResponseWriter) Write(b []byte) (int, error) {
	if !m.written {
		m.WriteHeader(http.StatusOK)
	}
	n, err := m.ResponseWriter.Write(b)
	m.bytesWritten += n
	return n, err
}

// computeRequestSize estimates the size of an HTTP request in bytes.
func computeRequestSize(r *http.Request) int64 {
	size := int64(0)

	// Request line (method + path + protocol)
	size += int64(len(r.Method))
	size += int64(len(r.URL.String()))
	size += int64(len(r.Proto))

	// Headers
	for name, values := range r.Header {
		size += int64(len(name))
		for _, value := range values {
			size += int64(len(value))
		}
	}

	// Body (if present)
	if r.ContentLength > 0 {
		size += r.ContentLength
	}

	return size
}

// UnaryServerInterceptor is a gRPC unary server interceptor that automatically
// records standard metrics for gRPC calls including duration and count.
// It initializes standard metrics on first use with the provided namespace.
func UnaryServerInterceptor(namespace string) grpc.UnaryServerInterceptor {
	// Initialize standard metrics on first interceptor creation
	if err := InitStandardMetrics(namespace); err != nil {
		// Log error but continue - metrics are non-critical
		fmt.Printf("failed to initialize standard metrics: %v\n", err)
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call handler
		resp, err := handler(ctx, req)

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := grpcStatusCode(err)

		if grpcCallDuration != nil {
			grpcCallDuration.Observe(duration, info.FullMethod, statusCode)
		}

		if grpcCallCount != nil {
			grpcCallCount.Inc(info.FullMethod, statusCode)
		}

		return resp, err
	}
}

// StreamServerInterceptor is a gRPC stream server interceptor that automatically
// records standard metrics for gRPC streaming calls including duration and count.
// It initializes standard metrics on first use with the provided namespace.
func StreamServerInterceptor(namespace string) grpc.StreamServerInterceptor {
	// Initialize standard metrics on first interceptor creation
	if err := InitStandardMetrics(namespace); err != nil {
		// Log error but continue - metrics are non-critical
		fmt.Printf("failed to initialize standard metrics: %v\n", err)
	}

	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// Call handler
		err := handler(srv, ss)

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := grpcStatusCode(err)

		if grpcCallDuration != nil {
			grpcCallDuration.Observe(duration, info.FullMethod, statusCode)
		}

		if grpcCallCount != nil {
			grpcCallCount.Inc(info.FullMethod, statusCode)
		}

		return err
	}
}

// grpcStatusCode extracts the gRPC status code from an error.
// Returns "OK" if err is nil.
func grpcStatusCode(err error) string {
	if err == nil {
		return "OK"
	}

	// Handle context cancellation
	if err == context.Canceled {
		return "CANCELLED"
	}
	if err == context.DeadlineExceeded {
		return "DEADLINE_EXCEEDED"
	}

	// Handle EOF
	if err == io.EOF {
		return "UNAVAILABLE"
	}

	// Extract gRPC status code
	st, ok := status.FromError(err)
	if ok {
		return st.Code().String()
	}

	return "UNKNOWN"
}
