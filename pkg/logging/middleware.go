package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// HTTPMiddleware is an HTTP middleware that logs request and response details.
// It automatically generates a request ID if one doesn't exist and logs:
// - Request start (method, path, request_id)
// - Request end (method, path, status, duration, request_id)
func HTTPMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate or extract request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}

			// Add request ID to context
			ctx := WithRequestID(r.Context(), requestID)
			ctx = WithLogger(ctx, logger)
			r = r.WithContext(ctx)

			// Log request start
			logger.Info().
				Str(RequestID, requestID).
				Str(Method, r.Method).
				Str(Path, r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("request started")

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Call next handler
			next.ServeHTTP(wrapped, r)

			// Log request end
			duration := time.Since(start).Milliseconds()
			logEvent := logger.Info()

			// Use Error level for 5xx errors
			if wrapped.statusCode >= 500 {
				logEvent = logger.Error()
			}

			logEvent.
				Str(RequestID, requestID).
				Str(Method, r.Method).
				Str(Path, r.URL.Path).
				Int(StatusCode, wrapped.statusCode).
				Int64(Duration, duration).
				Msg("request completed")
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// generateRequestID generates a random request ID.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(b)
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs RPC calls.
// It logs:
// - RPC start (method)
// - RPC end (method, status, duration)
func UnaryServerInterceptor(logger *Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Generate request ID
		requestID := generateRequestID()
		ctx = WithRequestID(ctx, requestID)
		ctx = WithLogger(ctx, logger)

		// Log RPC start
		logger.Info().
			Str(RequestID, requestID).
			Str(Method, info.FullMethod).
			Msg("grpc call started")

		// Call handler
		resp, err := handler(ctx, req)

		// Log RPC end
		duration := time.Since(start).Milliseconds()
		st, _ := status.FromError(err)

		logEvent := logger.Info()
		if err != nil {
			logEvent = logger.Error().
				Str(Error, err.Error()).
				Str("grpc_code", st.Code().String())
		}

		logEvent.
			Str(RequestID, requestID).
			Str(Method, info.FullMethod).
			Int64(Duration, duration).
			Msg("grpc call completed")

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that logs stream operations.
func StreamServerInterceptor(logger *Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// Generate request ID
		requestID := generateRequestID()
		ctx := WithRequestID(ss.Context(), requestID)
		ctx = WithLogger(ctx, logger)

		// Create wrapped stream with enriched context
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Log stream start
		logger.Info().
			Str(RequestID, requestID).
			Str(Method, info.FullMethod).
			Bool("is_client_stream", info.IsClientStream).
			Bool("is_server_stream", info.IsServerStream).
			Msg("grpc stream started")

		// Call handler
		err := handler(srv, wrapped)

		// Log stream end
		duration := time.Since(start).Milliseconds()
		st, _ := status.FromError(err)

		logEvent := logger.Info()
		if err != nil {
			logEvent = logger.Error().
				Str(Error, err.Error()).
				Str("grpc_code", st.Code().String())
		}

		logEvent.
			Str(RequestID, requestID).
			Str(Method, info.FullMethod).
			Int64(Duration, duration).
			Msg("grpc stream completed")

		return err
	}
}

// wrappedServerStream wraps grpc.ServerStream to provide enriched context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
