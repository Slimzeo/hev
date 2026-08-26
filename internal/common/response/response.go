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
	Data          any        `json:"data"`
}

// Prompt returns the recovery guidance for a CLI status.
func Prompt(statusCode StatusCode) string {
	switch statusCode {
	case StatusCodeInvalidArgument:
		return "run hev --help to inspect command usage"
	case StatusCodeNotFound:
		return "create the environment before using it"
	case StatusCodeConflict:
		return "inspect the existing environment configuration"
	default:
		return "retry the command or inspect stderr diagnostics"
	}
}

// Write encodes one CLI v2 response.
func Write(output io.Writer, code StatusCode, message, prompt string, data any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(BaseResponse{
		SchemaVersion: constants.CLIResponseSchemaVersion,
		Code:          code,
		Message:       message,
		Prompt:        prompt,
		Data:          data,
	}); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}
