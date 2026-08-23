package domain

import (
	"errors"
	"fmt"
)

// ErrorCode identifies a stable failure for transport adapters.
type ErrorCode string

const (
	ErrorCodeInvalidArgument          ErrorCode = "INVALID_ARGUMENT"
	ErrorCodeEnvironmentNotFound      ErrorCode = "ENV_NOT_FOUND"
	ErrorCodeEnvironmentAlreadyExists ErrorCode = "ENV_ALREADY_EXISTS"
	ErrorCodeSkillAlreadyBound        ErrorCode = "SKILL_ALREADY_BOUND"
	ErrorCodeSkillConflict            ErrorCode = "SKILL_CONFLICT"
	ErrorCodeInternal                 ErrorCode = "INTERNAL_ERROR"
)

// Error reports a domain failure with a stable machine-readable code.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError constructs a stable domain error.
func NewError(code ErrorCode, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ErrorCodeOf returns the stable code carried by err.
func ErrorCodeOf(err error) (ErrorCode, bool) {
	var domainError *Error
	if !errors.As(err, &domainError) {
		return "", false
	}
	return domainError.Code, true
}
