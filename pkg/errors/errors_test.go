package errors

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
)

// TestErrorTypes verifies all error types are created correctly and implement error interface
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "PermanentError without cause",
			err:  NewPermanent("permanent failure", nil),
			want: "permanent failure",
		},
		{
			name: "PermanentError with cause",
			err:  NewPermanent("permanent failure", errors.New("root cause")),
			want: "permanent failure: root cause",
		},
		{
			name: "TemporaryError without cause",
			err:  NewTemporary("temporary failure", nil),
			want: "temporary failure",
		},
		{
			name: "TemporaryError with cause",
			err:  NewTemporary("temporary failure", errors.New("timeout")),
			want: "temporary failure: timeout",
		},
		{
			name: "NotFoundError",
			err:  NewNotFound("user", "123"),
			want: "user not found: 123",
		},
		{
			name: "NotFoundError with cause",
			err:  NewNotFoundWithCause("asset", "BTC", errors.New("db error")),
			want: "asset not found: BTC (db error)",
		},
		{
			name: "InvalidInputError",
			err:  NewInvalidInput("email", "must be valid email address"),
			want: "invalid input for email: must be valid email address",
		},
		{
			name: "InvalidInputError with cause",
			err:  NewInvalidInputWithCause("price", "must be positive", errors.New("validation failed")),
			want: "invalid input for price: must be positive (validation failed)",
		},
		{
			name: "UnauthorizedError",
			err:  NewUnauthorized("invalid API key"),
			want: "unauthorized: invalid API key",
		},
		{
			name: "UnauthorizedError with cause",
			err:  NewUnauthorizedWithCause("token expired", errors.New("jwt validation failed")),
			want: "unauthorized: token expired (jwt validation failed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorUnwrap verifies error unwrapping works correctly
func TestErrorUnwrap(t *testing.T) {
	rootErr := errors.New("root cause")

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "PermanentError unwraps",
			err:  NewPermanent("wrapper", rootErr),
			want: rootErr,
		},
		{
			name: "TemporaryError unwraps",
			err:  NewTemporary("wrapper", rootErr),
			want: rootErr,
		},
		{
			name: "NotFoundError unwraps",
			err:  NewNotFoundWithCause("user", "123", rootErr),
			want: rootErr,
		},
		{
			name: "InvalidInputError unwraps",
			err:  NewInvalidInputWithCause("field", "msg", rootErr),
			want: rootErr,
		},
		{
			name: "UnauthorizedError unwraps",
			err:  NewUnauthorizedWithCause("msg", rootErr),
			want: rootErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Unwrap(tt.err); got != tt.want {
				t.Errorf("Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTypeChecking verifies type checking functions work correctly
func TestTypeChecking(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isPerm   bool
		isTemp   bool
		isNotF   bool
		isInvIn  bool
		isUnauth bool
	}{
		{
			name:   "PermanentError",
			err:    NewPermanent("perm", nil),
			isPerm: true,
		},
		{
			name:   "TemporaryError",
			err:    NewTemporary("temp", nil),
			isTemp: true,
		},
		{
			name:   "NotFoundError",
			err:    NewNotFound("user", "123"),
			isNotF: true,
		},
		{
			name:    "InvalidInputError",
			err:     NewInvalidInput("email", "invalid"),
			isInvIn: true,
		},
		{
			name:     "UnauthorizedError",
			err:      NewUnauthorized("no access"),
			isUnauth: true,
		},
		{
			name: "standard error is none",
			err:  errors.New("standard"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.isPerm {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.isPerm)
			}
			if got := IsTemporary(tt.err); got != tt.isTemp {
				t.Errorf("IsTemporary() = %v, want %v", got, tt.isTemp)
			}
			if got := IsNotFound(tt.err); got != tt.isNotF {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.isNotF)
			}
			if got := IsInvalidInput(tt.err); got != tt.isInvIn {
				t.Errorf("IsInvalidInput() = %v, want %v", got, tt.isInvIn)
			}
			if got := IsUnauthorized(tt.err); got != tt.isUnauth {
				t.Errorf("IsUnauthorized() = %v, want %v", got, tt.isUnauth)
			}
		})
	}
}

// TestWrapping verifies error wrapping preserves types
func TestWrapping(t *testing.T) {
	tests := []struct {
		name       string
		original   error
		wrapMsg    string
		checkType  func(error) bool
		wantErrMsg string
	}{
		{
			name:       "wrap PermanentError",
			original:   NewPermanent("original", nil),
			wrapMsg:    "wrapped",
			checkType:  IsPermanent,
			wantErrMsg: "wrapped: original",
		},
		{
			name:       "wrap TemporaryError",
			original:   NewTemporary("original", nil),
			wrapMsg:    "wrapped",
			checkType:  IsTemporary,
			wantErrMsg: "wrapped: original",
		},
		{
			name:       "wrap NotFoundError",
			original:   NewNotFound("user", "123"),
			wrapMsg:    "wrapped",
			checkType:  IsNotFound,
			wantErrMsg: "user not found: 123 (user not found: 123)",
		},
		{
			name:       "wrap InvalidInputError",
			original:   NewInvalidInput("email", "invalid format"),
			wrapMsg:    "wrapped",
			checkType:  IsInvalidInput,
			wantErrMsg: "invalid input for email: wrapped (invalid input for email: invalid format)",
		},
		{
			name:       "wrap UnauthorizedError",
			original:   NewUnauthorized("no token"),
			wrapMsg:    "wrapped",
			checkType:  IsUnauthorized,
			wantErrMsg: "unauthorized: wrapped (unauthorized: no token)",
		},
		{
			name:       "wrap standard error becomes PermanentError",
			original:   errors.New("standard"),
			wrapMsg:    "wrapped",
			checkType:  IsPermanent,
			wantErrMsg: "wrapped: standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := Wrap(tt.original, tt.wrapMsg)
			if !tt.checkType(wrapped) {
				t.Errorf("Wrap() did not preserve error type")
			}
			if wrapped.Error() != tt.wantErrMsg {
				t.Errorf("Wrap() error message = %v, want %v", wrapped.Error(), tt.wantErrMsg)
			}
		})
	}
}

// TestWrapf verifies formatted wrapping works correctly
func TestWrapf(t *testing.T) {
	original := NewTemporary("timeout", nil)
	wrapped := Wrapf(original, "operation failed after %d attempts", 3)

	if !IsTemporary(wrapped) {
		t.Error("Wrapf() did not preserve error type")
	}

	want := "operation failed after 3 attempts: timeout"
	if got := wrapped.Error(); got != want {
		t.Errorf("Wrapf() = %v, want %v", got, want)
	}
}

// TestWrapNil verifies wrapping nil returns nil
func TestWrapNil(t *testing.T) {
	if got := Wrap(nil, "message"); got != nil {
		t.Errorf("Wrap(nil) = %v, want nil", got)
	}
	if got := Wrapf(nil, "message %s", "arg"); got != nil {
		t.Errorf("Wrapf(nil) = %v, want nil", got)
	}
}

// TestHTTPStatusCode verifies HTTP status code mapping
func TestHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: http.StatusOK,
		},
		{
			name: "NotFoundError",
			err:  NewNotFound("user", "123"),
			want: http.StatusNotFound,
		},
		{
			name: "InvalidInputError",
			err:  NewInvalidInput("email", "invalid"),
			want: http.StatusBadRequest,
		},
		{
			name: "UnauthorizedError",
			err:  NewUnauthorized("no token"),
			want: http.StatusUnauthorized,
		},
		{
			name: "TemporaryError",
			err:  NewTemporary("timeout", nil),
			want: http.StatusServiceUnavailable,
		},
		{
			name: "PermanentError",
			err:  NewPermanent("config error", nil),
			want: http.StatusInternalServerError,
		},
		{
			name: "unknown error",
			err:  errors.New("unknown"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTTPStatusCode(tt.err); got != tt.want {
				t.Errorf("HTTPStatusCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWriteHTTPError verifies HTTP error writing
func TestWriteHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "NotFoundError",
			err:        NewNotFound("user", "123"),
			wantStatus: http.StatusNotFound,
			wantBody:   "user not found: 123\n",
		},
		{
			name:       "InvalidInputError",
			err:        NewInvalidInput("email", "invalid format"),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid input for email: invalid format\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteHTTPError(w, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("WriteHTTPError() status = %v, want %v", w.Code, tt.wantStatus)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("WriteHTTPError() body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

// TestGRPCStatus verifies gRPC status code mapping
func TestGRPCStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "nil error",
			err:  nil,
			want: codes.OK,
		},
		{
			name: "NotFoundError",
			err:  NewNotFound("user", "123"),
			want: codes.NotFound,
		},
		{
			name: "InvalidInputError",
			err:  NewInvalidInput("email", "invalid"),
			want: codes.InvalidArgument,
		},
		{
			name: "UnauthorizedError",
			err:  NewUnauthorized("no token"),
			want: codes.Unauthenticated,
		},
		{
			name: "TemporaryError",
			err:  NewTemporary("timeout", nil),
			want: codes.Unavailable,
		},
		{
			name: "PermanentError",
			err:  NewPermanent("config error", nil),
			want: codes.Internal,
		},
		{
			name: "unknown error",
			err:  errors.New("unknown"),
			want: codes.Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := GRPCStatus(tt.err)
			if got := status.Code(); got != tt.want {
				t.Errorf("GRPCStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestToGRPCError verifies conversion to gRPC error
func TestToGRPCError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "nil error",
			err:  nil,
			want: codes.OK,
		},
		{
			name: "NotFoundError",
			err:  NewNotFound("user", "123"),
			want: codes.NotFound,
		},
		{
			name: "TemporaryError",
			err:  NewTemporary("timeout", nil),
			want: codes.Unavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := ToGRPCError(tt.err)
			if tt.err == nil {
				if grpcErr != nil {
					t.Errorf("ToGRPCError(nil) = %v, want nil", grpcErr)
				}
				return
			}

			status := GRPCStatus(tt.err)
			if status.Code() != tt.want {
				t.Errorf("ToGRPCError() code = %v, want %v", status.Code(), tt.want)
			}
		})
	}
}

// TestRecoveryMiddleware verifies panic recovery in HTTP handlers
func TestRecoveryMiddleware(t *testing.T) {
	handler := RecoveryMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Should not panic
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("RecoveryMiddleware status = %v, want %v", w.Code, http.StatusInternalServerError)
	}

	if body := w.Body.String(); body == "" {
		t.Error("RecoveryMiddleware should write error message")
	}
}

// TestRecoveryMiddlewareNoPanic verifies normal operation without panic
func TestRecoveryMiddlewareNoPanic(t *testing.T) {
	handler := RecoveryMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "success")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("RecoveryMiddleware status = %v, want %v", w.Code, http.StatusOK)
	}

	if body := w.Body.String(); body != "success" {
		t.Errorf("RecoveryMiddleware body = %q, want %q", body, "success")
	}
}

// TestCustomRecoveryFunc verifies custom recovery function
func TestCustomRecoveryFunc(t *testing.T) {
	customFunc := func(p interface{}) error {
		return NewTemporary(fmt.Sprintf("custom: %v", p), nil)
	}

	handler := RecoveryMiddleware(customFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Custom recovery status = %v, want %v", w.Code, http.StatusServiceUnavailable)
	}
}

// TestNotFoundErrorFields verifies NotFoundError field accessors
func TestNotFoundErrorFields(t *testing.T) {
	err := NewNotFound("user", "123").(*NotFoundError)

	if got := err.Resource(); got != "user" {
		t.Errorf("Resource() = %v, want %v", got, "user")
	}
	if got := err.ID(); got != "123" {
		t.Errorf("ID() = %v, want %v", got, "123")
	}
}

// TestInvalidInputErrorFields verifies InvalidInputError field accessors
func TestInvalidInputErrorFields(t *testing.T) {
	err := NewInvalidInput("email", "invalid format").(*InvalidInputError)

	if got := err.Field(); got != "email" {
		t.Errorf("Field() = %v, want %v", got, "email")
	}
	if got := err.Message(); got != "invalid format" {
		t.Errorf("Message() = %v, want %v", got, "invalid format")
	}
}
