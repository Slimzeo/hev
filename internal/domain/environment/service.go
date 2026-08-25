package environment

import (
	"context"
	"fmt"
)

// Store persists the current Environment records used by Service.
type Store interface {
	Create(context.Context, Environment) (Environment, error)
	Default(context.Context) (Environment, error)
	GetByIDOrName(context.Context, string) (Environment, error)
	UpdateMany(context.Context, []string, func([]Environment) error) ([]Environment, error)
}

// IDGenerator supplies stable Environment IDs.
type IDGenerator func() EnvironmentID

// Service implements Environment operations independent of any CLI or host.
type Service struct {
	store Store
	newID IDGenerator
}

// NewService constructs an Environment service.
func NewService(store Store, newID IDGenerator) *Service {
	return &Service{store: store, newID: newID}
}

// Create creates an empty Environment at revision one.
func (s *Service) Create(ctx context.Context, name string) (Environment, error) {
	if err := ValidateName(name); err != nil {
		return Environment{}, err
	}
	created := Environment{
		ID:       s.newID(),
		Name:     name,
		Revision: 1,
		Skills:   []EnvironmentSkillSpec{},
	}
	if err := created.Validate(); err != nil {
		return Environment{}, err
	}
	return s.store.Create(ctx, created)
}

// AddSkill binds one logical Skill to all named Environments atomically.
func (s *Service) AddSkill(
	ctx context.Context,
	skillKey SkillKey,
	environmentNames []string,
	policy EnvironmentSkillPolicy,
) (EnvironmentSkillSpec, []Environment, error) {
	if err := ValidateSkillKey(skillKey); err != nil {
		return EnvironmentSkillSpec{}, nil, err
	}
	if len(environmentNames) == 0 {
		return EnvironmentSkillSpec{}, nil, NewError(StatusCodeInvalidArgument, "at least one --env is required")
	}
	if err := policy.Validate(); err != nil {
		return EnvironmentSkillSpec{}, nil, err
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if err := ValidateName(name); err != nil {
			return EnvironmentSkillSpec{}, nil, err
		}
		if _, exists := seen[name]; exists {
			return EnvironmentSkillSpec{}, nil, NewError(StatusCodeInvalidArgument, "environment %q was supplied more than once", name)
		}
		seen[name] = struct{}{}
	}

	spec := EnvironmentSkillSpec{SkillKey: skillKey, Policy: policy}
	environments, err := s.store.UpdateMany(ctx, environmentNames, func(environments []Environment) error {
		for _, current := range environments {
			for _, existing := range current.Skills {
				if existing.SkillKey == spec.SkillKey {
					return NewError(
						StatusCodeConflict,
						"skill %q is already bound to environment %q",
						spec.SkillKey, current.Name,
					)
				}
			}
		}
		for index := range environments {
			environments[index].Skills = append(environments[index].Skills, spec)
		}
		return nil
	})
	if err != nil {
		return EnvironmentSkillSpec{}, nil, fmt.Errorf("add skill %q: %w", skillKey, err)
	}
	return spec, environments, nil
}

// Default returns the latest default Environment.
func (s *Service) Default(ctx context.Context) (Environment, error) {
	current, err := s.store.Default(ctx)
	if err != nil {
		return Environment{}, fmt.Errorf("resolve default environment: %w", err)
	}
	return current, nil
}

// Resolve returns the latest Environment for an ID or name.
func (s *Service) Resolve(ctx context.Context, identifier string) (Environment, error) {
	if identifier == "" {
		return Environment{}, NewError(StatusCodeInvalidArgument, "environment identifier must not be empty")
	}

	current, err := s.store.GetByIDOrName(ctx, identifier)
	if err != nil {
		return Environment{}, fmt.Errorf("resolve environment: %w", err)
	}
	return current, nil
}
