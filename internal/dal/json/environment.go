package json

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Slimzeo/hev/internal/model"
	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
)

const schemaVersion = 1
const legacyBaseEnvironmentID model.EnvironmentID = "env_base"

var baseEnvironment = model.Environment{
	ID:       "base",
	Name:     "base",
	Revision: 1,
	Skills:   []model.EnvironmentSkill{},
}

// EnvironmentStore persists the current Environment records in one JSON file.
type EnvironmentStore struct {
	path string
}

type storeFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Environments  []model.Environment `json:"environments"`
}

// NewEnvironmentStore constructs a JSON-backed store at path.
func NewEnvironmentStore(path string) *EnvironmentStore {
	return &EnvironmentStore{path: path}
}

// Create atomically inserts an Environment.
func (s *EnvironmentStore) Create(ctx context.Context, environment model.Environment) (model.Environment, error) {
	if err := environment.Validate(); err != nil {
		return model.Environment{}, err
	}
	if environment.Revision != 1 {
		return model.Environment{}, model.NewError(
			model.StatusCodeInvalidArgument,
			"new environment %q revision must be one",
			environment.Name,
		)
	}

	var created model.Environment
	err := s.withLockedFile(ctx, true, func(file *storeFile) error {
		for _, existing := range file.Environments {
			if existing.ID == environment.ID ||
				existing.Name == environment.Name ||
				string(existing.ID) == environment.Name ||
				existing.Name == string(environment.ID) {
				return model.NewError(model.StatusCodeConflict, "environment already exists: %s", environment.Name)
			}
		}
		created = model.Clone(environment)
		file.Environments = append(file.Environments, created)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("create environment: %w", err)
	}
	return model.Clone(created), nil
}

// Default returns the current default Environment.
func (s *EnvironmentStore) Default(ctx context.Context) (model.Environment, error) {
	return s.GetByIDOrName(ctx, baseEnvironment.Name)
}

// List returns detached current Environment records ordered by name.
func (s *EnvironmentStore) List(ctx context.Context) ([]model.Environment, error) {
	var environments []model.Environment
	err := s.withLockedFile(ctx, false, func(file *storeFile) error {
		environments = make([]model.Environment, len(file.Environments))
		for index, environment := range file.Environments {
			environments[index] = model.Clone(environment)
		}
		sort.Slice(environments, func(i, j int) bool {
			return environments[i].Name < environments[j].Name
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	return environments, nil
}

// GetByIDOrName reads one current Environment by ID or name.
func (s *EnvironmentStore) GetByIDOrName(ctx context.Context, identifier string) (model.Environment, error) {
	var environment model.Environment
	err := s.withLockedFile(ctx, false, func(file *storeFile) error {
		stored, found := findEnvironment(file.Environments, identifier)
		if !found {
			return model.NewError(model.StatusCodeNotFound, "environment not found: %s", identifier)
		}
		environment = model.Clone(stored)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("get environment: %w", err)
	}
	return environment, nil
}

// UpdateMany atomically mutates current Environment records in identifier order.
func (s *EnvironmentStore) UpdateMany(
	ctx context.Context,
	identifiers []string,
	update func([]model.Environment) error,
) ([]model.Environment, error) {
	var updated []model.Environment
	err := s.withLockedFile(ctx, true, func(file *storeFile) error {
		indexes := make([]int, len(identifiers))
		selected := make([]model.Environment, len(identifiers))
		seen := make(map[model.EnvironmentID]struct{}, len(identifiers))
		for requestedIndex, identifier := range identifiers {
			storedIndex := environmentIndexByIDOrName(file.Environments, identifier)
			if storedIndex < 0 {
				return model.NewError(model.StatusCodeNotFound, "environment not found: %s", identifier)
			}
			if _, exists := seen[file.Environments[storedIndex].ID]; exists {
				return model.NewError(model.StatusCodeInvalidArgument, "environment %q was supplied more than once", identifier)
			}
			seen[file.Environments[storedIndex].ID] = struct{}{}
			indexes[requestedIndex] = storedIndex
			selected[requestedIndex] = model.Clone(file.Environments[storedIndex])
		}

		if err := update(selected); err != nil {
			return err
		}
		updated = make([]model.Environment, len(indexes))
		for selectedIndex, storedIndex := range indexes {
			previous := file.Environments[storedIndex]
			next := selected[selectedIndex]
			if next.ID != previous.ID || next.Name != previous.Name || next.Revision != previous.Revision {
				return model.NewError(
					model.StatusCodeInvalidArgument,
					"environment update cannot change id, name, or revision",
				)
			}
			next.Revision++
			if err := next.Validate(); err != nil {
				return err
			}
			file.Environments[storedIndex] = next
			updated[selectedIndex] = model.Clone(next)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update environments: %w", err)
	}
	return updated, nil
}

func (s *EnvironmentStore) withLockedFile(
	ctx context.Context,
	write bool,
	operation func(*storeFile) error,
) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	fileLock := flock.New(s.path+".lock", flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(0o600))
	locked, err := fileLock.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil {
		return fmt.Errorf("lock store: %w", err)
	}
	if !locked {
		return fmt.Errorf("lock store: %w", ctx.Err())
	}
	defer func() {
		if err := fileLock.Unlock(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("unlock store: %w", err)
		}
	}()

	file, dirty, err := s.readFile()
	if err != nil {
		return err
	}
	if err := operation(&file); err != nil {
		return err
	}
	if !write && !dirty {
		return nil
	}
	return s.writeFile(file)
}

func (s *EnvironmentStore) readFile() (storeFile, bool, error) {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return storeFile{
			SchemaVersion: schemaVersion,
			Environments:  []model.Environment{model.Clone(baseEnvironment)},
		}, true, nil
	}
	if err != nil {
		return storeFile{}, false, fmt.Errorf("read store: %w", err)
	}
	var file storeFile
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return storeFile{}, false, fmt.Errorf("decode store: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return storeFile{}, false, errors.New("decode store: expected exactly one JSON object")
	}
	if file.SchemaVersion != schemaVersion {
		return storeFile{}, false, fmt.Errorf("unsupported store schema version: %d", file.SchemaVersion)
	}
	if file.Environments == nil {
		return storeFile{}, false, errors.New("decode store: environments must be an array")
	}
	dirty := false
	for index := range file.Environments {
		if file.Environments[index].ID == legacyBaseEnvironmentID && file.Environments[index].Name == baseEnvironment.Name {
			file.Environments[index].ID = baseEnvironment.ID
			dirty = true
		}
	}
	seenIDs := make(map[model.EnvironmentID]struct{}, len(file.Environments))
	seenNames := make(map[string]struct{}, len(file.Environments))
	for _, environment := range file.Environments {
		if err := environment.Validate(); err != nil {
			return storeFile{}, false, fmt.Errorf("invalid stored environment: %w", err)
		}
		if _, exists := seenIDs[environment.ID]; exists {
			return storeFile{}, false, fmt.Errorf("invalid store: duplicate environment id %q", environment.ID)
		}
		if _, exists := seenNames[environment.Name]; exists {
			return storeFile{}, false, fmt.Errorf("invalid store: duplicate environment name %q", environment.Name)
		}
		if _, exists := seenNames[string(environment.ID)]; exists {
			return storeFile{}, false, fmt.Errorf("invalid store: environment id %q conflicts with an environment name", environment.ID)
		}
		if _, exists := seenIDs[model.EnvironmentID(environment.Name)]; exists {
			return storeFile{}, false, fmt.Errorf("invalid store: environment name %q conflicts with an environment id", environment.Name)
		}
		seenIDs[environment.ID] = struct{}{}
		seenNames[environment.Name] = struct{}{}
	}
	if len(file.Environments) == 0 {
		file.Environments = []model.Environment{model.Clone(baseEnvironment)}
		dirty = true
	}
	return file, dirty, nil
}

func (s *EnvironmentStore) writeFile(file storeFile) error {
	sort.Slice(file.Environments, func(i, j int) bool {
		return file.Environments[i].Name < file.Environments[j].Name
	})
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	content = append(content, '\n')
	if err := atomic.WriteFile(s.path, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func findEnvironment(environments []model.Environment, identifier string) (model.Environment, bool) {
	for _, environment := range environments {
		if string(environment.ID) == identifier || environment.Name == identifier {
			return environment, true
		}
	}
	return model.Environment{}, false
}

func environmentIndexByIDOrName(environments []model.Environment, identifier string) int {
	for index, environment := range environments {
		if string(environment.ID) == identifier || environment.Name == identifier {
			return index
		}
	}
	return -1
}
