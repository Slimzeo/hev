package response

import (
	"errors"
	"fmt"
	"io"
)

// Error carries one classified application failure.
type Error struct {
	StatusCode StatusCode
	Message    string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError constructs a classified application failure.
func NewError(statusCode StatusCode, format string, args ...any) error {
	return &Error{StatusCode: statusCode, Message: fmt.Sprintf(format, args...)}
}

// StatusCodeOf returns the numeric status carried by err.
func StatusCodeOf(err error) (StatusCode, bool) {
	var classified *Error
	if !errors.As(err, &classified) {
		return 0, false
	}
	return classified.StatusCode, true
}

// WriteError classifies and encodes one failed CLI v2 response.
func WriteError(output io.Writer, err error) error {
	statusCode := StatusCodeInternal
	message := err.Error()
	var classified *Error
	if errors.As(err, &classified) {
		statusCode = classified.StatusCode
		message = classified.Message
	}
	return Write(output, statusCode, message, Prompt(statusCode), struct{}{})
}
