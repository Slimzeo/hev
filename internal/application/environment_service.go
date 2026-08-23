package application

import (
	"context"
	"fmt"

	"github.com/Slimzeo/hev/internal/domain"
)

// IDGenerator supplies stable Environment IDs.
type IDGenerator func() domain.EnvironmentID

// EnvironmentService implements host-neutral Environment use cases.
type EnvironmentService struct {
	store EnvironmentStore
	newID IDGenerator
}

// NewEnvironmentService constructs the Environment application service.
func NewEnvironmentService(store EnvironmentStore, newID IDGenerator) *EnvironmentService {
	return &EnvironmentService{store: store, newID: newID}
}

// CreateEnvironment creates an empty Environment at revision one.
func (s *EnvironmentService) CreateEnvironment(ctx context.Context, name string) (domain.Environment, error) {
	if err := domain.ValidateEnvironmentName(name); err != nil {
		return domain.Environment{}, err
	}
	environment := domain.Environment{
		ID:       s.newID(),
		Name:     name,
		Revision: 1,
		Skills:   []domain.EnvironmentSkillSpec{},
	}
	if err := environment.Validate(); err != nil {
		return domain.Environment{}, err
	}
	return s.store.Create(ctx, environment)
}

// AddEnvironmentSkill binds one logical skill to all named Environments atomically.
func (s *EnvironmentService) AddEnvironmentSkill(
	ctx context.Context,
	skillKey domain.SkillKey,
	environmentNames []string,
	policy domain.EnvironmentSkillPolicy,
) (domain.EnvironmentSkillSpec, []domain.Environment, error) {
	if err := domain.ValidateSkillKey(skillKey); err != nil {
		return domain.EnvironmentSkillSpec{}, nil, err
	}
	if len(environmentNames) == 0 {
		return domain.EnvironmentSkillSpec{}, nil, domain.NewError(domain.ErrorCodeInvalidArgument, "at least one --env is required")
	}
	if err := policy.Validate(); err != nil {
		return domain.EnvironmentSkillSpec{}, nil, err
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if err := domain.ValidateEnvironmentName(name); err != nil {
			return domain.EnvironmentSkillSpec{}, nil, err
		}
		if _, exists := seen[name]; exists {
			return domain.EnvironmentSkillSpec{}, nil, domain.NewError(domain.ErrorCodeInvalidArgument, "environment %q was supplied more than once", name)
		}
		seen[name] = struct{}{}
	}

	spec := domain.EnvironmentSkillSpec{SkillKey: skillKey, Policy: policy}
	environments, err := s.store.UpdateMany(ctx, environmentNames, func(environments []domain.Environment) error {
		for _, environment := range environments {
			for _, existing := range environment.Skills {
				if existing.SkillKey == spec.SkillKey {
					return domain.NewError(
						domain.ErrorCodeSkillAlreadyBound,
						"skill %q is already bound to environment %q",
						spec.SkillKey, environment.Name,
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
		return domain.EnvironmentSkillSpec{}, nil, fmt.Errorf("add skill %q: %w", skillKey, err)
	}
	return spec, environments, nil
}

// ResolveEnvironmentGroup returns the latest Environment records in input order.
func (s *EnvironmentService) ResolveEnvironmentGroup(ctx context.Context, identifiers []string) (domain.ResolvedEnvironmentSnapshot, error) {
	if len(identifiers) == 0 {
		return domain.ResolvedEnvironmentSnapshot{}, domain.NewError(domain.ErrorCodeInvalidArgument, "at least one environment is required")
	}

	for _, identifier := range identifiers {
		if identifier == "" {
			return domain.ResolvedEnvironmentSnapshot{}, domain.NewError(domain.ErrorCodeInvalidArgument, "environment identifier must not be empty")
		}
	}

	environments, err := s.store.GetManyByIDOrName(ctx, identifiers)
	if err != nil {
		return domain.ResolvedEnvironmentSnapshot{}, fmt.Errorf("resolve environments: %w", err)
	}
	return domain.ResolveEnvironmentSnapshot(environments)
}
