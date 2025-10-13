package errors

import (
	"net/http"
)

// HTTPStatusCode returns the appropriate HTTP status code for the given error.
// It maps error types to standard HTTP status codes:
//   - NotFoundError -> 404 Not Found
//   - InvalidInputError -> 400 Bad Request
//   - UnauthorizedError -> 401 Unauthorized
//   - TemporaryError -> 503 Service Unavailable
//   - PermanentError -> 500 Internal Server Error
//   - Unknown errors -> 500 Internal Server Error
func HTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch {
	case IsNotFound(err):
		return http.StatusNotFound // 404
	case IsInvalidInput(err):
		return http.StatusBadRequest // 400
	case IsUnauthorized(err):
		return http.StatusUnauthorized // 401
	case IsTemporary(err):
		return http.StatusServiceUnavailable // 503
	case IsPermanent(err):
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // 500
	}
}

// WriteHTTPError writes an error response to an HTTP response writer.
// It automatically determines the status code based on the error type
// and writes a JSON error response.
func WriteHTTPError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	statusCode := HTTPStatusCode(err)
	http.Error(w, err.Error(), statusCode)
}
