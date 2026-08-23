package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Slimzeo/hev/internal/application"
	"github.com/Slimzeo/hev/internal/cli"
	"github.com/Slimzeo/hev/internal/domain"
	jsonstore "github.com/Slimzeo/hev/internal/store/json"
	"github.com/google/uuid"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: resolve home directory: %v\n", err)
		os.Exit(1)
	}

	store := jsonstore.NewEnvironmentStore(filepath.Join(home, ".hev", "environments.json"))
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID {
		return domain.EnvironmentID("env_" + uuid.NewString())
	})
	os.Exit(cli.Execute(context.Background(), service, os.Stdout, os.Stderr, os.Args[1:]))
}
