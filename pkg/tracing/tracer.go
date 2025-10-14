// Package tracing provides OpenTelemetry distributed tracing with W3C trace context propagation.
// It supports OTLP exporters (gRPC/HTTP), automatic span creation via middleware, and graceful shutdown.
//
// Example usage:
//
//	cfg := config.TracingConfig{
//	    Enabled:    true,
//	    Endpoint:   "localhost:4317",
//	    SampleRate: 0.1,
//	    ExportMode: "grpc",
//	}
//	tp, shutdown, err := tracing.NewTracerProvider(ctx, cfg, "my-service")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown(ctx)
package tracing

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ShutdownFunc is a function to gracefully shutdown the tracer provider.
// It should be called with a context when the application terminates to flush pending spans.
type ShutdownFunc func(context.Context) error

// NewTracerProvider creates and initializes a TracerProvider with OTLP exporter.
// It configures the tracer with the provided config and service name, setting up
// resource attributes, sampling, and the appropriate exporter (gRPC or HTTP).
//
// The context is used for initialization operations and should have a reasonable timeout.
// The returned ShutdownFunc must be called to flush pending spans before process termination.
//
// If tracing is disabled in config, it returns a no-op tracer provider and shutdown function.
func NewTracerProvider(ctx context.Context, cfg config.TracingConfig, serviceName string) (*sdktrace.TracerProvider, ShutdownFunc, error) {
	// If tracing is disabled, return no-op provider
	if !cfg.Enabled {
		noopShutdown := func(context.Context) error { return nil }
		return sdktrace.NewTracerProvider(), noopShutdown, nil
	}

	// Validate required configuration
	if cfg.Endpoint == "" {
		return nil, nil, fmt.Errorf("tracing endpoint is required when tracing is enabled")
	}

	// Use configured service name or fall back to provided serviceName
	svcName := cfg.ServiceName
	if svcName == "" {
		svcName = serviceName
	}
	if svcName == "" {
		return nil, nil, fmt.Errorf("service name is required for tracing")
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(svcName),
			semconv.ServiceVersionKey.String("1.0.0"), // TODO: Make configurable
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create appropriate exporter based on export mode
	var exporter sdktrace.SpanExporter
	switch cfg.ExportMode {
	case "http":
		exporter, err = createHTTPExporter(ctx, cfg)
	case "grpc", "":
		exporter, err = createGRPCExporter(ctx, cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported export mode: %s (use 'grpc' or 'http')", cfg.ExportMode)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Configure sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Configure batch timeout if specified, otherwise use default (5s)
	batchTimeout := 5 * time.Second
	if cfg.BatchTimeout > 0 {
		batchTimeout = cfg.BatchTimeout
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(batchTimeout),
		),
	)

	// Set as global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator for W3C trace context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create shutdown function
	shutdown := func(ctx context.Context) error {
		// Allow up to 30 seconds for shutdown
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		return tp.Shutdown(shutdownCtx)
	}

	return tp, shutdown, nil
}

// createGRPCExporter creates an OTLP gRPC trace exporter.
func createGRPCExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}

	// Configure connection security
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// Add dial options for gRPC
	opts = append(opts, otlptracegrpc.WithDialOption(
		grpc.WithBlock(),
	))

	return otlptracegrpc.New(ctx, opts...)
}

// createHTTPExporter creates an OTLP HTTP trace exporter.
func createHTTPExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}

	// Configure connection security
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(ctx, opts...)
}

// GetTracer returns a tracer for the given name from the global tracer provider.
// This is a convenience function for getting a tracer after initialization.
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
