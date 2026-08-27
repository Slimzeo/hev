package handler

import (
	"github.com/Slimzeo/hev/internal/common"
	"github.com/Slimzeo/hev/internal/packer"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

// NewCreateEnvironmentCommand builds `hev env create`.
func NewCreateEnvironmentCommand(environmentService *service.EnvironmentService) *cobra.Command {
	return &cobra.Command{
		Use:   "create <env-name>",
		Short: "Create an Environment",
		Long: `Create an Environment for the current Coding Agent source.

The new Environment starts at revision 1 with the hev-guide Skill enabled.
Creating an Environment does not select it for any Session.`,
		Example: `  hev env create coding
  hev env create content-review`,
		Args: common.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			created, err := environmentService.Create(command.Context(), args[0])
			if err != nil {
				return err
			}
			return packer.WriteCreatedEnvironment(command.OutOrStdout(), common.IsJSONOutput(command), created)
		},
	}
}

// NewListEnvironmentCommand builds `hev env list`.
func NewListEnvironmentCommand(environmentService *service.EnvironmentService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Environments",
		Long: `List every Environment stored for the current Coding Agent source.

This does not show which Environment a Session currently uses. Use env status
for Session state.`,
		Example: `  hev env list
  hev env list --output json`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			environments, err := environmentService.List(command.Context())
			if err != nil {
				return err
			}
			return packer.WriteEnvironments(command.OutOrStdout(), common.IsJSONOutput(command), environments)
		},
	}
}

// NewUseEnvironmentCommand builds `hev env use`.
func NewUseEnvironmentCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "use <env-id-or-name>",
		Short: "Select one Environment for a Session",
		Long: `Select exactly one existing Environment for a host Session.

This replaces that Session's previous selection. Environment contents are
resolved again on later reads, so Skill changes do not require reactivation.`,
		Example: `  hev env use coding --session-id session-123
  hev env use env_123 --session-id session-123`,
		Args: common.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			session, err := environmentService.Use(command.Context(), sessionID, args[0])
			if err != nil {
				return err
			}
			return packer.WriteSelectedSession(command.OutOrStdout(), common.IsJSONOutput(command), session)
		},
	}
	command.Flags().StringVar(&sessionID, "session-id", "", "required opaque host Session ID")
	return command
}

// NewEnvironmentStatusCommand builds `hev env status`.
func NewEnvironmentStatusCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "status",
		Short: "Show a Session's current Environment",
		Long: `Show the latest Environment selected for a host Session.

If the Session has no selection, hev reports "hev not activated" and the host
keeps its native unfiltered Skill view.`,
		Example: `  hev env status --session-id session-123
  hev env status --session-id session-123 --output json`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			session, err := environmentService.Current(command.Context(), sessionID)
			if err != nil {
				return err
			}
			return packer.WriteSessionStatus(command.OutOrStdout(), common.IsJSONOutput(command), session)
		},
	}
	command.Flags().StringVar(&sessionID, "session-id", "", "required opaque host Session ID")
	return command
}

// NewQuitEnvironmentCommand builds `hev env quit`.
func NewQuitEnvironmentCommand(environmentService *service.EnvironmentService) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "quit",
		Short: "Leave one Session Environment tier",
		Long: `Leave one hev Environment tier for a host Session.

From a non-base Environment, quit selects base. From base, quit deactivates
hev for that Session. Calling quit while inactive leaves it inactive.`,
		Example: `  hev env quit --session-id session-123
  hev env status --session-id session-123`,
		Args: common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			session, err := environmentService.Quit(command.Context(), sessionID)
			if err != nil {
				return err
			}
			return packer.WriteChangedSession(command.OutOrStdout(), common.IsJSONOutput(command), session)
		},
	}
	command.Flags().StringVar(&sessionID, "session-id", "", "required opaque host Session ID")
	return command
}
