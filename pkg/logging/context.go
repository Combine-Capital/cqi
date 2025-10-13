package logging

import (
	"context"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	loggerContextKey    = contextKey("cqi.logger")
	traceIDContextKey   = contextKey("cqi.trace_id")
	spanIDContextKey    = contextKey("cqi.span_id")
	requestIDContextKey = contextKey("cqi.request_id")
)

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

// FromContext extracts a logger from the context.
// If no logger is found, it returns a default logger.
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*Logger); ok {
		// Enrich logger with trace/span IDs from context if available
		return enrichLoggerFromContext(ctx, logger)
	}

	// Return default logger if none in context
	defaultLogger := New(defaultLogConfig())
	return enrichLoggerFromContext(ctx, defaultLogger)
}

// defaultLogConfig returns a default log configuration.
func defaultLogConfig() config.LogConfig {
	return config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
}

// enrichLoggerFromContext adds trace/span/request IDs from context to logger.
func enrichLoggerFromContext(ctx context.Context, logger *Logger) *Logger {
	fields := make(map[string]interface{})

	if traceID := GetTraceID(ctx); traceID != "" {
		fields[TraceID] = traceID
	}

	if spanID := GetSpanID(ctx); spanID != "" {
		fields[SpanID] = spanID
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		fields[RequestID] = requestID
	}

	if len(fields) > 0 {
		return logger.WithFields(fields)
	}

	return logger
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey, traceID)
}

// GetTraceID retrieves the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDContextKey).(string); ok {
		return traceID
	}
	return ""
}

// WithSpanID adds a span ID to the context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDContextKey, spanID)
}

// GetSpanID retrieves the span ID from the context.
func GetSpanID(ctx context.Context) string {
	if spanID, ok := ctx.Value(spanIDContextKey).(string); ok {
		return spanID
	}
	return ""
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDContextKey).(string); ok {
		return requestID
	}
	return ""
}

// WithTraceContext adds both trace and span IDs to the context.
// This is a convenience function for adding trace context in one call.
func WithTraceContext(ctx context.Context, traceID, spanID string) context.Context {
	ctx = WithTraceID(ctx, traceID)
	ctx = WithSpanID(ctx, spanID)
	return ctx
}

// Ctx returns a zerolog.Context that can be used to add fields to a log entry.
// This is a convenience function for getting a context-aware logger.
func Ctx(ctx context.Context) *zerolog.Logger {
	logger := FromContext(ctx)
	zlog := logger.GetZerolog()
	return zlog
}
