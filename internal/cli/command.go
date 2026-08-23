package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/application"
	"github.com/Slimzeo/hev/internal/domain"
	"github.com/spf13/cobra"
)

const outputJSON = "json"

// Execute runs the HEV command tree and emits structured failures when JSON output was requested.
func Execute(ctx context.Context, service *application.EnvironmentService, stdout, stderr io.Writer, args []string) int {
	command := NewRootCommand(service)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs(args)
	command.SetContext(ctx)

	if err := command.Execute(); err != nil {
		if outputFormat(command, args) == outputJSON {
			if responseErr := writeFailure(stdout, err); responseErr != nil {
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
func NewRootCommand(service *application.EnvironmentService) *cobra.Command {
	root := &cobra.Command{
		Use:           "hev",
		Short:         "Manage agent skill environments",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.NoArgs(command, args); err != nil {
				return &commandError{cause: err}
			}
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &commandError{cause: err}
	})
	root.PersistentFlags().String("output", "text", "output format: text or json")

	environment := &cobra.Command{Use: "env", Short: "Manage environments"}
	environment.AddCommand(newCreateEnvironmentCommand(service))
	environment.AddCommand(newActivateEnvironmentCommand(service))

	skill := &cobra.Command{Use: "skill", Short: "Manage environment skill bindings"}
	skill.AddCommand(newAddSkillCommand(service))

	root.AddCommand(environment, skill)
	return root
}

type createEnvironmentData struct {
	Environment domain.Environment `json:"environment"`
}

func newCreateEnvironmentCommand(service *application.EnvironmentService) *cobra.Command {
	return &cobra.Command{
		Use:   "create <env-name>",
		Short: "Create an empty environment",
		Args:  exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validateOutput(command); err != nil {
				return err
			}
			environment, err := service.CreateEnvironment(command.Context(), args[0])
			if err != nil {
				return err
			}
			if isJSONOutput(command) {
				return writeSuccess(command.OutOrStdout(), "environment created", createEnvironmentData{Environment: environment})
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "created environment %s (%s)\n", environment.Name, environment.ID)
			return err
		},
	}
}

type environmentSummary struct {
	ID       domain.EnvironmentID `json:"id"`
	Name     string               `json:"name"`
	Revision uint64               `json:"revision"`
}

type addSkillData struct {
	EnvironmentSkill domain.EnvironmentSkillSpec `json:"environmentSkill"`
	Environments     []environmentSummary        `json:"environments"`
}

func newAddSkillCommand(service *application.EnvironmentService) *cobra.Command {
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
			spec, environments, err := service.AddEnvironmentSkill(
				command.Context(),
				domain.SkillKey(args[0]),
				environmentNames,
				domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyKind(policy)},
			)
			if err != nil {
				return err
			}

			summaries := make([]environmentSummary, len(environments))
			for index, environment := range environments {
				summaries[index] = environmentSummary{
					ID: environment.ID, Name: environment.Name, Revision: environment.Revision,
				}
			}
			if isJSONOutput(command) {
				return writeSuccess(command.OutOrStdout(), "skill added to environment", addSkillData{
					EnvironmentSkill: spec, Environments: summaries,
				})
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "added skill %s to %d environment(s)\n", spec.SkillKey, len(environments))
			return err
		},
	}
	command.Flags().StringArrayVar(&environmentNames, "env", nil, "environment name (repeatable)")
	command.Flags().StringVar(&policy, "policy", string(domain.SkillPolicyAuto), "skill policy: auto or off")
	return command
}

type activateEnvironmentData struct {
	Snapshot domain.ResolvedEnvironmentSnapshot `json:"snapshot"`
}

func newActivateEnvironmentCommand(service *application.EnvironmentService) *cobra.Command {
	return &cobra.Command{
		Use:   "activate <env-id-or-name> [env-id-or-name...]",
		Short: "Resolve the latest environment snapshot",
		Args:  minimumArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := validateOutput(command); err != nil {
				return err
			}
			snapshot, err := service.ResolveEnvironmentGroup(command.Context(), args)
			if err != nil {
				return err
			}
			if isJSONOutput(command) {
				return writeSuccess(command.OutOrStdout(), "environment snapshot resolved", activateEnvironmentData{Snapshot: snapshot})
			}
			for _, environment := range snapshot.Environments {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "%s@%d\n", environment.Name, environment.Revision); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(command, args); err != nil {
			return &commandError{cause: err}
		}
		return nil
	}
}

func minimumArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(count)(command, args); err != nil {
			return &commandError{cause: err}
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
		return &commandError{cause: fmt.Errorf("unsupported output format %q", format)}
	}
	return nil
}
