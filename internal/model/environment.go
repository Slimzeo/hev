package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// StatusCode is the numeric status returned by the public CLI contract.
type StatusCode int

const (
	StatusCodeOK              StatusCode = 200
	StatusCodeInvalidArgument StatusCode = 400
	StatusCodeNotFound        StatusCode = 404
	StatusCodeConflict        StatusCode = 409
	StatusCodeInternal        StatusCode = 500
)

// Error reports an Environment domain failure with a numeric status.
type Error struct {
	StatusCode StatusCode
	Message    string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError constructs an Environment domain error with a numeric status.
func NewError(statusCode StatusCode, format string, args ...any) error {
	return &Error{StatusCode: statusCode, Message: fmt.Sprintf(format, args...)}
}

// StatusCodeOf returns the numeric status carried by err.
func StatusCodeOf(err error) (StatusCode, bool) {
	var domainError *Error
	if !errors.As(err, &domainError) {
		return 0, false
	}
	return domainError.StatusCode, true
}

// EnvironmentID is the stable identifier of an Environment.
type EnvironmentID string

// Environment is the persisted aggregate managed by hev.
type Environment struct {
	ID       EnvironmentID      `json:"id"`
	Name     string             `json:"name"`
	Revision uint64             `json:"revision"`
	Skills   []EnvironmentSkill `json:"skills"`
}

// ValidateName applies the public Environment name grammar.
func ValidateName(name string) error {
	if !keyPattern.MatchString(name) {
		return NewError(StatusCodeInvalidArgument, "invalid environment name %q: use lowercase kebab-case", name)
	}
	return nil
}

// Validate verifies the invariants owned by an Environment.
func (e Environment) Validate() error {
	if strings.TrimSpace(string(e.ID)) == "" {
		return NewError(StatusCodeInvalidArgument, "environment id must not be empty")
	}
	if err := ValidateName(e.Name); err != nil {
		return err
	}
	if e.Revision == 0 {
		return NewError(StatusCodeInvalidArgument, "environment %q revision must be greater than zero", e.Name)
	}
	if e.Skills == nil {
		return NewError(StatusCodeInvalidArgument, "environment %q skills must be an array", e.Name)
	}

	seen := make(map[SkillKey]struct{}, len(e.Skills))
	for _, skill := range e.Skills {
		if err := (Skill{Key: skill.SkillKey}).Validate(); err != nil {
			return err
		}
		if err := skill.Policy.Validate(); err != nil {
			return fmt.Errorf("skill %q: %w", skill.SkillKey, err)
		}
		if _, exists := seen[skill.SkillKey]; exists {
			return NewError(StatusCodeInvalidArgument, "environment %q contains duplicate skill %q", e.Name, skill.SkillKey)
		}
		seen[skill.SkillKey] = struct{}{}
	}
	return nil
}

// Clone returns a copy whose Skill slice does not alias the source.
func Clone(value Environment) Environment {
	skills := make([]EnvironmentSkill, len(value.Skills))
	copy(skills, value.Skills)
	value.Skills = skills
	return value
}
