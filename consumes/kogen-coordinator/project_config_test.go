package consumes

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// build/build.go, build/resume.go and build/recover.go read the project configuration of the
// checkout, build/builder.go and build/host.go the commands and the acceptance in it and the
// models in .kogen/models.yaml, build/build.go and build/executor_external.go the roles.
func TestLoadConfigReadsTheProjectConfiguration(t *testing.T) {
	var _ func(string) (*project.Config, error) = project.LoadConfig
	equal(t, "project.ConfigPath", project.ConfigPath, ".kogen/project.yaml")
	var _ string = project.ConfigPath

	f := newProject(t)
	cfg, err := project.LoadConfig(f.Dir)
	check(t, err)
	var _ []string = cfg.Setup
	var _ []string = cfg.Fix
	var _ []project.Check = cfg.Checks
	var _ *project.Acceptance = cfg.Acceptance
	var _ []string = cfg.Protect
	var _ map[string]project.Role = cfg.Models

	equalList(t, "project.Config.Setup", cfg.Setup, []string{"npm ci", "go mod download"})
	equalList(t, "project.Config.Fix", cfg.Fix, []string{"gofmt -w ."})
	equalList(t, "project.Config.Protect", cfg.Protect, []string{"guard.cfg"})
	if len(cfg.Checks) != 2 {
		t.Fatalf("project.Config.Checks = %+v, want two", cfg.Checks)
	}
	var format, goTest project.Check = cfg.Checks[0], cfg.Checks[1]
	var _ string = format.Name
	var _ string = format.Run
	equal(t, "project.Check.Name", format.Name, "format")
	equal(t, "project.Check.Run", format.Run, `test -z "$(gofmt -l .)"`)
	equal(t, "project.Check.Name of the second check (configured order)", goTest.Name, "go-test")
	equal(t, "project.Check.Run of the second check", goTest.Run, "go test ./...")

	if cfg.Acceptance == nil {
		t.Fatal("project.Config.Acceptance is nil for a configured acceptance")
	}
	var _ string = cfg.Acceptance.Run
	var _ string = cfg.Acceptance.StepsDir
	equal(t, "project.Acceptance.Run", cfg.Acceptance.Run, "sh accept.sh")
	equal(t, "project.Acceptance.StepsDir (without its trailing slash)", cfg.Acceptance.StepsDir, "features/steps")

	build, review := cfg.Models["build"], cfg.Models["review"]
	var _ string = build.Provider
	var _ string = build.Model
	var _ string = build.Effort
	var _ string = build.Executor
	var _ func(project.Role) bool = project.Role.External
	equal(t, "project.Role.Provider", build.Provider, "anthropic")
	equal(t, "project.Role.Model", build.Model, "build-model")
	equal(t, "project.Role.Effort", build.Effort, "high")
	equal(t, "project.Role.Executor of a native role", build.Executor, "")
	equal(t, "project.Role.External of a native role", build.External(), false)
	equal(t, "project.Role.Executor of an external role", review.Executor, "claude-code")
	equal(t, "project.Role.External of an external role", review.External(), true)
	equal(t, "project.Role.Model of an external role", review.Model, "review-model")
	equal(t, "project.Role.Effort of an external role", review.Effort, "max")
}

// A configuration that names no acceptance has none (build/build.go reads it only when set);
// a role the file does not name has a default, so cfg.Models["build"] and ["review"] always
// exist for build's provider sign-in and executor selection.
func TestLoadConfigDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, project.ConfigPath, "version: 1\n")
	cfg, err := project.LoadConfig(root)
	check(t, err)
	if cfg.Acceptance != nil {
		t.Errorf("project.Config.Acceptance of a configuration without one = %+v, want nil", cfg.Acceptance)
	}
	equal(t, "project.Config.Checks without checks", len(cfg.Checks), 0)
	equal(t, "project.Config.Setup without setup", len(cfg.Setup), 0)
	equal(t, "project.Config.Fix without fix", len(cfg.Fix), 0)
	equal(t, "project.Config.Protect without protect", len(cfg.Protect), 0)
	for _, name := range []string{"shape", "build", "review"} {
		role, ok := cfg.Models[name]
		if !ok || role.Model == "" || role.Effort == "" {
			t.Errorf("project.Config.Models[%q] = %+v (present %v), want a default model and effort", name, role, ok)
		}
		equal(t, "project.Role.External of the default "+name+" role", role.External(), false)
	}

	only := t.TempDir()
	writeFile(t, only, project.ConfigPath, "version: 1\ncommands:\n  acceptance:\n    run: sh accept.sh\n")
	cfg, err = project.LoadConfig(only)
	check(t, err)
	if cfg.Acceptance == nil || cfg.Acceptance.Run != "sh accept.sh" || cfg.Acceptance.StepsDir != "" {
		t.Errorf("project.Config.Acceptance of a run without steps_dir = %+v, want {Run: sh accept.sh}", cfg.Acceptance)
	}
}

// The errors build passes on to the user as the stdout `error` object.
func TestLoadConfigErrors(t *testing.T) {
	root := t.TempDir()
	_, err := project.LoadConfig(root)
	e := wantSysError(t, "project.LoadConfig without a configuration", err, "environment", "not_initialized", 3)
	equal(t, "project.LoadConfig not_initialized detail", e.Detail, root+" has no .kogen/project.yaml; run kogen init")

	writeFile(t, root, project.ConfigPath, "version: 1\nbogus: 1\n")
	_, err = project.LoadConfig(root)
	e = wantSysError(t, "project.LoadConfig of an invalid configuration", err, "environment", "project_config_invalid", 3)
	equalList(t, "project.LoadConfig invalid lines", e.Lines, []string{"line 2: unknown key bogus"})

	writeFile(t, root, project.ConfigPath, "commands:\n  setup: []\n")
	_, err = project.LoadConfig(root)
	wantSysError(t, "project.LoadConfig of an old configuration", err, "environment", "old_kogen_config", 3)
}

// build/core_state.go checkpoints the configuration as JSON (the field names are the keys) and
// build/state.go compares two configurations by that JSON, so the keys, the omitted role
// fields and the round trip are the format of persisted state.
func TestConfigJSONIsTheCheckpointFormat(t *testing.T) {
	cfg := &project.Config{
		Setup:      []string{"s"},
		Fix:        []string{"f"},
		Checks:     []project.Check{{Name: "unit", Run: "u"}},
		Acceptance: &project.Acceptance{Run: "r", StepsDir: "features/steps"},
		Protect:    []string{"p"},
		Models: map[string]project.Role{
			"build":  {Provider: "anthropic", Model: "m", Effort: "high"},
			"review": {Model: "r", Effort: "low", Executor: "claude-code", MaxRounds: 3},
		},
	}
	b, err := json.Marshal(cfg)
	check(t, err)
	var doc map[string]any
	check(t, json.Unmarshal(b, &doc))
	for key, want := range map[string]string{
		"Setup":      `["s"]`,
		"Fix":        `["f"]`,
		"Checks":     `[{"Name":"unit","Run":"u"}]`,
		"Acceptance": `{"Run":"r","StepsDir":"features/steps"}`,
		"Protect":    `["p"]`,
		"Models": `{"build":{"Provider":"anthropic","Model":"m","Effort":"high"},` +
			`"review":{"Provider":"","Model":"r","Effort":"low","Executor":"claude-code","MaxRounds":3}}`,
	} {
		var wantValue any
		check(t, json.Unmarshal([]byte(want), &wantValue))
		if !reflect.DeepEqual(doc[key], wantValue) {
			t.Errorf("project.Config JSON key %s = %v, want %s", key, doc[key], want)
		}
	}

	var back project.Config
	check(t, json.Unmarshal(b, &back))
	if !reflect.DeepEqual(&back, cfg) {
		// The unrelated Shape keys decode to their zero values; compare what build reads.
		back.ShapeWorkers, back.ShapeAudit, back.ShapeDecisionCheck, back.ReviewMaterial = cfg.ShapeWorkers, cfg.ShapeAudit, cfg.ShapeDecisionCheck, cfg.ReviewMaterial
		if !reflect.DeepEqual(&back, cfg) {
			t.Errorf("project.Config JSON round trip = %+v, want %+v", back, *cfg)
		}
	}

	none := &project.Config{}
	b, err = json.Marshal(none)
	check(t, err)
	check(t, json.Unmarshal(b, &doc))
	equal(t, "project.Config JSON of no acceptance", doc["Acceptance"], nil)
}
