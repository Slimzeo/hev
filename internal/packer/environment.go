package packer

import (
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/model"
)

type environmentData struct {
	Environment model.Environment `json:"environment"`
}

type environmentResponse struct {
	response.BaseResponse
	Data environmentData `json:"data"`
}

type environmentSummary struct {
	Source   model.Source        `json:"source"`
	ID       model.EnvironmentID `json:"id"`
	Name     string              `json:"name"`
	Revision uint64              `json:"revision"`
}

type environmentListData struct {
	Environments []environmentSummary `json:"environments"`
}

type environmentListResponse struct {
	response.BaseResponse
	Data environmentListData `json:"data"`
}

type sessionData struct {
	Session model.Session `json:"session"`
}

type sessionResponse struct {
	response.BaseResponse
	Data sessionData `json:"data"`
}

// WriteCreatedEnvironment writes one Environment creation result.
func WriteCreatedEnvironment(output io.Writer, jsonOutput bool, environment model.Environment) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(output, "created environment %s (%s)\n", environment.Name, environment.ID)
		return err
	}
	return response.Write(output, environmentResponse{
		BaseResponse: response.Success("environment created"),
		Data:         environmentData{Environment: environment},
	})
}

// WriteEnvironments writes the Environment list in the selected output format.
func WriteEnvironments(output io.Writer, jsonOutput bool, environments []model.Environment) error {
	if !jsonOutput {
		for _, environment := range environments {
			if _, err := fmt.Fprintf(output, "%s (%s rev %d)\n", environment.Name, environment.ID, environment.Revision); err != nil {
				return err
			}
		}
		return nil
	}

	summaries := make([]environmentSummary, len(environments))
	for index, environment := range environments {
		summaries[index] = environmentSummary{
			Source: environment.Source, ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return response.Write(output, environmentListResponse{
		BaseResponse: response.Success("environments listed"),
		Data:         environmentListData{Environments: summaries},
	})
}

// WriteSelectedSession writes the result of selecting an Environment.
func WriteSelectedSession(output io.Writer, jsonOutput bool, session model.Session) error {
	return writeSession(output, jsonOutput, "environment selected", session)
}

// WriteSessionStatus writes the current Session state.
func WriteSessionStatus(output io.Writer, jsonOutput bool, session model.Session) error {
	return writeSession(output, jsonOutput, "session status resolved", session)
}

// WriteChangedSession writes the result of leaving an Environment tier.
func WriteChangedSession(output io.Writer, jsonOutput bool, session model.Session) error {
	return writeSession(output, jsonOutput, "session environment changed", session)
}

func writeSession(output io.Writer, jsonOutput bool, message string, session model.Session) error {
	if !jsonOutput {
		if session.Environment == nil {
			_, err := fmt.Fprintln(output, "hev not activated")
			return err
		}
		_, err := fmt.Fprintf(
			output,
			"%s (%s rev %d)\n",
			session.Environment.Name,
			session.Environment.ID,
			session.Environment.Revision,
		)
		return err
	}
	return response.Write(output, sessionResponse{
		BaseResponse: response.Success(message),
		Data:         sessionData{Session: session},
	})
}
