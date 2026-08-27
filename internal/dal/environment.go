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

// EnvironmentDAL persists Environment records and their Session bindings.
type EnvironmentDAL struct {
	source          model.Source
	environmentPath string
	sessionPath     string
}

type storeFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Environments  []model.Environment `json:"environments"`
}

type sessionBinding struct {
	SessionID     string              `json:"sessionId"`
	EnvironmentID model.EnvironmentID `json:"environmentId"`
}

type sessionStoreFile struct {
	SchemaVersion int              `json:"schemaVersion"`
	Bindings      []sessionBinding `json:"bindings"`
}

// NewEnvironmentDAL binds Environment persistence to one host state directory.
func NewEnvironmentDAL(source model.Source, stateDir string) *EnvironmentDAL {
	return &EnvironmentDAL{
		source:          source,
		environmentPath: filepath.Join(stateDir, constants.EnvironmentStoreFileName),
		sessionPath:     filepath.Join(stateDir, constants.SessionStoreFileName),
	}
}

// Source returns the Coding Agent platform isolated by this store.
func (d *EnvironmentDAL) Source() model.Source {
	return d.source
}

// Create atomically inserts an Environment.
func (d *EnvironmentDAL) Create(ctx context.Context, environment model.Environment) (model.Environment, error) {
	if environment.Source != d.source {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("environment source %q does not match store source %q", environment.Source, d.source),
			"Retry through the Coding Agent adapter that owns this Environment.",
		)
	}
	if err := validateEnvironment(environment); err != nil {
		return model.Environment{}, err
	}
	if environment.Revision != 1 {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("new environment %q revision must be one", environment.Name),
			"Create a new Environment at revision 1.",
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
					fmt.Sprintf("environment already exists: %s", environment.Name),
					"List the available Environments and choose a different name.",
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

// Rename atomically changes one Environment name and increments its revision.
func (d *EnvironmentDAL) Rename(
	ctx context.Context,
	environmentID model.EnvironmentID,
	newName string,
) (model.Environment, error) {
	var renamed model.Environment
	err := d.withLockedFile(ctx, true, func(file *storeFile) error {
		index := environmentIndexByIDOrName(file.Environments, string(environmentID))
		if index < 0 {
			return commonresponse.NewError(
				commonresponse.StatusCodeNotFound,
				fmt.Sprintf("environment not found: %s", environmentID),
				"List the available Environments and retry with an existing Environment ID or name.",
			)
		}
		if file.Environments[index].ID == constants.BaseEnvironmentID {
			return commonresponse.NewError(
				commonresponse.StatusCodeConflict,
				"base environment cannot be renamed",
				"Choose a non-base Environment to rename.",
			)
		}
		if file.Environments[index].Name == newName {
			renamed = cloneEnvironment(file.Environments[index])
			return nil
		}
		for candidateIndex, candidate := range file.Environments {
			if candidateIndex == index {
				continue
			}
			if candidate.Name == newName || string(candidate.ID) == newName {
				return commonresponse.NewError(
					commonresponse.StatusCodeConflict,
					fmt.Sprintf("environment name already exists: %s", newName),
					"Choose a different Environment name.",
				)
			}
		}

		next := cloneEnvironment(file.Environments[index])
		next.Name = newName
		next.Revision++
		if err := validateEnvironment(next); err != nil {
			return err
		}
		file.Environments[index] = next
		renamed = cloneEnvironment(next)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("rename environment: %w", err)
	}
	return renamed, nil
}

// Delete atomically removes one Environment record.
func (d *EnvironmentDAL) Delete(
	ctx context.Context,
	environmentID model.EnvironmentID,
) (model.Environment, error) {
	var deleted model.Environment
	err := d.withLockedFile(ctx, true, func(file *storeFile) error {
		index := environmentIndexByIDOrName(file.Environments, string(environmentID))
		if index < 0 {
			return commonresponse.NewError(
				commonresponse.StatusCodeNotFound,
				fmt.Sprintf("environment not found: %s", environmentID),
				"List the available Environments and retry with an existing Environment ID or name.",
			)
		}
		if file.Environments[index].ID == constants.BaseEnvironmentID {
			return commonresponse.NewError(
				commonresponse.StatusCodeConflict,
				"base environment cannot be deleted",
				"Choose a non-base Environment to delete.",
			)
		}
		deleted = cloneEnvironment(file.Environments[index])
		file.Environments = append(file.Environments[:index], file.Environments[index+1:]...)
		return nil
	})
	if err != nil {
		return model.Environment{}, fmt.Errorf("delete environment: %w", err)
	}
	return deleted, nil
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
				fmt.Sprintf("environment not found: %s", identifier),
				"List the available Environments and retry with an existing Environment ID or name.",
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
					fmt.Sprintf("environment not found: %s", identifier),
					"List the available Environments and retry with an existing Environment ID or name.",
				)
			}
			if _, exists := seen[file.Environments[storedIndex].ID]; exists {
				return commonresponse.NewError(
					commonresponse.StatusCodeInvalidArgument,
					fmt.Sprintf("environment %q was supplied more than once", identifier),
					"Provide each target Environment only once.",
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
			if next.Source != previous.Source || next.ID != previous.ID ||
				next.Name != previous.Name || next.Revision != previous.Revision {
				return commonresponse.NewError(
					commonresponse.StatusCodeInvalidArgument,
					"environment update cannot change source, id, name, or revision",
					"Retry without changing the Environment source, ID, name, or revision.",
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

// GetSessionEnvironment returns the Environment ID selected by one Session.
func (d *EnvironmentDAL) GetSessionEnvironment(
	ctx context.Context,
	sessionID string,
) (model.EnvironmentID, bool, error) {
	var environmentID model.EnvironmentID
	found := false
	err := d.withLockedSessionFile(ctx, false, func(file *sessionStoreFile) error {
		for _, binding := range file.Bindings {
			if binding.SessionID == sessionID {
				environmentID = binding.EnvironmentID
				found = true
				break
			}
		}
		return nil
	})
	if err != nil {
		return "", false, fmt.Errorf("get session binding: %w", err)
	}
	return environmentID, found, nil
}

// SetSessionEnvironment creates or replaces one Session binding.
func (d *EnvironmentDAL) SetSessionEnvironment(
	ctx context.Context,
	sessionID string,
	environmentID model.EnvironmentID,
) error {
	if sessionID == "" {
		return errors.New("set session binding: session id must not be empty")
	}
	if strings.TrimSpace(string(environmentID)) == "" {
		return errors.New("set session binding: environment id must not be empty")
	}
	err := d.withLockedSessionFile(ctx, true, func(file *sessionStoreFile) error {
		for index := range file.Bindings {
			if file.Bindings[index].SessionID == sessionID {
				file.Bindings[index].EnvironmentID = environmentID
				return nil
			}
		}
		file.Bindings = append(file.Bindings, sessionBinding{
			SessionID: sessionID, EnvironmentID: environmentID,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("set session binding: %w", err)
	}
	return nil
}

// LeaveSessionEnvironment moves one Session toward base without overwriting a concurrent selection.
func (d *EnvironmentDAL) LeaveSessionEnvironment(
	ctx context.Context,
	sessionID string,
	expectedID model.EnvironmentID,
	baseID model.EnvironmentID,
) (model.EnvironmentID, bool, error) {
	if sessionID == "" {
		return "", false, errors.New("leave session environment: session id must not be empty")
	}
	var resolvedID model.EnvironmentID
	active := false
	err := d.withLockedSessionFile(ctx, true, func(file *sessionStoreFile) error {
		index := -1
		for bindingIndex, binding := range file.Bindings {
			if binding.SessionID == sessionID {
				index = bindingIndex
				break
			}
		}
		if index < 0 {
			return nil
		}

		currentID := file.Bindings[index].EnvironmentID
		if expectedID != "" && currentID != expectedID {
			resolvedID = currentID
			active = true
			return nil
		}
		if currentID == baseID {
			file.Bindings = append(file.Bindings[:index], file.Bindings[index+1:]...)
			return nil
		}

		file.Bindings[index].EnvironmentID = baseID
		resolvedID = baseID
		active = true
		return nil
	})
	if err != nil {
		return "", false, fmt.Errorf("leave session environment: %w", err)
	}
	return resolvedID, active, nil
}

func (d *EnvironmentDAL) withLockedFile(
	ctx context.Context,
	write bool,
	operation func(*storeFile) error,
) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(d.environmentPath), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	fileLock := flock.New(d.environmentPath+".lock", flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(0o600))
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

func (d *EnvironmentDAL) withLockedSessionFile(
	ctx context.Context,
	write bool,
	operation func(*sessionStoreFile) error,
) (returnErr error) {
	if err := os.MkdirAll(filepath.Dir(d.sessionPath), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	fileLock := flock.New(d.sessionPath+".lock", flock.SetFlag(os.O_CREATE|os.O_RDWR), flock.SetPermissions(0o600))
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

	file, exists, err := d.readSessionFile()
	if err != nil {
		return err
	}
	if err := operation(&file); err != nil {
		return err
	}
	if !write || (!exists && len(file.Bindings) == 0) {
		return nil
	}
	return writeSessionFile(d.sessionPath, file)
}

func (d *EnvironmentDAL) readSessionFile() (sessionStoreFile, bool, error) {
	content, err := os.ReadFile(d.sessionPath)
	if errors.Is(err, fs.ErrNotExist) {
		return sessionStoreFile{
			SchemaVersion: constants.SessionStoreSchemaVersion,
			Bindings:      []sessionBinding{},
		}, false, nil
	}
	if err != nil {
		return sessionStoreFile{}, false, fmt.Errorf("read store: %w", err)
	}

	var file sessionStoreFile
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return sessionStoreFile{}, false, fmt.Errorf("decode store: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return sessionStoreFile{}, false, errors.New("decode store: expected exactly one JSON object")
	}
	if file.SchemaVersion != constants.SessionStoreSchemaVersion {
		return sessionStoreFile{}, false, fmt.Errorf("unsupported store schema version: %d", file.SchemaVersion)
	}
	if file.Bindings == nil {
		return sessionStoreFile{}, false, errors.New("decode store: bindings must be an array")
	}

	seen := make(map[string]struct{}, len(file.Bindings))
	for _, binding := range file.Bindings {
		if binding.SessionID == "" {
			return sessionStoreFile{}, false, errors.New("invalid store: session id must not be empty")
		}
		if strings.TrimSpace(string(binding.EnvironmentID)) == "" {
			return sessionStoreFile{}, false, fmt.Errorf(
				"invalid store: session %q environment id must not be empty",
				binding.SessionID,
			)
		}
		if _, exists := seen[binding.SessionID]; exists {
			return sessionStoreFile{}, false, fmt.Errorf("invalid store: duplicate session id %q", binding.SessionID)
		}
		seen[binding.SessionID] = struct{}{}
	}
	return file, true, nil
}

func writeSessionFile(path string, file sessionStoreFile) error {
	sort.Slice(file.Bindings, func(i, j int) bool {
		return file.Bindings[i].SessionID < file.Bindings[j].SessionID
	})
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	content = append(content, '\n')
	if err := atomic.WriteFile(path, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func (d *EnvironmentDAL) readFile() (storeFile, bool, error) {
	content, err := os.ReadFile(d.environmentPath)
	if errors.Is(err, fs.ErrNotExist) {
		return storeFile{
			SchemaVersion: constants.EnvironmentStoreSchemaVersion,
			Environments:  []model.Environment{d.baseEnvironment()},
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
		if file.Environments[index].Source == "" {
			file.Environments[index].Source = d.source
			dirty = true
		}
		if file.Environments[index].ID == constants.LegacyBaseEnvironmentID &&
			file.Environments[index].Name == constants.BaseEnvironmentName {
			file.Environments[index].ID = constants.BaseEnvironmentID
			dirty = true
		}
	}
	seenIDs := make(map[model.EnvironmentID]struct{}, len(file.Environments))
	seenNames := make(map[string]struct{}, len(file.Environments))
	for _, environment := range file.Environments {
		if environment.Source != d.source {
			return storeFile{}, false, fmt.Errorf(
				"invalid store: environment %q source %q does not match %q",
				environment.Name,
				environment.Source,
				d.source,
			)
		}
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
		file.Environments = []model.Environment{d.baseEnvironment()}
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

func (d *EnvironmentDAL) baseEnvironment() model.Environment {
	environment := cloneEnvironment(baseEnvironment)
	environment.Source = d.source
	return environment
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
	if err := atomic.WriteFile(d.environmentPath, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	return nil
}

func validateEnvironment(environment model.Environment) error {
	if !environment.Source.Valid() {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("unsupported environment source %q", environment.Source),
			"Use a supported Coding Agent adapter to manage this Environment.",
		)
	}
	if strings.TrimSpace(string(environment.ID)) == "" {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment id must not be empty",
			"Retry the operation. If it still fails, inspect the hev logs.",
		)
	}
	if !keyPattern.MatchString(environment.Name) {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("invalid environment name %q", environment.Name),
			"Use a lowercase kebab-case Environment name such as \"coding-tools\".",
		)
	}
	if environment.Revision == 0 {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("environment %q revision must be greater than zero", environment.Name),
			"Repair the persisted Environment revision, then retry.",
		)
	}
	if environment.Skills == nil {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("environment %q skills must be an array", environment.Name),
			"Repair the persisted Environment Skills list, then retry.",
		)
	}

	seenSkills := make(map[model.SkillKey]struct{}, len(environment.Skills))
	for _, skill := range environment.Skills {
		if !keyPattern.MatchString(string(skill.SkillKey)) {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("invalid skill key %q", skill.SkillKey),
				"Use a lowercase kebab-case Skill key such as \"code-review\".",
			)
		}
		if skill.Policy.Kind != constants.SkillPolicyAuto && skill.Policy.Kind != constants.SkillPolicyOff {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("skill %q: unsupported skill policy: %s", skill.SkillKey, skill.Policy.Kind),
				"Use policy auto or off.",
			)
		}
		if _, exists := seenSkills[skill.SkillKey]; exists {
			return commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("environment %q contains duplicate skill %q", environment.Name, skill.SkillKey),
				"Remove the duplicate Skill binding from the Environment, then retry.",
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
