package errors

import (
	"errors"
)

// As is a re-export of errors.As for convenient access in error handling code.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Is is a re-export of errors.Is for convenient access in error handling code.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// IsPermanent checks if an error is or wraps a PermanentError.
func IsPermanent(err error) bool {
	var perr *PermanentError
	return errors.As(err, &perr)
}

// IsTemporary checks if an error is or wraps a TemporaryError.
func IsTemporary(err error) bool {
	var terr *TemporaryError
	return errors.As(err, &terr)
}

// IsNotFound checks if an error is or wraps a NotFoundError.
func IsNotFound(err error) bool {
	var nferr *NotFoundError
	return errors.As(err, &nferr)
}

// IsInvalidInput checks if an error is or wraps an InvalidInputError.
func IsInvalidInput(err error) bool {
	var iierr *InvalidInputError
	return errors.As(err, &iierr)
}

// IsUnauthorized checks if an error is or wraps an UnauthorizedError.
func IsUnauthorized(err error) bool {
	var uaerr *UnauthorizedError
	return errors.As(err, &uaerr)
}
