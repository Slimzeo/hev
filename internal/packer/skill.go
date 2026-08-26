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
			ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
		}
	}
	return response.Write(
		output,
		response.StatusCodeOK,
		"skill added to environment",
		"",
		addSkillData{EnvironmentSkill: binding, Environments: summaries},
	)
}
