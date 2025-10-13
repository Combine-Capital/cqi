package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCStatus converts an error to a gRPC status.
// It maps error types to appropriate gRPC status codes:
//   - NotFoundError -> codes.NotFound
//   - InvalidInputError -> codes.InvalidArgument
//   - UnauthorizedError -> codes.Unauthenticated
//   - TemporaryError -> codes.Unavailable
//   - PermanentError -> codes.Internal
//   - Unknown errors -> codes.Unknown
func GRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	var code codes.Code
	switch {
	case IsNotFound(err):
		code = codes.NotFound
	case IsInvalidInput(err):
		code = codes.InvalidArgument
	case IsUnauthorized(err):
		code = codes.Unauthenticated
	case IsTemporary(err):
		code = codes.Unavailable
	case IsPermanent(err):
		code = codes.Internal
	default:
		code = codes.Unknown
	}

	return status.New(code, err.Error())
}

// ToGRPCError converts an error to a gRPC error that can be returned from a gRPC handler.
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	return GRPCStatus(err).Err()
}
