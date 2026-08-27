package response

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/constants"
)

// StatusCode is the numeric status returned by the public CLI contract.
type StatusCode int

const (
	StatusCodeOK              StatusCode = 200
	StatusCodeInvalidArgument StatusCode = 400
	StatusCodeNotFound        StatusCode = 404
	StatusCodeConflict        StatusCode = 409
	StatusCodeInternal        StatusCode = 500
)

// BaseResponse is the stable hev CLI JSON response.
type BaseResponse struct {
	SchemaVersion int        `json:"schemaVersion"`
	Code          StatusCode `json:"code"`
	Message       string     `json:"message"`
	Prompt        string     `json:"prompt"`
}

// Success constructs the common fields of one successful response.
func Success(message string) BaseResponse {
	return BaseResponse{
		SchemaVersion: constants.CLIResponseSchemaVersion,
		Code:          StatusCodeOK,
		Message:       message,
		Prompt:        "",
	}
}

// Write encodes one concrete CLI v2 response.
func Write(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
