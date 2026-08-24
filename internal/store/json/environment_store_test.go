package jsonstore_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Slimzeo/hev/internal/application"
	"github.com/Slimzeo/hev/internal/domain"
	jsonstore "github.com/Slimzeo/hev/internal/store/json"
)

var _ application.EnvironmentStore = (*jsonstore.EnvironmentStore)(nil)

func TestEnvironmentStoreCreateAndReload_BitsUT(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "environments.json")
	store := jsonstore.NewEnvironmentStore(path)
	auto := domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto}
	beta := storeEnvironment(
		"env-beta",
		"beta",
		1,
		domain.EnvironmentSkillSpec{SkillKey: "search", Policy: auto},
	)

	created, err := store.Create(context.Background(), beta)
	if err != nil {
		t.Fatalf("Create beta returned error: %v", err)
	}
	if !reflect.DeepEqual(created, beta) {
		t.Fatalf("created environment = %#v, want %#v", created, beta)
	}
	if _, err := store.Create(context.Background(), storeEnvironment("env-alpha", "alpha", 1)); err != nil {
		t.Fatalf("Create alpha returned error: %v", err)
	}

	beta.Skills[0].SkillKey = "changed-input"
	created.Skills[0].SkillKey = "changed-result"
	reloaded := jsonstore.NewEnvironmentStore(path)
	for _, identifier := range []string{"env-beta", "beta"} {
		got, err := getOne(t, reloaded, identifier)
		if err != nil {
			t.Fatalf("GetManyByIDOrName(%q) returned error: %v", identifier, err)
		}
		if got.Skills[0].SkillKey != "search" {
			t.Fatalf("GetManyByIDOrName(%q) skill = %q, want search", identifier, got.Skills[0].SkillKey)
		}
		got.Skills[0].SkillKey = "changed-read"
	}
	got, err := getOne(t, reloaded, "beta")
	if err != nil {
		t.Fatalf("second GetManyByIDOrName returned error: %v", err)
	}
	if got.Skills[0].SkillKey != "search" {
		t.Fatalf("persisted skill changed through returned alias: got %q", got.Skills[0].SkillKey)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !bytes.HasSuffix(content, []byte{'\n'}) {
		t.Fatal("store file does not end with a newline")
	}
	var persisted persistedStoreFile
	if err := json.Unmarshal(content, &persisted); err != nil {
		t.Fatalf("persisted JSON is invalid: %v", err)
	}
	if persisted.SchemaVersion != 1 {
		t.Errorf("schema version = %d, want 1", persisted.SchemaVersion)
	}
	wantNames := []string{"alpha", "base", "beta"}
	gotNames := make([]string, len(persisted.Environments))
	for index, environment := range persisted.Environments {
		gotNames[index] = environment.Name
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("persisted environment order = %v, want %v", gotNames, wantNames)
	}
}

func TestEnvironmentStoreInitializesAndPersistsBaseEnvironment_BitsUT(t *testing.T) {
	tests := []struct {
		name  string
		setup func(string)
	}{
		{
			name:  "missing file",
			setup: func(string) {},
		},
		{
			name: "empty environments array",
			setup: func(path string) {
				if err := os.WriteFile(path, []byte("{\"schemaVersion\":1,\"environments\":[]}\n"), 0o600); err != nil {
					t.Fatalf("WriteFile returned error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "environments.json")
			tt.setup(path)
			store := jsonstore.NewEnvironmentStore(path)

			environment, err := getOne(t, store, "base")
			if err != nil {
				t.Fatalf("GetManyByIDOrName returned error: %v", err)
			}
			want := storeEnvironment("env_base", "base", 1)
			if !reflect.DeepEqual(environment, want) {
				t.Fatalf("base environment = %#v, want %#v", environment, want)
			}

			content := mustReadFile(t, path)
			var persisted persistedStoreFile
			if err := json.Unmarshal(content, &persisted); err != nil {
				t.Fatalf("persisted JSON is invalid: %v", err)
			}
			if !reflect.DeepEqual(persisted.Environments, []domain.Environment{want}) {
				t.Fatalf("persisted environments = %#v, want %#v", persisted.Environments, []domain.Environment{want})
			}
		})
	}
}

func TestEnvironmentStoreCreateRejectsInvalidOrDuplicateEnvironment_BitsUT(t *testing.T) {
	tests := []struct {
		name      string
		candidate domain.Environment
		wantCode  domain.ErrorCode
	}{
		{name: "new revision is not one", candidate: storeEnvironment("env-beta", "beta", 2), wantCode: domain.ErrorCodeInvalidArgument},
		{name: "nil skills", candidate: domain.Environment{ID: "env-beta", Name: "beta", Revision: 1, Skills: nil}, wantCode: domain.ErrorCodeInvalidArgument},
		{name: "duplicate id", candidate: storeEnvironment("env-alpha", "beta", 1), wantCode: domain.ErrorCodeEnvironmentAlreadyExists},
		{name: "duplicate name", candidate: storeEnvironment("env-beta", "alpha", 1), wantCode: domain.ErrorCodeEnvironmentAlreadyExists},
		{name: "id conflicts with existing name", candidate: storeEnvironment("alpha", "beta", 1), wantCode: domain.ErrorCodeEnvironmentAlreadyExists},
		{name: "name conflicts with existing id", candidate: storeEnvironment("env-beta", "env-alpha", 1), wantCode: domain.ErrorCodeEnvironmentAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "environments.json")
			store := jsonstore.NewEnvironmentStore(path)
			if _, err := store.Create(context.Background(), storeEnvironment("env-alpha", "alpha", 1)); err != nil {
				t.Fatalf("seed Create returned error: %v", err)
			}
			before := mustReadFile(t, path)

			_, err := store.Create(context.Background(), tt.candidate)
			requireStoreDomainCode(t, err, tt.wantCode)
			after := mustReadFile(t, path)
			if !bytes.Equal(after, before) {
				t.Fatalf("failed Create changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
			}
		})
	}
}

func TestEnvironmentStoreGetManyByIDOrName_NotFound_BitsUT(t *testing.T) {
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))

	_, err := store.GetManyByIDOrName(context.Background(), []string{"missing"})
	requireStoreDomainCode(t, err, domain.ErrorCodeEnvironmentNotFound)
	for _, part := range []string{"get environments", "environment not found: missing"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
}

func TestEnvironmentStoreGetManyByIDOrName_PreservesRequestOrder_BitsUT(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	store := jsonstore.NewEnvironmentStore(path)
	for _, environment := range []domain.Environment{
		storeEnvironment("env-alpha", "alpha", 1),
		storeEnvironment("env-beta", "beta", 1),
	} {
		if _, err := store.Create(context.Background(), environment); err != nil {
			t.Fatalf("seed Create(%q) returned error: %v", environment.Name, err)
		}
	}

	environments, err := store.GetManyByIDOrName(context.Background(), []string{"beta", "env-alpha"})
	if err != nil {
		t.Fatalf("GetManyByIDOrName returned error: %v", err)
	}
	wantIDs := []domain.EnvironmentID{"env-beta", "env-alpha"}
	for index, wantID := range wantIDs {
		if environments[index].ID != wantID {
			t.Errorf("environment %d id = %q, want %q", index, environments[index].ID, wantID)
		}
	}
}

func TestEnvironmentStoreUpdateMany_PersistsAtomicallyInRequestOrder_BitsUT(t *testing.T) {
	path := filepath.Join(t.TempDir(), "environments.json")
	store := jsonstore.NewEnvironmentStore(path)
	for _, environment := range []domain.Environment{
		storeEnvironment("env-alpha", "alpha", 1),
		storeEnvironment("env-beta", "beta", 1),
	} {
		if _, err := store.Create(context.Background(), environment); err != nil {
			t.Fatalf("seed Create(%q) returned error: %v", environment.Name, err)
		}
	}
	wantSpec := domain.EnvironmentSkillSpec{
		SkillKey: "search",
		Policy:   domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto},
	}

	updated, err := store.UpdateMany(context.Background(), []string{"env-beta", "alpha"}, func(environments []domain.Environment) error {
		wantIDs := []domain.EnvironmentID{"env-beta", "env-alpha"}
		for index, environment := range environments {
			if environment.ID != wantIDs[index] {
				t.Errorf("callback environment %d id = %q, want %q", index, environment.ID, wantIDs[index])
			}
			environments[index].Skills = append(environments[index].Skills, wantSpec)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateMany returned error: %v", err)
	}
	wantIDs := []domain.EnvironmentID{"env-beta", "env-alpha"}
	if len(updated) != len(wantIDs) {
		t.Fatalf("UpdateMany returned %d environments, want %d", len(updated), len(wantIDs))
	}
	for index, environment := range updated {
		if environment.ID != wantIDs[index] {
			t.Errorf("updated environment %d id = %q, want %q", index, environment.ID, wantIDs[index])
		}
		if environment.Revision != 2 {
			t.Errorf("updated environment %d revision = %d, want 2", index, environment.Revision)
		}
		if len(environment.Skills) != 1 || environment.Skills[0] != wantSpec {
			t.Errorf("updated environment %d skills = %#v, want [%#v]", index, environment.Skills, wantSpec)
		}
	}

	updated[0].Skills[0].SkillKey = "changed-result"
	for _, name := range []string{"alpha", "beta"} {
		got, err := getOne(t, jsonstore.NewEnvironmentStore(path), name)
		if err != nil {
			t.Fatalf("GetManyByIDOrName(%q) returned error: %v", name, err)
		}
		if got.Revision != 2 {
			t.Errorf("persisted %q revision = %d, want 2", name, got.Revision)
		}
		if len(got.Skills) != 1 || got.Skills[0] != wantSpec {
			t.Errorf("persisted %q skills = %#v, want [%#v]", name, got.Skills, wantSpec)
		}
	}
}

func TestEnvironmentStoreUpdateMany_RollsBackOnFailure_BitsUT(t *testing.T) {
	callbackErr := errors.New("callback failed")
	tests := []struct {
		name          string
		identifiers   []string
		update        func([]domain.Environment) error
		wantCode      domain.ErrorCode
		wantCause     error
		wantCalled    bool
		wantErrorPart string
	}{
		{
			name:          "missing target",
			identifiers:   []string{"alpha", "missing"},
			update:        addSkillUpdate("search"),
			wantCode:      domain.ErrorCodeEnvironmentNotFound,
			wantErrorPart: "environment not found: missing",
		},
		{
			name:        "same environment selected by id and name",
			identifiers: []string{"env-alpha", "alpha"},
			update:      addSkillUpdate("search"),
			wantCode:    domain.ErrorCodeInvalidArgument,
		},
		{
			name:        "callback error after mutation",
			identifiers: []string{"alpha", "beta"},
			update: func(environments []domain.Environment) error {
				environments[0].Skills = append(environments[0].Skills, domain.EnvironmentSkillSpec{
					SkillKey: "search",
					Policy:   domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto},
				})
				return callbackErr
			},
			wantCause:  callbackErr,
			wantCalled: true,
		},
		{
			name:        "callback changes identity",
			identifiers: []string{"alpha", "beta"},
			update: func(environments []domain.Environment) error {
				environments[1].Name = "renamed"
				return nil
			},
			wantCode:   domain.ErrorCodeInvalidArgument,
			wantCalled: true,
		},
		{
			name:        "callback produces invalid aggregate",
			identifiers: []string{"alpha", "beta"},
			update: func(environments []domain.Environment) error {
				environments[1].Skills = nil
				return nil
			},
			wantCode:   domain.ErrorCodeInvalidArgument,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "environments.json")
			store := jsonstore.NewEnvironmentStore(path)
			for _, environment := range []domain.Environment{
				storeEnvironment("env-alpha", "alpha", 1),
				storeEnvironment("env-beta", "beta", 1),
			} {
				if _, err := store.Create(context.Background(), environment); err != nil {
					t.Fatalf("seed Create(%q) returned error: %v", environment.Name, err)
				}
			}
			before := mustReadFile(t, path)
			called := false

			_, err := store.UpdateMany(context.Background(), tt.identifiers, func(environments []domain.Environment) error {
				called = true
				return tt.update(environments)
			})
			if tt.wantCode != "" {
				requireStoreDomainCode(t, err, tt.wantCode)
			} else if !errors.Is(err, tt.wantCause) {
				t.Fatalf("UpdateMany error = %v, want cause %v", err, tt.wantCause)
			}
			if called != tt.wantCalled {
				t.Errorf("callback called = %t, want %t", called, tt.wantCalled)
			}
			if tt.wantErrorPart != "" && !strings.Contains(err.Error(), tt.wantErrorPart) {
				t.Errorf("error %q does not contain %q", err, tt.wantErrorPart)
			}
			after := mustReadFile(t, path)
			if !bytes.Equal(after, before) {
				t.Fatalf("failed UpdateMany changed persisted bytes:\nbefore: %s\nafter: %s", before, after)
			}
		})
	}
}

func TestEnvironmentStoreRejectsInvalidPersistedData_BitsUT(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantErrorPart string
	}{
		{name: "empty file", content: "", wantErrorPart: "get environments: decode store"},
		{name: "malformed json", content: "{", wantErrorPart: "get environments: decode store"},
		{name: "unsupported schema", content: `{"schemaVersion":2,"environments":[]}`, wantErrorPart: "unsupported store schema version: 2"},
		{name: "missing environments array", content: `{"schemaVersion":1}`, wantErrorPart: "decode store: environments must be an array"},
		{name: "null environments array", content: `{"schemaVersion":1,"environments":null}`, wantErrorPart: "decode store: environments must be an array"},
		{
			name:          "invalid environment",
			content:       `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":null}]}`,
			wantErrorPart: "invalid stored environment",
		},
		{
			name:          "duplicate environment id",
			content:       `{"schemaVersion":1,"environments":[{"id":"env-shared","name":"alpha","revision":1,"skills":[]},{"id":"env-shared","name":"beta","revision":1,"skills":[]}]}`,
			wantErrorPart: `invalid store: duplicate environment id "env-shared"`,
		},
		{
			name:          "duplicate environment name",
			content:       `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[]},{"id":"env-other","name":"alpha","revision":1,"skills":[]}]}`,
			wantErrorPart: `invalid store: duplicate environment name "alpha"`,
		},
		{
			name:          "environment id conflicts with a name",
			content:       `{"schemaVersion":1,"environments":[{"id":"env-alpha","name":"alpha","revision":1,"skills":[]},{"id":"alpha","name":"beta","revision":1,"skills":[]}]}`,
			wantErrorPart: `invalid store: environment id "alpha" conflicts with an environment name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "environments.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			store := jsonstore.NewEnvironmentStore(path)

			_, err := store.GetManyByIDOrName(context.Background(), []string{"alpha"})
			if err == nil {
				t.Fatal("GetManyByIDOrName returned nil error for invalid persisted data")
			}
			if !strings.Contains(err.Error(), tt.wantErrorPart) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErrorPart)
			}
		})
	}
}

func TestEnvironmentStorePropagatesDirectoryCreationError_BitsUT(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	store := jsonstore.NewEnvironmentStore(filepath.Join(parentFile, "environments.json"))

	_, err := store.GetManyByIDOrName(context.Background(), []string{"alpha"})
	if err == nil {
		t.Fatal("GetManyByIDOrName returned nil error when the store parent cannot be created")
	}
	if !strings.Contains(err.Error(), "get environments: create store directory") {
		t.Fatalf("error %q does not include operation and directory context", err)
	}
}

type persistedStoreFile struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Environments  []domain.Environment `json:"environments"`
}

func storeEnvironment(
	id domain.EnvironmentID,
	name string,
	revision uint64,
	skills ...domain.EnvironmentSkillSpec,
) domain.Environment {
	return domain.Environment{
		ID:       id,
		Name:     name,
		Revision: revision,
		Skills:   append([]domain.EnvironmentSkillSpec{}, skills...),
	}
}

func addSkillUpdate(skillKey domain.SkillKey) func([]domain.Environment) error {
	return func(environments []domain.Environment) error {
		for index := range environments {
			environments[index].Skills = append(environments[index].Skills, domain.EnvironmentSkillSpec{
				SkillKey: skillKey,
				Policy:   domain.EnvironmentSkillPolicy{Kind: domain.SkillPolicyAuto},
			})
		}
		return nil
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

func getOne(t *testing.T, store *jsonstore.EnvironmentStore, identifier string) (domain.Environment, error) {
	t.Helper()
	environments, err := store.GetManyByIDOrName(context.Background(), []string{identifier})
	if err != nil {
		return domain.Environment{}, err
	}
	return environments[0], nil
}

func requireStoreDomainCode(t *testing.T, err error, want domain.ErrorCode) {
	t.Helper()
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
