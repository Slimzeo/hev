package v1_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestFixturesMatchSchema(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	fixtures, err := filepath.Glob(filepath.Join("fixtures", "*.json"))
	if err != nil {
		t.Fatalf("list fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no contract fixtures found")
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			content, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
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
