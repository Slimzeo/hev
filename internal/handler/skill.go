package handler

import (
	"github.com/Slimzeo/hev/internal/common"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/packer"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

// NewAddSkillCommand builds `hev skill add`.
func NewAddSkillCommand(environmentService *service.Service) *cobra.Command {
	var policy string
	command := &cobra.Command{
		Use:   "add <skill-key> <env-name> [env-name...]",
		Short: "Add a skill to one or more environments",
		Args:  common.MinimumArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			binding, environments, err := environmentService.AddSkill(
				command.Context(),
				model.Skill{Key: model.SkillKey(args[0])},
				args[1:],
				model.EnvironmentSkillPolicy{Kind: model.SkillPolicyKind(policy)},
			)
			if err != nil {
				return err
			}
			return packer.WriteAddedSkill(command.OutOrStdout(), common.IsJSONOutput(command), binding, environments)
		},
	}
	command.Flags().StringVar(&policy, "policy", constants.SkillPolicyAuto, "skill policy: auto or off")
	return command
}
