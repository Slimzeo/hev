package routers

import (
	"context"
	"fmt"
	"io"

	"github.com/Slimzeo/hev/internal/common"
	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

// Execute initializes and runs the hev CLI command tree.
func Execute(
	ctx context.Context,
	environmentService *service.EnvironmentService,
	stdout, stderr io.Writer,
	args []string,
) int {
	command := NewRootCommand(environmentService)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs(args)
	command.SetContext(ctx)

	if err := command.Execute(); err != nil {
		if common.OutputFormat(command, args) == constants.OutputJSON {
			if responseErr := commonresponse.WriteError(stdout, err); responseErr != nil {
				fmt.Fprintf(stderr, "hev: %v; encode failure response: %v\n", err, responseErr)
			}
		} else {
			_, message, prompt := commonresponse.ErrorDetails(err)
			fmt.Fprintf(stderr, "hev: %s\nhint: %s\n", message, prompt)
		}
		return 1
	}
	return 0
}

// NewRootCommand constructs the hev CLI and registers its route groups.
func NewRootCommand(
	environmentService *service.EnvironmentService,
) *cobra.Command {
	root := &cobra.Command{
		Use:   "hev",
		Short: "Manage agent skill environments",
		Long: `Manage isolated Skill environments for Coding Agent sessions.

An Environment contains Skill bindings and policies. Session-aware commands
select at most one Environment for one host Session.`,
		Example: `  hev env list
  hev env create coding
  hev env rename coding-tools backend-tools
  hev env use coding --session-id <session-id>
  hev skill add code-review coding --policy auto
  hev skill list coding`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          common.ExactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			err.Error(),
			fmt.Sprintf("Run %q to inspect the supported flags.", command.CommandPath()+" --help"),
		)
	})
	root.PersistentFlags().String("output", constants.OutputText, "output format: text or json")
	registerHevRoutes(root, environmentService)
	return root
}
