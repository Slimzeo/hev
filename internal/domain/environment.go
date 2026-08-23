package domain

import (
	"fmt"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// EnvironmentID is the stable identifier of an environment.
type EnvironmentID string

// SkillKey is HEV's host-neutral identifier for a skill.
type SkillKey string

// EnvironmentSkillSpec describes how an environment exposes one skill.
type EnvironmentSkillSpec struct {
	SkillKey SkillKey               `json:"skillKey"`
	Policy   EnvironmentSkillPolicy `json:"policy"`
}

// Environment is the persisted aggregate managed by HEV.
type Environment struct {
	ID       EnvironmentID          `json:"id"`
	Name     string                 `json:"name"`
	Revision uint64                 `json:"revision"`
	Skills   []EnvironmentSkillSpec `json:"skills"`
}

// ResolvedEnvironmentSnapshot is an ordered, validated view of current environments.
type ResolvedEnvironmentSnapshot struct {
	Environments []Environment `json:"environments"`
}

// ValidateEnvironmentName applies the public environment-name grammar.
func ValidateEnvironmentName(name string) error {
	if !keyPattern.MatchString(name) {
		return NewError(ErrorCodeInvalidArgument, "invalid environment name %q: use lowercase kebab-case", name)
	}
	return nil
}

// ValidateSkillKey applies the host-neutral skill-key grammar.
func ValidateSkillKey(key SkillKey) error {
	if !keyPattern.MatchString(string(key)) {
		return NewError(ErrorCodeInvalidArgument, "invalid skill key %q: use lowercase kebab-case", key)
	}
	return nil
}

// Validate verifies the invariants owned by an environment.
func (e Environment) Validate() error {
	if strings.TrimSpace(string(e.ID)) == "" {
		return NewError(ErrorCodeInvalidArgument, "environment id must not be empty")
	}
	if err := ValidateEnvironmentName(e.Name); err != nil {
		return err
	}
	if e.Revision == 0 {
		return NewError(ErrorCodeInvalidArgument, "environment %q revision must be greater than zero", e.Name)
	}
	if e.Skills == nil {
		return NewError(ErrorCodeInvalidArgument, "environment %q skills must be an array", e.Name)
	}

	seen := make(map[SkillKey]struct{}, len(e.Skills))
	for _, skill := range e.Skills {
		if err := ValidateSkillKey(skill.SkillKey); err != nil {
			return err
		}
		if err := skill.Policy.Validate(); err != nil {
			return fmt.Errorf("skill %q: %w", skill.SkillKey, err)
		}
		if _, exists := seen[skill.SkillKey]; exists {
			return NewError(ErrorCodeInvalidArgument, "environment %q contains duplicate skill %q", e.Name, skill.SkillKey)
		}
		seen[skill.SkillKey] = struct{}{}
	}
	return nil
}

// ResolveEnvironmentSnapshot validates an ordered environment group and rejects duplicate skills.
func ResolveEnvironmentSnapshot(environments []Environment) (ResolvedEnvironmentSnapshot, error) {
	if len(environments) == 0 {
		return ResolvedEnvironmentSnapshot{}, NewError(ErrorCodeInvalidArgument, "at least one environment is required")
	}

	seenEnvironments := make(map[EnvironmentID]struct{}, len(environments))
	skillOwners := make(map[SkillKey]string)
	resolved := make([]Environment, len(environments))
	for index, environment := range environments {
		if err := environment.Validate(); err != nil {
			return ResolvedEnvironmentSnapshot{}, err
		}
		if _, exists := seenEnvironments[environment.ID]; exists {
			return ResolvedEnvironmentSnapshot{}, NewError(ErrorCodeInvalidArgument, "environment %q was selected more than once", environment.Name)
		}
		seenEnvironments[environment.ID] = struct{}{}
		for _, skill := range environment.Skills {
			if owner, exists := skillOwners[skill.SkillKey]; exists {
				return ResolvedEnvironmentSnapshot{}, NewError(
					ErrorCodeSkillConflict,
					"skill %q is configured by both environment %q and %q",
					skill.SkillKey, owner, environment.Name,
				)
			}
			skillOwners[skill.SkillKey] = environment.Name
		}
		resolved[index] = cloneEnvironment(environment)
	}

	return ResolvedEnvironmentSnapshot{Environments: resolved}, nil
}

// CloneEnvironment returns a copy whose skill slice does not alias the source.
func CloneEnvironment(environment Environment) Environment {
	return cloneEnvironment(environment)
}

func cloneEnvironment(environment Environment) Environment {
	skills := make([]EnvironmentSkillSpec, len(environment.Skills))
	copy(skills, environment.Skills)
	environment.Skills = skills
	return environment
}
