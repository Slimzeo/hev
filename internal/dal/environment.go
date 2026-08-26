package dal

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
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
)

var keyPattern = regexp.MustCompile(constants.KebabCasePattern)

var baseEnvironment = model.Environment{
	ID:       constants.BaseEnvironmentID,
	Name:     constants.BaseEnvironmentName,
	Revision: 1,
	Skills: []model.EnvironmentSkill{{
		SkillKey: constants.DefaultGuideSkillKey,
		Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
	}},
}

// EnvironmentDAL persists Environment models in one JSON file.
type EnvironmentDAL struct {
	path string
}

type storeFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Environments  []model.Environment `json:"environments"`
}

// NewEnvironmentDAL binds Environment persistence to path.
func NewEnvironmentDAL(path string) *EnvironmentDAL {
	return &EnvironmentDAL{path: path}
}

// Create atomically inserts an Environment.
func (d *EnvironmentDAL) Create(ctx context.Context, environment model.Environment) (model.Environment, error) {
	if err := validateEnvironment(environment); err != nil {
		return model.Environment{}, err
	}
	if environment.Revision != 1 {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"new environment %q revision must be one",
			environment.Name,
		)
	}

	var created model.Environment
	err := d.withLockedFile(ctx, true, func(file *storeFile) error {
		for _, existing := range file.Environments {
			if existing.ID == environment.ID ||
				existing.Name == environment.Name ||
				string(existing.ID) == environment.Name ||
				existing.Name == string(environment.ID) {
				return commonresponse.NewError(
					commonresponse.StatusCodeConflict,
					"environment already exists: %s",
					environment.Name,
				)
			}
		}
		created = cloneEnvironment(environment)
		file.Environments = append(file.Environments, created)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("create environment: %w", err)
	}
	return cloneEnvironment(created), nil
}

// Default returns the current base Environment.
func (d *EnvironmentDAL) Default(ctx context.Context) (model.Environment, error) {
	return d.GetByIDOrName(ctx, constants.BaseEnvironmentName)
}

// List returns detached current Environment records ordered by name.
func (d *EnvironmentDAL) List(ctx context.Context) ([]model.Environment, error) {
	var environments []model.Environment
	err := d.withLockedFile(ctx, false, func(file *storeFile) error {
		environments = cloneEnvironments(file.Environments)
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
func (d *EnvironmentDAL) GetByIDOrName(ctx context.Context, identifier string) (model.Environment, error) {
	var environment model.Environment
	err := d.withLockedFile(ctx, false, func(file *storeFile) error {
		stored, found := findEnvironment(file.Environments, identifier)
		if !found {
			return commonresponse.NewError(
				commonresponse.StatusCodeNotFound,
				"environment not found: %s",
				identifier,
			)
		}
		environment = cloneEnvironment(stored)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("get environment: %w", err)
	}
	return environment, nil
}

// UpdateMany atomically mutates current Environment records in identifier order.
func (d *EnvironmentDAL) UpdateMany(
	ctx context.Context,
	identifiers []string,
	update func([]model.Environment) error,
) ([]model.Environment, error) {
	var updated []model.Environment
	err := d.withLockedFile(ctx, true, func(file *storeFile) error {
		indexes := make([]int, len(identifiers))
		selected := make([]model.Environment, len(identifiers))
		seen := make(map[model.EnvironmentID]struct{}, len(identifiers))
		for requestedIndex, identifier := range identifiers {
			storedIndex := environmentIndexByIDOrName(file.Environments, identifier)
			if storedIndex < 0 {
				return commonresponse.NewError(
					commonresponse.StatusCodeNotFound,
					"environment not found: %s",
					identifier,
				)
			}
			if _, exists := seen[file.Environments[storedIndex].ID]; exists {
				return commonresponse.NewError(
					commonresponse.StatusCodeInvalidArgument,
					"environment %q was supplied more than once",
					identifier,
				)
			}
			seen[file.Environments[storedIndex].ID] = struct{}{}
			indexes[requestedIndex] = storedIndex
			selected[requestedIndex] = cloneEnvironment(file.Environments[storedIndex])
		}

		if err := update(selected); err != nil {
			return err
		}
		updated = make([]model.Environment, len(indexes))
		for selectedIndex, storedIndex := range indexes {
			previous := file.Environments[storedIndex]
			next := selected[selectedIndex]
			if next.ID != previous.ID || next.Name != previous.Name || next.Revision != previous.Revision {
				return commonresponse.NewError(
					commonresponse.StatusCodeInvalidArgument,
					"environment update cannot change id, name, or revision",
				)
			}
			next.Revision++
			if err := validateEnvironment(next); err != nil {
				return err
			}
			file.Environments[storedIndex] = next
			updated[selectedIndex] = cloneEnvironment(next)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update environments: %w", err)
	}
	return updated, nil
}

func (d *EnvironmentDAL) withLockedFile(
	ctx context.Context,
	write bool,
	operation func(*storeFile) error,
) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	fileLock := flock.New(d.path+".lock", flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(0o600))
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

	file, dirty, err := d.readFile()
	if err != nil {
		return err
	}
	if err := operation(&file); err != nil {
		return err
	}
	if !write && !dirty {
		return nil
	}
	return d.writeFile(file)
}

func (d *EnvironmentDAL) readFile() (storeFile, bool, error) {
	content, err := os.ReadFile(d.path)
	if errors.Is(err, fs.ErrNotExist) {
		return storeFile{
			SchemaVersion: constants.EnvironmentStoreSchemaVersion,
			Environments:  []model.Environment{cloneEnvironment(baseEnvironment)},
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
	if file.SchemaVersion != constants.EnvironmentStoreSchemaVersion {
		return storeFile{}, false, fmt.Errorf("unsupported store schema version: %d", file.SchemaVersion)
	}
	if file.Environments == nil {
		return storeFile{}, false, errors.New("decode store: environments must be an array")
	}

	dirty := false
	for index := range file.Environments {
		if file.Environments[index].ID == constants.LegacyBaseEnvironmentID &&
			file.Environments[index].Name == constants.BaseEnvironmentName {
			file.Environments[index].ID = constants.BaseEnvironmentID
			dirty = true
		}
	}
	seenIDs := make(map[model.EnvironmentID]struct{}, len(file.Environments))
	seenNames := make(map[string]struct{}, len(file.Environments))
	for _, environment := range file.Environments {
		if err := validateEnvironment(environment); err != nil {
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
		file.Environments = []model.Environment{cloneEnvironment(baseEnvironment)}
		dirty = true
	} else if addMissingBaseDefaults(&file) {
		dirty = true
	}
	return file, dirty, nil
}

func addMissingBaseDefaults(file *storeFile) bool {
	index := environmentIndexByIDOrName(file.Environments, constants.BaseEnvironmentName)
	if index < 0 {
		return false
	}
	for _, skill := range file.Environments[index].Skills {
		if skill.SkillKey == constants.DefaultGuideSkillKey {
			return false
		}
	}
	file.Environments[index].Skills = append(
		file.Environments[index].Skills,
		model.EnvironmentSkill{
			SkillKey: constants.DefaultGuideSkillKey,
			Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
		},
	)
	file.Environments[index].Revision++
	return true
}

func (d *EnvironmentDAL) writeFile(file storeFile) error {
	sort.Slice(file.Environments, func(i, j int) bool {
		return file.Environments[i].Name < file.Environments[j].Name
	})
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	content = append(content, '\n')
	if err := atomic.WriteFile(d.path, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func validateEnvironment(environment model.Environment) error {
	if strings.TrimSpace(string(environment.ID)) == "" {
		return commonresponse.NewError(commonresponse.StatusCodeInvalidArgument, "environment id must not be empty")
	}
	if !keyPattern.MatchString(environment.Name) {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"invalid environment name %q: use lowercase kebab-case",
			environment.Name,
		)
	}
	if environment.Revision == 0 {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment %q revision must be greater than zero",
			environment.Name,
		)
	}
	if environment.Skills == nil {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment %q skills must be an array",
			environment.Name,
		)
	}

	seenSkills := make(map[model.SkillKey]struct{}, len(environment.Skills))
	for _, skill := range environment.Skills {
		if !keyPattern.MatchString(string(skill.SkillKey)) {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				"invalid skill key %q: use lowercase kebab-case",
				skill.SkillKey,
			)
		}
		if skill.Policy.Kind != constants.SkillPolicyAuto && skill.Policy.Kind != constants.SkillPolicyOff {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				"skill %q: unsupported skill policy: %s",
				skill.SkillKey,
				skill.Policy.Kind,
			)
		}
		if _, exists := seenSkills[skill.SkillKey]; exists {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				"environment %q contains duplicate skill %q",
				environment.Name,
				skill.SkillKey,
			)
		}
		seenSkills[skill.SkillKey] = struct{}{}
	}
	return nil
}

func cloneEnvironment(environment model.Environment) model.Environment {
	environment.Skills = slices.Clone(environment.Skills)
	return environment
}

func cloneEnvironments(environments []model.Environment) []model.Environment {
	cloned := make([]model.Environment, len(environments))
	for index, environment := range environments {
		cloned[index] = cloneEnvironment(environment)
	}
	return cloned
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
