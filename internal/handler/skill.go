package handler

import (
	"github.com/Slimzeo/hev/internal/common"
	commonresponse "github.com/Slimzeo/hev/internal/common/response"
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
an Environment. Policy auto includes the Skill in automatic model discovery,
while off excludes it from that catalog.`,
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

// NewRemoveSkillCommand builds `hev skill remove`.
func NewRemoveSkillCommand(environmentService *service.EnvironmentService) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <skill-key> <env-name> [env-name...]",
		Short: "Remove a Skill from one or more Environments",
		Long: `Remove one Skill binding from one or more existing Environments atomically.

Every target must already contain the binding. Removing hev-guide from base is
not allowed.`,
		Example: `  hev skill remove code-review coding
  hev skill remove knowledge-sync coding writing`,
		Args: common.MinimumArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			environments, err := environmentService.RemoveSkill(
				command.Context(),
				model.Skill{Key: model.SkillKey(args[0])},
				args[1:],
			)
			if err != nil {
				return err
			}
			return packer.WriteRemovedSkill(
				command.OutOrStdout(),
				common.IsJSONOutput(command),
				model.SkillKey(args[0]),
				environments,
			)
		},
	}
}

// NewListSkillCommand builds `hev skill list`.
func NewListSkillCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "list [env-id-or-name]",
		Short: "List Skills configured for an Environment",
		Long: `List every Skill binding in an Environment, including bindings whose policy is off.

Pass an Environment ID or name to inspect it without changing any Session.
Without an Environment argument, --session-id selects the current Session.`,
		Example: `  hev skill list --session-id session-123
  hev skill list coding
  hev skill list --session-id session-123 --output json`,
		Args: common.MaximumArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			if len(args) == 1 {
				if sessionID != "" {
					return commonresponse.NewError(
						commonresponse.StatusCodeInvalidArgument,
						"environment argument and session id cannot be used together",
						"Use either an Environment ID or name, or --session-id for the current Environment.",
					)
				}
				environment, err := environmentService.Resolve(command.Context(), args[0])
				if err != nil {
					return err
				}
				return packer.WriteEnvironmentSkills(
					command.OutOrStdout(),
					common.IsJSONOutput(command),
					environment,
				)
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
	command.Flags().StringVar(&sessionID, "session-id", "", "host Session ID when no Environment is provided")
	return command
}
