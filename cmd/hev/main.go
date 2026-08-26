package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Slimzeo/hev/internal/dal"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/routers"
	environmentservice "github.com/Slimzeo/hev/internal/service"
	"github.com/google/uuid"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hev: resolve home directory: %v\n", err)
		os.Exit(1)
	}

	store := dal.NewEnvironmentDAL(filepath.Join(home, ".hev", "environments.json"))
	environmentService := environmentservice.New(store, func() model.EnvironmentID {
		return model.EnvironmentID("env_" + uuid.NewString())
	})
	os.Exit(routers.Execute(context.Background(), environmentService, os.Stdout, os.Stderr, os.Args[1:]))
}
