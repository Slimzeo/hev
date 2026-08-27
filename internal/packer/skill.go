package packer

import (
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/model"
)

type addSkillData struct {
	EnvironmentSkill model.EnvironmentSkill `json:"environmentSkill"`
	Environments     []environmentSummary   `json:"environments"`
}

type addedSkillResponse struct {
	response.BaseResponse
	Data addSkillData `json:"data"`
}

type removeSkillData struct {
	SkillKey     model.SkillKey       `json:"skillKey"`
	Environments []environmentSummary `json:"environments"`
}

type removedSkillResponse struct {
	response.BaseResponse
	Data removeSkillData `json:"data"`
}

// WriteAddedSkill writes one Skill binding result in the selected output format.
func WriteAddedSkill(
	output io.Writer,
	jsonOutput bool,
	binding model.EnvironmentSkill,
	environments []model.Environment,
) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(output, "added skill %s to %d environment(s)\n", binding.SkillKey, len(environments))
		return err
	}

	summaries := make([]environmentSummary, len(environments))
	for index, environment := range environments {
		summaries[index] = environmentSummary{
			Source: environment.Source, ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return response.Write(output, addedSkillResponse{
		BaseResponse: response.Success("skill added to environment"),
		Data: addSkillData{
			EnvironmentSkill: binding,
			Environments:     summaries,
		},
	})
}

// WriteRemovedSkill writes one Skill removal result in the selected output format.
func WriteRemovedSkill(
	output io.Writer,
	jsonOutput bool,
	skillKey model.SkillKey,
	environments []model.Environment,
) error {
	if !jsonOutput {
		_, err := fmt.Fprintf(output, "removed skill %s from %d environment(s)\n", skillKey, len(environments))
		return err
	}

	summaries := make([]environmentSummary, len(environments))
	for index, environment := range environments {
		summaries[index] = environmentSummary{
			Source: environment.Source, ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return response.Write(output, removedSkillResponse{
		BaseResponse: response.Success("skill removed from environment"),
		Data: removeSkillData{
			SkillKey:     skillKey,
			Environments: summaries,
		},
	})
}

// WriteEnvironmentSkills writes the Skill bindings configured for one named Environment.
func WriteEnvironmentSkills(output io.Writer, jsonOutput bool, environment model.Environment) error {
	if jsonOutput {
		return response.Write(output, environmentResponse{
			BaseResponse: response.Success("environment skills listed"),
			Data:         environmentData{Environment: environment},
		})
	}
	return writeEnvironmentSkillLines(output, environment)
}

// WriteSessionSkills writes the Skill bindings configured for one Session's current Environment.
func WriteSessionSkills(output io.Writer, jsonOutput bool, session model.Session) error {
	if jsonOutput {
		return response.Write(output, sessionResponse{
			BaseResponse: response.Success("session skills listed"),
			Data:         sessionData{Session: session},
		})
	}
	if session.Environment == nil {
		_, err := fmt.Fprintln(output, "hev not activated")
		return err
	}
	return writeEnvironmentSkillLines(output, *session.Environment)
}

func writeEnvironmentSkillLines(output io.Writer, environment model.Environment) error {
	if len(environment.Skills) == 0 {
		_, err := fmt.Fprintf(output, "%s: no skills configured\n", environment.Name)
		return err
	}
	if _, err := fmt.Fprintf(output, "%s:\n", environment.Name); err != nil {
		return err
	}
	for _, skill := range environment.Skills {
		if _, err := fmt.Fprintf(output, "- %s (%s)\n", skill.SkillKey, skill.Policy.Kind); err != nil {
			return err
		}
	}
	return nil
}
