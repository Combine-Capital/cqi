package errors

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryFunc is a function that handles a recovered panic.
// It receives the recovered value and returns an error.
type RecoveryFunc func(interface{}) error

// DefaultRecoveryFunc is the default recovery function that converts panics to PermanentErrors.
func DefaultRecoveryFunc(p interface{}) error {
	return NewPermanent(fmt.Sprintf("panic recovered: %v\nstack trace:\n%s", p, debug.Stack()), nil)
}

// RecoveryMiddleware is an HTTP middleware that recovers from panics and converts them to errors.
// It takes an optional recovery function; if nil, DefaultRecoveryFunc is used.
func RecoveryMiddleware(recoveryFunc RecoveryFunc) func(http.Handler) http.Handler {
	if recoveryFunc == nil {
		recoveryFunc = DefaultRecoveryFunc
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					err := recoveryFunc(p)
					WriteHTTPError(w, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that recovers from panics.
// It takes an optional recovery function; if nil, DefaultRecoveryFunc is used.
func UnaryServerInterceptor(recoveryFunc RecoveryFunc) grpc.UnaryServerInterceptor {
	if recoveryFunc == nil {
		recoveryFunc = DefaultRecoveryFunc
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if p := recover(); p != nil {
				recErr := recoveryFunc(p)
				err = status.Errorf(codes.Internal, "%v", recErr)
			}
		}()

		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that recovers from panics.
// It takes an optional recovery function; if nil, DefaultRecoveryFunc is used.
func StreamServerInterceptor(recoveryFunc RecoveryFunc) grpc.StreamServerInterceptor {
	if recoveryFunc == nil {
		recoveryFunc = DefaultRecoveryFunc
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				recErr := recoveryFunc(p)
				err = status.Errorf(codes.Internal, "%v", recErr)
			}
		}()

		return handler(srv, ss)
	}
}
