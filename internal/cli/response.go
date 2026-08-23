package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/domain"
)

const responseSchemaVersion = 1

type response struct {
	SchemaVersion int    `json:"schemaVersion"`
	Code          int    `json:"code"`
	Message       string `json:"message"`
	Prompt        string `json:"prompt"`
	Data          any    `json:"data"`
}

type errorData struct {
	ErrorCode domain.ErrorCode `json:"errorCode"`
}

type commandError struct {
	cause error
}

func (e *commandError) Error() string {
	return e.cause.Error()
}

func (e *commandError) Unwrap() error {
	return e.cause
}

func writeSuccess(output io.Writer, message string, data any) error {
	return writeResponse(output, response{
		SchemaVersion: responseSchemaVersion,
		Code:          200,
		Message:       message,
		Prompt:        "",
		Data:          data,
	})
}

func writeFailure(output io.Writer, err error) error {
	code, prompt, status := classifyError(err)
	message := err.Error()
	var domainError *domain.Error
	if errors.As(err, &domainError) {
		message = domainError.Message
	}
	return writeResponse(output, response{
		SchemaVersion: responseSchemaVersion,
		Code:          status,
		Message:       message,
		Prompt:        prompt,
		Data:          errorData{ErrorCode: code},
	})
}

func writeResponse(output io.Writer, value response) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}

func classifyError(err error) (domain.ErrorCode, string, int) {
	if code, ok := domain.ErrorCodeOf(err); ok {
		switch code {
		case domain.ErrorCodeInvalidArgument:
			return code, "run hev --help to inspect command usage", 400
		case domain.ErrorCodeEnvironmentNotFound:
			return code, "create the environment before using it", 404
		case domain.ErrorCodeEnvironmentAlreadyExists, domain.ErrorCodeSkillAlreadyBound, domain.ErrorCodeSkillConflict:
			return code, "inspect the existing environment configuration", 409
		}
	}

	var commandFailure *commandError
	if errors.As(err, &commandFailure) {
		return domain.ErrorCodeInvalidArgument, "run hev --help to inspect command usage", 400
	}
	return domain.ErrorCodeInternal, "retry the command or inspect stderr diagnostics", 500
}
