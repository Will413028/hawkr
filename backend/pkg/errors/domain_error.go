package errors

import (
	stderrors "errors"
	"fmt"
)

// ErrorCode represents a domain-specific error code.
// These codes are mapped to gRPC status codes by the error interceptor.
type ErrorCode int

const (
	// Generic error codes (HTTP-like)
	CodeUnknown      ErrorCode = 500
	CodeNotFound     ErrorCode = 404
	CodeConflict     ErrorCode = 409
	CodeInvalidInput ErrorCode = 400
	CodeUnauthorized ErrorCode = 401
	CodeForbidden    ErrorCode = 403
	CodeUnavailable  ErrorCode = 503

	// Generic precondition failure (system state prevents the operation)
	CodePreconditionFailed ErrorCode = 428

	// Seller-specific error codes (1200-1299)
	CodeInvalidTransition ErrorCode = 1200
)

// DomainCoder is the interface that domain-layer errors implement.
// The error interceptor uses this to map errors to gRPC status codes
// without requiring a concrete type dependency (duck typing).
type DomainCoder interface {
	DomainCode() int
	DomainMessage() string
}

// DomainError represents a business domain error with a code and message.
type DomainError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying cause for errors.Is/errors.As support.
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// DomainCode returns the error code for the interceptor to use.
func (e *DomainError) DomainCode() int {
	return int(e.Code)
}

// DomainMessage returns the user-facing message (without internal cause).
func (e *DomainError) DomainMessage() string {
	return e.Message
}

// New creates a new DomainError with the given code and message.
func New(code ErrorCode, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}

// Wrap creates a new DomainError wrapping an existing error.
func Wrap(code ErrorCode, message string, cause error) *DomainError {
	return &DomainError{Code: code, Message: message, Cause: cause}
}

// --- Convenience constructors ---

// NewNotFound creates a NotFound error.
func NewNotFound(message string) *DomainError {
	return &DomainError{Code: CodeNotFound, Message: message}
}

// NewConflict creates a Conflict error.
func NewConflict(message string) *DomainError {
	return &DomainError{Code: CodeConflict, Message: message}
}

// NewInvalidInput creates an InvalidInput error.
func NewInvalidInput(message string) *DomainError {
	return &DomainError{Code: CodeInvalidInput, Message: message}
}

// NewUnauthorized creates an Unauthorized error.
func NewUnauthorized(message string) *DomainError {
	return &DomainError{Code: CodeUnauthorized, Message: message}
}

// NewForbidden creates a Forbidden error.
func NewForbidden(message string) *DomainError {
	return &DomainError{Code: CodeForbidden, Message: message}
}

// NewInvalidTransition creates an error for invalid state transitions.
func NewInvalidTransition(message string) *DomainError {
	return &DomainError{Code: CodeInvalidTransition, Message: message}
}

// NewPreconditionFailed creates an error when a precondition is not met (e.g., required fields missing).
func NewPreconditionFailed(message string) *DomainError {
	return &DomainError{Code: CodePreconditionFailed, Message: message}
}

// NewUnavailable creates an error when an upstream service or dependency is temporarily unavailable
// (e.g., external API network error, third-party system error). Clients can retry.
func NewUnavailable(message string) *DomainError {
	return &DomainError{Code: CodeUnavailable, Message: message}
}

// NewBusiness creates a fine-grained business error with a code from bizcode package.
func NewBusiness(code int32, message string) *DomainError {
	return &DomainError{Code: ErrorCode(code), Message: message}
}

// IsDomainError checks if the given error implements DomainCoder.
func IsDomainError(err error) bool {
	var de *DomainError
	if stderrors.As(err, &de) {
		return true
	}
	var dc DomainCoder
	return stderrors.As(err, &dc)
}

// IsNotFound checks if the given error is a NotFound DomainError.
func IsNotFound(err error) bool {
	return GetCode(err) == CodeNotFound
}

// IsCode checks if the given error has a specific error code.
func IsCode(err error, code int32) bool {
	return GetCode(err) == ErrorCode(code)
}

// GetCode extracts the error code from a DomainError or DomainCoder, or returns CodeUnknown.
func GetCode(err error) ErrorCode {
	var de *DomainError
	if stderrors.As(err, &de) {
		return de.Code
	}
	var dc DomainCoder
	if stderrors.As(err, &dc) {
		return ErrorCode(dc.DomainCode())
	}
	return CodeUnknown
}
