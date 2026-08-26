package handler

import (
	"github.com/Slimzeo/hev/internal/common"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/packer"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

// NewCreateEnvironmentCommand builds `hev env create`.
func NewCreateEnvironmentCommand(environmentService *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "create <env-name>",
		Short: "Create an empty environment",
		Args:  common.ExactArgs(1),
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
func NewListEnvironmentCommand(environmentService *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List environments",
		Args:  common.ExactArgs(0),
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
func NewUseEnvironmentCommand(environmentService *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "use [env-id-or-name]",
		Short: "Resolve the latest environment",
		Args:  common.MaximumArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := common.ValidateOutput(command); err != nil {
				return err
			}
			var current model.Environment
			var err error
			if len(args) == 0 {
				current, err = environmentService.Default(command.Context())
			} else {
				current, err = environmentService.Resolve(command.Context(), args[0])
			}
			if err != nil {
				return err
			}
			return packer.WriteResolvedEnvironment(command.OutOrStdout(), common.IsJSONOutput(command), current)
		},
	}
}
