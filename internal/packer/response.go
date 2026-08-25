package packer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/domain/environment"
)

const responseSchemaVersion = 2

// BaseResponse is the stable hev CLI JSON response.
type BaseResponse struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Code          environment.StatusCode `json:"code"`
	Message       string                 `json:"message"`
	Prompt        string                 `json:"prompt"`
	Data          any                    `json:"data"`
}

type environmentData struct {
	Environment environment.Environment `json:"environment"`
}

type environmentSummary struct {
	ID       environment.EnvironmentID `json:"id"`
	Name     string                    `json:"name"`
	Revision uint64                    `json:"revision"`
}

type addSkillData struct {
	EnvironmentSkill environment.EnvironmentSkillSpec `json:"environmentSkill"`
	Environments     []environmentSummary             `json:"environments"`
}

// WriteEnvironment writes a successful response containing one Environment.
func WriteEnvironment(output io.Writer, message string, value environment.Environment) error {
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          environment.StatusCodeOK,
		Message:       message,
		Prompt:        "",
		Data:          environmentData{Environment: value},
	})
}

// WriteSkillAdded writes a successful response for one atomic Skill update.
func WriteSkillAdded(
	output io.Writer,
	spec environment.EnvironmentSkillSpec,
	environments []environment.Environment,
) error {
	summaries := make([]environmentSummary, len(environments))
	for index, current := range environments {
		summaries[index] = environmentSummary{
			ID: current.ID, Name: current.Name, Revision: current.Revision,
		}
	}
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          environment.StatusCodeOK,
		Message:       "skill added to environment",
		Prompt:        "",
		Data:          addSkillData{EnvironmentSkill: spec, Environments: summaries},
	})
}

// WriteFailure writes one classified CLI failure response.
func WriteFailure(output io.Writer, err error) error {
	statusCode, prompt := classifyError(err)
	message := err.Error()
	var environmentError *environment.Error
	if errors.As(err, &environmentError) {
		message = environmentError.Message
	}
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          statusCode,
		Message:       message,
		Prompt:        prompt,
		Data:          struct{}{},
	})
}

func write(output io.Writer, response BaseResponse) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	return nil
}

func classifyError(err error) (environment.StatusCode, string) {
	if statusCode, ok := environment.StatusCodeOf(err); ok {
		switch statusCode {
		case environment.StatusCodeInvalidArgument:
			return statusCode, "run hev --help to inspect command usage"
		case environment.StatusCodeNotFound:
			return statusCode, "create the environment before using it"
		case environment.StatusCodeConflict:
			return statusCode, "inspect the existing environment configuration"
		case environment.StatusCodeInternal:
			return statusCode, "retry the command or inspect stderr diagnostics"
		}
	}
	return environment.StatusCodeInternal, "retry the command or inspect stderr diagnostics"
}
