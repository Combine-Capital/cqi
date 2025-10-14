package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan creates a new span with the given name and options.
// It automatically links the span to its parent span from the context.
// The returned context contains the new span and should be passed to downstream operations.
//
// Example:
//
//	ctx, span := tracing.StartSpan(ctx, "operation-name",
//	    trace.WithAttributes(attribute.String("user.id", "123")))
//	defer span.End()
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("cqi")
	return tracer.Start(ctx, name, opts...)
}

// StartSpanWithTracer creates a new span using a specific tracer.
// This is useful when you want to use a tracer with a specific name or version.
func StartSpanWithTracer(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName, opts...)
}

// SpanFromContext retrieves the current span from the context.
// Returns a no-op span if no span is present in the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the given span.
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// SetSpanAttributes adds attributes to the span in the context.
// This is a convenience function that extracts the span and sets attributes.
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// SetSpanError marks the span as errored and records the error message.
// The span status is set to Error and the error is recorded as an event.
func SetSpanError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanStatus sets the status code and description of the span.
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// AddSpanEvent adds an event to the span with the given name and attributes.
// Events are timestamped occurrences that happened during the span's lifetime.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// Common attribute helpers for convenience

// HTTPAttributes returns common HTTP attributes for a span.
func HTTPAttributes(method, path, host string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.String("http.host", host),
		attribute.Int("http.status_code", statusCode),
	}
}

// GRPCAttributes returns common gRPC attributes for a span.
func GRPCAttributes(method, service string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("rpc.system", "grpc"),
		attribute.String("rpc.service", service),
		attribute.String("rpc.method", method),
		attribute.Int("rpc.grpc.status_code", statusCode),
	}
}

// DatabaseAttributes returns common database attributes for a span.
func DatabaseAttributes(system, operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.system", system),
		attribute.String("db.operation", operation),
		attribute.String("db.sql.table", table),
	}
}

// CacheAttributes returns common cache attributes for a span.
func CacheAttributes(system, operation, key string, hit bool) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cache.system", system),
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
		attribute.Bool("cache.hit", hit),
	}
}

// MessagingAttributes returns common messaging attributes for a span.
func MessagingAttributes(system, destination, operation string, messageSize int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("messaging.system", system),
		attribute.String("messaging.destination", destination),
		attribute.String("messaging.operation", operation),
		attribute.Int("messaging.message.payload_size_bytes", messageSize),
	}
}
