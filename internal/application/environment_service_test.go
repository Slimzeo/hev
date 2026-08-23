package application_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Slimzeo/hev/internal/application"
	"github.com/Slimzeo/hev/internal/domain"
)

func TestEnvironmentServiceCreateEnvironment_BitsUT(t *testing.T) {
	tests := []struct {
		name        string
		envName     string
		generatedID domain.EnvironmentID
		wantCode    domain.ErrorCode
		wantIDCalls int
		wantCreates int
	}{
		{name: "creates revision one with no skills", envName: "alpha-env", generatedID: "env-1", wantIDCalls: 1, wantCreates: 1},
		{name: "rejects invalid name before generating id", envName: "Alpha Env", generatedID: "env-1", wantCode: domain.ErrorCodeInvalidArgument},
		{name: "rejects empty generated id before persistence", envName: "alpha-env", generatedID: "", wantCode: domain.ErrorCodeInvalidArgument, wantIDCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubEnvironmentStore{}
			idCalls := 0
			service := application.NewEnvironmentService(store, func() domain.EnvironmentID {
				idCalls++
				return tt.generatedID
			})

			got, err := service.CreateEnvironment(context.Background(), tt.envName)
			requireDomainCode(t, err, tt.wantCode)
			if idCalls != tt.wantIDCalls {
				t.Errorf("ID generator called %d times, want %d", idCalls, tt.wantIDCalls)
			}
			if len(store.createCalls) != tt.wantCreates {
				t.Fatalf("store Create called %d times, want %d", len(store.createCalls), tt.wantCreates)
			}
			if tt.wantCode != "" {
				return
			}

			want := domain.Environment{
				ID:       tt.generatedID,
				Name:     tt.envName,
				Revision: 1,
				Skills:   []domain.EnvironmentSkillSpec{},
			}
			if !reflect.DeepEqual(store.createCalls[0], want) {
				t.Fatalf("persisted environment = %#v, want %#v", store.createCalls[0], want)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("created environment = %#v, want %#v", got, want)
			}
			if got.Skills == nil {
				t.Fatal("created environment has nil skills, want an empty array")
			}
		})
	}
}

func TestEnvironmentServiceCreateEnvironment_PropagatesStoreError_BitsUT(t *testing.T) {
	wantErr := errors.New("store unavailable")
	store := &stubEnvironmentStore{createErr: wantErr}
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "env-1" })

	_, err := service.CreateEnvironment(context.Background(), "alpha")
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateEnvironment error = %v, want %v", err, wantErr)
	}
}

func TestEnvironmentServiceAddEnvironmentSkill_ValidatesBeforeStore_BitsUT(t *testing.T) {
	auto := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}
	tests := []struct {
		name             string
		skillKey         domain.SkillKey
		environmentNames []string
		policy           domain.EnvironmentSkillPolicy
	}{
		{name: "invalid skill key", skillKey: "Bad Skill", environmentNames: []string{"alpha"}, policy: auto},
		{name: "no environments", skillKey: "search", environmentNames: nil, policy: auto},
		{name: "unsupported policy", skillKey: "search", environmentNames: []string{"alpha"}, policy: domain.EnvironmentSkillPolicy{Kind: "always"}},
		{name: "invalid environment name", skillKey: "search", environmentNames: []string{"Alpha"}, policy: auto},
		{name: "duplicate environment name", skillKey: "search", environmentNames: []string{"alpha", "alpha"}, policy: auto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubEnvironmentStore{}
			service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "unused" })

			_, _, err := service.AddEnvironmentSkill(
				context.Background(),
				tt.skillKey,
				tt.environmentNames,
				tt.policy,
			)
			requireDomainCode(t, err, domain.ErrorCodeInvalidArgument)
			if len(store.updateManyCalls) != 0 {
				t.Fatalf("store UpdateMany called %d times after invalid input", len(store.updateManyCalls))
			}
		})
	}
}

func TestEnvironmentServiceAddEnvironmentSkill_UpdatesEveryTargetInOrder_BitsUT(t *testing.T) {
	store := &stubEnvironmentStore{
		updateManyInput: []domain.Environment{
			newEnvironment("env-beta", "beta", 1),
			newEnvironment("env-alpha", "alpha", 1),
		},
	}
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "unused" })
	wantSpec := domain.EnvironmentSkillSpec{
		SkillKey: "search",
		Policy:   domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto},
	}

	gotSpec, gotEnvironments, err := service.AddEnvironmentSkill(
		context.Background(),
		wantSpec.SkillKey,
		[]string{"beta", "alpha"},
		wantSpec.Policy,
	)
	if err != nil {
		t.Fatalf("AddEnvironmentSkill returned error: %v", err)
	}
	if gotSpec != wantSpec {
		t.Fatalf("returned spec = %#v, want %#v", gotSpec, wantSpec)
	}
	if len(store.updateManyCalls) != 1 {
		t.Fatalf("store UpdateMany called %d times, want 1", len(store.updateManyCalls))
	}
	wantNames := []string{"beta", "alpha"}
	if !reflect.DeepEqual(store.updateManyCalls[0].identifiers, wantNames) {
		t.Errorf("store target order = %v, want %v", store.updateManyCalls[0].identifiers, wantNames)
	}
	for index, environment := range store.updateManyCalls[0].after {
		if len(environment.Skills) != 1 || environment.Skills[0] != wantSpec {
			t.Errorf("updated environment %d skills = %#v, want [%#v]", index, environment.Skills, wantSpec)
		}
	}
	if !reflect.DeepEqual(gotEnvironments, store.updateManyCalls[0].committed) {
		t.Fatalf("returned environments = %#v, want committed %#v", gotEnvironments, store.updateManyCalls[0].committed)
	}
	for index, environment := range gotEnvironments {
		if environment.Revision != 2 {
			t.Errorf("returned environment %d revision = %d, want 2", index, environment.Revision)
		}
	}
}

func TestEnvironmentServiceAddEnvironmentSkill_PreflightsAllTargets_BitsUT(t *testing.T) {
	auto := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}
	store := &stubEnvironmentStore{
		updateManyInput: []domain.Environment{
			newEnvironment("env-alpha", "alpha", 1),
			withSkill(newEnvironment("env-beta", "beta", 4), "search", auto),
		},
	}
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "unused" })

	_, _, err := service.AddEnvironmentSkill(context.Background(), "search", []string{"alpha", "beta"}, auto)
	requireDomainCode(t, err, domain.ErrorCodeSkillAlreadyBound)
	if len(store.updateManyCalls) != 1 {
		t.Fatalf("store UpdateMany called %d times, want 1", len(store.updateManyCalls))
	}
	if !reflect.DeepEqual(store.updateManyCalls[0].after, store.updateManyCalls[0].before) {
		t.Fatalf("failed multi-environment update mutated targets: before=%#v after=%#v", store.updateManyCalls[0].before, store.updateManyCalls[0].after)
	}
}

func TestEnvironmentServiceAddEnvironmentSkill_PreservesStoreErrorCode_BitsUT(t *testing.T) {
	store := &stubEnvironmentStore{
		updateManyErr: domain.NewError(domain.ErrorCodeEnvironmentNotFound, "environment not found: beta"),
	}
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "unused" })

	_, _, err := service.AddEnvironmentSkill(
		context.Background(),
		"search",
		[]string{"alpha", "beta"},
		domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto},
	)
	requireDomainCode(t, err, domain.ErrorCodeEnvironmentNotFound)
	for _, part := range []string{"add skill \"search\"", "environment not found: beta"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
}

func TestEnvironmentServiceResolveEnvironmentGroup_BitsUT(t *testing.T) {
	auto := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}
	tests := []struct {
		name             string
		identifiers      []string
		records          map[string]domain.Environment
		errors           map[string]error
		wantIDs          []domain.EnvironmentID
		wantCalls        []string
		wantCode         domain.ErrorCode
		wantMessageParts []string
	}{
		{
			name:        "preserves identifier order",
			identifiers: []string{"beta", "env-alpha"},
			records: map[string]domain.Environment{
				"beta":      newEnvironment("env-beta", "beta", 1),
				"env-alpha": newEnvironment("env-alpha", "alpha", 1),
			},
			wantIDs:   []domain.EnvironmentID{"env-beta", "env-alpha"},
			wantCalls: []string{"beta", "env-alpha"},
		},
		{
			name:             "rejects empty group without lookup",
			wantCode:         domain.ErrorCodeInvalidArgument,
			wantMessageParts: []string{"at least one environment"},
		},
		{
			name:             "rejects empty identifier after earlier lookup",
			identifiers:      []string{"alpha", ""},
			records:          map[string]domain.Environment{"alpha": newEnvironment("env-alpha", "alpha", 1)},
			wantCode:         domain.ErrorCodeInvalidArgument,
			wantMessageParts: []string{"identifier must not be empty"},
		},
		{
			name:        "stops at first lookup error",
			identifiers: []string{"alpha", "missing", "beta"},
			records:     map[string]domain.Environment{"alpha": newEnvironment("env-alpha", "alpha", 1)},
			errors: map[string]error{
				"missing": domain.NewError(domain.ErrorCodeEnvironmentNotFound, "environment not found: missing"),
			},
			wantCalls:        []string{"alpha", "missing", "beta"},
			wantCode:         domain.ErrorCodeEnvironmentNotFound,
			wantMessageParts: []string{"resolve environments", "environment not found: missing"},
		},
		{
			name:        "rejects duplicate resolved id",
			identifiers: []string{"env-alpha", "alpha"},
			records: map[string]domain.Environment{
				"env-alpha": newEnvironment("env-alpha", "alpha", 1),
				"alpha":     newEnvironment("env-alpha", "alpha", 1),
			},
			wantCalls:        []string{"env-alpha", "alpha"},
			wantCode:         domain.ErrorCodeInvalidArgument,
			wantMessageParts: []string{"selected more than once"},
		},
		{
			name:        "rejects skill conflict",
			identifiers: []string{"alpha", "beta"},
			records: map[string]domain.Environment{
				"alpha": withSkill(newEnvironment("env-alpha", "alpha", 1), "search", auto),
				"beta":  withSkill(newEnvironment("env-beta", "beta", 1), "search", auto),
			},
			wantCalls:        []string{"alpha", "beta"},
			wantCode:         domain.ErrorCodeSkillConflict,
			wantMessageParts: []string{"search", "alpha", "beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubEnvironmentStore{getManyResults: tt.records, getManyErrors: tt.errors}
			service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "unused" })

			snapshot, err := service.ResolveEnvironmentGroup(context.Background(), tt.identifiers)
			requireDomainCode(t, err, tt.wantCode)
			wantBatches := [][]string(nil)
			if tt.wantCalls != nil {
				wantBatches = [][]string{tt.wantCalls}
			}
			if !reflect.DeepEqual(store.getManyCalls, wantBatches) {
				t.Fatalf("lookup batches = %v, want %v", store.getManyCalls, wantBatches)
			}
			if err != nil {
				for _, part := range tt.wantMessageParts {
					if !strings.Contains(err.Error(), part) {
						t.Errorf("error %q does not contain %q", err, part)
					}
				}
				return
			}

			if len(snapshot.Environments) != len(tt.wantIDs) {
				t.Fatalf("resolved %d environments, want %d", len(snapshot.Environments), len(tt.wantIDs))
			}
			for index, wantID := range tt.wantIDs {
				if gotID := snapshot.Environments[index].ID; gotID != wantID {
					t.Errorf("environment %d id = %q, want %q", index, gotID, wantID)
				}
			}
		})
	}
}

type updateManyCall struct {
	identifiers []string
	before      []domain.Environment
	after       []domain.Environment
	committed   []domain.Environment
}

type stubEnvironmentStore struct {
	createCalls     []domain.Environment
	createErr       error
	getManyCalls    [][]string
	getManyResults  map[string]domain.Environment
	getManyErrors   map[string]error
	updateManyCalls []updateManyCall
	updateManyInput []domain.Environment
	updateManyErr   error
}

func (s *stubEnvironmentStore) Create(_ context.Context, environment domain.Environment) (domain.Environment, error) {
	s.createCalls = append(s.createCalls, environment)
	if s.createErr != nil {
		return domain.Environment{}, s.createErr
	}
	return environment, nil
}

func (s *stubEnvironmentStore) GetManyByIDOrName(_ context.Context, identifiers []string) ([]domain.Environment, error) {
	s.getManyCalls = append(s.getManyCalls, append([]string(nil), identifiers...))
	environments := make([]domain.Environment, len(identifiers))
	for index, identifier := range identifiers {
		if err := s.getManyErrors[identifier]; err != nil {
			return nil, err
		}
		environments[index] = domain.CloneEnvironment(s.getManyResults[identifier])
	}
	return environments, nil
}

func (s *stubEnvironmentStore) UpdateMany(
	_ context.Context,
	identifiers []string,
	update func([]domain.Environment) error,
) ([]domain.Environment, error) {
	selected := cloneEnvironments(s.updateManyInput)
	s.updateManyCalls = append(s.updateManyCalls, updateManyCall{
		identifiers: append([]string(nil), identifiers...),
		before:      cloneEnvironments(selected),
	})
	call := &s.updateManyCalls[len(s.updateManyCalls)-1]
	if s.updateManyErr != nil {
		call.after = cloneEnvironments(selected)
		return nil, s.updateManyErr
	}
	if err := update(selected); err != nil {
		call.after = cloneEnvironments(selected)
		return nil, err
	}
	call.after = cloneEnvironments(selected)
	for index := range selected {
		selected[index].Revision++
	}
	call.committed = cloneEnvironments(selected)
	return cloneEnvironments(selected), nil
}

func cloneEnvironments(environments []domain.Environment) []domain.Environment {
	cloned := make([]domain.Environment, len(environments))
	for index, environment := range environments {
		cloned[index] = domain.CloneEnvironment(environment)
	}
	return cloned
}

func newEnvironment(id domain.EnvironmentID, name string, revision uint64) domain.Environment {
	return domain.Environment{
		ID:       id,
		Name:     name,
		Revision: revision,
		Skills:   []domain.EnvironmentSkillSpec{},
	}
}

func withSkill(
	environment domain.Environment,
	skillKey domain.SkillKey,
	policy domain.EnvironmentSkillPolicy,
) domain.Environment {
	environment.Skills = append(environment.Skills, domain.EnvironmentSkillSpec{SkillKey: skillKey, Policy: policy})
	return environment
}

func requireDomainCode(t *testing.T, err error, want domain.ErrorCode) {
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
