package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// APIKeyMiddleware returns an HTTP middleware that validates API keys.
// It checks the Authorization header for "Bearer {key}" format and validates
// the key against the provided list of valid keys.
//
// On success, it injects an AuthContext into the request context with:
// - AuthType set to AuthTypeAPIKey
// - ServiceID set to the API key (for identification in logs)
//
// On failure, it returns 401 Unauthorized.
func APIKeyMiddleware(validKeys []string) func(http.Handler) http.Handler {
	// Build a map for O(1) lookup
	keyMap := make(map[string]bool, len(validKeys))
	for _, key := range validKeys {
		keyMap[key] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteHTTPError(w, errors.NewUnauthorized("missing Authorization header"))
				return
			}

			// Parse "Bearer {key}" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				errors.WriteHTTPError(w, errors.NewUnauthorized("invalid Authorization header format, expected 'Bearer {key}'"))
				return
			}

			apiKey := parts[1]

			// Validate API key
			if !keyMap[apiKey] {
				errors.WriteHTTPError(w, errors.NewUnauthorized("invalid API key"))
				return
			}

			// Create auth context and inject into request context
			authCtx := &AuthContext{
				ServiceID: apiKey, // Use API key as service identifier
				AuthType:  AuthTypeAPIKey,
			}

			ctx := setAuthContext(r.Context(), authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyUnaryInterceptor returns a gRPC unary server interceptor that validates API keys.
// It checks the "authorization" metadata for "Bearer {key}" format and validates
// the key against the provided list of valid keys.
//
// On success, it injects an AuthContext into the request context.
// On failure, it returns codes.Unauthenticated status.
func APIKeyUnaryInterceptor(validKeys []string) grpc.UnaryServerInterceptor {
	// Build a map for O(1) lookup
	keyMap := make(map[string]bool, len(validKeys))
	for _, key := range validKeys {
		keyMap[key] = true
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authCtx, err := validateAPIKeyFromMetadata(ctx, keyMap)
		if err != nil {
			return nil, err
		}

		// Inject auth context into request context
		ctx = setAuthContext(ctx, authCtx)

		return handler(ctx, req)
	}
}

// APIKeyStreamInterceptor returns a gRPC stream server interceptor that validates API keys.
// It checks the "authorization" metadata for "Bearer {key}" format and validates
// the key against the provided list of valid keys.
//
// On success, it injects an AuthContext into the request context.
// On failure, it returns codes.Unauthenticated status.
func APIKeyStreamInterceptor(validKeys []string) grpc.StreamServerInterceptor {
	// Build a map for O(1) lookup
	keyMap := make(map[string]bool, len(validKeys))
	for _, key := range validKeys {
		keyMap[key] = true
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authCtx, err := validateAPIKeyFromMetadata(ss.Context(), keyMap)
		if err != nil {
			return err
		}

		// Wrap the stream with authenticated context
		wrappedStream := &authenticatedStream{
			ServerStream: ss,
			ctx:          setAuthContext(ss.Context(), authCtx),
		}

		return handler(srv, wrappedStream)
	}
}

// validateAPIKeyFromMetadata extracts and validates an API key from gRPC metadata.
func validateAPIKeyFromMetadata(ctx context.Context, keyMap map[string]bool) (*AuthContext, error) {
	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := authHeaders[0]

	// Parse "Bearer {key}" format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format, expected 'Bearer {key}'")
	}

	apiKey := parts[1]

	// Validate API key
	if !keyMap[apiKey] {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	// Create auth context
	return &AuthContext{
		ServiceID: apiKey,
		AuthType:  AuthTypeAPIKey,
	}, nil
}

// authenticatedStream wraps a grpc.ServerStream with an authenticated context.
type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the authenticated context.
func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}
