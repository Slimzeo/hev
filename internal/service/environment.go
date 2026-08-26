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

// Store persists the current Environment records used by Service.
type Store interface {
	Create(context.Context, model.Environment) (model.Environment, error)
	Default(context.Context) (model.Environment, error)
	List(context.Context) ([]model.Environment, error)
	GetByIDOrName(context.Context, string) (model.Environment, error)
	UpdateMany(context.Context, []string, func([]model.Environment) error) ([]model.Environment, error)
}

// IDGenerator supplies stable Environment IDs.
type IDGenerator func() model.EnvironmentID

// Service implements Environment operations independent of any CLI or host.
type Service struct {
	store Store
	newID IDGenerator
}

// New constructs an Environment service.
func New(store Store, newID IDGenerator) *Service {
	return &Service{store: store, newID: newID}
}

// Create creates an Environment with the default guide Skill at revision one.
func (s *Service) Create(ctx context.Context, name string) (model.Environment, error) {
	if !keyPattern.MatchString(name) {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"invalid environment name %q: use lowercase kebab-case",
			name,
		)
	}
	id := s.newID()
	if strings.TrimSpace(string(id)) == "" {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment id must not be empty",
		)
	}
	created := model.Environment{
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

// Default returns the latest default Environment.
func (s *Service) Default(ctx context.Context) (model.Environment, error) {
	current, err := s.store.Default(ctx)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve default environment: %w", err)
	}
	return current, nil
}

// List returns all current Environments.
func (s *Service) List(ctx context.Context) ([]model.Environment, error) {
	environments, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	return environments, nil
}

// Resolve returns the latest Environment for an ID or name.
func (s *Service) Resolve(ctx context.Context, identifier string) (model.Environment, error) {
	if identifier == "" {
		return model.Environment{}, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"environment identifier must not be empty",
		)
	}

	current, err := s.store.GetByIDOrName(ctx, identifier)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve environment: %w", err)
	}
	return current, nil
}
