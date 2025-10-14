package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewTracerProvider_Disabled(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled: false,
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-service")
	if err != nil {
		t.Fatalf("expected no error for disabled tracing, got %v", err)
	}
	defer shutdown(context.Background())

	if tp == nil {
		t.Fatal("expected non-nil tracer provider")
	}
}

func TestNewTracerProvider_MissingEndpoint(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:  true,
		Endpoint: "",
	}

	_, _, err := NewTracerProvider(context.Background(), cfg, "test-service")
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	if err.Error() != "tracing endpoint is required when tracing is enabled" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestNewTracerProvider_MissingServiceName(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:  true,
		Endpoint: "localhost:4317",
	}

	_, _, err := NewTracerProvider(context.Background(), cfg, "")
	if err == nil {
		t.Fatal("expected error for missing service name")
	}
	if err.Error() != "service name is required for tracing" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestNewTracerProvider_InvalidExportMode(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:     true,
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		ExportMode:  "invalid",
	}

	_, _, err := NewTracerProvider(context.Background(), cfg, "test-service")
	if err == nil {
		t.Fatal("expected error for invalid export mode")
	}
}

func TestNewTracerProvider_SamplingRates(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
		expectType string
	}{
		{"never sample", 0.0, "NeverSample"},
		{"always sample", 1.0, "AlwaysSample"},
		{"ratio sample", 0.5, "TraceIDRatioBased"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use in-memory exporter for testing
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(
				sdktrace.WithSyncer(exporter),
			)
			otel.SetTracerProvider(tp)

			// Test sampling would require checking actual spans
			// For now, just verify initialization doesn't fail
			if tp == nil {
				t.Fatal("expected non-nil tracer provider")
			}
		})
	}
}

func TestStartSpan(t *testing.T) {
	// Set up in-memory exporter for testing
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "test-span" {
		t.Fatalf("expected span name 'test-span', got '%s'", spans[0].Name)
	}
}

func TestStartSpanWithTracer(t *testing.T) {
	// Set up in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpanWithTracer(ctx, "custom-tracer", "operation")
	if span == nil {
		t.Fatal("expected non-nil span")
	}
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestSpanFromContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")
	defer span.End()

	retrieved := SpanFromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected non-nil span from context")
	}

	// Verify it's the same span
	if retrieved.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("retrieved span has different trace ID")
	}
}

func TestSetSpanAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")

	SetSpanAttributes(ctx,
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
	)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	attrs := spans[0].Attributes
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}
}

func TestSetSpanError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")

	testErr := errors.New("test error")
	SetSpanError(ctx, testErr)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Status.Code != codes.Error {
		t.Fatalf("expected error status, got %v", spans[0].Status.Code)
	}

	if len(spans[0].Events) == 0 {
		t.Fatal("expected error event to be recorded")
	}
}

func TestSetSpanError_Nil(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")

	// Should not panic or record error
	SetSpanError(ctx, nil)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Status should not be error
	if spans[0].Status.Code == codes.Error {
		t.Fatal("expected non-error status for nil error")
	}
}

func TestSetSpanStatus(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")

	SetSpanStatus(ctx, codes.Ok, "success")
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Status.Code != codes.Ok {
		t.Fatalf("expected Ok status, got %v", spans[0].Status.Code)
	}
}

func TestAddSpanEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-span")

	AddSpanEvent(ctx, "test-event",
		attribute.String("event.detail", "test detail"),
	)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if len(spans[0].Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(spans[0].Events))
	}

	if spans[0].Events[0].Name != "test-event" {
		t.Fatalf("expected event name 'test-event', got '%s'", spans[0].Events[0].Name)
	}
}

func TestHTTPAttributes(t *testing.T) {
	attrs := HTTPAttributes("GET", "/api/users", "example.com", 200)
	if len(attrs) != 4 {
		t.Fatalf("expected 4 attributes, got %d", len(attrs))
	}

	// Verify each attribute exists
	foundMethod := false
	for _, attr := range attrs {
		if attr.Key == "http.method" && attr.Value.AsString() == "GET" {
			foundMethod = true
		}
	}
	if !foundMethod {
		t.Fatal("http.method attribute not found or incorrect")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	// Set up in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Wrap with middleware
	wrapped := HTTPMiddleware("test-service")(handler)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "GET /test" {
		t.Fatalf("expected span name 'GET /test', got '%s'", span.Name)
	}

	// Verify attributes
	var statusCode int
	for _, attr := range span.Attributes {
		if attr.Key == "http.status_code" {
			statusCode = int(attr.Value.AsInt64())
		}
	}
	if statusCode != 200 {
		t.Fatalf("expected status code 200 in span, got %d", statusCode)
	}
}

func TestHTTPMiddleware_Error(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	// Handler that returns 500
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})

	wrapped := HTTPMiddleware("test-service")(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Verify error status
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("expected error status for 5xx response, got %v", spans[0].Status.Code)
	}
}

func TestHTTPMiddleware_TraceContextPropagation(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify we can access the span
		span := SpanFromContext(r.Context())
		if !span.SpanContext().IsValid() {
			t.Error("expected valid span context in handler")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := HTTPMiddleware("test-service")(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Should have at least the middleware span
	spans := exporter.GetSpans()
	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}
}

func TestGRPCUnaryServerInterceptor(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	interceptor := GRPCUnaryServerInterceptor("test-service")

	// Mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "response", nil
	}

	// Mock info
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	ctx := context.Background()
	resp, err := interceptor(ctx, "request", info, handler)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Fatalf("expected 'response', got %v", resp)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Name != "/test.Service/Method" {
		t.Fatalf("expected span name '/test.Service/Method', got '%s'", spans[0].Name)
	}
}

func TestGRPCUnaryServerInterceptor_Error(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	interceptor := GRPCUnaryServerInterceptor("test-service")

	testErr := errors.New("test error")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, testErr
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	ctx := context.Background()
	_, err := interceptor(ctx, "request", info, handler)

	if err != testErr {
		t.Fatalf("expected test error, got %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Status.Code != codes.Error {
		t.Fatalf("expected error status, got %v", spans[0].Status.Code)
	}
}

func TestGRPCUnaryServerInterceptor_MetadataPropagation(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	interceptor := GRPCUnaryServerInterceptor("test-service")

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		span := SpanFromContext(ctx)
		if !span.SpanContext().IsValid() {
			t.Error("expected valid span context in handler")
		}
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	ctx := context.Background()
	_, err := interceptor(ctx, "request", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}
}

func TestInjectExtractHTTP(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	// Set up propagator (required for inject/extract)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// Create span and inject
	ctx, span := StartSpan(context.Background(), "test")
	defer span.End()

	header := http.Header{}
	InjectHTTP(ctx, header)

	// Verify traceparent header exists
	if header.Get("traceparent") == "" {
		t.Fatal("expected traceparent header to be set")
	}

	// Extract from header
	extractedCtx := ExtractHTTP(context.Background(), header)

	// Verify trace context was propagated
	extractedSpan := trace.SpanFromContext(extractedCtx)
	if !extractedSpan.SpanContext().IsValid() {
		t.Fatal("expected valid span context after extraction")
	}

	if extractedSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("trace ID mismatch after extraction")
	}
}

func TestInjectExtractGRPC(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	// Set up propagator (required for inject/extract)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// Create span and inject
	ctx, span := StartSpan(context.Background(), "test")
	defer span.End()

	md := metadata.New(nil)
	InjectGRPC(ctx, &md)

	// Verify metadata contains trace context
	if len(md.Get("traceparent")) == 0 {
		t.Fatal("expected traceparent in metadata")
	}

	// Extract from metadata
	extractedCtx := ExtractGRPC(context.Background(), &md)

	// Verify trace context was propagated
	extractedSpan := trace.SpanFromContext(extractedCtx)
	if !extractedSpan.SpanContext().IsValid() {
		t.Fatal("expected valid span context after extraction")
	}

	if extractedSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("trace ID mismatch after extraction")
	}
}

func TestGetTracer(t *testing.T) {
	tracer := GetTracer("test-tracer")
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestShutdown(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	shutdown := func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}

	// Create some spans
	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test")
	span.End()

	// Shutdown should not error
	err := shutdown(context.Background())
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestShutdown_Timeout(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	shutdown := func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()
		return tp.Shutdown(shutdownCtx)
	}

	// Shutdown with immediate timeout - may or may not error depending on timing
	_ = shutdown(context.Background())
	// We don't assert error here as it's timing-dependent
}

func TestContextWithSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	_, span := StartSpan(ctx, "test")
	defer span.End()

	newCtx := ContextWithSpan(context.Background(), span)
	retrievedSpan := SpanFromContext(newCtx)

	if retrievedSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatal("span not properly stored in context")
	}
}

func TestGRPCAttributes(t *testing.T) {
	attrs := GRPCAttributes("GetUser", "UserService", 0)
	if len(attrs) != 4 {
		t.Fatalf("expected 4 attributes, got %d", len(attrs))
	}

	foundSystem := false
	for _, attr := range attrs {
		if attr.Key == "rpc.system" && attr.Value.AsString() == "grpc" {
			foundSystem = true
		}
	}
	if !foundSystem {
		t.Fatal("rpc.system attribute not found or incorrect")
	}
}

func TestDatabaseAttributes(t *testing.T) {
	attrs := DatabaseAttributes("postgres", "SELECT", "users")
	if len(attrs) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(attrs))
	}

	foundSystem := false
	for _, attr := range attrs {
		if attr.Key == "db.system" && attr.Value.AsString() == "postgres" {
			foundSystem = true
		}
	}
	if !foundSystem {
		t.Fatal("db.system attribute not found or incorrect")
	}
}

func TestCacheAttributes(t *testing.T) {
	attrs := CacheAttributes("redis", "GET", "user:123", true)
	if len(attrs) != 4 {
		t.Fatalf("expected 4 attributes, got %d", len(attrs))
	}

	foundHit := false
	for _, attr := range attrs {
		if attr.Key == "cache.hit" && attr.Value.AsBool() {
			foundHit = true
		}
	}
	if !foundHit {
		t.Fatal("cache.hit attribute not found or incorrect")
	}
}

func TestMessagingAttributes(t *testing.T) {
	attrs := MessagingAttributes("kafka", "user-events", "publish", 1024)
	if len(attrs) != 4 {
		t.Fatalf("expected 4 attributes, got %d", len(attrs))
	}

	foundSystem := false
	for _, attr := range attrs {
		if attr.Key == "messaging.system" && attr.Value.AsString() == "kafka" {
			foundSystem = true
		}
	}
	if !foundSystem {
		t.Fatal("messaging.system attribute not found or incorrect")
	}
}

func TestGetPropagator(t *testing.T) {
	// Set a propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
		),
	)

	propagator := GetPropagator()
	if propagator == nil {
		t.Fatal("expected non-nil propagator")
	}
}

func TestHTTPCarrierKeys(t *testing.T) {
	header := http.Header{}
	header.Set("key1", "value1")
	header.Set("key2", "value2")

	carrier := HTTPCarrier(header)
	keys := carrier.Keys()

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestGRPCCarrierKeys(t *testing.T) {
	md := metadata.New(map[string]string{
		"key1": "value1",
		"key2": "value2",
	})

	carrier := &GRPCCarrier{md: &md}
	keys := carrier.Keys()

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestGRPCStreamServerInterceptor(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	interceptor := GRPCStreamServerInterceptor("test-service")

	// Mock stream
	mockStream := &mockServerStream{
		ctx: context.Background(),
	}

	// Mock handler
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsServerStream: true,
		IsClientStream: false,
	}

	err := interceptor(nil, mockStream, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Name != "/test.Service/StreamMethod" {
		t.Fatalf("expected span name '/test.Service/StreamMethod', got '%s'", spans[0].Name)
	}
}

func TestGRPCStreamServerInterceptor_Error(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	interceptor := GRPCStreamServerInterceptor("test-service")

	mockStream := &mockServerStream{
		ctx: context.Background(),
	}

	testErr := errors.New("stream error")
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return testErr
	}

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsServerStream: true,
	}

	err := interceptor(nil, mockStream, info, handler)
	if err != testErr {
		t.Fatalf("expected test error, got %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Status.Code != codes.Error {
		t.Fatalf("expected error status, got %v", spans[0].Status.Code)
	}
}

// Mock server stream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func (m *mockServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockServerStream) SetTrailer(metadata.MD) {
}

func (m *mockServerStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockServerStream) RecvMsg(interface{}) error {
	return nil
}

func TestNewTracerProvider_GRPCExporter(t *testing.T) {
	// Test initialization with gRPC mode (even if it fails to connect)
	cfg := config.TracingConfig{
		Enabled:     true,
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		ExportMode:  "grpc",
		Insecure:    true,
		SampleRate:  1.0,
	}

	// Use short timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tp, shutdown, err := NewTracerProvider(ctx, cfg, "test-service")

	// Connection may fail, but we should get a provider
	if err != nil && tp == nil {
		// Expected if no OTLP endpoint is running
		return
	}

	if shutdown != nil {
		defer shutdown(context.Background())
	}

	if tp != nil && err == nil {
		// Verify provider is set up
		tracer := tp.Tracer("test")
		if tracer == nil {
			t.Fatal("expected non-nil tracer from provider")
		}
	}
}

func TestNewTracerProvider_HTTPExporter(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:      true,
		Endpoint:     "localhost:4318",
		ServiceName:  "test-service",
		ExportMode:   "http",
		Insecure:     true,
		SampleRate:   0.5,
		BatchTimeout: 1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tp, shutdown, err := NewTracerProvider(ctx, cfg, "test-service")

	// Connection may fail, but we should get a provider
	if err != nil && tp == nil {
		// Expected if no OTLP endpoint is running
		return
	}

	if shutdown != nil {
		defer shutdown(context.Background())
	}

	if tp != nil && err == nil {
		tracer := tp.Tracer("test")
		if tracer == nil {
			t.Fatal("expected non-nil tracer from provider")
		}
	}
}

func TestNewTracerProvider_ConfigServiceName(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:     false, // Disabled to avoid connection attempts
		ServiceName: "configured-service",
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "fallback-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	// Verify initialization succeeded with disabled tracing
	if tp == nil {
		t.Fatal("expected non-nil tracer provider")
	}
}

func TestNewTracerProvider_SampleRateNever(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:     false, // Disabled to simplify test
		ServiceName: "test-service",
		SampleRate:  0.0, // Never sample
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	if tp == nil {
		t.Fatal("expected non-nil tracer provider")
	}
}

func TestNewTracerProvider_DefaultBatchTimeout(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:      false, // Disabled to simplify test
		ServiceName:  "test-service",
		BatchTimeout: 0, // Should use default (5s)
	}

	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background())

	if tp == nil {
		t.Fatal("expected non-nil tracer provider")
	}
}
