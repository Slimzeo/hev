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

type environmentSummary struct {
	ID       model.EnvironmentID `json:"id"`
	Name     string              `json:"name"`
	Revision uint64              `json:"revision"`
}

type environmentListData struct {
	Environments []environmentSummary `json:"environments"`
}

// WriteCreatedEnvironment writes one Environment creation result.
func WriteCreatedEnvironment(output io.Writer, jsonOutput bool, environment model.Environment) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(output, "created environment %s (%s)\n", environment.Name, environment.ID)
		return err
	}
	return response.Write(
		output,
		response.StatusCodeOK,
		"environment created",
		"",
		environmentData{Environment: environment},
	)
}

// WriteResolvedEnvironment writes one Environment resolution result.
func WriteResolvedEnvironment(output io.Writer, jsonOutput bool, environment model.Environment) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(output, "%s@%d\n", environment.Name, environment.Revision)
		return err
	}
	return response.Write(
		output,
		response.StatusCodeOK,
		"environment resolved",
		"",
		environmentData{Environment: environment},
	)
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
			ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return response.Write(
		output,
		response.StatusCodeOK,
		"environments listed",
		"",
		environmentListData{Environments: summaries},
	)
}
