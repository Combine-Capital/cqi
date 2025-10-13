// Package logging provides structured logging with zerolog for trace context propagation.
// It supports configurable log levels, output formats (JSON/console), and automatic
// extraction of trace/span IDs from context for distributed tracing correlation.
//
// Example usage:
//
//	cfg := config.LogConfig{
//	    Level:  "info",
//	    Format: "json",
//	    Output: "stdout",
//	}
//	logger := logging.New(cfg)
//	logger.Info().Str("user_id", "123").Msg("user logged in")
package logging

// Standard field names for structured logging.
// These constants ensure consistent field naming across all services.
const (
	// TraceID is the field name for distributed trace ID (W3C trace context).
	TraceID = "trace_id"

	// SpanID is the field name for current span ID within a trace.
	SpanID = "span_id"

	// ServiceName is the field name for the service generating the log.
	ServiceName = "service_name"

	// Timestamp is the field name for when the log was created.
	Timestamp = "timestamp"

	// Level is the field name for log level (debug, info, warn, error).
	Level = "level"

	// Message is the field name for the log message.
	Message = "message"

	// Error is the field name for error information.
	Error = "error"

	// RequestID is the field name for HTTP request ID.
	RequestID = "request_id"

	// Method is the field name for HTTP method.
	Method = "method"

	// Path is the field name for HTTP path.
	Path = "path"

	// StatusCode is the field name for HTTP status code.
	StatusCode = "status_code"

	// Duration is the field name for operation duration.
	Duration = "duration_ms"

	// UserID is the field name for authenticated user ID.
	UserID = "user_id"

	// Component is the field name for the component/package generating the log.
	Component = "component"
)

// Context keys for storing values in context.Context.
const (
	// loggerKey is the context key for storing logger instances.
	loggerKey = "cqi.logger"

	// traceIDKey is the context key for storing trace IDs.
	traceIDKey = "cqi.trace_id"

	// spanIDKey is the context key for storing span IDs.
	spanIDKey = "cqi.span_id"

	// requestIDKey is the context key for storing request IDs.
	requestIDKey = "cqi.request_id"
)
