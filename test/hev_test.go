package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonstore "github.com/Slimzeo/hev/internal/dal/json"
	"github.com/Slimzeo/hev/internal/handler"
	"github.com/Slimzeo/hev/internal/model"
	environmentservice "github.com/Slimzeo/hev/internal/service"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var _ environmentservice.Store = (*jsonstore.EnvironmentStore)(nil)

type testResponse struct {
	SchemaVersion int             `json:"schemaVersion"`
	Code          int             `json:"code"`
	Message       string          `json:"message"`
	Prompt        string          `json:"prompt"`
	Data          json.RawMessage `json:"data"`
}

func TestSkillValidate(t *testing.T) {
	tests := []struct {
		name       string
		skill      model.Skill
		wantStatus model.StatusCode
	}{
		{name: "valid Skill", skill: model.Skill{Key: "search"}},
		{name: "empty key", skill: model.Skill{}, wantStatus: model.StatusCodeInvalidArgument},
		{name: "invalid key", skill: model.Skill{Key: "Bad Skill"}, wantStatus: model.StatusCodeInvalidArgument},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireStatus(t, test.skill.Validate(), test.wantStatus)
		})
	}
}

func TestEnvironmentValidate(t *testing.T) {
	valid := testEnvironment(
		"env-1",
		"alpha-env",
		1,
		model.EnvironmentSkill{
			SkillKey: "search",
			Policy:   model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto},
		},
	)
	tests := []struct {
		name       string
		value      model.Environment
		wantStatus model.StatusCode
	}{
		{name: "valid aggregate", value: valid},
		{name: "empty id", value: replaceEnvironment(valid, func(value *model.Environment) { value.ID = " \t" }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "invalid name", value: replaceEnvironment(valid, func(value *model.Environment) { value.Name = "Alpha_Env" }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "zero revision", value: replaceEnvironment(valid, func(value *model.Environment) { value.Revision = 0 }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "nil skills", value: replaceEnvironment(valid, func(value *model.Environment) { value.Skills = nil }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "invalid skill key", value: replaceEnvironment(valid, func(value *model.Environment) { value.Skills[0].SkillKey = "Bad Skill" }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "unsupported policy", value: replaceEnvironment(valid, func(value *model.Environment) { value.Skills[0].Policy.Kind = "always" }), wantStatus: model.StatusCodeInvalidArgument},
		{name: "duplicate skill", value: replaceEnvironment(valid, func(value *model.Environment) {
			value.Skills = append(value.Skills, value.Skills[0])
		}), wantStatus: model.StatusCodeInvalidArgument},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireStatus(t, test.value.Validate(), test.wantStatus)
		})
	}
}

func TestEnvironmentServiceCreate(t *testing.T) {
	tests := []struct {
		name        string
		envName     string
		generatedID model.EnvironmentID
		wantStatus  model.StatusCode
		wantIDCalls int
		wantCreates int
	}{
		{name: "creates revision one with no skills", envName: "alpha-env", generatedID: "env-1", wantIDCalls: 1, wantCreates: 1},
		{name: "rejects invalid name before generating id", envName: "Alpha Env", generatedID: "env-1", wantStatus: model.StatusCodeInvalidArgument},
		{name: "rejects empty generated id before persistence", envName: "alpha-env", generatedID: "", wantStatus: model.StatusCodeInvalidArgument, wantIDCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &stubEnvironmentStore{}
			idCalls := 0
			service := environmentservice.New(store, func() model.EnvironmentID {
				idCalls++
				return test.generatedID
			})

			created, err := service.Create(context.Background(), test.envName)
			requireStatus(t, err, test.wantStatus)
			if idCalls != test.wantIDCalls {
				t.Errorf("ID generator called %d times, want %d", idCalls, test.wantIDCalls)
			}
			if len(store.createCalls) != test.wantCreates {
				t.Fatalf("store Create called %d times, want %d", len(store.createCalls), test.wantCreates)
			}
			if test.wantStatus != 0 {
				return
			}

			want := testEnvironment(test.generatedID, test.envName, 1)
			if !reflect.DeepEqual(store.createCalls[0], want) || !reflect.DeepEqual(created, want) {
				t.Fatalf("created=%#v persisted=%#v, want %#v", created, store.createCalls[0], want)
			}
			if created.Skills == nil {
				t.Fatal("created Environment has nil Skills")
			}
		})
	}

	t.Run("propagates store error", func(t *testing.T) {
		wantErr := errors.New("store unavailable")
		service := environmentservice.New(
			&stubEnvironmentStore{createErr: wantErr},
			func() model.EnvironmentID { return "env-1" },
		)
		_, err := service.Create(context.Background(), "alpha")
		if !errors.Is(err, wantErr) {
			t.Fatalf("Create error = %v, want %v", err, wantErr)
		}
	})
}

func TestEnvironmentServiceAddSkill(t *testing.T) {
	auto := model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto}

	t.Run("validates before persistence", func(t *testing.T) {
		tests := []struct {
			name             string
			skillKey         model.SkillKey
			environmentNames []string
			policy           model.EnvironmentSkillPolicy
		}{
			{name: "invalid skill key", skillKey: "Bad Skill", environmentNames: []string{"alpha"}, policy: auto},
			{name: "no environments", skillKey: "search", policy: auto},
			{name: "unsupported policy", skillKey: "search", environmentNames: []string{"alpha"}, policy: model.EnvironmentSkillPolicy{Kind: "always"}},
			{name: "invalid environment name", skillKey: "search", environmentNames: []string{"Alpha"}, policy: auto},
			{name: "duplicate environment name", skillKey: "search", environmentNames: []string{"alpha", "alpha"}, policy: auto},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				store := &stubEnvironmentStore{}
				service := environmentservice.New(store, func() model.EnvironmentID { return "unused" })
				_, _, err := service.AddSkill(context.Background(), model.Skill{Key: test.skillKey}, test.environmentNames, test.policy)
				requireStatus(t, err, model.StatusCodeInvalidArgument)
				if len(store.updateManyCalls) != 0 {
					t.Fatalf("store UpdateMany called %d times", len(store.updateManyCalls))
				}
			})
		}
	})

	t.Run("updates every target in request order", func(t *testing.T) {
		store := &stubEnvironmentStore{updateManyInput: []model.Environment{
			testEnvironment("env-beta", "beta", 1),
			testEnvironment("env-alpha", "alpha", 1),
		}}
		service := environmentservice.New(store, func() model.EnvironmentID { return "unused" })
		wantBinding := model.EnvironmentSkill{SkillKey: "search", Policy: auto}

		gotBinding, gotEnvironments, err := service.AddSkill(
			context.Background(),
			model.Skill{Key: wantBinding.SkillKey},
			[]string{"beta", "alpha"},
			wantBinding.Policy,
		)
		if err != nil {
			t.Fatalf("AddSkill returned error: %v", err)
		}
		if gotBinding != wantBinding {
			t.Fatalf("returned spec = %#v, want %#v", gotBinding, wantBinding)
		}
		wantNames := []string{"beta", "alpha"}
		if len(store.updateManyCalls) != 1 || !reflect.DeepEqual(store.updateManyCalls[0].identifiers, wantNames) {
			t.Fatalf("store target order = %#v, want %#v", store.updateManyCalls, wantNames)
		}
		for index, current := range store.updateManyCalls[0].after {
			if len(current.Skills) != 1 || current.Skills[0] != wantBinding {
				t.Errorf("updated Environment %d Skills = %#v", index, current.Skills)
			}
		}
		if !reflect.DeepEqual(gotEnvironments, store.updateManyCalls[0].committed) {
			t.Fatalf("returned Environments = %#v, want %#v", gotEnvironments, store.updateManyCalls[0].committed)
		}
	})

	t.Run("preflights all targets before mutation", func(t *testing.T) {
		store := &stubEnvironmentStore{updateManyInput: []model.Environment{
			testEnvironment("env-alpha", "alpha", 1),
			withSkill(testEnvironment("env-beta", "beta", 4), "search", auto),
		}}
		service := environmentservice.New(store, func() model.EnvironmentID { return "unused" })

		_, _, err := service.AddSkill(context.Background(), model.Skill{Key: "search"}, []string{"alpha", "beta"}, auto)
		requireStatus(t, err, model.StatusCodeConflict)
		if len(store.updateManyCalls) != 1 || !reflect.DeepEqual(store.updateManyCalls[0].after, store.updateManyCalls[0].before) {
			t.Fatalf("failed batch update mutated targets: %#v", store.updateManyCalls)
		}
	})

	t.Run("preserves store status", func(t *testing.T) {
		store := &stubEnvironmentStore{
			updateManyErr: model.NewError(model.StatusCodeNotFound, "environment not found: beta"),
		}
		service := environmentservice.New(store, func() model.EnvironmentID { return "unused" })
		_, _, err := service.AddSkill(context.Background(), model.Skill{Key: "search"}, []string{"alpha", "beta"}, auto)
		requireStatus(t, err, model.StatusCodeNotFound)
		for _, part := range []string{"add skill \"search\"", "environment not found: beta"} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("error %q does not contain %q", err, part)
			}
		}
	})
}

func TestEnvironmentServiceResolve(t *testing.T) {
	auto := model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto}
	want := withSkill(testEnvironment("env-alpha", "alpha", 2), "search", auto)
	store := &stubEnvironmentStore{
		defaultResult: want,
		getResults:    map[string]model.Environment{"alpha": want},
		getErrors: map[string]error{
			"missing": model.NewError(model.StatusCodeNotFound, "environment not found: missing"),
		},
	}
	service := environmentservice.New(store, func() model.EnvironmentID { return "unused" })

	t.Run("default", func(t *testing.T) {
		got, err := service.Default(context.Background())
		if err != nil || !reflect.DeepEqual(got, want) || store.defaultCalls != 1 {
			t.Fatalf("Default = %#v, calls=%d, err=%v", got, store.defaultCalls, err)
		}
	})

	t.Run("by name returns a detached value", func(t *testing.T) {
		got, err := service.Resolve(context.Background(), "alpha")
		if err != nil {
			t.Fatalf("Resolve returned error: %v", err)
		}
		got.Skills[0].SkillKey = "changed"
		if store.getResults["alpha"].Skills[0].SkillKey != "search" {
			t.Fatal("Resolve returned a Skill slice aliased to store data")
		}
	})

	t.Run("empty identifier", func(t *testing.T) {
		before := len(store.getCalls)
		_, err := service.Resolve(context.Background(), "")
		requireStatus(t, err, model.StatusCodeInvalidArgument)
		if len(store.getCalls) != before {
			t.Fatal("empty identifier reached the store")
		}
	})

	t.Run("preserves not-found status", func(t *testing.T) {
		_, err := service.Resolve(context.Background(), "missing")
		requireStatus(t, err, model.StatusCodeNotFound)
		if !strings.Contains(err.Error(), "resolve environment") {
			t.Fatalf("error %q lacks operation context", err)
		}
	})
}

func TestJSONEnvironmentStore(t *testing.T) {
	t.Run("initializes base and persists created Environments", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "environments.json")
		store := jsonstore.NewEnvironmentStore(path)
		base, err := store.Default(context.Background())
		if err != nil {
			t.Fatalf("Default returned error: %v", err)
		}
		if !reflect.DeepEqual(base, testEnvironment("env_base", "base", 1)) {
			t.Fatalf("default Environment = %#v", base)
		}

		auto := model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto}
		beta := testEnvironment(
			"env-beta",
			"beta",
			1,
			model.EnvironmentSkill{SkillKey: "search", Policy: auto},
		)
		created, err := store.Create(context.Background(), beta)
		if err != nil {
			t.Fatalf("Create beta returned error: %v", err)
		}
		if _, err := store.Create(context.Background(), testEnvironment("env-alpha", "alpha", 1)); err != nil {
			t.Fatalf("Create alpha returned error: %v", err)
		}
		beta.Skills[0].SkillKey = "changed-input"
		created.Skills[0].SkillKey = "changed-result"
		reloaded := jsonstore.NewEnvironmentStore(path)
		for _, identifier := range []string{"env-beta", "beta"} {
			got, getErr := reloaded.GetByIDOrName(context.Background(), identifier)
			if getErr != nil || got.Skills[0].SkillKey != "search" {
				t.Fatalf("GetByIDOrName(%q) = %#v, err=%v", identifier, got, getErr)
			}
		}

		content := mustReadFile(t, path)
		if !bytes.HasSuffix(content, []byte{'\n'}) {
			t.Fatal("store file does not end with a newline")
		}
		var persisted persistedStoreFile
		if err := json.Unmarshal(content, &persisted); err != nil {
			t.Fatalf("persisted JSON is invalid: %v", err)
		}
		wantNames := []string{"alpha", "base", "beta"}
		gotNames := make([]string, len(persisted.Environments))
		for index, current := range persisted.Environments {
			gotNames[index] = current.Name
		}
		if persisted.SchemaVersion != 1 || !reflect.DeepEqual(gotNames, wantNames) {
			t.Fatalf("persisted store = %#v, names=%v", persisted, gotNames)
		}
	})

	t.Run("seeds base into an empty persisted array", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "environments.json")
		if err := os.WriteFile(path, []byte("{\"schemaVersion\":1,\"environments\":[]}\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		store := jsonstore.NewEnvironmentStore(path)
		base, err := store.Default(context.Background())
		if err != nil || base.ID != "env_base" || base.Name != "base" {
			t.Fatalf("Default = %#v, err=%v", base, err)
		}
	})

	t.Run("rejects invalid and duplicate creates without changing bytes", func(t *testing.T) {
		tests := []struct {
			name       string
			candidate  model.Environment
			wantStatus model.StatusCode
		}{
			{name: "revision is not one", candidate: testEnvironment("env-beta", "beta", 2), wantStatus: model.StatusCodeInvalidArgument},
			{name: "nil Skills", candidate: model.Environment{ID: "env-beta", Name: "beta", Revision: 1}, wantStatus: model.StatusCodeInvalidArgument},
			{name: "duplicate id", candidate: testEnvironment("env-alpha", "beta", 1), wantStatus: model.StatusCodeConflict},
			{name: "duplicate name", candidate: testEnvironment("env-beta", "alpha", 1), wantStatus: model.StatusCodeConflict},
			{name: "id conflicts with name", candidate: testEnvironment("alpha", "beta", 1), wantStatus: model.StatusCodeConflict},
			{name: "name conflicts with id", candidate: testEnvironment("env-beta", "env-alpha", 1), wantStatus: model.StatusCodeConflict},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "environments.json")
				store := jsonstore.NewEnvironmentStore(path)
				if _, err := store.Create(context.Background(), testEnvironment("env-alpha", "alpha", 1)); err != nil {
					t.Fatalf("seed Create returned error: %v", err)
				}
				before := mustReadFile(t, path)
				_, err := store.Create(context.Background(), test.candidate)
				requireStatus(t, err, test.wantStatus)
				if after := mustReadFile(t, path); !bytes.Equal(after, before) {
					t.Fatalf("failed Create changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
				}
			})
		}
	})

	t.Run("updates multiple Environments atomically in request order", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "environments.json")
		store := jsonstore.NewEnvironmentStore(path)
		for _, current := range []model.Environment{
			testEnvironment("env-alpha", "alpha", 1),
			testEnvironment("env-beta", "beta", 1),
		} {
			if _, err := store.Create(context.Background(), current); err != nil {
				t.Fatalf("seed Create(%q) returned error: %v", current.Name, err)
			}
		}
		wantBinding := model.EnvironmentSkill{
			SkillKey: "search",
			Policy:   model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto},
		}

		updated, err := store.UpdateMany(context.Background(), []string{"env-beta", "alpha"}, func(values []model.Environment) error {
			wantIDs := []model.EnvironmentID{"env-beta", "env-alpha"}
			for index, value := range values {
				if value.ID != wantIDs[index] {
					t.Errorf("callback Environment %d ID = %q, want %q", index, value.ID, wantIDs[index])
				}
				values[index].Skills = append(values[index].Skills, wantBinding)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("UpdateMany returned error: %v", err)
		}
		wantIDs := []model.EnvironmentID{"env-beta", "env-alpha"}
		for index, current := range updated {
			if current.ID != wantIDs[index] || current.Revision != 2 || !reflect.DeepEqual(current.Skills, []model.EnvironmentSkill{wantBinding}) {
				t.Errorf("updated Environment %d = %#v", index, current)
			}
		}

		before := mustReadFile(t, path)
		callbackErr := errors.New("callback failed")
		_, err = store.UpdateMany(context.Background(), []string{"alpha", "beta"}, func(values []model.Environment) error {
			values[0].Skills = nil
			return callbackErr
		})
		if !errors.Is(err, callbackErr) {
			t.Fatalf("UpdateMany error = %v, want %v", err, callbackErr)
		}
		if after := mustReadFile(t, path); !bytes.Equal(after, before) {
			t.Fatalf("failed UpdateMany changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
		}
	})

	t.Run("rejects missing, duplicate, identity-changing, and invalid updates", func(t *testing.T) {
		tests := []struct {
			name        string
			identifiers []string
			update      func([]model.Environment) error
			wantStatus  model.StatusCode
			wantCalled  bool
		}{
			{name: "missing target", identifiers: []string{"alpha", "missing"}, update: addSkillUpdate("search"), wantStatus: model.StatusCodeNotFound},
			{name: "same Environment by id and name", identifiers: []string{"env-alpha", "alpha"}, update: addSkillUpdate("search"), wantStatus: model.StatusCodeInvalidArgument},
			{name: "identity change", identifiers: []string{"alpha", "beta"}, update: func(values []model.Environment) error { values[1].Name = "renamed"; return nil }, wantStatus: model.StatusCodeInvalidArgument, wantCalled: true},
			{name: "invalid aggregate", identifiers: []string{"alpha", "beta"}, update: func(values []model.Environment) error { values[1].Skills = nil; return nil }, wantStatus: model.StatusCodeInvalidArgument, wantCalled: true},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "environments.json")
				store := jsonstore.NewEnvironmentStore(path)
				for _, current := range []model.Environment{
					testEnvironment("env-alpha", "alpha", 1),
					testEnvironment("env-beta", "beta", 1),
				} {
					if _, err := store.Create(context.Background(), current); err != nil {
						t.Fatalf("seed Create returned error: %v", err)
					}
				}
				before := mustReadFile(t, path)
				called := false
				_, err := store.UpdateMany(context.Background(), test.identifiers, func(values []model.Environment) error {
					called = true
					return test.update(values)
				})
				requireStatus(t, err, test.wantStatus)
				if called != test.wantCalled {
					t.Errorf("callback called = %t, want %t", called, test.wantCalled)
				}
				if after := mustReadFile(t, path); !bytes.Equal(after, before) {
					t.Fatalf("failed UpdateMany changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
				}
			})
		}
	})

	t.Run("rejects malformed persisted data", func(t *testing.T) {
		tests := []struct {
			name          string
			content       string
			wantErrorPart string
		}{
			{name: "empty file", content: "", wantErrorPart: "get environment: decode store"},
			{name: "malformed JSON", content: "{", wantErrorPart: "get environment: decode store"},
			{name: "unsupported schema", content: `{"schemaVersion":2,"environments":[]}`, wantErrorPart: "unsupported store schema version: 2"},
			{name: "missing Environments", content: `{"schemaVersion":1}`, wantErrorPart: "environments must be an array"},
			{name: "invalid Environment", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":null}]}`, wantErrorPart: "invalid stored environment"},
			{name: "duplicate ID", content: `{"schemaVersion":1,"environments":[{"id":"env-shared","name":"alpha","revision":1,"skills":[]},{"id":"env-shared","name":"beta","revision":1,"skills":[]}]}`, wantErrorPart: "duplicate environment id"},
			{name: "duplicate name", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[]},{"id":"env-other","name":"alpha","revision":1,"skills":[]}]}`, wantErrorPart: "duplicate environment name"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "environments.json")
				if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
					t.Fatalf("WriteFile returned error: %v", err)
				}
				_, err := jsonstore.NewEnvironmentStore(path).GetByIDOrName(context.Background(), "alpha")
				if err == nil || !strings.Contains(err.Error(), test.wantErrorPart) {
					t.Fatalf("error = %v, want part %q", err, test.wantErrorPart)
				}
			})
		}
	})

	t.Run("reports directory creation failure", func(t *testing.T) {
		parentFile := filepath.Join(t.TempDir(), "parent-file")
		if err := os.WriteFile(parentFile, []byte("not a directory"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		_, err := jsonstore.NewEnvironmentStore(filepath.Join(parentFile, "environments.json")).GetByIDOrName(context.Background(), "alpha")
		if err == nil || !strings.Contains(err.Error(), "get environment: create store directory") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestJSONCommands(t *testing.T) {
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
	service := environmentservice.New(store, func() model.EnvironmentID { return "env_coding" })

	created := runJSONCommand(t, service, "env", "create", "coding", "--output", "json")
	var createData struct {
		Environment model.Environment `json:"environment"`
	}
	decodeData(t, created, &createData)
	if created.Message != "environment created" || createData.Environment.ID != "env_coding" || createData.Environment.Revision != 1 {
		t.Fatalf("create response = %#v, data=%#v", created, createData)
	}

	added := runJSONCommand(t, service, "skill", "add", "code-review", "--env", "coding", "--policy", "off", "--output", "json")
	if added.Message != "skill added to environment" {
		t.Fatalf("add message = %q", added.Message)
	}

	used := runJSONCommand(t, service, "env", "use", "coding", "--output", "json")
	var useData struct {
		Environment model.Environment `json:"environment"`
	}
	decodeData(t, used, &useData)
	if useData.Environment.Revision != 2 || len(useData.Environment.Skills) != 1 || useData.Environment.Skills[0].Policy.Kind != model.SkillPolicyOff {
		t.Fatalf("resolved Environment = %#v", useData.Environment)
	}

	defaultResponse := runJSONCommand(t, service, "env", "use", "--output", "json")
	decodeData(t, defaultResponse, &useData)
	if useData.Environment.ID != "env_base" || useData.Environment.Name != "base" {
		t.Fatalf("default Environment = %#v", useData.Environment)
	}
}

func TestJSONCommandFailures(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{name: "missing Environment", args: []string{"env", "use", "missing", "--output", "json"}, wantCode: 404},
		{name: "multiple Environment arguments", args: []string{"env", "use", "alpha", "beta", "--output", "json"}, wantCode: 400},
		{name: "unsupported policy", args: []string{"skill", "add", "search", "--env", "base", "--policy", "always", "--output", "json"}, wantCode: 400},
		{name: "unknown command", args: []string{"unknown", "--output", "json"}, wantCode: 400},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
			service := environmentservice.New(store, func() model.EnvironmentID { return "env_unused" })
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exitCode := handler.Execute(context.Background(), service, &stdout, &stderr, test.args); exitCode != 1 {
				t.Fatalf("exit code = %d, want 1", exitCode)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			var response testResponse
			if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v; output=%q", err, stdout.String())
			}
			if response.SchemaVersion != 2 || response.Code != test.wantCode {
				t.Fatalf("response = %#v", response)
			}
			var data map[string]any
			decodeData(t, response, &data)
			if len(data) != 0 {
				t.Fatalf("error data = %#v, want empty object", data)
			}
		})
	}
}

func TestCLIContract(t *testing.T) {
	schema := compileResponseSchema(t)
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
	service := environmentservice.New(store, func() model.EnvironmentID { return "env_coding" })

	for _, test := range []struct {
		args        []string
		wantSuccess bool
	}{
		{args: []string{"env", "create", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "add", "code-review", "--env", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "use", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "use", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "use", "coding", "base", "--output", "json"}},
		{args: []string{"env", "use", "missing", "--output", "json"}},
		{args: []string{"env", "create", "--output", "json"}},
		{args: []string{"skill", "add", "code-review", "--env", "coding", "--policy", "always", "--output", "json"}},
		{args: []string{"unknown", "--output", "json"}},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := handler.Execute(context.Background(), service, &stdout, &stderr, test.args)
		if stderr.Len() != 0 {
			t.Fatalf("Execute(%q) stderr = %q", test.args, stderr.String())
		}
		if (exitCode == 0) != test.wantSuccess {
			t.Fatalf("Execute(%q) exit code = %d", test.args, exitCode)
		}
		validateSingleJSONValue(t, schema, test.args, stdout.Bytes())
	}

	fixtures, err := filepath.Glob(filepath.Join("..", "contracts", "cli", "v2", "fixtures", "*.json"))
	if err != nil {
		t.Fatalf("list fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no CLI contract fixtures found")
	}
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			content := mustReadFile(t, fixture)
			var value any
			if err := json.Unmarshal(content, &value); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if err := schema.Validate(value); err != nil {
				t.Fatalf("validate fixture: %v", err)
			}
		})
	}
}

type updateManyCall struct {
	identifiers []string
	before      []model.Environment
	after       []model.Environment
	committed   []model.Environment
}

type stubEnvironmentStore struct {
	createCalls     []model.Environment
	createErr       error
	defaultCalls    int
	defaultResult   model.Environment
	defaultErr      error
	getCalls        []string
	getResults      map[string]model.Environment
	getErrors       map[string]error
	updateManyCalls []updateManyCall
	updateManyInput []model.Environment
	updateManyErr   error
}

func (s *stubEnvironmentStore) Create(_ context.Context, value model.Environment) (model.Environment, error) {
	s.createCalls = append(s.createCalls, value)
	if s.createErr != nil {
		return model.Environment{}, s.createErr
	}
	return value, nil
}

func (s *stubEnvironmentStore) Default(_ context.Context) (model.Environment, error) {
	s.defaultCalls++
	return model.Clone(s.defaultResult), s.defaultErr
}

func (s *stubEnvironmentStore) GetByIDOrName(_ context.Context, identifier string) (model.Environment, error) {
	s.getCalls = append(s.getCalls, identifier)
	if err := s.getErrors[identifier]; err != nil {
		return model.Environment{}, err
	}
	return model.Clone(s.getResults[identifier]), nil
}

func (s *stubEnvironmentStore) UpdateMany(
	_ context.Context,
	identifiers []string,
	update func([]model.Environment) error,
) ([]model.Environment, error) {
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

type persistedStoreFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Environments  []model.Environment `json:"environments"`
}

func testEnvironment(
	id model.EnvironmentID,
	name string,
	revision uint64,
	skills ...model.EnvironmentSkill,
) model.Environment {
	return model.Environment{
		ID:       id,
		Name:     name,
		Revision: revision,
		Skills:   append([]model.EnvironmentSkill{}, skills...),
	}
}

func replaceEnvironment(value model.Environment, replace func(*model.Environment)) model.Environment {
	value = model.Clone(value)
	replace(&value)
	return value
}

func withSkill(
	value model.Environment,
	skillKey model.SkillKey,
	policy model.EnvironmentSkillPolicy,
) model.Environment {
	value.Skills = append(value.Skills, model.EnvironmentSkill{SkillKey: skillKey, Policy: policy})
	return value
}

func cloneEnvironments(values []model.Environment) []model.Environment {
	cloned := make([]model.Environment, len(values))
	for index, value := range values {
		cloned[index] = model.Clone(value)
	}
	return cloned
}

func addSkillUpdate(skillKey model.SkillKey) func([]model.Environment) error {
	return func(values []model.Environment) error {
		for index := range values {
			values[index].Skills = append(values[index].Skills, model.EnvironmentSkill{
				SkillKey: skillKey,
				Policy:   model.EnvironmentSkillPolicy{Kind: model.SkillPolicyAuto},
			})
		}
		return nil
	}
}

func requireStatus(t *testing.T, err error, want model.StatusCode) {
	t.Helper()
	if want == 0 {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected status %d, got nil", want)
	}
	got, ok := model.StatusCodeOf(err)
	if !ok || got != want {
		t.Fatalf("status = %d, ok=%t, want %d; error=%v", got, ok, want, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) returned error: %v", path, err)
	}
	return content
}

func runJSONCommand(t *testing.T, service *environmentservice.Service, args ...string) testResponse {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := handler.Execute(context.Background(), service, &stdout, &stderr, args); exitCode != 0 {
		t.Fatalf("Execute(%q) exit code = %d, stdout=%q stderr=%q", args, exitCode, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("Execute(%q) stderr = %q", args, stderr.String())
	}

	var response testResponse
	decoder := json.NewDecoder(&stdout)
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode response: %v; output=%q", err, stdout.String())
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		t.Fatalf("Execute(%q) wrote more than one JSON value: %q", args, stdout.String())
	}
	if response.SchemaVersion != 2 || response.Code != 200 || response.Prompt != "" {
		t.Fatalf("response = %#v", response)
	}
	return response
}

func decodeData(t *testing.T, response testResponse, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Data, target); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
}

func compileResponseSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "contracts", "cli", "v2", "schema.json")
	content := mustReadFile(t, path)
	var document any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode CLI schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", document); err != nil {
		t.Fatalf("add CLI schema: %v", err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		t.Fatalf("compile CLI schema: %v", err)
	}
	return schema
}

func validateSingleJSONValue(t *testing.T, schema *jsonschema.Schema, args []string, content []byte) {
	t.Helper()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("Execute(%q) output is not JSON: %v; output=%q", args, err, content)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		t.Fatalf("Execute(%q) wrote more than one JSON value: %q", args, content)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("Execute(%q) output does not match CLI contract: %v; output=%q", args, err, content)
	}
}
