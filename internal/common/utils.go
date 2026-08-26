package common

import (
	"github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/spf13/cobra"
)

// ExactArgs classifies Cobra argument-count failures as invalid arguments.
func ExactArgs(count int) cobra.PositionalArgs {
	return classifyArgs(cobra.ExactArgs(count))
}

// MaximumArgs classifies Cobra argument-count failures as invalid arguments.
func MaximumArgs(count int) cobra.PositionalArgs {
	return classifyArgs(cobra.MaximumNArgs(count))
}

// MinimumArgs classifies Cobra argument-count failures as invalid arguments.
func MinimumArgs(count int) cobra.PositionalArgs {
	return classifyArgs(cobra.MinimumNArgs(count))
}

// IsJSONOutput reports whether command requested the JSON protocol.
func IsJSONOutput(command *cobra.Command) bool {
	format, err := command.Flags().GetString("output")
	return err == nil && format == constants.OutputJSON
}

// OutputFormat resolves the requested format even when Cobra rejected arguments first.
func OutputFormat(command *cobra.Command, args []string) string {
	jsonOutput := false
	for index, arg := range args {
		if arg == "--output=json" {
			jsonOutput = true
		}
		if arg == "--output" && index+1 < len(args) {
			jsonOutput = args[index+1] == constants.OutputJSON
		}
	}
	if jsonOutput {
		return constants.OutputJSON
	}
	format, err := command.Flags().GetString("output")
	if err == nil {
		return format
	}
	return constants.OutputText
}

// ValidateOutput rejects unsupported output formats.
func ValidateOutput(command *cobra.Command) error {
	format, err := command.Flags().GetString("output")
	if err != nil {
		return err
	}
	if format != constants.OutputText && format != constants.OutputJSON {
		return response.NewError(response.StatusCodeInvalidArgument, "unsupported output format %q", format)
	}
	return nil
}

func classifyArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := validate(command, args); err != nil {
			return response.NewError(response.StatusCodeInvalidArgument, "%v", err)
		}
		return nil
	}
}
