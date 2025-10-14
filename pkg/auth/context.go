package auth

import (
	"context"

	"github.com/Combine-Capital/cqi/pkg/errors"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey int

const (
	authContextKey contextKey = iota
)

// GetAuthContext retrieves the AuthContext from the context.Context.
// Returns an error if no authentication context is found.
func GetAuthContext(ctx context.Context) (*AuthContext, error) {
	auth, ok := ctx.Value(authContextKey).(*AuthContext)
	if !ok || auth == nil {
		return nil, errors.NewUnauthorized("no authentication context found")
	}
	return auth, nil
}

// MustGetAuthContext retrieves the AuthContext from the context.Context.
// Panics if no authentication context is found. Use this only when you are
// certain that the context has been authenticated by middleware.
func MustGetAuthContext(ctx context.Context) *AuthContext {
	auth, err := GetAuthContext(ctx)
	if err != nil {
		panic(err)
	}
	return auth
}

// setAuthContext stores the AuthContext in the context.Context.
// This is an internal function used by authentication middleware.
func setAuthContext(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, auth)
}
