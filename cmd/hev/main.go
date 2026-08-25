package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	jsonstore "github.com/Slimzeo/hev/internal/dal/json"
	"github.com/Slimzeo/hev/internal/handler"
	"github.com/Slimzeo/hev/internal/model"
	environmentservice "github.com/Slimzeo/hev/internal/service"
	"github.com/google/uuid"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: resolve home directory: %v\n", err)
		os.Exit(1)
	}

	store := jsonstore.NewEnvironmentStore(filepath.Join(home, ".hev", "environments.json"))
	environmentService := environmentservice.New(store, func() model.EnvironmentID {
		return model.EnvironmentID("env_" + uuid.NewString())
	})
	os.Exit(handler.Execute(context.Background(), environmentService, os.Stdout, os.Stderr, os.Args[1:]))
}
