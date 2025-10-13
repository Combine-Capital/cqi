package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
)

// TestNew verifies logger creation with different configurations
func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.LogConfig
		want zerolog.Level
	}{
		{
			name: "debug level",
			cfg: config.LogConfig{
				Level:  "debug",
				Format: "json",
				Output: "stdout",
			},
			want: zerolog.DebugLevel,
		},
		{
			name: "info level",
			cfg: config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			want: zerolog.InfoLevel,
		},
		{
			name: "warn level",
			cfg: config.LogConfig{
				Level:  "warn",
				Format: "json",
				Output: "stdout",
			},
			want: zerolog.WarnLevel,
		},
		{
			name: "error level",
			cfg: config.LogConfig{
				Level:  "error",
				Format: "json",
				Output: "stdout",
			},
			want: zerolog.ErrorLevel,
		},
		{
			name: "default level",
			cfg: config.LogConfig{
				Level:  "invalid",
				Format: "json",
				Output: "stdout",
			},
			want: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.cfg)
			if logger.Level() != tt.want {
				t.Errorf("New() level = %v, want %v", logger.Level(), tt.want)
			}
		})
	}
}

// TestLogLevels verifies all log levels work correctly
func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf).Level(zerolog.DebugLevel)
	logger := &Logger{zlog: zlog}

	tests := []struct {
		name     string
		logFunc  func() *zerolog.Event
		wantStr  string
		minLevel zerolog.Level
	}{
		{
			name:     "debug",
			logFunc:  logger.Debug,
			wantStr:  "debug",
			minLevel: zerolog.DebugLevel,
		},
		{
			name:     "info",
			logFunc:  logger.Info,
			wantStr:  "info",
			minLevel: zerolog.InfoLevel,
		},
		{
			name:     "warn",
			logFunc:  logger.Warn,
			wantStr:  "warn",
			minLevel: zerolog.WarnLevel,
		},
		{
			name:     "error",
			logFunc:  logger.Error,
			wantStr:  "error",
			minLevel: zerolog.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc().Msg("test message")

			got := buf.String()
			if !strings.Contains(got, tt.wantStr) {
				t.Errorf("log output = %v, want to contain %v", got, tt.wantStr)
			}
			if !strings.Contains(got, "test message") {
				t.Errorf("log output = %v, want to contain 'test message'", got)
			}
		})
	}
}

// TestWithComponent verifies component field is added
func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	componentLogger := logger.WithComponent("test-component")
	componentLogger.Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if comp, ok := logEntry[Component]; !ok || comp != "test-component" {
		t.Errorf("component = %v, want 'test-component'", comp)
	}
}

// TestWithServiceName verifies service name field is added
func TestWithServiceName(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	serviceLogger := logger.WithServiceName("test-service")
	serviceLogger.Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if svc, ok := logEntry[ServiceName]; !ok || svc != "test-service" {
		t.Errorf("service_name = %v, want 'test-service'", svc)
	}
}

// TestWithFields verifies multiple fields are added
func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	fields := map[string]interface{}{
		"user_id": "123",
		"action":  "login",
		"count":   42,
	}

	fieldsLogger := logger.WithFields(fields)
	fieldsLogger.Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	for k, v := range fields {
		if got, ok := logEntry[k]; !ok {
			t.Errorf("field %s not found in log", k)
		} else {
			// Convert float64 back to int for comparison
			switch expected := v.(type) {
			case int:
				if int(got.(float64)) != expected {
					t.Errorf("field %s = %v, want %v", k, got, v)
				}
			default:
				if got != v {
					t.Errorf("field %s = %v, want %v", k, got, v)
				}
			}
		}
	}
}

// TestContextPropagation verifies logger context propagation
func TestContextPropagation(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	ctx := context.Background()
	ctx = WithLogger(ctx, logger)
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithSpanID(ctx, "span-456")
	ctx = WithRequestID(ctx, "req-789")

	// Verify values are stored
	if got := GetTraceID(ctx); got != "trace-123" {
		t.Errorf("GetTraceID() = %v, want 'trace-123'", got)
	}
	if got := GetSpanID(ctx); got != "span-456" {
		t.Errorf("GetSpanID() = %v, want 'span-456'", got)
	}
	if got := GetRequestID(ctx); got != "req-789" {
		t.Errorf("GetRequestID() = %v, want 'req-789'", got)
	}

	// Verify logger from context has these fields
	ctxLogger := FromContext(ctx)
	ctxLogger.Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if got, ok := logEntry[TraceID]; !ok || got != "trace-123" {
		t.Errorf("trace_id = %v, want 'trace-123'", got)
	}
	if got, ok := logEntry[SpanID]; !ok || got != "span-456" {
		t.Errorf("span_id = %v, want 'span-456'", got)
	}
	if got, ok := logEntry[RequestID]; !ok || got != "req-789" {
		t.Errorf("request_id = %v, want 'req-789'", got)
	}
}

// TestFromContextNoLogger verifies default logger is returned when none in context
func TestFromContextNoLogger(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)

	if logger == nil {
		t.Error("FromContext() returned nil, want default logger")
	}
}

// TestWithTraceContext verifies trace context convenience function
func TestWithTraceContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceContext(ctx, "trace-abc", "span-def")

	if got := GetTraceID(ctx); got != "trace-abc" {
		t.Errorf("GetTraceID() = %v, want 'trace-abc'", got)
	}
	if got := GetSpanID(ctx); got != "span-def" {
		t.Errorf("GetSpanID() = %v, want 'span-def'", got)
	}
}

// TestHTTPMiddleware verifies HTTP middleware logging
func TestHTTPMiddleware(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		if requestID := GetRequestID(r.Context()); requestID == "" {
			t.Error("request_id not found in context")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-req-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse log output (contains two entries: start and end)
	logs := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}

	// Verify start log
	var startLog map[string]interface{}
	if err := json.Unmarshal([]byte(logs[0]), &startLog); err != nil {
		t.Fatalf("failed to parse start log: %v", err)
	}

	if msg, ok := startLog["message"]; !ok || msg != "request started" {
		t.Errorf("start log message = %v, want 'request started'", msg)
	}
	if reqID, ok := startLog[RequestID]; !ok || reqID != "test-req-id" {
		t.Errorf("start log request_id = %v, want 'test-req-id'", reqID)
	}

	// Verify end log
	var endLog map[string]interface{}
	if err := json.Unmarshal([]byte(logs[1]), &endLog); err != nil {
		t.Fatalf("failed to parse end log: %v", err)
	}

	if msg, ok := endLog["message"]; !ok || msg != "request completed" {
		t.Errorf("end log message = %v, want 'request completed'", msg)
	}
	if status, ok := endLog[StatusCode]; !ok || int(status.(float64)) != 200 {
		t.Errorf("end log status_code = %v, want 200", status)
	}
	if _, ok := endLog[Duration]; !ok {
		t.Error("end log missing duration_ms field")
	}
}

// TestHTTPMiddlewareGeneratesRequestID verifies request ID generation
func TestHTTPMiddlewareGeneratesRequestID(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	var capturedRequestID string
	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// Don't set X-Request-ID header, should be generated
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if capturedRequestID == "" {
		t.Error("request_id was not generated")
	}
}

// TestHTTPMiddleware5xxErrors verifies error logging for 5xx status codes
func TestHTTPMiddleware5xxErrors(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	handler := HTTPMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Parse log output
	logs := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}

	// Verify end log uses error level
	var endLog map[string]interface{}
	if err := json.Unmarshal([]byte(logs[1]), &endLog); err != nil {
		t.Fatalf("failed to parse end log: %v", err)
	}

	if level, ok := endLog["level"]; !ok || level != "error" {
		t.Errorf("end log level = %v, want 'error' for 5xx status", level)
	}
}

// TestResponseWriterCapture verifies response writer captures status code
func TestResponseWriterCapture(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: rec,
				statusCode:     http.StatusOK,
			}

			rw.WriteHeader(tt.statusCode)

			if rw.statusCode != tt.statusCode {
				t.Errorf("statusCode = %v, want %v", rw.statusCode, tt.statusCode)
			}
		})
	}
}

// TestGenerateRequestID verifies request ID generation
func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("generateRequestID() returned empty string")
	}
	if id2 == "" {
		t.Error("generateRequestID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateRequestID() returned duplicate IDs")
	}
}

// TestParseLogLevel verifies log level parsing
func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  zerolog.Level
	}{
		{"debug", "debug", zerolog.DebugLevel},
		{"info", "info", zerolog.InfoLevel},
		{"warn", "warn", zerolog.WarnLevel},
		{"warning", "warning", zerolog.WarnLevel},
		{"error", "error", zerolog.ErrorLevel},
		{"fatal", "fatal", zerolog.FatalLevel},
		{"panic", "panic", zerolog.PanicLevel},
		{"invalid", "invalid", zerolog.InfoLevel},
		{"uppercase", "INFO", zerolog.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogLevel(tt.level)
			if got != tt.want {
				t.Errorf("parseLogLevel(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

// BenchmarkLoggerInfo benchmarks info level logging
func BenchmarkLoggerInfo(b *testing.B) {
	logger := New(config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info().Str("key", "value").Msg("test message")
	}
}

// BenchmarkLoggerWithFields benchmarks logging with fields
func BenchmarkLoggerWithFields(b *testing.B) {
	logger := New(config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	fields := map[string]interface{}{
		"user_id": "123",
		"action":  "test",
		"count":   42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithFields(fields).Info().Msg("test message")
	}
}

// BenchmarkContextPropagation benchmarks context-based logging
func BenchmarkContextPropagation(b *testing.B) {
	logger := New(config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	ctx := context.Background()
	ctx = WithLogger(ctx, logger)
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithSpanID(ctx, "span-456")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FromContext(ctx).Info().Msg("test message")
	}
}
