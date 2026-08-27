package routers

import (
	"github.com/Slimzeo/hev/internal/common"
	"github.com/Slimzeo/hev/internal/handler"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

func registerHevRoutes(
	root *cobra.Command,
	environmentService *service.EnvironmentService,
) {
	environment := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long: `Create and inspect Environments, or select one for a host Session.

A Session can use only one Environment at a time. Without a selection, hev is
inactive and the host keeps its native Skill visibility.`,
		Example: `  hev env list
  hev env create coding
  hev env rename coding-tools backend-tools
  hev env delete temporary-tools
  hev env use coding --session-id <session-id>
  hev env status --session-id <session-id>
  hev env quit --session-id <session-id>`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	environment.AddCommand(
		handler.NewCreateEnvironmentCommand(environmentService),
		handler.NewRenameEnvironmentCommand(environmentService),
		handler.NewDeleteEnvironmentCommand(environmentService),
		handler.NewListEnvironmentCommand(environmentService),
		handler.NewUseEnvironmentCommand(environmentService),
		handler.NewEnvironmentStatusCommand(environmentService),
		handler.NewQuitEnvironmentCommand(environmentService),
	)

	skill := &cobra.Command{
		Use:   "skill",
		Short: "Manage Environment Skill bindings",
		Long: `Configure which native Skills are available in each Environment.

Adding a Skill changes Environment configuration; it does not activate that
Environment for a Session.`,
		Example: `  hev skill add code-review coding --policy auto
  hev skill remove code-review coding
  hev skill add knowledge-sync coding writing --policy off
  hev skill list coding
  hev skill list --session-id <session-id>`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	skill.AddCommand(
		handler.NewAddSkillCommand(environmentService),
		handler.NewRemoveSkillCommand(environmentService),
		handler.NewListSkillCommand(environmentService),
	)

	root.AddCommand(environment, skill)
}
