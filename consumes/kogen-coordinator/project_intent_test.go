package consumes

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// Every file of an Intent is found from its slug: build/build.go, build/builder.go,
// build/state.go, build/blocker_grounding.go and build/commit.go.
func TestIntentPaths(t *testing.T) {
	var _ func(string) string = project.IntentDir
	var _ func(string) string = project.IntentMDPath
	var _ func(string) string = project.FeaturePath
	var _ func(string) string = project.ModelPath
	var _ func(string, string) string = project.ApprovalPath

	equal(t, "project.IntentDir", project.IntentDir("s"), ".kogen/intents/s")
	equal(t, "project.IntentMDPath", project.IntentMDPath("s"), ".kogen/intents/s/intent.md")
	equal(t, "project.FeaturePath", project.FeaturePath("s"), ".kogen/intents/s/acceptance.feature")
	equal(t, "project.ModelPath", project.ModelPath("s"), ".kogen/intents/s/model.qnt")
	equal(t, "project.ApprovalPath", project.ApprovalPath("/root", "s"), "/root/.kogen/local/approvals/s.json")
}

// build/build.go requires the Intent's folder before it reads the approval; the error is the
// stdout `error` object (intent/not_found, exit 2).
func TestRequireIntentDir(t *testing.T) {
	var _ func(string, string) error = project.RequireIntentDir

	f := newProject(t)
	if err := project.RequireIntentDir(f.Dir, fixtureSlug); err != nil {
		t.Errorf("project.RequireIntentDir of an existing Intent = %v", err)
	}
	e := wantSysError(t, "project.RequireIntentDir of a missing Intent", project.RequireIntentDir(f.Dir, "other"), "intent", "not_found", 2)
	equal(t, "project.RequireIntentDir detail", e.Detail, "Intent does not exist")
}

// build/build.go loads the approved Intent (its title for the commit message, its Model: line,
// its slug and text, its acceptance list for the dirty-path rule); build/state.go and
// build/resume.go rebuild it from the text the Build started with through ReadIntent's overlay.
func TestLoadIntentAndReadIntent(t *testing.T) {
	var _ func(string, string) (*project.Intent, error) = project.LoadIntent
	var _ func(string, string, map[string][]byte) (*project.Intent, error) = project.ReadIntent

	f := newProject(t)
	overlay := map[string][]byte{project.IntentMDPath(fixtureSlug): []byte(fixtureIntent)}
	for name, load := range map[string]func() (*project.Intent, error){
		"project.LoadIntent": func() (*project.Intent, error) { return project.LoadIntent(f.Dir, fixtureSlug) },
		"project.ReadIntent": func() (*project.Intent, error) { return project.ReadIntent(f.Dir, fixtureSlug, nil) },
		"project.ReadIntent with an overlay": func() (*project.Intent, error) {
			return project.ReadIntent(f.Dir, fixtureSlug, overlay)
		},
	} {
		in, err := load()
		check(t, err)
		var _ string = in.Slug
		var _ string = in.Text
		var _ string = in.Title
		var _ []string = in.Acceptance
		var _ *project.ModelSpec = in.Model
		equal(t, name+": project.Intent.Slug", in.Slug, fixtureSlug)
		equal(t, name+": project.Intent.Text", in.Text, fixtureIntent)
		equal(t, name+": project.Intent.Title", in.Title, "Fixture intent")
		equalList(t, name+": project.Intent.Acceptance", in.Acceptance,
			[]string{".kogen/intents/fixture/acceptance.feature", "features/steps/fixture_steps.js"})
		if in.Model == nil || !in.Model.None {
			t.Errorf("%s: project.Intent.Model = %+v, want {None: true}", name, in.Model)
		}
	}

	// The overlay wins over the disk: the Build's saved text is the contract it started with.
	saved := strings.Replace(fixtureIntent, "# Fixture intent", "# Saved title", 1)
	in, err := project.ReadIntent(f.Dir, fixtureSlug, map[string][]byte{project.IntentMDPath(fixtureSlug): []byte(saved)})
	check(t, err)
	equal(t, "project.ReadIntent overlay Text", in.Text, saved)
	equal(t, "project.ReadIntent overlay Title", in.Title, "Saved title")
	equal(t, "project.ReadIntent overlay leaves the disk", readFile(t, f.Dir+"/"+project.IntentMDPath(fixtureSlug)), fixtureIntent)

	// A missing intent.md is an error, as build/resume.go refuses the resume on it.
	e := wantSysError(t, "project.ReadIntent of a missing Intent", func() error { _, err := project.ReadIntent(f.Dir, "other", nil); return err }(), "intent", "parse", 1)
	equalList(t, "project.ReadIntent lines", e.Lines, []string{"intent.md does not exist"})
	if _, err := project.LoadIntent(f.Dir, "other"); err == nil {
		t.Errorf("project.LoadIntent of a missing Intent succeeded")
	}
}

// build/build.go refuses an Intent whose Model: line cannot be read (Intent.Model nil, intent/parse)
// and skips the model tools for `not applicable`; build/maintained.go promotes a Quint model
// and records its ModelSpec as JSON, which it reads back.
func TestIntentModelSpec(t *testing.T) {
	f := newProject(t)
	modelLine := ".kogen/intents/fixture/model.qnt invariants=Safe,Live seed=1 samples=20 steps=5"
	f.write(t, project.IntentMDPath(fixtureSlug), strings.Replace(fixtureIntent, "Model: not applicable — nothing to model", "Model: "+modelLine, 1))
	in, err := project.LoadIntent(f.Dir, fixtureSlug)
	check(t, err)
	if in.Model == nil {
		t.Fatalf("project.Intent.Model of %q is nil", modelLine)
	}
	var spec project.ModelSpec = *in.Model
	var _ bool = spec.None
	var _ string = spec.Path
	var _ []string = spec.Invariants
	var _ string = spec.Seed
	var _ string = spec.Samples
	var _ string = spec.Steps
	equal(t, "project.ModelSpec.None of a model", spec.None, false)
	equal(t, "project.ModelSpec.Path", spec.Path, project.ModelPath(fixtureSlug))
	equalList(t, "project.ModelSpec.Invariants", spec.Invariants, []string{"Safe", "Live"})
	equal(t, "project.ModelSpec.Seed", spec.Seed, "1")
	equal(t, "project.ModelSpec.Samples", spec.Samples, "20")
	equal(t, "project.ModelSpec.Steps", spec.Steps, "5")

	// build/maintained.go writes the spec as indented JSON with the field names as keys and reads it back.
	b, err := json.MarshalIndent(spec, "", "  ")
	check(t, err)
	var doc map[string]any
	check(t, json.Unmarshal(b, &doc))
	wantDoc := map[string]any{
		"None": false, "Path": project.ModelPath(fixtureSlug), "Invariants": []any{"Safe", "Live"},
		"Seed": "1", "Samples": "20", "Steps": "5",
	}
	if !reflect.DeepEqual(doc, wantDoc) {
		t.Errorf("project.ModelSpec JSON = %v, want %v", doc, wantDoc)
	}
	var back project.ModelSpec
	check(t, json.Unmarshal(b, &back))
	if !reflect.DeepEqual(back, spec) {
		t.Errorf("project.ModelSpec JSON round trip = %+v, want %+v", back, spec)
	}

	f.write(t, project.IntentMDPath(fixtureSlug), strings.Replace(fixtureIntent, "Model: not applicable — nothing to model", "Model: later", 1))
	in, err = project.LoadIntent(f.Dir, fixtureSlug)
	check(t, err)
	if in.Model != nil {
		t.Errorf("project.Intent.Model of an unreadable Model: line = %+v, want nil", in.Model)
	}
}

const stagesPlan = "## Stages\n```json\n" +
	`{"version":1,"stages":[` +
	`{"id":"foundation","title":"Foundation","goal":"First runnable section","checks":["first","second"]},` +
	`{"id":"surface","title":"Surface","goal":"Integrate","checks":["third"]}]}` +
	"\n```\n"

// build/build.go parses the approved Intent's optional `## Stages` plan: no section is the
// single whole-Intent route (nil, no error); a valid plan is the ordered stages; anything else
// is an error whose text starts `stages:` and becomes the intent/lint message.
func TestParseStages(t *testing.T) {
	var _ func(string) ([]project.Stage, error) = project.ParseStages

	plan, err := project.ParseStages("# Intent\n\n" + stagesPlan + "\n## Tests\n")
	check(t, err)
	if len(plan) != 2 {
		t.Fatalf("project.ParseStages = %+v, want two stages", plan)
	}
	var first project.Stage = plan[0]
	var _ string = first.ID
	var _ string = first.Title
	var _ string = first.Goal
	var _ []string = first.Checks
	equal(t, "project.Stage.ID", first.ID, "foundation")
	equal(t, "project.Stage.Title", first.Title, "Foundation")
	equal(t, "project.Stage.Goal", first.Goal, "First runnable section")
	equalList(t, "project.Stage.Checks", first.Checks, []string{"first", "second"})
	equal(t, "project.Stage.ID of the second stage (plan order)", plan[1].ID, "surface")
	equalList(t, "project.Stage.Checks of the second stage", plan[1].Checks, []string{"third"})

	for _, text := range []string{"", "## Foundation\nSome stages in prose.\n"} {
		if plan, err := project.ParseStages(text); plan != nil || err != nil {
			t.Errorf("project.ParseStages(%q) = %v, %v; want nil and no error", text, plan, err)
		}
	}
	const invalid = "stages: expected one JSON fence with version 1 and at least two unique stages (id, title, goal, nonempty checks); no extra fields"
	for name, text := range map[string]string{
		"not json":      "## Stages\nnot json\n",
		"one stage":     "## Stages\n```json\n" + `{"version":1,"stages":[{"id":"a","title":"A","goal":"g","checks":["c"]}]}` + "\n```\n",
		"duplicate ids": strings.Replace(stagesPlan, `"id":"surface"`, `"id":"foundation"`, 1),
		"empty checks":  strings.Replace(stagesPlan, `["third"]`, `[]`, 1),
	} {
		if _, err := project.ParseStages(text); err == nil || err.Error() != invalid {
			t.Errorf("project.ParseStages of %s: error = %v, want %q", name, err, invalid)
		}
	}
}

// build/state.go reads the Intent's feature to know the scenarios a complete green pass covers:
// a scenario per Scenario and per Examples row; an unreadable feature is an error.
func TestParseGherkinCountsTheScenarios(t *testing.T) {
	var _ func(string) ([]project.Scenario, error) = project.ParseGherkin

	var scenarios []project.Scenario
	scenarios, err := project.ParseGherkin(fixtureFeature)
	check(t, err)
	equal(t, "project.ParseGherkin scenarios of the fixture feature", len(scenarios), 2)
	equal(t, "project.Scenario.Name", scenarios[0].Name, "First")
	equal(t, "project.Scenario.Name of the second", scenarios[1].Name, "Later")

	outline := "Feature: F\n  Background:\n    Given the project is ready\n\n  Scenario: ordinary\n    Then it passes\n\n" +
		"  Scenario Outline: value <v>\n    Then <v> passes\n    Examples:\n      | v |\n      | a |\n      | b |\n      | c |\n"
	scenarios, err = project.ParseGherkin(outline)
	check(t, err)
	var names []string
	for _, s := range scenarios {
		names = append(names, s.Name)
	}
	equalList(t, "project.ParseGherkin names (a scenario per Examples row)", names, []string{"ordinary", "value a", "value b", "value c"})

	if scenarios, err := project.ParseGherkin("not a feature\n"); err == nil {
		t.Errorf("project.ParseGherkin of text that is no feature = %+v, want an error", scenarios)
	}
	if scenarios, err := project.ParseGherkin("Feature: A\n  Scenario: ordinary\n  Examples:\n"); err == nil {
		t.Errorf("project.ParseGherkin of Examples outside an outline = %+v, want an error", scenarios)
	}
}
