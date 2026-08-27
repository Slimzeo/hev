package service

import (
	"context"
	"fmt"

	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/model"
)

// AddSkill binds one logical Skill to all named Environments atomically.
func (s *EnvironmentService) AddSkill(
	ctx context.Context,
	skill model.Skill,
	environmentNames []string,
	policy model.EnvironmentSkillPolicy,
) (model.EnvironmentSkill, []model.Environment, error) {
	if !keyPattern.MatchString(string(skill.Key)) {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("invalid skill key %q", skill.Key),
			"Use a lowercase kebab-case Skill key such as \"code-review\".",
		)
	}
	if len(environmentNames) == 0 {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"at least one environment is required",
			"Provide at least one target Environment name after listing the available Environments.",
		)
	}
	if policy.Kind != constants.SkillPolicyAuto && policy.Kind != constants.SkillPolicyOff {
		return model.EnvironmentSkill{}, nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("unsupported skill policy: %s", policy.Kind),
			"Retry with --policy auto or --policy off.",
		)
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if !keyPattern.MatchString(name) {
			return model.EnvironmentSkill{}, nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("invalid environment name %q", name),
				"Use lowercase kebab-case Environment names such as \"coding-tools\".",
			)
		}
		if _, exists := seen[name]; exists {
			return model.EnvironmentSkill{}, nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("environment %q was supplied more than once", name),
				"Provide each target Environment only once.",
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
						fmt.Sprintf("skill %q is already bound to environment %q", binding.SkillKey, current.Name),
						"The Skill is already configured for this Environment; do not add the same binding again.",
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

// RemoveSkill removes one logical Skill from all named Environments atomically.
func (s *EnvironmentService) RemoveSkill(
	ctx context.Context,
	skill model.Skill,
	environmentNames []string,
) ([]model.Environment, error) {
	if !keyPattern.MatchString(string(skill.Key)) {
		return nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			fmt.Sprintf("invalid skill key %q", skill.Key),
			"Use a lowercase kebab-case Skill key such as code-review.",
		)
	}
	if len(environmentNames) == 0 {
		return nil, commonresponse.NewError(
			commonresponse.StatusCodeInvalidArgument,
			"at least one environment is required",
			"Provide at least one target Environment name after listing the available Environments.",
		)
	}

	seen := make(map[string]struct{}, len(environmentNames))
	for _, name := range environmentNames {
		if !keyPattern.MatchString(name) {
			return nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("invalid environment name %q", name),
				"Use lowercase kebab-case Environment names such as coding-tools.",
			)
		}
		if _, exists := seen[name]; exists {
			return nil, commonresponse.NewError(
				commonresponse.StatusCodeInvalidArgument,
				fmt.Sprintf("environment %q was supplied more than once", name),
				"Provide each target Environment only once.",
			)
		}
		seen[name] = struct{}{}
	}

	environments, err := s.store.UpdateMany(ctx, environmentNames, func(environments []model.Environment) error {
		indexes := make([]int, len(environments))
		for environmentIndex, environment := range environments {
			if environment.ID == constants.BaseEnvironmentID && skill.Key == constants.DefaultGuideSkillKey {
				return commonresponse.NewError(
					commonresponse.StatusCodeConflict,
					"hev-guide cannot be removed from base",
					"Keep hev-guide enabled in base, or remove it only from a non-base Environment.",
				)
			}
			indexes[environmentIndex] = -1
			for skillIndex, existing := range environment.Skills {
				if existing.SkillKey == skill.Key {
					indexes[environmentIndex] = skillIndex
					break
				}
			}
			if indexes[environmentIndex] < 0 {
				return commonresponse.NewError(
					commonresponse.StatusCodeNotFound,
					fmt.Sprintf("skill %q is not bound to environment %q", skill.Key, environment.Name),
					"List the Environment's Skills and retry with an existing binding.",
				)
			}
		}
		for environmentIndex, skillIndex := range indexes {
			skills := environments[environmentIndex].Skills
			environments[environmentIndex].Skills = append(skills[:skillIndex], skills[skillIndex+1:]...)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("remove skill %q: %w", skill.Key, err)
	}
	return environments, nil
}
