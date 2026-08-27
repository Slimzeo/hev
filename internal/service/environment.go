package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/model"
)

var keyPattern = regexp.MustCompile(constants.KebabCasePattern)

// EnvironmentStore persists the current Environment records.
type EnvironmentStore interface {
	Source() model.Source
	Create(context.Context, model.Environment) (model.Environment, error)
	Rename(context.Context, model.EnvironmentID, string) (model.Environment, error)
	Delete(context.Context, model.EnvironmentID) (model.Environment, error)
	Default(context.Context) (model.Environment, error)
	List(context.Context) ([]model.Environment, error)
	GetByIDOrName(context.Context, string) (model.Environment, error)
	UpdateMany(context.Context, []string, func([]model.Environment) error) ([]model.Environment, error)
	GetSessionEnvironment(context.Context, string) (model.EnvironmentID, bool, error)
	SetSessionEnvironment(context.Context, string, model.EnvironmentID) error
	LeaveSessionEnvironment(context.Context, string, model.EnvironmentID, model.EnvironmentID) (model.EnvironmentID, bool, error)
}

// IDGenerator supplies stable Environment IDs.
type IDGenerator func() model.EnvironmentID

// EnvironmentService implements Environment operations independent of any CLI or host.
type EnvironmentService struct {
	source model.Source
	store  EnvironmentStore
	newID  IDGenerator
}

// NewEnvironment constructs an Environment service.
func NewEnvironment(store EnvironmentStore, newID IDGenerator) *EnvironmentService {
	return &EnvironmentService{source: store.Source(), store: store, newID: newID}
}

// Create creates an Environment with the default guide Skill at revision one.
func (s *EnvironmentService) Create(ctx context.Context, name string) (model.Environment, error) {
	if !keyPattern.MatchString(name) {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("invalid environment name %q", name),
			"Use a lowercase kebab-case Environment name such as \"coding-tools\".",
		)
	}
	id := s.newID()
	if strings.TrimSpace(string(id)) == "" {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment id must not be empty",
			"Retry the operation. If it still fails, inspect the hev logs.",
		)
	}
	created := model.Environment{
		Source:   s.source,
		ID:       id,
		Name:     name,
		Revision: 1,
		Skills: []model.EnvironmentSkill{{
			SkillKey: constants.DefaultGuideSkillKey,
			Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
		}},
	}
	return s.store.Create(ctx, created)
}

// Rename changes one non-base Environment name while preserving its ID and bindings.
func (s *EnvironmentService) Rename(
	ctx context.Context,
	environmentRef string,
	newName string,
) (model.Environment, error) {
	if !keyPattern.MatchString(newName) {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("invalid environment name %q", newName),
			"Use a lowercase kebab-case Environment name such as coding-tools.",
		)
	}
	current, err := s.Resolve(ctx, environmentRef)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve environment before rename: %w", err)
	}
	if current.ID == constants.BaseEnvironmentID {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeConflict,
			"base environment cannot be renamed",
			"Choose a non-base Environment to rename.",
		)
	}
	renamed, err := s.store.Rename(ctx, current.ID, newName)
	if err != nil {
		return model.Environment{}, fmt.Errorf("rename environment: %w", err)
	}
	return renamed, nil
}

// Delete removes one non-base Environment.
func (s *EnvironmentService) Delete(ctx context.Context, environmentRef string) (model.Environment, error) {
	current, err := s.Resolve(ctx, environmentRef)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve environment before delete: %w", err)
	}
	if current.ID == constants.BaseEnvironmentID {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeConflict,
			"base environment cannot be deleted",
			"Choose a non-base Environment to delete.",
		)
	}
	deleted, err := s.store.Delete(ctx, current.ID)
	if err != nil {
		return model.Environment{}, fmt.Errorf("delete environment: %w", err)
	}
	return deleted, nil
}

// Default returns the latest default Environment.
func (s *EnvironmentService) Default(ctx context.Context) (model.Environment, error) {
	current, err := s.store.Default(ctx)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve default environment: %w", err)
	}
	return current, nil
}

// List returns all current Environments.
func (s *EnvironmentService) List(ctx context.Context) ([]model.Environment, error) {
	environments, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	return environments, nil
}

// Resolve returns the latest Environment for an ID or name.
func (s *EnvironmentService) Resolve(ctx context.Context, identifier string) (model.Environment, error) {
	if identifier == "" {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment identifier must not be empty",
			"Provide one Environment ID or name after listing the available Environments.",
		)
	}

	current, err := s.store.GetByIDOrName(ctx, identifier)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve environment: %w", err)
	}
	return current, nil
}

// Use selects one Environment for a host Session.
func (s *EnvironmentService) Use(
	ctx context.Context,
	sessionID string,
	environmentRef string,
) (model.Session, error) {
	if err := validateSessionID(sessionID); err != nil {
		return model.Session{}, err
	}

	environment, err := s.Resolve(ctx, environmentRef)
	if err != nil {
		return model.Session{}, fmt.Errorf("select environment for session: %w", err)
	}
	if err := s.store.SetSessionEnvironment(ctx, sessionID, environment.ID); err != nil {
		return model.Session{}, fmt.Errorf("persist session environment: %w", err)
	}
	return model.Session{Source: s.source, SessionID: sessionID, Environment: &environment}, nil
}

// Current resolves the latest Environment selected by a host Session.
func (s *EnvironmentService) Current(ctx context.Context, sessionID string) (model.Session, error) {
	if err := validateSessionID(sessionID); err != nil {
		return model.Session{}, err
	}

	environmentID, found, err := s.store.GetSessionEnvironment(ctx, sessionID)
	if err != nil {
		return model.Session{}, fmt.Errorf("read session environment: %w", err)
	}
	if !found {
		return model.Session{Source: s.source, SessionID: sessionID}, nil
	}
	environment, err := s.Resolve(ctx, string(environmentID))
	if err != nil {
		if statusCode, classified := commonresponse.StatusCodeOf(err); classified && statusCode == commonresponse.StatusCodeNotFound {
			base, baseErr := s.Default(ctx)
			if baseErr != nil {
				return model.Session{}, fmt.Errorf("resolve base environment for deleted session selection: %w", baseErr)
			}
			resolvedID, active, updateErr := s.store.LeaveSessionEnvironment(
				ctx, sessionID, environmentID, base.ID,
			)
			if updateErr != nil {
				return model.Session{}, fmt.Errorf("replace deleted session environment with base: %w", updateErr)
			}
			if !active {
				return model.Session{Source: s.source, SessionID: sessionID}, nil
			}
			if resolvedID == base.ID {
				return model.Session{Source: s.source, SessionID: sessionID, Environment: &base}, nil
			}
			replacement, resolveErr := s.Resolve(ctx, string(resolvedID))
			if resolveErr != nil {
				return model.Session{}, fmt.Errorf("resolve concurrent session environment: %w", resolveErr)
			}
			return model.Session{Source: s.source, SessionID: sessionID, Environment: &replacement}, nil
		}
		return model.Session{}, fmt.Errorf("resolve session environment: %w", err)
	}
	return model.Session{Source: s.source, SessionID: sessionID, Environment: &environment}, nil
}

// Quit leaves the current Environment tier for a host Session.
func (s *EnvironmentService) Quit(ctx context.Context, sessionID string) (model.Session, error) {
	if err := validateSessionID(sessionID); err != nil {
		return model.Session{}, err
	}
	selectedID, found, err := s.store.GetSessionEnvironment(ctx, sessionID)
	if err != nil {
		return model.Session{}, fmt.Errorf("read session environment before quit: %w", err)
	}
	if !found {
		return model.Session{Source: s.source, SessionID: sessionID}, nil
	}

	base, err := s.Default(ctx)
	if err != nil {
		return model.Session{}, fmt.Errorf("resolve base environment before quit: %w", err)
	}
	resolvedID, active, err := s.store.LeaveSessionEnvironment(ctx, sessionID, selectedID, base.ID)
	if err != nil {
		return model.Session{}, fmt.Errorf("quit session environment: %w", err)
	}
	if !active {
		return model.Session{Source: s.source, SessionID: sessionID}, nil
	}
	if resolvedID == base.ID {
		return model.Session{Source: s.source, SessionID: sessionID, Environment: &base}, nil
	}
	replacement, err := s.Resolve(ctx, string(resolvedID))
	if err != nil {
		return model.Session{}, fmt.Errorf("resolve concurrent session environment after quit: %w", err)
	}
	return model.Session{Source: s.source, SessionID: sessionID, Environment: &replacement}, nil
}

func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"session id must not be empty",
			"Pass the current host Session ID with --session-id.",
		)
	}
	return nil
}
