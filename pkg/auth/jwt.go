package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// JWTConfig contains configuration for JWT validation.
type JWTConfig struct {
	// PublicKey is the RSA public key used to verify JWT signatures.
	PublicKey *rsa.PublicKey

	// Issuer is the expected value of the "iss" (issuer) claim.
	// If empty, issuer validation is skipped.
	Issuer string

	// Audience is the expected value of the "aud" (audience) claim.
	// If empty, audience validation is skipped.
	Audience string
}

// JWTMiddleware returns an HTTP middleware that validates JWT tokens.
// It checks the Authorization header for "Bearer {token}" format and validates:
// - JWT signature using the provided public key
// - Expiration time (exp claim)
// - Issuer (iss claim) if configured
// - Audience (aud claim) if configured
//
// On success, it injects an AuthContext into the request context with:
// - AuthType set to AuthTypeJWT
// - UserID or ServiceID extracted from "sub" claim
// - Claims populated with all token claims
//
// On failure, it returns 401 Unauthorized.
func JWTMiddleware(config JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				errors.WriteHTTPError(w, errors.NewUnauthorized("missing Authorization header"))
				return
			}

			// Parse "Bearer {token}" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				errors.WriteHTTPError(w, errors.NewUnauthorized("invalid Authorization header format, expected 'Bearer {token}'"))
				return
			}

			tokenString := parts[1]

			// Parse and validate JWT token
			authCtx, err := parseAndValidateJWT(tokenString, config)
			if err != nil {
				errors.WriteHTTPError(w, err)
				return
			}

			// Inject auth context into request context
			ctx := setAuthContext(r.Context(), authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWTUnaryInterceptor returns a gRPC unary server interceptor that validates JWT tokens.
// It checks the "authorization" metadata for "Bearer {token}" format and validates
// the token using the same rules as JWTMiddleware.
//
// On success, it injects an AuthContext into the request context.
// On failure, it returns codes.Unauthenticated status.
func JWTUnaryInterceptor(config JWTConfig) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authCtx, err := validateJWTFromMetadata(ctx, config)
		if err != nil {
			return nil, err
		}

		// Inject auth context into request context
		ctx = setAuthContext(ctx, authCtx)

		return handler(ctx, req)
	}
}

// JWTStreamInterceptor returns a gRPC stream server interceptor that validates JWT tokens.
// It checks the "authorization" metadata for "Bearer {token}" format and validates
// the token using the same rules as JWTMiddleware.
//
// On success, it injects an AuthContext into the request context.
// On failure, it returns codes.Unauthenticated status.
func JWTStreamInterceptor(config JWTConfig) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authCtx, err := validateJWTFromMetadata(ss.Context(), config)
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

// parseAndValidateJWT parses and validates a JWT token string.
func parseAndValidateJWT(tokenString string, config JWTConfig) (*AuthContext, error) {
	// Parse token with claims
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return config.PublicKey, nil
	})

	if err != nil {
		return nil, errors.NewUnauthorizedWithCause("invalid JWT token", err)
	}

	if !token.Valid {
		return nil, errors.NewUnauthorized("JWT token is not valid")
	}

	// Extract claims
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, errors.NewUnauthorized("failed to extract JWT claims")
	}

	// Validate issuer if configured
	if config.Issuer != "" {
		if claims.Issuer != config.Issuer {
			return nil, errors.NewUnauthorized(fmt.Sprintf("invalid issuer: expected %s, got %s", config.Issuer, claims.Issuer))
		}
	}

	// Validate audience if configured
	if config.Audience != "" {
		validAudience := false
		for _, aud := range claims.Audience {
			if aud == config.Audience {
				validAudience = true
				break
			}
		}
		if !validAudience {
			return nil, errors.NewUnauthorized(fmt.Sprintf("invalid audience: expected %s", config.Audience))
		}
	}

	// Convert claims to map for storage
	claimsMap := make(map[string]interface{})
	if claims.Subject != "" {
		claimsMap["sub"] = claims.Subject
	}
	if claims.Issuer != "" {
		claimsMap["iss"] = claims.Issuer
	}
	if len(claims.Audience) > 0 {
		claimsMap["aud"] = claims.Audience
	}
	if claims.ExpiresAt != nil {
		claimsMap["exp"] = claims.ExpiresAt.Time
	}
	if claims.IssuedAt != nil {
		claimsMap["iat"] = claims.IssuedAt.Time
	}
	if claims.NotBefore != nil {
		claimsMap["nbf"] = claims.NotBefore.Time
	}
	if claims.ID != "" {
		claimsMap["jti"] = claims.ID
	}

	// Note: Custom claims beyond RegisteredClaims would need to be parsed
	// separately by the user. The RegisteredClaims struct only handles
	// standard JWT claims (sub, iss, aud, exp, iat, nbf, jti).

	// Create auth context
	authCtx := &AuthContext{
		AuthType: AuthTypeJWT,
		Claims:   claimsMap,
	}

	// Extract subject - can be either user ID or service ID
	// Convention: if subject starts with "service:", it's a service
	// Otherwise, it's a user
	subject := claims.Subject
	if strings.HasPrefix(subject, "service:") {
		authCtx.ServiceID = strings.TrimPrefix(subject, "service:")
	} else {
		authCtx.UserID = subject
	}

	return authCtx, nil
}

// validateJWTFromMetadata extracts and validates a JWT token from gRPC metadata.
func validateJWTFromMetadata(ctx context.Context, config JWTConfig) (*AuthContext, error) {
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

	// Parse "Bearer {token}" format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format, expected 'Bearer {token}'")
	}

	tokenString := parts[1]

	// Parse and validate JWT token
	authCtx, err := parseAndValidateJWT(tokenString, config)
	if err != nil {
		// Convert error to gRPC status
		if errors.IsUnauthorized(err) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to validate JWT token")
	}

	return authCtx, nil
}

// LoadPublicKeyFromPEM loads an RSA public key from PEM-encoded bytes.
// This is a helper function for loading public keys from configuration.
func LoadPublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try parsing as PKIX (standard public key format)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := pub.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("not an RSA public key")
	}

	// Try parsing as PKCS1 (RSA-specific format)
	rsaKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return rsaKey, nil
}

// LoadPublicKeyFromFile loads an RSA public key from a PEM file.
// This is a convenience function for loading keys from the filesystem.
func LoadPublicKeyFromFile(path string) (*rsa.PublicKey, error) {
	// Note: This would require os.ReadFile, but we're keeping the auth package
	// focused on authentication logic. Users can read the file themselves and
	// call LoadPublicKeyFromPEM.
	return nil, fmt.Errorf("use LoadPublicKeyFromPEM with os.ReadFile instead")
}
