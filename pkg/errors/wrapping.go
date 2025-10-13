package errors

import (
	"fmt"
)

// Wrap wraps an error with additional context while preserving the original error type.
// If err is already a typed error (Permanent, Temporary, etc.), it wraps it with the same type.
// Otherwise, it returns a PermanentError.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}

	// Preserve the original error type when wrapping
	switch {
	case IsPermanent(err):
		return NewPermanent(msg, err)
	case IsTemporary(err):
		return NewTemporary(msg, err)
	case IsNotFound(err):
		// Extract resource and ID from original NotFoundError if possible
		var nfe *NotFoundError
		if As(err, &nfe) {
			return NewNotFoundWithCause(nfe.resource, nfe.id, err)
		}
		return NewPermanent(msg, err)
	case IsInvalidInput(err):
		// Extract field from original InvalidInputError if possible
		var iie *InvalidInputError
		if As(err, &iie) {
			return NewInvalidInputWithCause(iie.field, msg, err)
		}
		return NewInvalidInput("", msg)
	case IsUnauthorized(err):
		return NewUnauthorizedWithCause(msg, err)
	default:
		return NewPermanent(msg, err)
	}
}

// Wrapf wraps an error with a formatted message while preserving the original error type.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return Wrap(err, fmt.Sprintf(format, args...))
}
