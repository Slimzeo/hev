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
	if len(session.Environment.Skills) == 0 {
		_, err := fmt.Fprintf(output, "%s: no skills configured\n", session.Environment.Name)
		return err
	}
	if _, err := fmt.Fprintf(output, "%s:\n", session.Environment.Name); err != nil {
		return err
	}
	for _, skill := range session.Environment.Skills {
		if _, err := fmt.Fprintf(output, "- %s (%s)\n", skill.SkillKey, skill.Policy.Kind); err != nil {
			return err
		}
	}
	return nil
}
