package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/dal"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/routers"
	"github.com/Slimzeo/hev/internal/service"
	"github.com/google/uuid"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: resolve home directory: %v\n", err)
		os.Exit(1)
	}

	source, args, err := extractSource(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: %v\n", err)
		os.Exit(1)
	}
	stateDir, err := resolveStateDir(source, home, os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: resolve state directory: %v\n", err)
		os.Exit(1)
	}
	environmentStore := dal.NewEnvironmentDAL(source, stateDir)
	environmentService := service.NewEnvironment(environmentStore, func() model.EnvironmentID {
		return model.EnvironmentID("env_" + uuid.NewString())
	})
	os.Exit(routers.Execute(
		context.Background(),
		environmentService,
		os.Stdout,
		os.Stderr,
		args,
	))
}

func extractSource(args []string) (model.Source, []string, error) {
	source := model.SourceStandalone
	commandArgs := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			commandArgs = append(commandArgs, args[index:]...)
			break
		}
		if value, found := strings.CutPrefix(arg, "--source="); found {
			if value == "" {
				return "", nil, fmt.Errorf("source must not be empty")
			}
			source = model.Source(value)
			continue
		}
		if arg == "--source" {
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return "", nil, fmt.Errorf("source requires a value")
			}
			source = model.Source(args[index+1])
			index++
			continue
		}
		commandArgs = append(commandArgs, arg)
	}
	if !source.Valid() {
		return "", nil, fmt.Errorf("unsupported source %q", source)
	}
	return source, commandArgs, nil
}

func resolveStateDir(
	source model.Source,
	home string,
	lookupEnv func(string) (string, bool),
) (string, error) {
	var root string
	switch source {
	case model.SourceStandalone:
		root = home
	case model.SourceDSH:
		root = environmentOrDefault(lookupEnv, constants.DSHHomeEnvironment, filepath.Join(home, constants.DSHHomeDirectory))
	case model.SourceClaudeCode:
		root = environmentOrDefault(lookupEnv, constants.ClaudeHomeEnvironment, filepath.Join(home, constants.ClaudeHomeDirectory))
	case model.SourceCodex:
		root = environmentOrDefault(lookupEnv, constants.CodexHomeEnvironment, filepath.Join(home, constants.CodexHomeDirectory))
	case model.SourceOpenCode:
		root = environmentOrDefault(lookupEnv, constants.OpenCodeHomeEnvironment, "")
		if root == "" {
			root = filepath.Join(
				environmentOrDefault(
					lookupEnv,
					constants.XDGConfigHomeEnvironment,
					filepath.Join(home, constants.DefaultConfigHomeDirectory),
				),
				constants.OpenCodeHomeDirectory,
			)
		}
	default:
		return "", fmt.Errorf("unsupported source %q", source)
	}
	root = expandHome(root, home)
	return filepath.Abs(filepath.Join(root, constants.HevStateDirectoryName))
}

func environmentOrDefault(lookupEnv func(string) (string, bool), name, fallback string) string {
	value, found := lookupEnv(name)
	if !found || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}
