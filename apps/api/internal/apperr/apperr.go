// Package apperr defines transport-independent application errors.
//
// Domain services return *Error values; the HTTP layer maps the Kind to a
// status code. Messages are safe to show to end users and must never contain
// personal data or internal details.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an application error.
type Kind int

const (
	KindInternal Kind = iota
	KindBadRequest
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindTooManyRequests
	KindUnavailable
)

// Error is an application error with a stable machine-readable code.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	// Fields carries per-field validation messages.
	Fields map[string]string
	// Details carries additional non-sensitive structured context, e.g. the
	// remaining quantity on an over-allocation conflict.
	Details map[string]any
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// WithDetail returns the error with an added detail entry.
func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[key] = value
	return e
}

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// IsKind reports whether err is an application error of the given kind.
func IsKind(err error, k Kind) bool {
	ae, ok := As(err)
	return ok && ae.Kind == k
}

func BadRequest(code, msg string) *Error {
	return &Error{Kind: KindBadRequest, Code: code, Message: msg}
}

func Validation(fields map[string]string) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: "Some fields are invalid.", Fields: fields}
}

func ValidationField(field, msg string) *Error {
	return Validation(map[string]string{field: msg})
}

func Unauthorized(msg string) *Error {
	return &Error{Kind: KindUnauthorized, Code: "unauthorized", Message: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Kind: KindForbidden, Code: "forbidden", Message: msg}
}

func ForbiddenCode(code, msg string) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: msg}
}

func NotFound(msg string) *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: msg}
}

func Conflict(code, msg string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: msg}
}

func TooManyRequests(msg string) *Error {
	return &Error{Kind: KindTooManyRequests, Code: "rate_limited", Message: msg}
}

func Unavailable(code, msg string) *Error {
	return &Error{Kind: KindUnavailable, Code: code, Message: msg}
}

// Internal wraps an unexpected error. The cause is logged, never returned to
// clients.
func Internal(cause error) *Error {
	return &Error{Kind: KindInternal, Code: "internal_error", Message: "An internal error occurred.", cause: cause}
}
