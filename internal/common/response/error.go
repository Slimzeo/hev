package response

import (
	"errors"
	"io"

	"github.com/Slimzeo/hev/internal/constants"
)

// Error carries one classified application failure.
type Error struct {
	StatusCode StatusCode
	Message    string
	Prompt     string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError constructs a classified failure with separate diagnostics and frontend guidance.
func NewError(statusCode StatusCode, message, prompt string) error {
	return &Error{StatusCode: statusCode, Message: message, Prompt: prompt}
}

// StatusCodeOf returns the numeric status carried by err.
func StatusCodeOf(err error) (StatusCode, bool) {
	var classified *Error
	if !errors.As(err, &classified) {
		return 0, false
	}
	return classified.StatusCode, true
}

// ErrorResponse is the CLI v2 representation of one failed operation.
type ErrorResponse struct {
	BaseResponse
	Data struct{} `json:"data"`
}

// ErrorDetails returns diagnostic text and frontend guidance for err.
func ErrorDetails(err error) (StatusCode, string, string) {
	statusCode := StatusCodeInternal
	prompt := "Retry the operation. If it still fails, inspect the hev logs."
	var classified *Error
	if errors.As(err, &classified) {
		statusCode = classified.StatusCode
		if classified.Prompt != "" {
			prompt = classified.Prompt
		}
	}
	return statusCode, err.Error(), prompt
}

// WriteError classifies and encodes one failed CLI v2 response.
func WriteError(output io.Writer, err error) error {
	statusCode, message, prompt := ErrorDetails(err)
	return Write(output, ErrorResponse{
		BaseResponse: BaseResponse{
			SchemaVersion: constants.CLIResponseSchemaVersion,
			Code:          statusCode,
			Message:       message,
			Prompt:        prompt,
		},
		Data: struct{}{},
	})
}
