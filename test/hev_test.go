package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	commonresponse "github.com/Slimzeo/hev/internal/common/response"
	"github.com/Slimzeo/hev/internal/constants"
	"github.com/Slimzeo/hev/internal/dal"
	"github.com/Slimzeo/hev/internal/model"
	"github.com/Slimzeo/hev/internal/routers"
	environmentservice "github.com/Slimzeo/hev/internal/service"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var _ environmentservice.EnvironmentStore = (*dal.EnvironmentDAL)(nil)

type testResponse struct {
	SchemaVersion int             `json:"schemaVersion"`
	Code          int             `json:"code"`
	Message       string          `json:"message"`
	Prompt        string          `json:"prompt"`
	Data          json.RawMessage `json:"data"`
}

func TestEnvironmentServiceCreate(t *testing.T) {
	tests := []struct {
		name        string
		envName     string
		generatedID model.EnvironmentID
		wantStatus  commonresponse.StatusCode
		wantIDCalls int
		wantCreates int
	}{
		{name: "creates revision one with the guide Skill", envName: "alpha-env", generatedID: "env-1", wantIDCalls: 1, wantCreates: 1},
		{name: "rejects invalid name before generating id", envName: "Alpha Env", generatedID: "env-1", wantStatus: commonresponse.StatusCodeInvalidArgument},
		{name: "rejects empty generated id before persistence", envName: "alpha-env", generatedID: "", wantStatus: commonresponse.StatusCodeInvalidArgument, wantIDCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &stubEnvironmentStore{}
			idCalls := 0
			service := environmentservice.NewEnvironment(store, func() model.EnvironmentID {
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

			want := testEnvironment(test.generatedID, test.envName, 1, defaultGuideBinding())
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
		service := environmentservice.NewEnvironment(
			&stubEnvironmentStore{createErr: wantErr},
			func() model.EnvironmentID { return "env-1" },
		)
		_, err := service.Create(context.Background(), "alpha")
		if !errors.Is(err, wantErr) {
			t.Fatalf("Create error = %v, want %v", err, wantErr)
		}
	})
}

func TestEnvironmentServiceRenameAndDelete(t *testing.T) {
	stateDir := t.TempDir()
	store := newTestEnvironmentDAL(stateDir)
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "env-coding" })
	ctx := context.Background()

	if _, err := service.Create(ctx, "coding"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.Use(ctx, "session-one", "coding"); err != nil {
		t.Fatalf("Use returned error: %v", err)
	}

	renamed, err := service.Rename(ctx, "coding", "backend")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	if renamed.ID != "env-coding" || renamed.Name != "backend" || renamed.Revision != 2 {
		t.Fatalf("renamed Environment = %#v", renamed)
	}
	current, err := service.Current(ctx, "session-one")
	if err != nil || current.Environment == nil || current.Environment.Name != "backend" {
		t.Fatalf("Current after rename = %#v, err=%v", current, err)
	}

	deleted, err := service.Delete(ctx, "backend")
	if err != nil || deleted.ID != "env-coding" {
		t.Fatalf("Delete = %#v, err=%v", deleted, err)
	}
	current, err = service.Current(ctx, "session-one")
	if err != nil || current.Environment == nil || current.Environment.ID != constants.BaseEnvironmentID {
		t.Fatalf("Current after delete = %#v, err=%v", current, err)
	}

	_, err = service.Rename(ctx, "base", "renamed-base")
	requireStatus(t, err, commonresponse.StatusCodeConflict)
	_, err = service.Delete(ctx, "base")
	requireStatus(t, err, commonresponse.StatusCodeConflict)

	if _, err := service.Create(ctx, "temporary"); err != nil {
		t.Fatalf("Create temporary returned error: %v", err)
	}
	if _, err := service.Use(ctx, "session-two", "temporary"); err != nil {
		t.Fatalf("Use temporary returned error: %v", err)
	}
	if _, err := service.Delete(ctx, "temporary"); err != nil {
		t.Fatalf("Delete temporary returned error: %v", err)
	}
	base, err := service.Quit(ctx, "session-two")
	if err != nil || base.Environment == nil || base.Environment.ID != constants.BaseEnvironmentID {
		t.Fatalf("Quit after delete = %#v, err=%v", base, err)
	}
	inactive, err := service.Quit(ctx, "session-two")
	if err != nil || inactive.Environment != nil {
		t.Fatalf("second Quit after delete = %#v, err=%v", inactive, err)
	}
}

func TestEnvironmentServiceAddSkill(t *testing.T) {
	auto := model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto}

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
				service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })
				_, _, err := service.AddSkill(context.Background(), model.Skill{Key: test.skillKey}, test.environmentNames, test.policy)
				requireStatus(t, err, commonresponse.StatusCodeInvalidArgument)
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
		service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })
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
		service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })

		_, _, err := service.AddSkill(context.Background(), model.Skill{Key: "search"}, []string{"alpha", "beta"}, auto)
		requireStatus(t, err, commonresponse.StatusCodeConflict)
		if len(store.updateManyCalls) != 1 || !reflect.DeepEqual(store.updateManyCalls[0].after, store.updateManyCalls[0].before) {
			t.Fatalf("failed batch update mutated targets: %#v", store.updateManyCalls)
		}
	})

	t.Run("preserves store status", func(t *testing.T) {
		store := &stubEnvironmentStore{
			updateManyErr: commonresponse.NewError(commonresponse.StatusCodeNotFound, "environment not found: beta", "list environments"),
		}
		service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })
		_, _, err := service.AddSkill(context.Background(), model.Skill{Key: "search"}, []string{"alpha", "beta"}, auto)
		requireStatus(t, err, commonresponse.StatusCodeNotFound)
		for _, part := range []string{"add skill \"search\"", "environment not found: beta"} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("error %q does not contain %q", err, part)
			}
		}
	})
}

func TestEnvironmentServiceRemoveSkill(t *testing.T) {
	ctx := context.Background()
	auto := model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto}
	stateDir := t.TempDir()
	store := newTestEnvironmentDAL(stateDir)
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "env-coding" })
	if _, err := service.Create(ctx, "coding"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, _, err := service.AddSkill(ctx, model.Skill{Key: "code-review"}, []string{"coding", "base"}, auto); err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}

	updated, err := service.RemoveSkill(ctx, model.Skill{Key: "code-review"}, []string{"coding", "base"})
	if err != nil {
		t.Fatalf("RemoveSkill returned error: %v", err)
	}
	for _, environment := range updated {
		if environment.Revision != 3 || len(environment.Skills) != 1 || environment.Skills[0] != defaultGuideBinding() {
			t.Errorf("updated Environment = %#v", environment)
		}
	}

	before := mustReadFile(t, filepath.Join(stateDir, constants.EnvironmentStoreFileName))
	_, err = service.RemoveSkill(ctx, model.Skill{Key: "missing"}, []string{"coding", "base"})
	requireStatus(t, err, commonresponse.StatusCodeNotFound)
	if after := mustReadFile(t, filepath.Join(stateDir, constants.EnvironmentStoreFileName)); !bytes.Equal(after, before) {
		t.Fatal("failed RemoveSkill changed persisted Environments")
	}
	_, err = service.RemoveSkill(ctx, model.Skill{Key: constants.DefaultGuideSkillKey}, []string{"base"})
	requireStatus(t, err, commonresponse.StatusCodeConflict)

	if _, _, err := service.AddSkill(ctx, model.Skill{Key: "coding-only"}, []string{"coding"}, auto); err != nil {
		t.Fatalf("AddSkill coding-only returned error: %v", err)
	}
	before = mustReadFile(t, filepath.Join(stateDir, constants.EnvironmentStoreFileName))
	_, err = service.RemoveSkill(ctx, model.Skill{Key: "coding-only"}, []string{"coding", "base"})
	requireStatus(t, err, commonresponse.StatusCodeNotFound)
	if after := mustReadFile(t, filepath.Join(stateDir, constants.EnvironmentStoreFileName)); !bytes.Equal(after, before) {
		t.Fatal("partially failed RemoveSkill changed persisted Environments")
	}
}

func TestEnvironmentServiceResolve(t *testing.T) {
	auto := model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto}
	want := withSkill(testEnvironment("env-alpha", "alpha", 2), "search", auto)
	store := &stubEnvironmentStore{
		defaultResult: want,
		getResults:    map[string]model.Environment{"alpha": want},
		getErrors: map[string]error{
			"missing": commonresponse.NewError(commonresponse.StatusCodeNotFound, "environment not found: missing", "list environments"),
		},
	}
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })

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
		requireStatus(t, err, commonresponse.StatusCodeInvalidArgument)
		if len(store.getCalls) != before {
			t.Fatal("empty identifier reached the store")
		}
	})

	t.Run("preserves not-found status", func(t *testing.T) {
		_, err := service.Resolve(context.Background(), "missing")
		requireStatus(t, err, commonresponse.StatusCodeNotFound)
		if !strings.Contains(err.Error(), "resolve environment") {
			t.Fatalf("error %q lacks operation context", err)
		}
	})
}

func TestEnvironmentServiceList(t *testing.T) {
	want := []model.Environment{
		testEnvironment("base", "base", 1),
		testEnvironment("env-coding", "coding", 2),
	}
	store := &stubEnvironmentStore{listResult: want}
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "unused" })

	got, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !reflect.DeepEqual(got, want) || store.listCalls != 1 {
		t.Fatalf("List = %#v, calls=%d, want %#v", got, store.listCalls, want)
	}
}

func TestEnvironmentSessionOperations(t *testing.T) {
	stateDir := t.TempDir()
	environmentStore := newTestEnvironmentDAL(stateDir)
	environmentService := environmentservice.NewEnvironment(environmentStore, func() model.EnvironmentID {
		return "env-coding"
	})
	ctx := context.Background()

	inactive, err := environmentService.Current(ctx, "session-one")
	if err != nil || inactive.SessionID != "session-one" || inactive.Environment != nil {
		t.Fatalf("inactive Current = %#v, err=%v", inactive, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, constants.SessionStoreFileName)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inactive Current created a binding store: %v", err)
	}

	if _, err := environmentService.Create(ctx, "coding"); err != nil {
		t.Fatalf("Create coding returned error: %v", err)
	}
	selected, err := environmentService.Use(ctx, "session-one", "coding")
	if err != nil || selected.Environment == nil || selected.Environment.Name != "coding" {
		t.Fatalf("Use = %#v, err=%v", selected, err)
	}
	reloadedEnvironmentService := environmentservice.NewEnvironment(
		newTestEnvironmentDAL(stateDir),
		func() model.EnvironmentID { return "unused" },
	)
	reloaded, err := reloadedEnvironmentService.Current(ctx, "session-one")
	if err != nil || reloaded.Environment == nil || reloaded.Environment.Name != "coding" {
		t.Fatalf("Current after DAL reload = %#v, err=%v", reloaded, err)
	}

	_, _, err = environmentService.AddSkill(
		ctx,
		model.Skill{Key: "code-review"},
		[]string{"coding"},
		model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
	)
	if err != nil {
		t.Fatalf("AddSkill returned error: %v", err)
	}
	current, err := environmentService.Current(ctx, "session-one")
	if err != nil || current.Environment == nil || current.Environment.Revision != 2 {
		t.Fatalf("Current after Environment update = %#v, err=%v", current, err)
	}
	if len(current.Environment.Skills) != 2 || current.Environment.Skills[1].SkillKey != "code-review" {
		t.Fatalf("Current Skills = %#v", current.Environment.Skills)
	}

	base, err := environmentService.Quit(ctx, "session-one")
	if err != nil || base.Environment == nil || base.Environment.ID != constants.BaseEnvironmentID {
		t.Fatalf("first Quit = %#v, err=%v", base, err)
	}
	inactive, err = environmentService.Quit(ctx, "session-one")
	if err != nil || inactive.Environment != nil {
		t.Fatalf("second Quit = %#v, err=%v", inactive, err)
	}
	inactive, err = environmentService.Quit(ctx, "session-one")
	if err != nil || inactive.Environment != nil {
		t.Fatalf("inactive Quit = %#v, err=%v", inactive, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, constants.SessionStoreFileName)); err != nil {
		t.Fatalf("Session store missing after transitions: %v", err)
	}

	if _, err := environmentService.Use(ctx, "session-two", "coding"); err != nil {
		t.Fatalf("Use session-two returned error: %v", err)
	}
	other, err := environmentService.Current(ctx, "session-two")
	if err != nil || other.Environment == nil || other.Environment.Name != "coding" {
		t.Fatalf("session-two Current = %#v, err=%v", other, err)
	}
	first, err := environmentService.Current(ctx, "session-one")
	if err != nil || first.Environment != nil {
		t.Fatalf("session-one changed with session-two = %#v, err=%v", first, err)
	}

	_, err = environmentService.Current(ctx, "")
	requireStatus(t, err, commonresponse.StatusCodeInvalidArgument)
}

func TestEnvironmentDALRejectsMalformedSessionData(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantErrorPart string
	}{
		{name: "missing bindings", content: `{"schemaVersion":1}`, wantErrorPart: "bindings must be an array"},
		{name: "unsupported schema", content: `{"schemaVersion":2,"bindings":[]}`, wantErrorPart: "unsupported store schema version"},
		{name: "empty session", content: `{"schemaVersion":1,"bindings":[{"sessionId":"","environmentId":"base"}]}`, wantErrorPart: "session id must not be empty"},
		{name: "empty environment", content: `{"schemaVersion":1,"bindings":[{"sessionId":"one","environmentId":""}]}`, wantErrorPart: "environment id must not be empty"},
		{name: "duplicate session", content: `{"schemaVersion":1,"bindings":[{"sessionId":"one","environmentId":"base"},{"sessionId":"one","environmentId":"other"}]}`, wantErrorPart: "duplicate session id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), constants.SessionStoreFileName)
			if err := os.WriteFile(path, []byte(test.content+"\n"), 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			store := newTestEnvironmentDAL(filepath.Dir(path))
			_, _, err := store.GetSessionEnvironment(context.Background(), "one")
			if err == nil || !strings.Contains(err.Error(), test.wantErrorPart) {
				t.Fatalf("error = %v, want part %q", err, test.wantErrorPart)
			}
		})
	}
}

func TestEnvironmentDAL(t *testing.T) {
	t.Run("initializes base and persists created Environments", func(t *testing.T) {
		stateDir := filepath.Join(t.TempDir(), "nested")
		path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
		store := newTestEnvironmentDAL(stateDir)
		base, err := store.Default(context.Background())
		if err != nil {
			t.Fatalf("Default returned error: %v", err)
		}
		if !reflect.DeepEqual(base, defaultBaseEnvironment(1)) {
			t.Fatalf("default Environment = %#v", base)
		}

		auto := model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto}
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
		reloaded := newTestEnvironmentDAL(stateDir)
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

	t.Run("renames and deletes non-base Environments", func(t *testing.T) {
		stateDir := t.TempDir()
		store := newTestEnvironmentDAL(stateDir)
		if _, err := store.Create(context.Background(), testEnvironment("env-alpha", "alpha", 1)); err != nil {
			t.Fatalf("seed Create returned error: %v", err)
		}

		renamed, err := store.Rename(context.Background(), "env-alpha", "beta")
		if err != nil || renamed.ID != "env-alpha" || renamed.Name != "beta" || renamed.Revision != 2 {
			t.Fatalf("Rename = %#v, err=%v", renamed, err)
		}
		unchanged, err := store.Rename(context.Background(), "env-alpha", "beta")
		if err != nil || unchanged.Revision != 2 {
			t.Fatalf("idempotent Rename = %#v, err=%v", unchanged, err)
		}
		deleted, err := store.Delete(context.Background(), "beta")
		if err != nil || deleted.ID != "env-alpha" {
			t.Fatalf("Delete = %#v, err=%v", deleted, err)
		}
		if _, err := store.GetByIDOrName(context.Background(), "env-alpha"); err == nil {
			t.Fatal("deleted Environment remains readable")
		}

		_, err = store.Rename(context.Background(), constants.BaseEnvironmentID, "renamed")
		requireStatus(t, err, commonresponse.StatusCodeConflict)
		_, err = store.Delete(context.Background(), constants.BaseEnvironmentID)
		requireStatus(t, err, commonresponse.StatusCodeConflict)
	})

	t.Run("seeds base into an empty persisted array", func(t *testing.T) {
		stateDir := t.TempDir()
		path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
		if err := os.WriteFile(path, []byte("{\"schemaVersion\":1,\"environments\":[]}\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		store := newTestEnvironmentDAL(stateDir)
		base, err := store.Default(context.Background())
		if err != nil || !reflect.DeepEqual(base, defaultBaseEnvironment(1)) {
			t.Fatalf("Default = %#v, err=%v", base, err)
		}
	})

	t.Run("migrates the legacy base ID and adds the guide Skill", func(t *testing.T) {
		stateDir := t.TempDir()
		path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
		content := `{"schemaVersion":1,"environments":[{"id":"env_base","name":"base","revision":4,"skills":[]}]}` + "\n"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		store := newTestEnvironmentDAL(stateDir)

		base, err := store.Default(context.Background())
		if err != nil || !reflect.DeepEqual(base, defaultBaseEnvironment(5)) {
			t.Fatalf("Default = %#v, err=%v", base, err)
		}

		var persisted persistedStoreFile
		if err := json.Unmarshal(mustReadFile(t, path), &persisted); err != nil {
			t.Fatalf("decode persisted migration: %v", err)
		}
		if len(persisted.Environments) != 1 || !reflect.DeepEqual(persisted.Environments[0], defaultBaseEnvironment(5)) {
			t.Fatalf("persisted migration = %#v", persisted.Environments)
		}
		before := mustReadFile(t, path)
		again, err := newTestEnvironmentDAL(stateDir).Default(context.Background())
		if err != nil || !reflect.DeepEqual(again, defaultBaseEnvironment(5)) {
			t.Fatalf("second Default = %#v, err=%v", again, err)
		}
		if after := mustReadFile(t, path); !bytes.Equal(after, before) {
			t.Fatalf("idempotent migration changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
		}
	})

	t.Run("rejects invalid and duplicate creates without changing bytes", func(t *testing.T) {
		tests := []struct {
			name       string
			candidate  model.Environment
			wantStatus commonresponse.StatusCode
		}{
			{name: "wrong source", candidate: replaceEnvironment(testEnvironment("env-beta", "beta", 1), func(value *model.Environment) { value.Source = model.SourceCodex }), wantStatus: commonresponse.StatusCodeInvalidArgument},
			{name: "revision is not one", candidate: testEnvironment("env-beta", "beta", 2), wantStatus: commonresponse.StatusCodeInvalidArgument},
			{name: "nil Skills", candidate: model.Environment{ID: "env-beta", Name: "beta", Revision: 1}, wantStatus: commonresponse.StatusCodeInvalidArgument},
			{name: "duplicate id", candidate: testEnvironment("env-alpha", "beta", 1), wantStatus: commonresponse.StatusCodeConflict},
			{name: "duplicate name", candidate: testEnvironment("env-beta", "alpha", 1), wantStatus: commonresponse.StatusCodeConflict},
			{name: "id conflicts with name", candidate: testEnvironment("alpha", "beta", 1), wantStatus: commonresponse.StatusCodeConflict},
			{name: "name conflicts with id", candidate: testEnvironment("env-beta", "env-alpha", 1), wantStatus: commonresponse.StatusCodeConflict},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				stateDir := t.TempDir()
				path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
				store := newTestEnvironmentDAL(stateDir)
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
		stateDir := t.TempDir()
		path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
		store := newTestEnvironmentDAL(stateDir)
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
			Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
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
			wantStatus  commonresponse.StatusCode
			wantCalled  bool
		}{
			{name: "missing target", identifiers: []string{"alpha", "missing"}, update: addSkillUpdate("search"), wantStatus: commonresponse.StatusCodeNotFound},
			{name: "same Environment by id and name", identifiers: []string{"env-alpha", "alpha"}, update: addSkillUpdate("search"), wantStatus: commonresponse.StatusCodeInvalidArgument},
			{name: "identity change", identifiers: []string{"alpha", "beta"}, update: func(values []model.Environment) error { values[1].Name = "renamed"; return nil }, wantStatus: commonresponse.StatusCodeInvalidArgument, wantCalled: true},
			{name: "invalid aggregate", identifiers: []string{"alpha", "beta"}, update: func(values []model.Environment) error { values[1].Skills = nil; return nil }, wantStatus: commonresponse.StatusCodeInvalidArgument, wantCalled: true},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				stateDir := t.TempDir()
				path := filepath.Join(stateDir, constants.EnvironmentStoreFileName)
				store := newTestEnvironmentDAL(stateDir)
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
			{name: "invalid source", content: `{"schemaVersion":1,"environments":[{"source":"unknown","id":"env-alpha","name":"alpha","revision":1,"skills":[]}]}`, wantErrorPart: "source \"unknown\" does not match \"standalone\""},
			{name: "invalid Skill", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[{"skillKey":"Bad Skill","policy":{"kind":"auto"}}]}]}`, wantErrorPart: "invalid skill key"},
			{name: "invalid Skill policy", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[{"skillKey":"search","policy":{"kind":"always"}}]}]}`, wantErrorPart: "unsupported skill policy"},
			{name: "duplicate Skill", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[{"skillKey":"search","policy":{"kind":"auto"}},{"skillKey":"search","policy":{"kind":"off"}}]}]}`, wantErrorPart: "contains duplicate skill"},
			{name: "duplicate ID", content: `{"schemaVersion":1,"environments":[{"id":"env-shared","name":"alpha","revision":1,"skills":[]},{"id":"env-shared","name":"beta","revision":1,"skills":[]}]}`, wantErrorPart: "duplicate environment id"},
			{name: "duplicate name", content: `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[]},{"id":"env-other","name":"alpha","revision":1,"skills":[]}]}`, wantErrorPart: "duplicate environment name"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "environments.json")
				if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
					t.Fatalf("WriteFile returned error: %v", err)
				}
				_, err := newTestEnvironmentDAL(filepath.Dir(path)).GetByIDOrName(context.Background(), "alpha")
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
		_, err := newTestEnvironmentDAL(filepath.Join(parentFile, "state")).GetByIDOrName(context.Background(), "alpha")
		if err == nil || !strings.Contains(err.Error(), "get environment: create store directory") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestJSONCommands(t *testing.T) {
	stateDir := t.TempDir()
	store := newTestEnvironmentDAL(stateDir)
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "env_coding" })

	created := runJSONCommand(t, service, "env", "create", "coding", "--output", "json")
	var createData struct {
		Environment model.Environment `json:"environment"`
	}
	decodeData(t, created, &createData)
	if created.Message != "environment created" || createData.Environment.ID != "env_coding" || createData.Environment.Revision != 1 {
		t.Fatalf("create response = %#v, data=%#v", created, createData)
	}

	added := runJSONCommand(t, service, "skill", "add", "code-review", "coding", "base", "--policy", "off", "--output", "json")
	if added.Message != "skill added to environment" {
		t.Fatalf("add message = %q", added.Message)
	}
	var addData struct {
		Environments []struct {
			Name string `json:"name"`
		} `json:"environments"`
	}
	decodeData(t, added, &addData)
	if got := []string{addData.Environments[0].Name, addData.Environments[1].Name}; !reflect.DeepEqual(got, []string{"coding", "base"}) {
		t.Fatalf("updated Environment names = %v", got)
	}

	used := runJSONCommand(t, service, "env", "use", "coding", "--session-id", "session-one", "--output", "json")
	var useData struct {
		Session model.Session `json:"session"`
	}
	decodeData(t, used, &useData)
	if useData.Session.SessionID != "session-one" || useData.Session.Environment == nil ||
		useData.Session.Environment.Revision != 2 ||
		len(useData.Session.Environment.Skills) != 2 ||
		useData.Session.Environment.Skills[0] != defaultGuideBinding() ||
		useData.Session.Environment.Skills[1].Policy.Kind != constants.SkillPolicyOff {
		t.Fatalf("selected Session = %#v", useData.Session)
	}

	status := runJSONCommand(t, service, "env", "status", "--session-id", "session-one", "--output", "json")
	decodeData(t, status, &useData)
	if useData.Session.Environment == nil || useData.Session.Environment.ID != "env_coding" {
		t.Fatalf("Session status = %#v", useData.Session)
	}

	skills := runJSONCommand(t, service, "skill", "list", "--session-id", "session-one", "--output", "json")
	decodeData(t, skills, &useData)
	if useData.Session.Environment == nil || len(useData.Session.Environment.Skills) != 2 {
		t.Fatalf("Session Skill list = %#v", useData.Session)
	}

	namedSkills := runJSONCommand(t, service, "skill", "list", "coding", "--output", "json")
	var environmentData struct {
		Environment model.Environment `json:"environment"`
	}
	decodeData(t, namedSkills, &environmentData)
	if environmentData.Environment.Name != "coding" || len(environmentData.Environment.Skills) != 2 {
		t.Fatalf("named Environment Skill list = %#v", environmentData.Environment)
	}
	inspectionStatus := runJSONCommand(t, service, "env", "status", "--session-id", "inspection-only", "--output", "json")
	var inspectionData struct {
		Session model.Session `json:"session"`
	}
	decodeData(t, inspectionStatus, &inspectionData)
	if inspectionData.Session.Environment != nil {
		t.Fatalf("named Skill list activated Session: %#v", inspectionData.Session)
	}

	removed := runJSONCommand(t, service, "skill", "remove", "code-review", "coding", "base", "--output", "json")
	if removed.Message != "skill removed from environment" {
		t.Fatalf("remove message = %q", removed.Message)
	}

	renamed := runJSONCommand(t, service, "env", "rename", "coding", "backend", "--output", "json")
	decodeData(t, renamed, &environmentData)
	if environmentData.Environment.ID != "env_coding" || environmentData.Environment.Name != "backend" || environmentData.Environment.Revision != 4 {
		t.Fatalf("renamed Environment = %#v", environmentData.Environment)
	}

	deleted := runJSONCommand(t, service, "env", "delete", "backend", "--output", "json")
	decodeData(t, deleted, &environmentData)
	if environmentData.Environment.ID != "env_coding" || environmentData.Environment.Name != "backend" {
		t.Fatalf("deleted Environment = %#v", environmentData.Environment)
	}
	status = runJSONCommand(t, service, "env", "status", "--session-id", "session-one", "--output", "json")
	decodeData(t, status, &useData)
	if useData.Session.Environment == nil || useData.Session.Environment.ID != constants.BaseEnvironmentID {
		t.Fatalf("Session after Environment deletion = %#v", useData.Session)
	}

	listed := runJSONCommand(t, service, "env", "list", "--output", "json")
	var listData struct {
		Environments []struct {
			ID   model.EnvironmentID `json:"id"`
			Name string              `json:"name"`
		} `json:"environments"`
	}
	decodeData(t, listed, &listData)
	if got := []string{listData.Environments[0].Name}; !reflect.DeepEqual(got, []string{"base"}) {
		t.Fatalf("listed Environment names = %v", got)
	}
}

func TestJSONCommandFailures(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantPrompt string
	}{
		{name: "missing Environment", args: []string{"env", "use", "missing", "--session-id", "session-one", "--output", "json"}, wantCode: 404, wantPrompt: "List the available Environments and retry with an existing Environment ID or name."},
		{name: "multiple Environment arguments", args: []string{"env", "use", "alpha", "beta", "--output", "json"}, wantCode: 400, wantPrompt: `Run "hev env use --help" to inspect this command's required arguments.`},
		{name: "missing Environment argument", args: []string{"skill", "add", "search", "--output", "json"}, wantCode: 400, wantPrompt: `Run "hev skill add --help" to inspect this command's required arguments.`},
		{name: "rejects removed env flag", args: []string{"skill", "add", "search", "--env", "base", "--output", "json"}, wantCode: 400, wantPrompt: `Run "hev skill add --help" to inspect the supported flags.`},
		{name: "unsupported policy", args: []string{"skill", "add", "search", "base", "--policy", "always", "--output", "json"}, wantCode: 400, wantPrompt: "Retry with --policy auto or --policy off."},
		{name: "named list conflicts with session", args: []string{"skill", "list", "base", "--session-id", "session-one", "--output", "json"}, wantCode: 400, wantPrompt: "Use either an Environment ID or name, or --session-id for the current Environment."},
		{name: "cannot rename base", args: []string{"env", "rename", "base", "renamed", "--output", "json"}, wantCode: 409, wantPrompt: "Choose a non-base Environment to rename."},
		{name: "cannot delete base", args: []string{"env", "delete", "base", "--output", "json"}, wantCode: 409, wantPrompt: "Choose a non-base Environment to delete."},
		{name: "cannot remove base guide", args: []string{"skill", "remove", "hev-guide", "base", "--output", "json"}, wantCode: 409, wantPrompt: "Keep hev-guide enabled in base, or remove it only from a non-base Environment."},
		{name: "unknown command", args: []string{"unknown", "--output", "json"}, wantCode: 400, wantPrompt: `Run "hev --help" to inspect this command's required arguments.`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateDir := t.TempDir()
			store := newTestEnvironmentDAL(stateDir)
			service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "env_unused" })
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if exitCode := routers.Execute(
				context.Background(), service, &stdout, &stderr, test.args,
			); exitCode != 1 {
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
			if response.Prompt != test.wantPrompt {
				t.Fatalf("prompt = %q, want %q", response.Prompt, test.wantPrompt)
			}
			var data map[string]any
			decodeData(t, response, &data)
			if len(data) != 0 {
				t.Fatalf("error data = %#v, want empty object", data)
			}
		})
	}
}

func TestTextCommandFailureIncludesHint(t *testing.T) {
	service := environmentservice.NewEnvironment(
		newTestEnvironmentDAL(t.TempDir()),
		func() model.EnvironmentID { return "unused" },
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := routers.Execute(
		context.Background(),
		service,
		&stdout,
		&stderr,
		[]string{"env", "use", "missing", "--session-id", "session-one"},
	)
	if exitCode != 1 || stdout.Len() != 0 {
		t.Fatalf("exit code = %d, stdout = %q", exitCode, stdout.String())
	}
	want := "hev: select environment for session: resolve environment: get environment: environment not found: missing\n" +
		"hint: List the available Environments and retry with an existing Environment ID or name.\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCommandHelp(t *testing.T) {
	service := environmentservice.NewEnvironment(
		newTestEnvironmentDAL(t.TempDir()),
		func() model.EnvironmentID { return "unused" },
	)
	for _, test := range []struct {
		args []string
		want []string
	}{
		{args: []string{"--help"}, want: []string{"Manage isolated Skill environments", "hev env use coding --session-id <session-id>"}},
		{args: []string{"env", "use", "--help"}, want: []string{"Select exactly one existing Environment", "hev env use coding --session-id session-123"}},
		{args: []string{"env", "quit", "--help"}, want: []string{"non-base Environment, quit selects base", "base, quit deactivates"}},
		{args: []string{"env", "rename", "--help"}, want: []string{"preserving its stable ID", "lowercase kebab-case"}},
		{args: []string{"env", "delete", "--help"}, want: []string{"bound to the deleted Environment", "base Environment cannot be deleted"}},
		{args: []string{"skill", "add", "--help"}, want: []string{"does not install the Skill", "--policy off"}},
		{args: []string{"skill", "remove", "--help"}, want: []string{"atomically", "hev-guide from base"}},
		{args: []string{"skill", "list", "--help"}, want: []string{"without changing any Session", "Environment ID or name"}},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if exitCode := routers.Execute(context.Background(), service, &stdout, &stderr, test.args); exitCode != 0 {
			t.Fatalf("Execute(%q) exit code = %d, stderr=%q", test.args, exitCode, stderr.String())
		}
		for _, part := range test.want {
			if !strings.Contains(stdout.String(), part) {
				t.Errorf("Execute(%q) output does not contain %q: %s", test.args, part, stdout.String())
			}
		}
	}
}

func TestCLIContract(t *testing.T) {
	schema := compileResponseSchema(t)
	stateDir := t.TempDir()
	store := newTestEnvironmentDAL(stateDir)
	service := environmentservice.NewEnvironment(store, func() model.EnvironmentID { return "env_coding" })

	for _, test := range []struct {
		args        []string
		wantSuccess bool
	}{
		{args: []string{"env", "create", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "add", "code-review", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "list", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "rename", "coding", "backend", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "remove", "code-review", "backend", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "list", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "use", "backend", "--session-id", "contract-session", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "status", "--session-id", "contract-session", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "list", "--session-id", "contract-session", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "quit", "--session-id", "contract-session", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "delete", "backend", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "use", "--session-id", "contract-session", "--output", "json"}},
		{args: []string{"env", "use", "coding", "base", "--session-id", "contract-session", "--output", "json"}},
		{args: []string{"env", "use", "missing", "--session-id", "contract-session", "--output", "json"}},
		{args: []string{"env", "status", "--output", "json"}},
		{args: []string{"env", "create", "--output", "json"}},
		{args: []string{"skill", "add", "code-review", "coding", "--policy", "always", "--output", "json"}},
		{args: []string{"unknown", "--output", "json"}},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := routers.Execute(
			context.Background(), service, &stdout, &stderr, test.args,
		)
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
	renameResult    model.Environment
	renameErr       error
	deleteResult    model.Environment
	deleteErr       error
	defaultCalls    int
	defaultResult   model.Environment
	defaultErr      error
	listCalls       int
	listResult      []model.Environment
	listErr         error
	getCalls        []string
	getResults      map[string]model.Environment
	getErrors       map[string]error
	updateManyCalls []updateManyCall
	updateManyInput []model.Environment
	updateManyErr   error
}

func (s *stubEnvironmentStore) Source() model.Source {
	return model.SourceStandalone
}

func (s *stubEnvironmentStore) Create(_ context.Context, value model.Environment) (model.Environment, error) {
	s.createCalls = append(s.createCalls, value)
	if s.createErr != nil {
		return model.Environment{}, s.createErr
	}
	return value, nil
}

func (s *stubEnvironmentStore) Rename(
	_ context.Context,
	_ model.EnvironmentID,
	_ string,
) (model.Environment, error) {
	return cloneEnvironment(s.renameResult), s.renameErr
}

func (s *stubEnvironmentStore) Delete(
	_ context.Context,
	_ model.EnvironmentID,
) (model.Environment, error) {
	return cloneEnvironment(s.deleteResult), s.deleteErr
}

func (s *stubEnvironmentStore) Default(_ context.Context) (model.Environment, error) {
	s.defaultCalls++
	return cloneEnvironment(s.defaultResult), s.defaultErr
}

func (s *stubEnvironmentStore) List(_ context.Context) ([]model.Environment, error) {
	s.listCalls++
	return cloneEnvironments(s.listResult), s.listErr
}

func (s *stubEnvironmentStore) GetByIDOrName(_ context.Context, identifier string) (model.Environment, error) {
	s.getCalls = append(s.getCalls, identifier)
	if err := s.getErrors[identifier]; err != nil {
		return model.Environment{}, err
	}
	return cloneEnvironment(s.getResults[identifier]), nil
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

func (s *stubEnvironmentStore) GetSessionEnvironment(
	_ context.Context,
	_ string,
) (model.EnvironmentID, bool, error) {
	return "", false, nil
}

func (s *stubEnvironmentStore) SetSessionEnvironment(
	_ context.Context,
	_ string,
	_ model.EnvironmentID,
) error {
	return nil
}

func (s *stubEnvironmentStore) LeaveSessionEnvironment(
	_ context.Context,
	_ string,
	_ model.EnvironmentID,
	_ model.EnvironmentID,
) (model.EnvironmentID, bool, error) {
	return "", false, nil
}

type persistedStoreFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Environments  []model.Environment `json:"environments"`
}

func newTestEnvironmentDAL(stateDir string) *dal.EnvironmentDAL {
	return dal.NewEnvironmentDAL(model.SourceStandalone, stateDir)
}

func testEnvironment(
	id model.EnvironmentID,
	name string,
	revision uint64,
	skills ...model.EnvironmentSkill,
) model.Environment {
	return model.Environment{
		Source:   model.SourceStandalone,
		ID:       id,
		Name:     name,
		Revision: revision,
		Skills:   append([]model.EnvironmentSkill{}, skills...),
	}
}

func defaultBaseEnvironment(revision uint64) model.Environment {
	return testEnvironment(
		"base",
		"base",
		revision,
		defaultGuideBinding(),
	)
}

func defaultGuideBinding() model.EnvironmentSkill {
	return model.EnvironmentSkill{
		SkillKey: constants.DefaultGuideSkillKey,
		Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
	}
}

func replaceEnvironment(value model.Environment, replace func(*model.Environment)) model.Environment {
	value = cloneEnvironment(value)
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
		cloned[index] = cloneEnvironment(value)
	}
	return cloned
}

func cloneEnvironment(value model.Environment) model.Environment {
	if value.Skills == nil {
		return value
	}
	skills := make([]model.EnvironmentSkill, len(value.Skills))
	copy(skills, value.Skills)
	value.Skills = skills
	return value
}

func addSkillUpdate(skillKey model.SkillKey) func([]model.Environment) error {
	return func(values []model.Environment) error {
		for index := range values {
			values[index].Skills = append(values[index].Skills, model.EnvironmentSkill{
				SkillKey: skillKey,
				Policy:   model.EnvironmentSkillPolicy{Kind: constants.SkillPolicyAuto},
			})
		}
		return nil
	}
}

func requireStatus(t *testing.T, err error, want commonresponse.StatusCode) {
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
	got, ok := commonresponse.StatusCodeOf(err)
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

func runJSONCommand(
	t *testing.T,
	environmentService *environmentservice.EnvironmentService,
	args ...string,
) testResponse {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := routers.Execute(
		context.Background(), environmentService, &stdout, &stderr, args,
	); exitCode != 0 {
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
