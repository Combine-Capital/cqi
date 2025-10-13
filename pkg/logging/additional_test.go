package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestWith verifies With() returns zerolog.Context
func TestWith(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	ctx := logger.With().Str("key", "value")
	newLogger := ctx.Logger()
	newLogger.Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if val, ok := logEntry["key"]; !ok || val != "value" {
		t.Errorf("key = %v, want 'value'", val)
	}
}

// TestGetZerolog verifies GetZerolog returns underlying logger
func TestGetZerolog(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	underlying := logger.GetZerolog()
	if underlying == nil {
		t.Error("GetZerolog() returned nil")
	}

	underlying.Info().Msg("test")
	if !bytes.Contains(buf.Bytes(), []byte("test")) {
		t.Error("underlying logger did not write to buffer")
	}
}

// TestSetLevel verifies SetLevel changes log level
func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf).Level(zerolog.InfoLevel)
	logger := &Logger{zlog: zlog}

	// Debug should not log at info level
	logger.Debug().Msg("debug message")
	if buf.Len() > 0 {
		t.Error("debug message logged at info level")
	}

	// Change to debug level
	logger.SetLevel(zerolog.DebugLevel)
	buf.Reset()

	logger.Debug().Msg("debug message")
	if !bytes.Contains(buf.Bytes(), []byte("debug message")) {
		t.Error("debug message not logged after changing level")
	}
}

// TestCtx verifies Ctx helper function
func TestCtx(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	ctx := context.Background()
	ctx = WithLogger(ctx, logger)
	ctx = WithTraceID(ctx, "trace-123")

	// Use Ctx helper
	Ctx(ctx).Info().Msg("test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if traceID, ok := logEntry[TraceID]; !ok || traceID != "trace-123" {
		t.Errorf("trace_id = %v, want 'trace-123'", traceID)
	}
}

// TestNewConsoleFormat verifies console format output
func TestNewConsoleFormat(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
	}

	logger := New(cfg)
	if logger == nil {
		t.Error("New() returned nil")
	}
}

// TestNewStderrOutput verifies stderr output configuration
func TestNewStderrOutput(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stderr",
	}

	logger := New(cfg)
	if logger == nil {
		t.Error("New() returned nil")
	}
}

// TestNewFileOutput verifies file output defaults to stdout
func TestNewFileOutput(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "/tmp/test.log",
	}

	logger := New(cfg)
	if logger == nil {
		t.Error("New() returned nil")
	}
}

// TestUnaryServerInterceptor verifies gRPC unary interceptor
func TestUnaryServerInterceptor(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	interceptor := UnaryServerInterceptor(logger)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// Verify request ID is in context
		if requestID := GetRequestID(ctx); requestID == "" {
			t.Error("request_id not found in context")
		}
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	resp, err := interceptor(context.Background(), "request", info, handler)

	if err != nil {
		t.Errorf("interceptor returned error: %v", err)
	}
	if resp != "response" {
		t.Errorf("interceptor response = %v, want 'response'", resp)
	}

	// Verify logs
	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}

	// Verify start log
	var startLog map[string]interface{}
	if err := json.Unmarshal(logs[0], &startLog); err != nil {
		t.Fatalf("failed to parse start log: %v", err)
	}
	if msg, ok := startLog["message"]; !ok || msg != "grpc call started" {
		t.Errorf("start log message = %v, want 'grpc call started'", msg)
	}

	// Verify end log
	var endLog map[string]interface{}
	if err := json.Unmarshal(logs[1], &endLog); err != nil {
		t.Fatalf("failed to parse end log: %v", err)
	}
	if msg, ok := endLog["message"]; !ok || msg != "grpc call completed" {
		t.Errorf("end log message = %v, want 'grpc call completed'", msg)
	}
}

// TestUnaryServerInterceptorError verifies error logging
func TestUnaryServerInterceptorError(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	interceptor := UnaryServerInterceptor(logger)

	testErr := status.Error(codes.Internal, "test error")
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, testErr
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	_, err := interceptor(context.Background(), "request", info, handler)

	if err != testErr {
		t.Errorf("interceptor error = %v, want %v", err, testErr)
	}

	// Verify error log
	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}

	var endLog map[string]interface{}
	if err := json.Unmarshal(logs[1], &endLog); err != nil {
		t.Fatalf("failed to parse end log: %v", err)
	}

	if level, ok := endLog["level"]; !ok || level != "error" {
		t.Errorf("end log level = %v, want 'error'", level)
	}
	if _, ok := endLog[Error]; !ok {
		t.Error("end log missing error field")
	}
}

// TestStreamServerInterceptor verifies gRPC stream interceptor
func TestStreamServerInterceptor(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	interceptor := StreamServerInterceptor(logger)

	called := false
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		called = true
		// Verify request ID is in context
		if requestID := GetRequestID(stream.Context()); requestID == "" {
			t.Error("request_id not found in stream context")
		}
		return nil
	}

	info := &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}

	mockStream := &mockServerStream{ctx: context.Background()}
	err := interceptor(nil, mockStream, info, handler)

	if err != nil {
		t.Errorf("interceptor returned error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}

	// Verify logs
	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}

	// Verify start log
	var startLog map[string]interface{}
	if err := json.Unmarshal(logs[0], &startLog); err != nil {
		t.Fatalf("failed to parse start log: %v", err)
	}
	if msg, ok := startLog["message"]; !ok || msg != "grpc stream started" {
		t.Errorf("start log message = %v, want 'grpc stream started'", msg)
	}
}

// TestStreamServerInterceptorError verifies stream error logging
func TestStreamServerInterceptorError(t *testing.T) {
	var buf bytes.Buffer
	zlog := zerolog.New(&buf)
	logger := &Logger{zlog: zlog}

	interceptor := StreamServerInterceptor(logger)

	testErr := errors.New("stream error")
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return testErr
	}

	info := &grpc.StreamServerInfo{
		FullMethod: "/test.Service/StreamMethod",
	}

	mockStream := &mockServerStream{ctx: context.Background()}
	err := interceptor(nil, mockStream, info, handler)

	if err != testErr {
		t.Errorf("interceptor error = %v, want %v", err, testErr)
	}

	// Verify error log
	logs := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var endLog map[string]interface{}
	if err := json.Unmarshal(logs[1], &endLog); err != nil {
		t.Fatalf("failed to parse end log: %v", err)
	}

	if level, ok := endLog["level"]; !ok || level != "error" {
		t.Errorf("end log level = %v, want 'error'", level)
	}
}

// TestWrappedServerStreamContext verifies wrapped stream returns enriched context
func TestWrappedServerStreamContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-req-id")

	mockStream := &mockServerStream{ctx: context.Background()}
	wrapped := &wrappedServerStream{
		ServerStream: mockStream,
		ctx:          ctx,
	}

	if reqID := GetRequestID(wrapped.Context()); reqID != "test-req-id" {
		t.Errorf("wrapped context request_id = %v, want 'test-req-id'", reqID)
	}
}

// TestResponseWriterWrite verifies Write calls WriteHeader
func TestResponseWriterWrite(t *testing.T) {
	rec := newMockResponseWriter()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	data := []byte("test data")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() n = %v, want %v", n, len(data))
	}
	if !rw.written {
		t.Error("Write() did not set written flag")
	}
	if string(rec.data) != "test data" {
		t.Errorf("Write() data = %v, want 'test data'", string(rec.data))
	}
}

// mockServerStream implements grpc.ServerStream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

// mockResponseWriter implements http.ResponseWriter for testing
type mockResponseWriter struct {
	header http.Header
	data   []byte
	code   int
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		header: make(http.Header),
		code:   200,
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.data = append(m.data, data...)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.code = code
}
