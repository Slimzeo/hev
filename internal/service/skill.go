package service

import (
	"context"
	"fmt"

	"github.com/Slimzeo/hev/internal/model"
)

// AddSkill binds one logical Skill to all named Environments atomically.
func (s *Service) AddSkill(
	ctx context.Context,
	skill model.Skill,
	environmentNames []string,
	policy model.EnvironmentSkillPolicy,
) (model.EnvironmentSkill, []model.Environment, error) {
	if err := skill.Validate(); err != nil {
		return model.EnvironmentSkill{}, nil, err
	}
	if len(environmentNames) == 0 {
		return model.EnvironmentSkill{}, nil, model.NewError(model.StatusCodeInvalidArgument, "at least one --env is required")
	}
	if err := policy.Validate(); err != nil {
		return model.EnvironmentSkill{}, nil, err
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if err := model.ValidateName(name); err != nil {
			return model.EnvironmentSkill{}, nil, err
		}
		if _, exists := seen[name]; exists {
			return model.EnvironmentSkill{}, nil, model.NewError(model.StatusCodeInvalidArgument, "environment %q was supplied more than once", name)
		}
		seen[name] = struct{}{}
	}

	binding := model.EnvironmentSkill{SkillKey: skill.Key, Policy: policy}
	environments, err := s.store.UpdateMany(ctx, environmentNames, func(environments []model.Environment) error {
		for _, current := range environments {
			for _, existing := range current.Skills {
				if existing.SkillKey == binding.SkillKey {
					return model.NewError(
						model.StatusCodeConflict,
						"skill %q is already bound to environment %q",
						binding.SkillKey, current.Name,
					)
				}
			}
		}
		for index := range environments {
			environments[index].Skills = append(environments[index].Skills, binding)
		}
		return nil
	})
	if err != nil {
		return model.EnvironmentSkill{}, nil, fmt.Errorf("add skill %q: %w", skill.Key, err)
	}
	return binding, environments, nil
}
