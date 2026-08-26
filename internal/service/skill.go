package service

import (
	"context"
	"fmt"

	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/model"
)

// AddSkill binds one logical Skill to all named Environments atomically.
func (s *Service) AddSkill(
	ctx context.Context,
	skill model.Skill,
	environmentNames []string,
	policy model.EnvironmentSkillPolicy,
) (model.EnvironmentSkill, []model.Environment, error) {
	if !keyPattern.MatchString(string(skill.Key)) {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"invalid skill key %q: use lowercase kebab-case",
			skill.Key,
		)
	}
	if len(environmentNames) == 0 {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"at least one environment is required",
		)
	}
	if policy.Kind != constants.SkillPolicyAuto && policy.Kind != constants.SkillPolicyOff {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"unsupported skill policy: %s",
			policy.Kind,
		)
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if !keyPattern.MatchString(name) {
			return model.EnvironmentSkill{}, nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				"invalid environment name %q: use lowercase kebab-case",
				name,
			)
		}
		if _, exists := seen[name]; exists {
			return model.EnvironmentSkill{}, nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				"environment %q was supplied more than once",
				name,
			)
		}
		seen[name] = struct{}{}
	}

	binding := model.EnvironmentSkill{SkillKey: skill.Key, Policy: policy}
	environments, err := s.store.UpdateMany(ctx, environmentNames, func(environments []model.Environment) error {
		for _, current := range environments {
			for _, existing := range current.Skills {
				if existing.SkillKey == binding.SkillKey {
					return commonresponse.NewError(
						commonresponse.StatusCodeConflict,
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
