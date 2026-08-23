package domain_test

import (
	"strings"
	"testing"

	"github.com/Slimzeo/hev/internal/domain"
)

func TestEnvironmentValidate_BitsUT(t *testing.T) {
	valid := domain.Environment{
		ID:       "env-1",
		Name:     "alpha-env",
		Revision: 1,
		Skills: []domain.EnvironmentSkillSpec{
			{SkillKey: "search", Policy: domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}},
		},
	}

	tests := []struct {
		name        string
		environment domain.Environment
		wantCode    domain.ErrorCode
	}{
		{name: "valid aggregate", environment: valid},
		{name: "empty id", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.ID = " \t" }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "invalid name", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.Name = "Alpha_Env" }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "zero revision", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.Revision = 0 }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "nil skills", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.Skills = nil }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "invalid skill key", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.Skills[0].SkillKey = "Bad Skill" }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "unsupported policy", environment: replaceEnvironment(valid, func(environment *domain.Environment) { environment.Skills[0].Policy.Kind = "always" }), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "duplicate skill", environment: replaceEnvironment(valid, func(environment *domain.Environment) {
			environment.Skills = append(environment.Skills, environment.Skills[0])
		}), wantCode: domain.ErrorCodeInvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.environment.Validate()
			assertDomainErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestResolveEnvironmentSnapshot_BitsUT(t *testing.T) {
	auto := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}
	off := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyOff}

	tests := []struct {
		name             string
		environments     []domain.Environment
		wantIDs          []domain.EnvironmentID
		wantCode         domain.ErrorCode
		wantMessageParts []string
	}{
		{
			name: "preserves caller order",
			environments: []domain.Environment{
				newEnvironment("env-beta", "beta", domain.EnvironmentSkillSpec{SkillKey: "search", Policy: auto}),
				newEnvironment("env-alpha", "alpha", domain.EnvironmentSkillSpec{SkillKey: "edit", Policy: off}),
			},
			wantIDs: []domain.EnvironmentID{"env-beta", "env-alpha"},
		},
		{
			name:             "requires an environment",
			wantCode:         domain.ErrorCodeInvalidArgument,
			wantMessageParts: []string{"at least one environment"},
		},
		{
			name: "rejects the same environment twice",
			environments: []domain.Environment{
				newEnvironment("env-shared", "alpha"),
				newEnvironment("env-shared", "beta"),
			},
			wantCode:         domain.ErrorCodeInvalidArgument,
			wantMessageParts: []string{"beta", "selected more than once"},
		},
		{
			name: "rejects a skill owned by two environments regardless of policy",
			environments: []domain.Environment{
				newEnvironment("env-alpha", "alpha", domain.EnvironmentSkillSpec{SkillKey: "search", Policy: auto}),
				newEnvironment("env-beta", "beta", domain.EnvironmentSkillSpec{SkillKey: "search", Policy: off}),
			},
			wantCode:         domain.ErrorCodeSkillConflict,
			wantMessageParts: []string{"search", "alpha", "beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := domain.ResolveEnvironmentSnapshot(tt.environments)
			assertDomainErrorCode(t, err, tt.wantCode)
			if err != nil {
				for _, part := range tt.wantMessageParts {
					if !strings.Contains(err.Error(), part) {
						t.Fatalf("error %q does not contain %q", err, part)
					}
				}
				return
			}

			if len(snapshot.Environments) != len(tt.wantIDs) {
				t.Fatalf("resolved %d environments, want %d", len(snapshot.Environments), len(tt.wantIDs))
			}
			for index, wantID := range tt.wantIDs {
				if gotID := snapshot.Environments[index].ID; gotID != wantID {
					t.Errorf("environment %d has id %q, want %q", index, gotID, wantID)
				}
			}

			tt.environments[0].Skills[0].SkillKey = "mutated-input"
			if got := snapshot.Environments[0].Skills[0].SkillKey; got != "search" {
				t.Fatalf("snapshot skill changed through input alias: got %q", got)
			}
		})
	}
}

func newEnvironment(id domain.EnvironmentID, name string, skills ...domain.EnvironmentSkillSpec) domain.Environment {
	return domain.Environment{
		ID:       id,
		Name:     name,
		Revision: 1,
		Skills:   append([]domain.EnvironmentSkillSpec{}, skills...),
	}
}

func replaceEnvironment(environment domain.Environment, replace func(*domain.Environment)) domain.Environment {
	environment = domain.CloneEnvironment(environment)
	replace(&environment)
	return environment
}

func assertDomainErrorCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected domain error %q, got nil", want)
	}
	got, ok := domain.ErrorCodeOf(err)
	if !ok {
		t.Fatalf("error %q has no domain code", err)
	}
	if got != want {
		t.Fatalf("domain error code = %q, want %q", got, want)
	}
}
