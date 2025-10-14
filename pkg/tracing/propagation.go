package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/metadata"
)

// HTTPCarrier adapts http.Header to TextMapCarrier interface.
type HTTPCarrier http.Header

// Get returns the value associated with the passed key.
func (c HTTPCarrier) Get(key string) string {
	return http.Header(c).Get(key)
}

// Set stores the key-value pair.
func (c HTTPCarrier) Set(key, value string) {
	http.Header(c).Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c HTTPCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// GRPCCarrier adapts gRPC metadata to TextMapCarrier interface.
type GRPCCarrier struct {
	md *metadata.MD
}

// Get returns the value associated with the passed key.
func (c *GRPCCarrier) Get(key string) string {
	values := c.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set stores the key-value pair.
func (c *GRPCCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (c *GRPCCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.md))
	for k := range *c.md {
		keys = append(keys, k)
	}
	return keys
}

// InjectHTTP injects trace context into HTTP request headers.
// It uses the W3C Trace Context propagation format (traceparent, tracestate).
//
// Example:
//
//	req, _ := http.NewRequest("GET", "http://example.com", nil)
//	tracing.InjectHTTP(ctx, req.Header)
func InjectHTTP(ctx context.Context, header http.Header) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, HTTPCarrier(header))
}

// ExtractHTTP extracts trace context from HTTP request headers.
// It returns a new context with the extracted trace information.
//
// Example:
//
//	ctx = tracing.ExtractHTTP(ctx, req.Header)
//	ctx, span := tracing.StartSpan(ctx, "handler")
//	defer span.End()
func ExtractHTTP(ctx context.Context, header http.Header) context.Context {
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, HTTPCarrier(header))
}

// InjectGRPC injects trace context into gRPC metadata.
// It uses the W3C Trace Context propagation format.
//
// Example:
//
//	md := metadata.New(nil)
//	tracing.InjectGRPC(ctx, &md)
//	ctx = metadata.NewOutgoingContext(ctx, md)
func InjectGRPC(ctx context.Context, md *metadata.MD) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, &GRPCCarrier{md: md})
}

// ExtractGRPC extracts trace context from gRPC metadata.
// It returns a new context with the extracted trace information.
//
// Example:
//
//	md, ok := metadata.FromIncomingContext(ctx)
//	if ok {
//	    ctx = tracing.ExtractGRPC(ctx, &md)
//	}
func ExtractGRPC(ctx context.Context, md *metadata.MD) context.Context {
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, &GRPCCarrier{md: md})
}

// GetPropagator returns the global text map propagator.
// This can be used for custom propagation scenarios.
func GetPropagator() propagation.TextMapPropagator {
	return otel.GetTextMapPropagator()
}
