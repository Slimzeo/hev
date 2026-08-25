package packer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/model"
)

const responseSchemaVersion = 2

// BaseResponse is the stable hev CLI JSON response.
type BaseResponse struct {
	SchemaVersion int              `json:"schemaVersion"`
	Code          model.StatusCode `json:"code"`
	Message       string           `json:"message"`
	Prompt        string           `json:"prompt"`
	Data          any              `json:"data"`
}

type environmentData struct {
	Environment model.Environment `json:"environment"`
}

type environmentSummary struct {
	ID       model.EnvironmentID `json:"id"`
	Name     string              `json:"name"`
	Revision uint64              `json:"revision"`
}

type addSkillData struct {
	EnvironmentSkill model.EnvironmentSkill `json:"environmentSkill"`
	Environments     []environmentSummary   `json:"environments"`
}

type environmentListData struct {
	Environments []environmentSummary `json:"environments"`
}

// WriteEnvironment writes a successful response containing one Environment.
func WriteEnvironment(output io.Writer, message string, value model.Environment) error {
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          model.StatusCodeOK,
		Message:       message,
		Prompt:        "",
		Data:          environmentData{Environment: value},
	})
}

// WriteSkillAdded writes a successful response for one atomic Skill update.
func WriteSkillAdded(
	output io.Writer,
	binding model.EnvironmentSkill,
	environments []model.Environment,
) error {
	summaries := make([]environmentSummary, len(environments))
	for index, current := range environments {
		summaries[index] = environmentSummary{
			ID: current.ID, Name: current.Name, Revision: current.Revision,
		}
	}
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          model.StatusCodeOK,
		Message:       "skill added to environment",
		Prompt:        "",
		Data:          addSkillData{EnvironmentSkill: binding, Environments: summaries},
	})
}

// WriteEnvironmentList writes a successful response containing all Environments.
func WriteEnvironmentList(output io.Writer, environments []model.Environment) error {
	summaries := make([]environmentSummary, len(environments))
	for index, environment := range environments {
		summaries[index] = environmentSummary{
			ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return write(output, BaseResponse{
		SchemaVersion: responseSchemaVersion,
		Code:          model.StatusCodeOK,
		Message:       "environments listed",
		Prompt:        "",
		Data:          environmentListData{Environments: summaries},
	})
}

// WriteFailure writes one classified CLI failure response.
func WriteFailure(output io.Writer, err error) error {
	statusCode, prompt := classifyError(err)
	message := err.Error()
	var domainError *model.Error
	if errors.As(err, &domainError) {
		message = domainError.Message
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

func classifyError(err error) (model.StatusCode, string) {
	if statusCode, ok := model.StatusCodeOf(err); ok {
		switch statusCode {
		case model.StatusCodeInvalidArgument:
			return statusCode, "run hev --help to inspect command usage"
		case model.StatusCodeNotFound:
			return statusCode, "create the environment before using it"
		case model.StatusCodeConflict:
			return statusCode, "inspect the existing environment configuration"
		case model.StatusCodeInternal:
			return statusCode, "retry the command or inspect stderr diagnostics"
		}
	}
	return model.StatusCodeInternal, "retry the command or inspect stderr diagnostics"
}
