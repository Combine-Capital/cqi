package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestAuthContext verifies AuthContext helper methods
func TestAuthContext(t *testing.T) {
	tests := []struct {
		name     string
		authCtx  *AuthContext
		wantUser bool
		wantSvc  bool
		claim    string
		claimVal interface{}
		claimStr string
	}{
		{
			name: "user authentication",
			authCtx: &AuthContext{
				UserID:   "user123",
				AuthType: AuthTypeJWT,
				Claims:   map[string]interface{}{"role": "admin"},
			},
			wantUser: true,
			wantSvc:  false,
			claim:    "role",
			claimVal: "admin",
			claimStr: "admin",
		},
		{
			name: "service authentication",
			authCtx: &AuthContext{
				ServiceID: "service456",
				AuthType:  AuthTypeAPIKey,
			},
			wantUser: false,
			wantSvc:  true,
			claim:    "missing",
			claimVal: nil,
			claimStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.authCtx.IsUser(); got != tt.wantUser {
				t.Errorf("IsUser() = %v, want %v", got, tt.wantUser)
			}
			if got := tt.authCtx.IsService(); got != tt.wantSvc {
				t.Errorf("IsService() = %v, want %v", got, tt.wantSvc)
			}
			if got := tt.authCtx.GetClaim(tt.claim); got != tt.claimVal {
				t.Errorf("GetClaim(%s) = %v, want %v", tt.claim, got, tt.claimVal)
			}
			if got := tt.authCtx.GetClaimString(tt.claim); got != tt.claimStr {
				t.Errorf("GetClaimString(%s) = %v, want %v", tt.claim, got, tt.claimStr)
			}
		})
	}
}

// TestGetAuthContext verifies context storage and retrieval
func TestGetAuthContext(t *testing.T) {
	authCtx := &AuthContext{
		UserID:   "user123",
		AuthType: AuthTypeJWT,
	}

	// Test with auth context
	ctx := setAuthContext(context.Background(), authCtx)
	retrieved, err := GetAuthContext(ctx)
	if err != nil {
		t.Errorf("GetAuthContext() error = %v, want nil", err)
	}
	if retrieved.UserID != authCtx.UserID {
		t.Errorf("GetAuthContext() UserID = %v, want %v", retrieved.UserID, authCtx.UserID)
	}

	// Test without auth context
	emptyCtx := context.Background()
	_, err = GetAuthContext(emptyCtx)
	if err == nil {
		t.Error("GetAuthContext() with empty context should return error")
	}
	if !errors.IsUnauthorized(err) {
		t.Errorf("GetAuthContext() error should be Unauthorized, got %v", err)
	}
}

// TestMustGetAuthContext verifies panic on missing context
func TestMustGetAuthContext(t *testing.T) {
	authCtx := &AuthContext{UserID: "user123"}
	ctx := setAuthContext(context.Background(), authCtx)

	// Should not panic
	retrieved := MustGetAuthContext(ctx)
	if retrieved.UserID != authCtx.UserID {
		t.Errorf("MustGetAuthContext() UserID = %v, want %v", retrieved.UserID, authCtx.UserID)
	}

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGetAuthContext() should panic with empty context")
		}
	}()
	MustGetAuthContext(context.Background())
}

// TestAPIKeyMiddleware verifies HTTP API key authentication
func TestAPIKeyMiddleware(t *testing.T) {
	validKeys := []string{"key1", "key2", "key3"}
	middleware := APIKeyMiddleware(validKeys)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, err := GetAuthContext(r.Context())
		if err != nil {
			t.Errorf("Handler: GetAuthContext() error = %v", err)
			return
		}
		if authCtx.AuthType != AuthTypeAPIKey {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeAPIKey)
		}
		if authCtx.ServiceID == "" {
			t.Error("Handler: ServiceID should not be empty")
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "valid API key",
			authHeader:     "Bearer key1",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "another valid API key",
			authHeader:     "Bearer key3",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "invalid API key",
			authHeader:     "Bearer invalid",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "missing Authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "malformed Authorization header",
			authHeader:     "key1",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "wrong scheme",
			authHeader:     "Basic key1",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("StatusCode = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

// TestAPIKeyUnaryInterceptor verifies gRPC API key authentication
func TestAPIKeyUnaryInterceptor(t *testing.T) {
	validKeys := []string{"key1", "key2"}
	interceptor := APIKeyUnaryInterceptor(validKeys)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		authCtx, err := GetAuthContext(ctx)
		if err != nil {
			return nil, err
		}
		if authCtx.AuthType != AuthTypeAPIKey {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeAPIKey)
		}
		return "success", nil
	}

	tests := []struct {
		name       string
		authHeader string
		wantErr    bool
		wantCode   codes.Code
	}{
		{
			name:       "valid API key",
			authHeader: "Bearer key1",
			wantErr:    false,
		},
		{
			name:       "invalid API key",
			authHeader: "Bearer invalid",
			wantErr:    true,
			wantCode:   codes.Unauthenticated,
		},
		{
			name:     "missing metadata",
			wantErr:  true,
			wantCode: codes.Unauthenticated,
		},
		{
			name:       "malformed header",
			authHeader: "key1",
			wantErr:    true,
			wantCode:   codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.authHeader != "" {
				md := metadata.New(map[string]string{
					"authorization": tt.authHeader,
				})
				ctx = metadata.NewIncomingContext(context.Background(), md)
			} else if tt.name != "missing metadata" {
				ctx = metadata.NewIncomingContext(context.Background(), metadata.New(nil))
			} else {
				ctx = context.Background()
			}

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("error is not a gRPC status: %v", err)
				} else if st.Code() != tt.wantCode {
					t.Errorf("error code = %v, want %v", st.Code(), tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestJWTMiddleware verifies HTTP JWT authentication
func TestJWTMiddleware(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	publicKey := &privateKey.PublicKey

	config := JWTConfig{
		PublicKey: publicKey,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	middleware := JWTMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, err := GetAuthContext(r.Context())
		if err != nil {
			t.Errorf("Handler: GetAuthContext() error = %v", err)
			return
		}
		if authCtx.AuthType != AuthTypeJWT {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeJWT)
		}
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware(handler)

	// Helper to create JWT tokens
	createToken := func(claims jwt.Claims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	tests := []struct {
		name           string
		token          string
		wantStatusCode int
	}{
		{
			name: "valid JWT with user subject",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantStatusCode: http.StatusOK,
		},
		{
			name: "valid JWT with service subject",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "service:my-service",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantStatusCode: http.StatusOK,
		},
		{
			name: "expired JWT",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			}),
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "wrong issuer",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "wrong-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "wrong audience",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"wrong-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "invalid token format",
			token:          "not.a.valid.jwt",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "missing authorization header",
			token:          "",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("StatusCode = %v, want %v (body: %s)", rec.Code, tt.wantStatusCode, rec.Body.String())
			}
		})
	}
}

// TestJWTUnaryInterceptor verifies gRPC JWT authentication
func TestJWTUnaryInterceptor(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	publicKey := &privateKey.PublicKey

	config := JWTConfig{
		PublicKey: publicKey,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	interceptor := JWTUnaryInterceptor(config)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		authCtx, err := GetAuthContext(ctx)
		if err != nil {
			return nil, err
		}
		if authCtx.AuthType != AuthTypeJWT {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeJWT)
		}
		return "success", nil
	}

	// Helper to create JWT tokens
	createToken := func(claims jwt.Claims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	tests := []struct {
		name     string
		token    string
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "valid JWT",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantErr: false,
		},
		{
			name: "expired JWT",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			}),
			wantErr:  true,
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "invalid token",
			token:    "invalid.token",
			wantErr:  true,
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := metadata.New(map[string]string{
				"authorization": "Bearer " + tt.token,
			})
			ctx := metadata.NewIncomingContext(context.Background(), md)

			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("error is not a gRPC status: %v", err)
				} else if st.Code() != tt.wantCode {
					t.Errorf("error code = %v, want %v", st.Code(), tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestLoadPublicKeyFromPEM verifies PEM parsing
func TestLoadPublicKeyFromPEM(t *testing.T) {
	// Generate a test key and encode it to PEM format
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	// Encode public key to PKIX format
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	// Create PEM block
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	// Test valid PEM
	publicKey, err := LoadPublicKeyFromPEM(pubKeyPEM)
	if err != nil {
		t.Errorf("LoadPublicKeyFromPEM() error = %v, want nil", err)
	}
	if publicKey == nil {
		t.Error("LoadPublicKeyFromPEM() returned nil public key")
	}

	// Test with nil input
	_, err = LoadPublicKeyFromPEM(nil)
	if err == nil {
		t.Error("LoadPublicKeyFromPEM(nil) should return error")
	}

	// Test with invalid PEM data
	invalidPEM := []byte("not a PEM file")
	_, err = LoadPublicKeyFromPEM(invalidPEM)
	if err == nil {
		t.Error("LoadPublicKeyFromPEM(invalidPEM) should return error")
	}

	// Test with non-RSA key (simulate by using wrong type)
	invalidKeyPEM := []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
-----END PUBLIC KEY-----`)
	_, err = LoadPublicKeyFromPEM(invalidKeyPEM)
	if err == nil {
		t.Error("LoadPublicKeyFromPEM(non-RSA) should return error")
	}
}

// TestJWTConfigValidation verifies JWT config validation
func TestJWTConfigValidation(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	publicKey := &privateKey.PublicKey

	// Test with no issuer/audience validation
	config := JWTConfig{
		PublicKey: publicKey,
		// Issuer and Audience left empty - should skip validation
	}

	createToken := func(claims jwt.Claims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	// Token with no issuer/audience should still be valid
	tokenString := createToken(jwt.RegisteredClaims{
		Subject:   "user123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	authCtx, err := parseAndValidateJWT(tokenString, config)
	if err != nil {
		t.Errorf("parseAndValidateJWT() error = %v, want nil", err)
	}
	if authCtx.UserID != "user123" {
		t.Errorf("UserID = %v, want user123", authCtx.UserID)
	}
}

// mockServerStream is a mock implementation of grpc.ServerStream for testing
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

// TestAPIKeyStreamInterceptor verifies gRPC stream API key authentication
func TestAPIKeyStreamInterceptor(t *testing.T) {
	validKeys := []string{"key1", "key2"}
	interceptor := APIKeyStreamInterceptor(validKeys)

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		authCtx, err := GetAuthContext(stream.Context())
		if err != nil {
			return err
		}
		if authCtx.AuthType != AuthTypeAPIKey {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeAPIKey)
		}
		return nil
	}

	tests := []struct {
		name       string
		authHeader string
		wantErr    bool
		wantCode   codes.Code
	}{
		{
			name:       "valid API key",
			authHeader: "Bearer key1",
			wantErr:    false,
		},
		{
			name:       "invalid API key",
			authHeader: "Bearer invalid",
			wantErr:    true,
			wantCode:   codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.authHeader != "" {
				md := metadata.New(map[string]string{
					"authorization": tt.authHeader,
				})
				ctx = metadata.NewIncomingContext(context.Background(), md)
			} else {
				ctx = context.Background()
			}

			stream := &mockServerStream{ctx: ctx}
			err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("error is not a gRPC status: %v", err)
				} else if st.Code() != tt.wantCode {
					t.Errorf("error code = %v, want %v", st.Code(), tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestJWTStreamInterceptor verifies gRPC stream JWT authentication
func TestJWTStreamInterceptor(t *testing.T) {
	// Generate test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	publicKey := &privateKey.PublicKey

	config := JWTConfig{
		PublicKey: publicKey,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	interceptor := JWTStreamInterceptor(config)

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		authCtx, err := GetAuthContext(stream.Context())
		if err != nil {
			return err
		}
		if authCtx.AuthType != AuthTypeJWT {
			t.Errorf("Handler: AuthType = %v, want %v", authCtx.AuthType, AuthTypeJWT)
		}
		return nil
	}

	// Helper to create JWT tokens
	createToken := func(claims jwt.Claims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	tests := []struct {
		name     string
		token    string
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "valid JWT",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			}),
			wantErr: false,
		},
		{
			name: "expired JWT",
			token: createToken(jwt.RegisteredClaims{
				Subject:   "user123",
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			}),
			wantErr:  true,
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := metadata.New(map[string]string{
				"authorization": "Bearer " + tt.token,
			})
			ctx := metadata.NewIncomingContext(context.Background(), md)

			stream := &mockServerStream{ctx: ctx}
			err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("error is not a gRPC status: %v", err)
				} else if st.Code() != tt.wantCode {
					t.Errorf("error code = %v, want %v", st.Code(), tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
