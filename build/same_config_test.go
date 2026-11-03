package build

import (
	"encoding/json"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// TestSameConfigIgnoresTheRemovedOptions: resume and recover compare the saved and the current
// configuration in everything but the opt-in options the Build no longer reads.
func TestSameConfigIgnoresTheRemovedOptions(t *testing.T) {
	t.Parallel()
	base := func() *project.Config {
		return &project.Config{
			Checks:  []project.Check{{Name: "unit", Run: "true"}},
			Protect: []string{"accept.sh"},
			Models:  map[string]project.Role{"build": {Model: "m", Effort: "high"}},
		}
	}
	// mutate edits a copy of the configuration through its JSON form, so that the cases name the
	// removed options by their configuration keys only.
	mutate := func(f func(doc map[string]any)) *project.Config {
		raw, err := json.Marshal(base())
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		f(doc)
		raw, err = json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		c := &project.Config{}
		if err := json.Unmarshal(raw, c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, tc := range []struct {
		name string
		edit func(doc map[string]any)
		same bool
	}{
		{"max_rounds", func(d map[string]any) {
			d["Models"].(map[string]any)["build"].(map[string]any)["MaxRounds"] = 4
		}, true},
		{"review_material on_demand", func(d map[string]any) { d["ReviewMaterial"] = map[string]any{"Mode": "on_demand"} }, true},
		{"review_material empty mode", func(d map[string]any) { d["ReviewMaterial"] = map[string]any{"Mode": ""} }, true},
		{"shape_workers", func(d map[string]any) { d["ShapeWorkers"] = map[string]any{"Enabled": true} }, true},
		{"shape_audit", func(d map[string]any) { d["ShapeAudit"] = map[string]any{"Enabled": true} }, true},
		{"shape_decision_check", func(d map[string]any) { d["ShapeDecisionCheck"] = map[string]any{"Enabled": true} }, true},
		{"checks", func(d map[string]any) {
			d["Checks"] = []any{map[string]any{"Name": "unit", "Run": "false"}}
		}, false},
		{"model", func(d map[string]any) {
			d["Models"].(map[string]any)["build"].(map[string]any)["Model"] = "other"
		}, false},
		{"protect", func(d map[string]any) { d["Protect"] = []any{"other.sh"} }, false},
	} {
		if got := sameConfig(base(), mutate(tc.edit)); got != tc.same {
			t.Errorf("%s: sameConfig = %v, want %v", tc.name, got, tc.same)
		}
	}
	if !sameConfig(nil, nil) || sameConfig(base(), nil) || sameConfig(nil, base()) {
		t.Error("nil configurations compare wrongly")
	}
}
