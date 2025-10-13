// Package errors provides structured error types for the CQI infrastructure library.
// It defines error categories (Permanent, Temporary, NotFound, InvalidInput, Unauthorized)
// that enable consistent error handling across all infrastructure components.
//
// Example usage:
//
//	if err := db.Query(ctx, sql); err != nil {
//	    return errors.NewTemporary("database connection lost", err)
//	}
//
//	if user == nil {
//	    return errors.NewNotFound("user", userID)
//	}
package errors

import (
	"fmt"
)

// PermanentError represents an error that won't succeed even if retried.
// Examples: invalid configuration, programming errors, data corruption.
type PermanentError struct {
	msg   string
	cause error
}

// NewPermanent creates a new permanent error with the given message and optional cause.
func NewPermanent(msg string, cause error) error {
	return &PermanentError{msg: msg, cause: cause}
}

func (e *PermanentError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

func (e *PermanentError) Unwrap() error {
	return e.cause
}

// TemporaryError represents an error that might succeed if retried.
// Examples: network timeouts, temporary service unavailability, rate limiting.
type TemporaryError struct {
	msg   string
	cause error
}

// NewTemporary creates a new temporary error with the given message and optional cause.
func NewTemporary(msg string, cause error) error {
	return &TemporaryError{msg: msg, cause: cause}
}

func (e *TemporaryError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

func (e *TemporaryError) Unwrap() error {
	return e.cause
}

// NotFoundError represents an error when a requested resource doesn't exist.
// Examples: user not found, asset not found, record not in database.
type NotFoundError struct {
	resource string
	id       string
	cause    error
}

// NewNotFound creates a new not found error for the given resource and ID.
func NewNotFound(resource, id string) error {
	return &NotFoundError{resource: resource, id: id}
}

// NewNotFoundWithCause creates a new not found error with an underlying cause.
func NewNotFoundWithCause(resource, id string, cause error) error {
	return &NotFoundError{resource: resource, id: id, cause: cause}
}

func (e *NotFoundError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s not found: %s (%v)", e.resource, e.id, e.cause)
	}
	return fmt.Sprintf("%s not found: %s", e.resource, e.id)
}

func (e *NotFoundError) Unwrap() error {
	return e.cause
}

// Resource returns the type of resource that wasn't found.
func (e *NotFoundError) Resource() string {
	return e.resource
}

// ID returns the identifier of the resource that wasn't found.
func (e *NotFoundError) ID() string {
	return e.id
}

// InvalidInputError represents an error due to invalid user input.
// Examples: validation failures, malformed requests, missing required fields.
type InvalidInputError struct {
	field string
	msg   string
	cause error
}

// NewInvalidInput creates a new invalid input error for the given field and message.
func NewInvalidInput(field, msg string) error {
	return &InvalidInputError{field: field, msg: msg}
}

// NewInvalidInputWithCause creates a new invalid input error with an underlying cause.
func NewInvalidInputWithCause(field, msg string, cause error) error {
	return &InvalidInputError{field: field, msg: msg, cause: cause}
}

func (e *InvalidInputError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("invalid input for %s: %s (%v)", e.field, e.msg, e.cause)
	}
	return fmt.Sprintf("invalid input for %s: %s", e.field, e.msg)
}

func (e *InvalidInputError) Unwrap() error {
	return e.cause
}

// Field returns the field name that had invalid input.
func (e *InvalidInputError) Field() string {
	return e.field
}

// Message returns the validation error message.
func (e *InvalidInputError) Message() string {
	return e.msg
}

// UnauthorizedError represents an authentication or authorization failure.
// Examples: invalid API key, expired JWT token, insufficient permissions.
type UnauthorizedError struct {
	msg   string
	cause error
}

// NewUnauthorized creates a new unauthorized error with the given message.
func NewUnauthorized(msg string) error {
	return &UnauthorizedError{msg: msg}
}

// NewUnauthorizedWithCause creates a new unauthorized error with an underlying cause.
func NewUnauthorizedWithCause(msg string, cause error) error {
	return &UnauthorizedError{msg: msg, cause: cause}
}

func (e *UnauthorizedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("unauthorized: %s (%v)", e.msg, e.cause)
	}
	return fmt.Sprintf("unauthorized: %s", e.msg)
}

func (e *UnauthorizedError) Unwrap() error {
	return e.cause
}
