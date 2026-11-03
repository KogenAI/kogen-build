package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-executor/executor"
)

// The builder's policy moved here from kogen-executor's agent package with its recorded bytes:
// testdata/prompts_golden.json and testdata/defs_golden.json are executor's entries for the
// builder, copied verbatim. They record the bytes builders have always received, so they change
// only deliberately; "go test ./... -update" rewrites them from the current output.

func policyJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestPromptsAreByteEqualToTheElixirFunctions compares the length and SHA-256 of the builder's
// instructions with what the Elixir function returned.
func TestPromptsAreByteEqualToTheElixirFunctions(t *testing.T) {
	type entry struct {
		Bytes  int    `json:"bytes"`
		Sha256 string `json:"sha256"`
	}
	if *update {
		text := BuilderInstructions()
		sum := sha256.Sum256([]byte(text))
		out, err := indent2Golden.encode(map[string]entry{"builder": {Bytes: len(text), Sha256: hex.EncodeToString(sum[:])}})
		if err != nil {
			t.Fatal(err)
		}
		writeGoldenFile(t, "testdata/prompts_golden.json", out)
	}
	data, err := os.ReadFile("testdata/prompts_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]entry
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != 1 {
		t.Fatalf("golden has %d prompts, the builder has 1", len(want))
	}
	t.Run("builder", func(t *testing.T) {
		w, ok := want["builder"]
		if !ok {
			t.Fatal("no golden entry for builder")
		}
		text := BuilderInstructions()
		sum := sha256.Sum256([]byte(text))
		if len(text) != w.Bytes || hex.EncodeToString(sum[:]) != w.Sha256 {
			t.Errorf("builder: %d bytes sha256 %x, want %d bytes sha256 %s", len(text), sum, w.Bytes, w.Sha256)
		}
		if strings.Contains(text, "{{") {
			t.Error("builder has an unreplaced placeholder")
		}
	})
}

// TestDefinitionsMatchTheElixirDefinitions compares finish and report_blocker with
// testdata/defs_golden.json, as parsed values.
func TestDefinitionsMatchTheElixirDefinitions(t *testing.T) {
	owned := []string{ReportBlockerTool, FinishTool}
	if *update {
		defs := map[string]any{}
		for _, name := range owned {
			var def any
			if err := json.Unmarshal(policyJSON(t, BuilderToolDefinitions(name)[0]), &def); err != nil {
				t.Fatal(err)
			}
			defs[name] = def
		}
		out, err := indent2Golden.encode(defs)
		if err != nil {
			t.Fatal(err)
		}
		writeGoldenFile(t, "testdata/defs_golden.json", out)
	}
	data, err := os.ReadFile("testdata/defs_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(owned) {
		t.Fatalf("%d builder-owned names, golden has %d", len(owned), len(want))
	}
	for _, name := range owned {
		t.Run(name, func(t *testing.T) {
			var got any
			if err := json.Unmarshal(policyJSON(t, BuilderToolDefinitions(name)[0]), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want[name]) {
				t.Errorf("definition differs\n got: %s\nwant: %s", policyJSON(t, got), policyJSON(t, want[name]))
			}
		})
	}
}

func TestEveryDefinitionIsPlainJSON(t *testing.T) {
	// Provider requests encode []any and map[string]any only through their own encoder; a typed
	// slice or a struct in a definition would be encoded differently.
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, e := range x {
				walk(path+"."+k, e)
			}
		case []any:
			for _, e := range x {
				walk(path+"[]", e)
			}
		case string, bool, int, int64, float64, nil:
		default:
			t.Errorf("%s is %T", path, v)
		}
	}
	for _, name := range []string{ReportBlockerTool, FinishTool} {
		walk(name, BuilderToolDefinitions(name)[0])
	}
}

func TestUnknownBuilderToolNamesPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("unknown tool did not panic")
		}
	}()
	BuilderToolDefinitions("nope")
}

// TestSubmissionSchemaIsTheExternalBuildersResult pins the external builder's structured
// submission, which moved here from kogen-executor's Claude Code adapter (its BuildSchema): the
// exact bytes it always sent as --json-schema (309 bytes, SHA-256 below), and that its blocker
// is exactly the executor contract's builder-reported blocker claim, which the adapter decodes.
func TestSubmissionSchemaIsTheExternalBuildersResult(t *testing.T) {
	sum := sha256.Sum256([]byte(SubmissionSchema))
	if len(SubmissionSchema) != 309 || hex.EncodeToString(sum[:]) != "e3679f8c9225fd0a9580b414767daeb8cf4de3df1d36ad2f9aee90e12ff77ce4" {
		t.Errorf("the submission schema changed: %d bytes, %x", len(SubmissionSchema), sum)
	}
	var schema struct {
		Type       string `json:"type"`
		Properties struct {
			Summary map[string]any `json:"summary"`
			Blocker struct {
				Type                 string                    `json:"type"`
				Properties           map[string]map[string]any `json:"properties"`
				Required             []string                  `json:"required"`
				AdditionalProperties bool                      `json:"additionalProperties"`
			} `json:"blocker"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(SubmissionSchema), &schema); err != nil {
		t.Fatal(err)
	}
	var fields []string
	claim := reflect.TypeOf(executor.BlockerClaim{})
	for i := range claim.NumField() {
		name, _, _ := strings.Cut(claim.Field(i).Tag.Get("json"), ",")
		fields = append(fields, name)
	}
	blocker := schema.Properties.Blocker
	if blocker.Type != "object" || blocker.AdditionalProperties || !reflect.DeepEqual(blocker.Required, fields) || len(blocker.Properties) != len(fields) {
		t.Errorf("the blocker %+v is not the executor's blocker claim %v", blocker, fields)
	}
	for _, f := range fields {
		if _, ok := blocker.Properties[f]; !ok {
			t.Errorf("the blocker has no %s", f)
		}
	}
}
