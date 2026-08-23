package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Slimzeo/hev/internal/application"
	"github.com/Slimzeo/hev/internal/domain"
	jsonstore "github.com/Slimzeo/hev/internal/store/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type testResponse struct {
	SchemaVersion int             `json:"schemaVersion"`
	Code          int             `json:"code"`
	Message       string          `json:"message"`
	Prompt        string          `json:"prompt"`
	Data          json.RawMessage `json:"data"`
}

func TestJSONCommandChain(t *testing.T) {
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
	ids := []domain.EnvironmentID{"env_coding"}
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID {
		id := ids[0]
		ids = ids[1:]
		return id
	})

	created := runJSONCommand(t, service, "env", "create", "coding", "--output", "json")
	if created.Message != "environment created" {
		t.Fatalf("create message = %q", created.Message)
	}
	var createData struct {
		Environment domain.Environment `json:"environment"`
	}
	decodeData(t, created, &createData)
	if createData.Environment.ID != "env_coding" || createData.Environment.Revision != 1 {
		t.Fatalf("unexpected created environment: %#v", createData.Environment)
	}
	if createData.Environment.Skills == nil || len(createData.Environment.Skills) != 0 {
		t.Fatalf("created skills = %#v, want empty array", createData.Environment.Skills)
	}

	added := runJSONCommand(
		t, service, "skill", "add", "code-review", "--env", "coding", "--policy", "off", "--output", "json",
	)
	if added.Message != "skill added to environment" {
		t.Fatalf("add message = %q", added.Message)
	}

	activated := runJSONCommand(t, service, "env", "activate", "coding", "--output", "json")
	var activateData struct {
		Snapshot domain.ResolvedEnvironmentSnapshot `json:"snapshot"`
	}
	decodeData(t, activated, &activateData)
	if len(activateData.Snapshot.Environments) != 1 {
		t.Fatalf("resolved environments = %#v", activateData.Snapshot.Environments)
	}
	environment := activateData.Snapshot.Environments[0]
	if environment.Revision != 2 {
		t.Fatalf("revision = %d, want 2", environment.Revision)
	}
	if len(environment.Skills) != 1 || environment.Skills[0].Policy.Kind != domain.SkillPolicyOff {
		t.Fatalf("skills = %#v", environment.Skills)
	}
}

func TestJSONFailureUsesStableEnvelope(t *testing.T) {
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "env_unused" })
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute(context.Background(), service, &stdout, &stderr, []string{"env", "activate", "missing", "--output", "json"})
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var response testResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; output=%q", err, stdout.String())
	}
	if response.SchemaVersion != 1 || response.Code != 404 {
		t.Fatalf("response = %#v", response)
	}
	var data errorData
	decodeData(t, response, &data)
	if data.ErrorCode != domain.ErrorCodeEnvironmentNotFound {
		t.Fatalf("error code = %q", data.ErrorCode)
	}
}

func TestJSONCommandOutputsMatchContract(t *testing.T) {
	schema := compileResponseSchema(t)
	store := jsonstore.NewEnvironmentStore(filepath.Join(t.TempDir(), "environments.json"))
	service := application.NewEnvironmentService(store, func() domain.EnvironmentID { return "env_coding" })

	for _, test := range []struct {
		args        []string
		wantSuccess bool
	}{
		{args: []string{"env", "create", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"skill", "add", "code-review", "--env", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "activate", "coding", "--output", "json"}, wantSuccess: true},
		{args: []string{"env", "activate", "missing", "--output", "json"}},
		{args: []string{"env", "create", "--output", "json"}},
		{args: []string{"skill", "add", "code-review", "--env", "coding", "--policy", "always", "--output", "json"}},
		{args: []string{"unknown", "--output", "json"}},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := Execute(context.Background(), service, &stdout, &stderr, test.args)
		if stderr.Len() != 0 {
			t.Fatalf("Execute(%q) stderr = %q, want empty", test.args, stderr.String())
		}
		if test.wantSuccess && exitCode != 0 {
			t.Fatalf("Execute(%q) exit code = %d, want success", test.args, exitCode)
		}
		if !test.wantSuccess && exitCode == 0 {
			t.Fatalf("Execute(%q) succeeded, want failure", test.args)
		}

		var value any
		decoder := json.NewDecoder(&stdout)
		if err := decoder.Decode(&value); err != nil {
			t.Fatalf("Execute(%q) output is not JSON: %v; output=%q", test.args, err, stdout.String())
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			t.Fatalf("Execute(%q) wrote more than one JSON value: %q", test.args, stdout.String())
		}
		if err := schema.Validate(value); err != nil {
			t.Fatalf("Execute(%q) output does not match CLI contract: %v; output=%q", test.args, err, stdout.String())
		}
	}
}

func runJSONCommand(t *testing.T, service *application.EnvironmentService, args ...string) testResponse {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := Execute(context.Background(), service, &stdout, &stderr, args); exitCode != 0 {
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
	if response.SchemaVersion != 1 || response.Code != 200 || response.Prompt != "" {
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
	path := filepath.Join("..", "..", "contracts", "cli", "v1", "schema.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CLI schema: %v", err)
	}
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
