package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONSchemaMatchesModel(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "schema", "v2.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema struct {
		Schema     string                     `json:"$schema"`
		ID         string                     `json:"$id"`
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("$schema = %q", schema.Schema)
	}
	if !strings.Contains(schema.ID, "schema/v2.json") {
		t.Errorf("$id = %q", schema.ID)
	}

	var version struct {
		Const string `json:"const"`
	}
	if err := json.Unmarshal(schema.Properties["schemaVersion"], &version); err != nil {
		t.Fatalf("schemaVersion definition: %v", err)
	}
	if version.Const != SchemaVersion {
		t.Fatalf("schema const %q != model.SchemaVersion %q", version.Const, SchemaVersion)
	}

	// Every required top-level key must actually be emitted by an empty
	// snapshot, otherwise the schema would describe fields that never appear.
	encoded, err := json.Marshal(Snapshot{})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	for _, key := range schema.Required {
		if _, ok := document[key]; !ok {
			t.Errorf("required key %q is not emitted by Snapshot", key)
		}
	}

	// Optional privacy metadata must stay optional in the contract.
	if _, ok := document["redaction"]; ok {
		t.Error("redaction should be omitted from a raw snapshot; the CLI adds it")
	}
	if _, ok := document["timings"]; ok {
		t.Error("timings should be omitted when nil")
	}
}

func TestJSONSchemaReferencesExist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "v2.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	defs, _ := root["$defs"].(map[string]any)
	if len(defs) == 0 {
		t.Fatal("schema has no $defs")
	}

	var walk func(node any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "$ref" {
					ref, _ := child.(string)
					const prefix = "#/$defs/"
					if !strings.HasPrefix(ref, prefix) {
						t.Errorf("unsupported $ref %q", ref)
						continue
					}
					if _, ok := defs[strings.TrimPrefix(ref, prefix)]; !ok {
						t.Errorf("$ref %q has no definition", ref)
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(root)
}
