// Package auth provides HTTP and gRPC authentication middleware with API key
// and JWT validation. It supports extracting user/service identity from requests
// and propagating authentication context through context.Context.
//
// Example usage with API key:
//
//	// HTTP middleware
//	validKeys := []string{"key1", "key2"}
//	http.Handle("/api/", auth.APIKeyMiddleware(validKeys)(handler))
//
//	// gRPC interceptor
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(auth.APIKeyUnaryInterceptor(validKeys)),
//	)
//
// Example usage with JWT:
//
//	publicKey, _ := auth.LoadPublicKeyFromPEM(pemBytes)
//	middleware := auth.JWTMiddleware(publicKey, "issuer", "audience")
//	http.Handle("/api/", middleware(handler))
package auth

// AuthType represents the type of authentication used.
type AuthType string

const (
	// AuthTypeAPIKey represents API key authentication.
	AuthTypeAPIKey AuthType = "API_KEY"

	// AuthTypeJWT represents JWT token authentication.
	AuthTypeJWT AuthType = "JWT"
)

// AuthContext contains authentication information extracted from a request.
// It is stored in context.Context and can be retrieved using GetAuthContext.
type AuthContext struct {
	// UserID is the unique identifier of the authenticated user.
	// Empty for service-to-service authentication.
	UserID string

	// ServiceID is the unique identifier of the authenticated service.
	// Empty for user authentication.
	ServiceID string

	// AuthType indicates the authentication method used (API_KEY or JWT).
	AuthType AuthType

	// Claims contains additional claims from JWT tokens.
	// For API key authentication, this will be nil or empty.
	Claims map[string]interface{}
}

// IsUser returns true if this is a user authentication context (UserID is set).
func (a *AuthContext) IsUser() bool {
	return a.UserID != ""
}

// IsService returns true if this is a service authentication context (ServiceID is set).
func (a *AuthContext) IsService() bool {
	return a.ServiceID != ""
}

// GetClaim returns a claim value from the Claims map.
// Returns nil if the claim doesn't exist.
func (a *AuthContext) GetClaim(key string) interface{} {
	if a.Claims == nil {
		return nil
	}
	return a.Claims[key]
}

// GetClaimString returns a claim value as a string.
// Returns empty string if the claim doesn't exist or is not a string.
func (a *AuthContext) GetClaimString(key string) string {
	val := a.GetClaim(key)
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}
