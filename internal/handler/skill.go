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
func NewAddSkillCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var policy string
	command := &cobra.Command{
		Use:   "add <skill-key> <env-name> [env-name...]",
		Short: "Add a Skill to one or more Environments",
		Long: `Bind one native Skill key to one or more existing Environments atomically.

This records Environment configuration; it does not install the Skill or select
an Environment. Policy auto exposes the Skill to the model, while off keeps the
binding configured but hidden.`,
		Example: `  hev skill add code-review coding
  hev skill add knowledge-sync coding writing --policy off`,
		Args: common.MinimumArgs(2),
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
	command.Flags().StringVar(&policy, "policy", constants.SkillPolicyAuto, "Skill visibility policy: auto or off")
	return command
}

// NewListSessionSkillCommand builds `hev skill list`.
func NewListSessionSkillCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "list",
		Short: "List Skills configured for a Session",
		Long: `List every Skill binding in a Session's current Environment, including
bindings whose policy is off. If hev is inactive for the Session, the command
reports "hev not activated".`,
		Example: `  hev skill list --session-id session-123
  hev skill list --session-id session-123 --output json`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			session, err := environmentService.Current(command.Context(), sessionID)
			if err != nil {
				return err
			}
			return packer.WriteSessionSkills(
				command.OutOrStdout(),
				common.IsJSONOutput(command),
				session,
			)
		},
	}
	command.Flags().StringVar(&sessionID, "session-id", "", "required opaque host Session ID")
	return command
}
