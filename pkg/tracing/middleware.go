package tracing

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// HTTPMiddleware is an HTTP middleware that creates a span for each incoming request.
// It automatically extracts trace context from request headers, creates a root span,
// and injects span information into the request context for downstream handlers.
//
// Example:
//
//	mux := http.NewServeMux()
//	handler := tracing.HTTPMiddleware("my-service")(mux)
//	http.ListenAndServe(":8080", handler)
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request
			ctx := ExtractHTTP(r.Context(), r.Header)

			// Start a new span for this request
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := StartSpanWithTracer(ctx, serviceName, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
					attribute.String("http.user_agent", r.Header.Get("User-Agent")),
					attribute.String("http.client_ip", r.RemoteAddr),
				),
			)
			defer span.End()

			// Wrap response writer to capture status code
			wrapped := &tracingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Update request with trace context
			r = r.WithContext(ctx)

			// Call next handler
			start := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)

			// Add response attributes to span
			span.SetAttributes(
				attribute.Int("http.status_code", wrapped.statusCode),
				attribute.Int64("http.response_time_ms", duration.Milliseconds()),
			)

			// Mark span as error for 5xx status codes
			if wrapped.statusCode >= 500 {
				span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// tracingResponseWriter wraps http.ResponseWriter to capture the status code.
type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *tracingResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *tracingResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// GRPCUnaryServerInterceptor returns a gRPC server interceptor that creates a span
// for each incoming unary RPC call. It automatically extracts trace context from
// gRPC metadata and propagates it through the call chain.
//
// Example:
//
//	s := grpc.NewServer(
//	    grpc.UnaryInterceptor(tracing.GRPCUnaryServerInterceptor("my-service")),
//	)
func GRPCUnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract trace context from incoming metadata
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = ExtractGRPC(ctx, &md)
		}

		// Start a new span for this RPC
		ctx, span := StartSpanWithTracer(ctx, serviceName, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", serviceName),
				attribute.String("rpc.method", info.FullMethod),
			),
		)
		defer span.End()

		// Call the handler
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// Record span attributes based on response
		span.SetAttributes(
			attribute.Int64("rpc.duration_ms", duration.Milliseconds()),
		)

		// Handle error status
		if err != nil {
			s, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.Int("rpc.grpc.status_code", int(s.Code())),
					attribute.String("rpc.grpc.status_message", s.Message()),
				)
				span.SetStatus(codes.Error, s.Message())
			} else {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		} else {
			span.SetAttributes(
				attribute.Int("rpc.grpc.status_code", 0), // OK
			)
			span.SetStatus(codes.Ok, "")
		}

		return resp, err
	}
}

// GRPCStreamServerInterceptor returns a gRPC server interceptor that creates a span
// for each incoming streaming RPC call.
//
// Example:
//
//	s := grpc.NewServer(
//	    grpc.StreamInterceptor(tracing.GRPCStreamServerInterceptor("my-service")),
//	)
func GRPCStreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Extract trace context from incoming metadata
		ctx := ss.Context()
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = ExtractGRPC(ctx, &md)
		}

		// Start a new span for this stream
		ctx, span := StartSpanWithTracer(ctx, serviceName, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", serviceName),
				attribute.String("rpc.method", info.FullMethod),
				attribute.Bool("rpc.grpc.is_server_stream", info.IsServerStream),
				attribute.Bool("rpc.grpc.is_client_stream", info.IsClientStream),
			),
		)
		defer span.End()

		// Wrap the server stream with traced context
		wrapped := &tracedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Call the handler
		start := time.Now()
		err := handler(srv, wrapped)
		duration := time.Since(start)

		// Record span attributes
		span.SetAttributes(
			attribute.Int64("rpc.duration_ms", duration.Milliseconds()),
		)

		// Handle error status
		if err != nil {
			s, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.Int("rpc.grpc.status_code", int(s.Code())),
					attribute.String("rpc.grpc.status_message", s.Message()),
				)
				span.SetStatus(codes.Error, s.Message())
			} else {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		} else {
			span.SetAttributes(
				attribute.Int("rpc.grpc.status_code", 0), // OK
			)
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// tracedServerStream wraps grpc.ServerStream to propagate context with trace.
type tracedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}
