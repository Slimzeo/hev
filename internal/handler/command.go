package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/packer"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

const outputJSON = "json"

// Execute runs the hev command tree and emits structured failures when JSON output was requested.
func Execute(ctx context.Context, environmentService *service.Service, stdout, stderr io.Writer, args []string) int {
	command := NewRootCommand(environmentService)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs(args)
	command.SetContext(ctx)

	if err := command.Execute(); err != nil {
		if outputFormat(command, args) == outputJSON {
			if responseErr := packer.WriteFailure(stdout, err); responseErr != nil {
				fmt.Fprintf(stderr, "hev: %v; encode failure response: %v\n", err, responseErr)
			}
		} else {
			fmt.Fprintf(stderr, "hev: %v\n", err)
		}
		return 1
	}
	return 0
}

// NewRootCommand constructs a fresh Cobra command tree around service.
func NewRootCommand(environmentService *service.Service) *cobra.Command {
	root := &cobra.Command{
		Use:           "hev",
		Short:         "Manage agent skill environments",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.NoArgs(command, args); err != nil {
				return model.NewError(model.StatusCodeInvalidArgument, "%v", err)
			}
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return model.NewError(model.StatusCodeInvalidArgument, "%v", err)
	})
	root.PersistentFlags().String("output", "text", "output format: text or json")

	environmentCommand := &cobra.Command{Use: "env", Short: "Manage environments"}
	environmentCommand.AddCommand(newCreateEnvironmentCommand(environmentService))
	environmentCommand.AddCommand(newUseEnvironmentCommand(environmentService))

	skillCommand := &cobra.Command{Use: "skill", Short: "Manage environment skill bindings"}
	skillCommand.AddCommand(newAddSkillCommand(environmentService))

	root.AddCommand(environmentCommand, skillCommand)
	return root
}

func newCreateEnvironmentCommand(environmentService *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "create <env-name>",
		Short: "Create an empty environment",
		Args:  exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validateOutput(command); err != nil {
				return err
			}
			created, err := environmentService.Create(command.Context(), args[0])
			if err != nil {
				return err
			}
			if isJSONOutput(command) {
				return packer.WriteEnvironment(command.OutOrStdout(), "environment created", created)
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "created environment %s (%s)\n", created.Name, created.ID)
			return err
		},
	}
}

func newAddSkillCommand(environmentService *service.Service) *cobra.Command {
	var environmentNames []string
	var policy string
	command := &cobra.Command{
		Use:   "add <skill-key>",
		Short: "Add a skill to one or more environments",
		Args:  exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validateOutput(command); err != nil {
				return err
			}
			binding, environments, err := environmentService.AddSkill(
				command.Context(),
				model.Skill{Key: model.SkillKey(args[0])},
				environmentNames,
				model.EnvironmentSkillPolicy{Kind: model.SkillPolicyKind(policy)},
			)
			if err != nil {
				return err
			}

			if isJSONOutput(command) {
				return packer.WriteSkillAdded(command.OutOrStdout(), binding, environments)
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "added skill %s to %d environment(s)\n", binding.SkillKey, len(environments))
			return err
		},
	}
	command.Flags().StringArrayVar(&environmentNames, "env", nil, "environment name (repeatable)")
	command.Flags().StringVar(&policy, "policy", string(model.SkillPolicyAuto), "skill policy: auto or off")
	return command
}

func newUseEnvironmentCommand(environmentService *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "use [env-id-or-name]",
		Short: "Resolve the latest environment",
		Args:  maximumArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validateOutput(command); err != nil {
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
			if isJSONOutput(command) {
				return packer.WriteEnvironment(command.OutOrStdout(), "environment resolved", current)
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "%s@%d\n", current.Name, current.Revision)
			return err
		},
	}
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(command, args); err != nil {
			return model.NewError(model.StatusCodeInvalidArgument, "%v", err)
		}
		return nil
	}
}

func maximumArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(count)(command, args); err != nil {
			return model.NewError(model.StatusCodeInvalidArgument, "%v", err)
		}
		return nil
	}
}

func isJSONOutput(command *cobra.Command) bool {
	format, err := command.Flags().GetString("output")
	return err == nil && format == outputJSON
}

func outputFormat(command *cobra.Command, args []string) string {
	foundJSON := false
	for index, arg := range args {
		if arg == "--output=json" {
			foundJSON = true
		}
		if arg == "--output" && index+1 < len(args) {
			foundJSON = args[index+1] == outputJSON
		}
	}
	if foundJSON {
		return outputJSON
	}
	format, err := command.Flags().GetString("output")
	if err == nil {
		return format
	}
	return "text"
}

func validateOutput(command *cobra.Command) error {
	format, err := command.Flags().GetString("output")
	if err != nil {
		return err
	}
	if format != "text" && format != outputJSON {
		return model.NewError(model.StatusCodeInvalidArgument, "unsupported output format %q", format)
	}
	return nil
}
