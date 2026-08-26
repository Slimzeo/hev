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
func Execute(ctx context.Context, environmentService *service.Service, stdout, stderr io.Writer, args []string) int {
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
			fmt.Fprintf(stderr, "hev: %v\n", err)
		}
		return 1
	}
	return 0
}

// NewRootCommand constructs the hev CLI and registers its route groups.
func NewRootCommand(environmentService *service.Service) *cobra.Command {
	root := &cobra.Command{
		Use:           "hev",
		Short:         "Manage agent skill environments",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.NoArgs(command, args); err != nil {
				return commonresponse.NewError(commonresponse.StatusCodeInvalidArgument, "%v", err)
			}
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return commonresponse.NewError(commonresponse.StatusCodeInvalidArgument, "%v", err)
	})
	root.PersistentFlags().String("output", constants.OutputText, "output format: text or json")
	registerHevRoutes(root, environmentService)
	return root
}
