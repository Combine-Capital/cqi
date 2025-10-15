package httpclient

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/metrics"
	"github.com/Combine-Capital/cqi/pkg/tracing"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"resty.dev/v3"
)

// WithLogging adds logging middleware to the HTTP client.
// It logs request start, response, duration, and status codes.
func (c *Client) WithLogging(logger *zerolog.Logger) *Client {
	c.resty.AddRequestMiddleware(func(client *resty.Client, req *resty.Request) error {
		ctx := req.Context()
		start := time.Now()

		// Extract request info
		method := "UNKNOWN"
		url := ""
		if req.URL != "" {
			url = req.URL
		}

		// Log request start
		logger.Debug().
			Str("method", method).
			Str("url", url).
			Msg("HTTP request starting")

		// Store start time in context for response logging
		req.SetContext(context.WithValue(ctx, "http_start_time", start))

		return nil
	})

	c.resty.AddResponseMiddleware(func(client *resty.Client, resp *resty.Response) error {
		// Get start time from context
		startVal := resp.Request.Context().Value("http_start_time")
		start, ok := startVal.(time.Time)
		if !ok {
			start = time.Now() // Fallback if not found
		}

		duration := time.Since(start)
		statusCode := resp.StatusCode()

		// Determine log level based on status code
		logEvent := logger.Info()
		if statusCode >= 500 {
			logEvent = logger.Error()
		} else if statusCode >= 400 {
			logEvent = logger.Warn()
		}

		logEvent.
			Str("method", resp.Request.Method).
			Str("url", resp.Request.URL).
			Int("status_code", statusCode).
			Dur("duration_ms", duration).
			Msg("HTTP request completed")

		return nil
	})

	return c
}

// WithMetrics adds metrics middleware to the HTTP client.
// It records request count and duration for all HTTP requests.
func (c *Client) WithMetrics(namespace string) *Client {
	// Create metrics collectors
	requestDuration, err := metrics.NewHistogram(metrics.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http_client",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP client requests in seconds",
		Labels:    []string{"method", "status_code"},
	})
	if err != nil {
		// If metrics fail to initialize, we'll skip metrics collection
		// This allows the client to work even if metrics aren't set up
		return c
	}

	requestCount, err := metrics.NewCounter(metrics.CounterOpts{
		Namespace: namespace,
		Subsystem: "http_client",
		Name:      "request_count_total",
		Help:      "Total count of HTTP client requests",
		Labels:    []string{"method", "status_code"},
	})
	if err != nil {
		return c
	}

	c.resty.AddRequestMiddleware(func(client *resty.Client, req *resty.Request) error {
		// Store start time
		ctx := req.Context()
		req.SetContext(context.WithValue(ctx, "metrics_start_time", time.Now()))
		return nil
	})

	c.resty.AddResponseMiddleware(func(client *resty.Client, resp *resty.Response) error {
		// Get start time
		startVal := resp.Request.Context().Value("metrics_start_time")
		start, ok := startVal.(time.Time)
		if !ok {
			start = time.Now()
		}

		duration := time.Since(start).Seconds()
		method := resp.Request.Method
		statusCode := fmt.Sprintf("%d", resp.StatusCode())

		// Record metrics
		requestDuration.Observe(duration, method, statusCode)
		requestCount.Inc(method, statusCode)

		return nil
	})

	return c
}

// WithTracing adds distributed tracing middleware to the HTTP client.
// It creates spans for each request with appropriate attributes.
func (c *Client) WithTracing(serviceName string) *Client {
	c.resty.AddRequestMiddleware(func(client *resty.Client, req *resty.Request) error {
		ctx := req.Context()

		// Start a span
		spanCtx, span := tracing.StartSpan(ctx, fmt.Sprintf("HTTP %s", req.Method))

		// Add attributes
		span.SetAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL),
			attribute.String("http.scheme", "http"),
		)

		// Store span in context
		req.SetContext(context.WithValue(spanCtx, "http_span", span))

		return nil
	})

	c.resty.AddResponseMiddleware(func(client *resty.Client, resp *resty.Response) error {
		// Get span from context
		spanVal := resp.Request.Context().Value("http_span")
		if spanVal == nil {
			return nil
		}

		span, ok := spanVal.(trace.Span)
		if !ok {
			return nil
		}
		defer span.End()

		// Add response attributes
		statusCode := resp.StatusCode()
		span.SetAttributes(
			attribute.Int("http.status_code", statusCode),
		)

		// Set span status based on HTTP status code
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		} else if statusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return nil
	})

	return c
}

// WithAuthToken adds Bearer token authentication to all requests.
func (c *Client) WithAuthToken(token string) *Client {
	c.resty.SetAuthToken(token)
	return c
}

// WithBasicAuth adds basic authentication to all requests.
func (c *Client) WithBasicAuth(username, password string) *Client {
	c.resty.SetBasicAuth(username, password)
	return c
}

// WithDefaultHeader adds a default header to all requests.
func (c *Client) WithDefaultHeader(key, value string) *Client {
	c.resty.SetHeader(key, value)
	return c
}

// WithDefaultHeaders adds multiple default headers to all requests.
func (c *Client) WithDefaultHeaders(headers map[string]string) *Client {
	c.resty.SetHeaders(headers)
	return c
}
