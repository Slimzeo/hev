package routers

import (
	"github.com/Slimzeo/hev/internal/handler"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/spf13/cobra"
)

func registerHevRoutes(root *cobra.Command, environmentService *service.Service) {
	// Session-dependent status, quit, and Skill listing routes are registered by the DSH runtime.
	environment := &cobra.Command{Use: "env", Short: "Manage environments"}
	environment.AddCommand(
		handler.NewCreateEnvironmentCommand(environmentService),
		handler.NewListEnvironmentCommand(environmentService),
		handler.NewUseEnvironmentCommand(environmentService),
	)

	skill := &cobra.Command{Use: "skill", Short: "Manage environment skill bindings"}
	skill.AddCommand(handler.NewAddSkillCommand(environmentService))

	root.AddCommand(environment, skill)
}
