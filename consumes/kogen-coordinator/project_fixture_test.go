package consumes

import (
	"testing"

	"github.com/KogenAI/kogen-coordinator/project"
)

// The Intent `fixture` of a project, as kogen-build's tests write one: an Intent with a feature
// and a step file, and a project configuration.
const (
	fixtureSlug    = "fixture"
	fixtureFeature = "Feature: Fixture\n  Scenario: First\n    Then the candidate succeeds\n  Scenario: Later\n    Then the candidate succeeds\n"
	fixtureIntent  = "# Sample intent\n\nSlug: fixture\nModel: not applicable — nothing to model\nAcceptance:\n" +
		"- .kogen/intents/fixture/acceptance.feature\n- features/steps/fixture_steps.js\n\nThe fixture.\n\n" +
		"## Decisions\n- Fixture decision. (source: request)\n\n## Tests\n- Fixture rule: a test (unit)\n\n" +
		"## Feasibility\n- none\n\n## Source comparison\n- Addressed: the fixture request.\n"
	fixtureConfig = `version: 1
commands:
  setup:
    - npm ci
    - go mod download
  fix:
    - gofmt -w .
  checks:
    - name: format
      run: test -z "$(gofmt -l .)"
    - name: go-test
      run: go test ./...
  acceptance:
    run: sh accept.sh
    steps_dir: features/steps/
protect:
  - guard.cfg
models:
  build: {provider: anthropic, model: build-model, effort: high}
  review: {executor: claude-code, model: review-model, effort: max}
`
)

// newProject is a repository holding the committed Intent `fixture` and the configuration.
func newProject(t *testing.T) *repoFx {
	t.Helper()
	f := newBareRepo(t)
	f.write(t, ".gitignore", ".kogen/local/\n")
	f.write(t, ".kogen/project.yaml", fixtureConfig)
	f.write(t, ".kogen/models.yaml", project.ModelsTemplate())
	f.write(t, ".kogen/intents/fixture/intent.md", fixtureIntent)
	f.write(t, ".kogen/intents/fixture/acceptance.feature", fixtureFeature)
	f.write(t, "features/steps/fixture_steps.js", "// steps\n")
	f.write(t, "guard.cfg", "guard\n")
	f.git(t, "add", ".")
	f.git(t, "commit", "-qm", "fixture")
	return f
}
